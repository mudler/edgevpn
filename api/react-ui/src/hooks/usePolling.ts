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

  const mounted = useRef(true)
  // The controller of the request currently in flight, doubling as the
  // non-overlap guard: non-null means busy. One ref rather than a separate
  // boolean, so the guard is *owned* by a single invocation and can only ever
  // be released by whoever still holds it. A shared boolean cleared
  // unconditionally would let an abandoned request release its replacement's
  // guard and reopen the stacking this hook exists to prevent.
  const activeRun = useRef<AbortController | null>(null)
  // Keep the latest fetcher in a ref so callers can pass an inline arrow
  // function without resetting the interval on every render.
  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const run = useCallback(async () => {
    if (activeRun.current || document.hidden) return
    setLoading(true)

    const controller = new AbortController()
    activeRun.current = controller

    // Bound every request, otherwise one connection that never answers holds
    // the guard forever and the loop is dead — a frozen spinner over stale
    // data, worse than the old UI, which at least kept retrying. The budget scales
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

    // True once this invocation has been abandoned — unmounted, or superseded
    // by an effect re-run. Its result is stale by definition, so it must not
    // touch state and must not release whatever guard is current now.
    const superseded = () => activeRun.current !== controller

    try {
      const result = await Promise.race([fetcherRef.current(controller.signal), abandoned])
      if (!mounted.current || superseded()) return
      setData(result)
      setError(null)
    } catch (e) {
      // Aborted because the component went away: nobody is left to tell.
      if (!mounted.current || superseded()) return
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
      // Release the guard only while still holding it. An abandoned invocation
      // reaches here *after* its replacement has already taken the guard, and
      // clearing unconditionally would leave the replacement unguarded.
      if (!superseded()) activeRun.current = null
      // Whoever ends up leaving nothing in flight owns turning the spinner off,
      // including an invocation abandoned with no replacement behind it — a
      // hook disabled mid-request would otherwise sit at loading forever.
      if (mounted.current && activeRun.current === null) setLoading(false)
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

      const abandoned = activeRun.current
      if (abandoned) {
        // Release the guard *synchronously*, before aborting. This invocation
        // is dead the moment cleanup runs, and nothing downstream depends on
        // its `finally` — which is a microtask away, whereas a re-run of this
        // effect is synchronous. StrictMode's development double-invoke is the
        // common case (cleanup and re-mount happen back to back, so the remount
        // would find the guard still held and skip its fetch, leaving the page
        // blank until the first tick), but the same shape applies to any
        // `enabled` or `intervalMs` change mid-flight.
        activeRun.current = null
        abandoned.abort()
      }

      if (id !== undefined) clearInterval(id)
      if (onVisibility) document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs, run])

  return { data, error, loading, refetch: run }
}
