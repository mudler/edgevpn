import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { StrictMode } from 'react'
import { renderHook, waitFor, act } from '@testing-library/react'
import { usePolling } from './usePolling'

// jsdom's document.hidden is a prototype getter driven by visibilityState, so
// shadow it with an own property we control from the tests.
let hidden = false

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  hidden = false
  Object.defineProperty(document, 'hidden', {
    configurable: true,
    get: () => hidden,
  })
})

afterEach(() => {
  vi.useRealTimers()
  Reflect.deleteProperty(document, 'hidden')
})

describe('usePolling', () => {
  it('fetches immediately on mount', async () => {
    const fetcher = vi.fn(async () => 'one')
    const { result } = renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(result.current.data).toBe('one'))
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('polls again after the interval elapses', async () => {
    const fetcher = vi.fn(async () => 'x')
    renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
  })

  it('does not stack requests when the fetcher is slower than the interval', async () => {
    let release: (v: string) => void = () => {}
    const fetcher = vi.fn(() => new Promise<string>((r) => { release = r }))
    renderHook(() => usePolling(fetcher, 100))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    // Let several intervals elapse while the first request is still open.
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(fetcher).toHaveBeenCalledTimes(1)
    await act(async () => { release('done') })
  })

  it('does not fetch when disabled', async () => {
    const fetcher = vi.fn(async () => 'x')
    renderHook(() => usePolling(fetcher, 1000, { enabled: false }))
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(fetcher).not.toHaveBeenCalled()
  })

  it('surfaces fetcher errors without stopping the loop', async () => {
    const fetcher = vi.fn()
      .mockRejectedValueOnce(new Error('nope'))
      .mockResolvedValue('recovered')
    const { result } = renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(result.current.error?.message).toBe('nope'))
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    await waitFor(() => expect(result.current.data).toBe('recovered'))
    expect(result.current.error).toBeNull()
  })

  it('does not fetch while the tab is hidden, and catches up the moment it is visible', async () => {
    hidden = true
    const fetcher = vi.fn(async () => 'x')
    const { result } = renderHook(() => usePolling(fetcher, 1000))

    // Several intervals go by with the tab hidden: not a single request.
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(fetcher).not.toHaveBeenCalled()

    hidden = false
    await act(async () => { document.dispatchEvent(new Event('visibilitychange')) })
    // Immediate: asserted without advancing the clock at all, so this cannot
    // be the next interval tick doing the work.
    expect(fetcher).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(result.current.data).toBe('x'))
  })

  it('resumes polling once a slow request finally settles', async () => {
    let release: (v: string) => void = () => {}
    const fetcher = vi.fn(() => new Promise<string>((r) => { release = r }))
    const { result } = renderHook(() => usePolling(fetcher, 100))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(fetcher).toHaveBeenCalledTimes(1)

    await act(async () => { release('done') })
    await waitFor(() => expect(result.current.data).toBe('done'))

    // The loop must pick straight back up rather than staying latched.
    await act(async () => { await vi.advanceTimersByTimeAsync(100) })
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
  })

  it('times out a hung request, surfaces it as an error, and keeps polling', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => new Promise(() => {})) // never settles, ignores the signal
      .mockResolvedValue('recovered')
    // 1500ms interval => timeout is the 10s floor, which is not a multiple of
    // the interval, so the timeout and an interval tick cannot collide.
    const { result } = renderHook(() => usePolling(fetcher, 1500))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    // Intervals keep elapsing while the request hangs; none of them stack.
    await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(result.current.error).toBeNull()

    // At the timeout the request is abandoned and the failure becomes visible.
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    await waitFor(() => expect(result.current.error).not.toBeNull())
    expect(result.current.error?.message).toMatch(/timed out after 10000ms/)
    // The spinner must not be left stuck on.
    expect(result.current.loading).toBe(false)

    // And the loop is still alive.
    await act(async () => { await vi.advanceTimersByTimeAsync(1500) })
    await waitFor(() => expect(result.current.data).toBe('recovered'))
    expect(result.current.error).toBeNull()
  })

  it('aborts the in-flight request on unmount and stops the interval', async () => {
    let signal: AbortSignal | undefined
    const fetcher = vi.fn((s: AbortSignal) => {
      signal = s
      return new Promise<string>(() => {})
    })
    const { result, unmount } = renderHook(() => usePolling(fetcher, 1000))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    expect(signal?.aborted).toBe(false)

    unmount()
    expect(signal?.aborted).toBe(true)

    // An unmount abort is not a user-visible failure, and nothing polls on.
    await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(result.current.error).toBeNull()
  })

  it('keeps the mount fetch when StrictMode double-invokes the effect', async () => {
    // Any real request takes at least a tick to come back, which is what makes
    // the double-invoke lossy: the abort beats the response.
    const fetcher = vi.fn(() => new Promise<string>((resolve) => {
      setTimeout(() => resolve('fresh'), 10)
    }))
    const { result } = renderHook(() => usePolling(fetcher, 1500), { wrapper: StrictMode })

    // Only 50ms of clock, far short of the 1500ms interval, so data arriving
    // here can only have come from the remounted effect's own fetch and not
    // from a later tick bailing the hook out.
    await act(async () => { await vi.advanceTimersByTimeAsync(50) })
    expect(result.current.data).toBe('fresh')
  })

  it('does not reopen request stacking when an effect re-run abandons a request', async () => {
    const fetcher = vi.fn(() => new Promise<string>(() => {})) // never settles
    const { rerender } = renderHook(
      ({ ms }) => usePolling(fetcher, ms),
      { initialProps: { ms: 100 } },
    )
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    // Changing the cadence re-runs the effect, abandoning the open request.
    // The replacement must go out immediately rather than being blocked by the
    // abandoned request's guard.
    await act(async () => { rerender({ ms: 200 }) })
    expect(fetcher).toHaveBeenCalledTimes(2)

    // ...and the abandoned request settling must not release the *replacement's*
    // guard, which would let the next ticks stack requests all over again.
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('clears loading when disabled mid-request, with nothing left to replace it', async () => {
    const fetcher = vi.fn(() => new Promise<string>(() => {})) // never settles
    const { result, rerender } = renderHook(
      ({ on }) => usePolling(fetcher, 1000, { enabled: on }),
      { initialProps: { on: true } },
    )
    await waitFor(() => expect(result.current.loading).toBe(true))

    await act(async () => { rerender({ on: false }) })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBeNull()
  })

  it('scales the request timeout with the interval once it clears the floor', async () => {
    const fetcher = vi.fn(() => new Promise<string>(() => {})) // always hangs
    const { result } = renderHook(() => usePolling(fetcher, 5000))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    // 3 x 5000ms clears the 10s floor, so nothing has given up yet here.
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000) })
    expect(result.current.error).toBeNull()

    await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
    await waitFor(() => expect(result.current.error).not.toBeNull())
    expect(result.current.error?.message).toMatch(/timed out after 15000ms/)
  })
})
