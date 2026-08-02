import { useCallback, useEffect, useRef, useState } from 'react'

type Options = { enabled?: boolean }

type Result<T> = {
  data: T | null
  error: Error | null
  loading: boolean
  refetch: () => void
}

/**
 * Poll an endpoint on an interval, with two guarantees the old Alpine UI
 * lacked:
 *
 *  - Visibility-aware: nothing is fetched while the tab is hidden, and a
 *    catch-up fetch runs the moment it becomes visible again.
 *  - Non-overlapping: a new request never starts while one is in flight,
 *    so a slow node cannot accumulate a backlog of stacked requests.
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
  // Keep the latest fetcher in a ref so callers can pass an inline arrow
  // function without resetting the interval on every render.
  const fetcherRef = useRef(fetcher)
  fetcherRef.current = fetcher

  const run = useCallback(async () => {
    if (inFlight.current || document.hidden) return
    inFlight.current = true
    setLoading(true)
    const controller = new AbortController()
    try {
      const result = await fetcherRef.current(controller.signal)
      if (!mounted.current) return
      setData(result)
      setError(null)
    } catch (e) {
      if (!mounted.current) return
      if ((e as Error).name === 'AbortError') return
      setError(e as Error)
    } finally {
      inFlight.current = false
      if (mounted.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    if (!enabled) return

    void run()
    const id = setInterval(() => { void run() }, intervalMs)

    // Catch up as soon as the tab is visible again, rather than waiting
    // out the remainder of the interval.
    const onVisibility = () => { if (!document.hidden) void run() }
    document.addEventListener('visibilitychange', onVisibility)

    return () => {
      mounted.current = false
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs, run])

  return { data, error, loading, refetch: run }
}
