import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { usePolling } from './usePolling'

beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
afterEach(() => { vi.useRealTimers() })

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
})
