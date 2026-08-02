import { useCallback, useEffect, useRef, useState } from 'react'

type Options = { enabled?: boolean }

type Result<T> = {
  data: T | null
  error: Error | null
  loading: boolean
  refetch: () => void
}

/** Signals that a request was abandoned rather than having failed on its own. */
function abortError(message: string): Error {
  const err = new Error(message)
  err.name = 'AbortError'
  return err
}

/**
 * Poll an endpoint on an interval, with three guarantees the old Alpine UI
 * lacked:
 *
 *  - Visibility-aware: nothing is fetched while the tab is hidden, and a
 *    catch-up fetch runs the moment it becomes visible again.
 *  - Non-overlapping: a new request never starts while one is in flight,
 *    so a slow node cannot accumulate a backlog of stacked requests.
 *  - Always recovering: every request is bounded by a timeout and is aborted
 *    on unmount, so no single hung connection can wedge the loop shut.
 *
 * Only the mounted route polls, since React Router unmounts the others.
 */
export function usePolling<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  intervalMs: number,
  opts: Options = {},
): Result<T> {
  const { enabled = true } = opts
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<Error | null>(null)
  const [loading, setLoading] = useState(false)

  const inFlight = useRef(false)
  const mounted = useRef(true)
  // The controller for the request currently in flight, so the effect cleanup
  // can cancel it on unmount or route change.
  const controllerRef = useRef<AbortController | null>(null)
  // Keep the latest fetcher in a ref so callers can pass an inline arrow
  // function without resetting the interval on every render.
  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const run = useCallback(async () => {
    if (inFlight.current || document.hidden) return
    inFlight.current = true
    setLoading(true)

    const controller = new AbortController()
    controllerRef.current = controller

    // Bound every request, otherwise one connection that never answers latches
    // inFlight forever and the loop is dead — a frozen spinner over stale data,
    // worse than the old UI, which at least kept retrying. The budget scales
    // with the caller's cadence because pages poll anywhere from 1500ms to
    // 5500ms: a fixed constant would either cut off the slow endpoints or leave
    // the fast ones wedged for far longer than they should be. Three intervals
    // tolerates a couple of skipped ticks, and the 10s floor keeps the fastest
    // pages from timing out a merely sluggish node.
    const timeoutMs = Math.max(intervalMs * 3, 10_000)
    let timedOut = false
    const timer = setTimeout(() => {
      timedOut = true
      controller.abort()
    }, timeoutMs)

    // Race the abort rather than trusting the fetcher to observe the signal:
    // a fetcher that ignores it would otherwise never settle and the recovery
    // above would never happen.
    const abandoned = new Promise<never>((_, reject) => {
      controller.signal.addEventListener('abort', () => {
        reject(abortError(timedOut ? `request timed out after ${timeoutMs}ms` : 'aborted'))
      })
    })

    try {
      const result = await Promise.race([fetcherRef.current(controller.signal), abandoned])
      if (!mounted.current) return
      setData(result)
      setError(null)
    } catch (e) {
      // Aborted because the component went away: nobody is left to tell.
      if (!mounted.current) return
      // A timeout is a real failure the user needs to see, so it must be
      // distinguished from the abort above rather than swallowed with it.
      if (timedOut) {
        setError(new Error(`Request timed out after ${timeoutMs}ms`))
        return
      }
      if ((e as Error).name === 'AbortError') return
      setError(e as Error)
    } finally {
      clearTimeout(timer)
      if (controllerRef.current === controller) controllerRef.current = null
      inFlight.current = false
      if (mounted.current) setLoading(false)
    }
  }, [intervalMs])

  useEffect(() => {
    mounted.current = true

    let id: ReturnType<typeof setInterval> | undefined
    let onVisibility: (() => void) | undefined

    if (enabled) {
      void run()
      id = setInterval(() => { void run() }, intervalMs)

      // Catch up as soon as the tab is visible again, rather than waiting
      // out the remainder of the interval.
      onVisibility = () => { if (!document.hidden) void run() }
      document.addEventListener('visibilitychange', onVisibility)
    }

    // Registered unconditionally, so the mounted invariant holds even for a
    // hook that was disabled for its whole life.
    return () => {
      mounted.current = false
      controllerRef.current?.abort()
      if (id !== undefined) clearInterval(id)
      if (onVisibility) document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs, run])

  return { data, error, loading, refetch: run }
}
