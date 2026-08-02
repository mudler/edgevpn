const UNITS = ['B', 'kB', 'MB', 'GB', 'TB', 'PB'] as const

/** Human-readable byte size. Ported from index.tmpl's bytesToSize. */
export function bytesToSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  // Clamp low as well as high: a fractional input (bandwidth rates are often
  // well under 1 B/s) gives a negative exponent, and UNITS[-1] is undefined —
  // which rendered literally as "424.4 undefined/s" in the peers table.
  const i = Math.min(
    Math.max(Math.floor(Math.log(bytes) / Math.log(1024)), 0),
    UNITS.length - 1,
  )
  const value = bytes / Math.pow(1024, i)
  // Bytes are whole; everything larger reads better with one decimal.
  return i === 0 ? `${Math.round(value)} B` : `${value.toFixed(1)} ${UNITS[i]}`
}

/** Same, with a per-second suffix. */
export function formatRate(bytesPerSec: number): string {
  return `${bytesToSize(bytesPerSec)}/s`
}

/**
 * Shorten a peer ID for display, keeping both ends so IDs stay
 * distinguishable — libp2p peer IDs share long common prefixes.
 */
export function truncateID(id: string, keep = 6): string {
  if (!id || id.length <= keep * 2 + 1) return id
  return `${id.slice(0, keep)}…${id.slice(-keep)}`
}
