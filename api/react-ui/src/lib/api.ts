import type {
  Block, DNSEntry, FileEntry, Machine, Peer, PeerStats,
  Service, Stats, Summary, User,
} from '../types/api'

/** An HTTP-level failure. Carries the status so callers can branch on 404. */
export class ApiError extends Error {
  status: number
  body: string
  constructor(status: number, body: string) {
    super(`HTTP ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal })
  if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => ''))
  return (await res.json()) as T
}

export const getSummary    = (s?: AbortSignal) => get<Summary>('/api/summary', s)
export const getMachines   = (s?: AbortSignal) => get<Machine[]>('/api/machines', s)
export const getNodes      = (s?: AbortSignal) => get<Peer[]>('/api/nodes', s)
export const getPeerstore  = (s?: AbortSignal) => get<Peer[]>('/api/peerstore', s)
export const getUsers      = (s?: AbortSignal) => get<User[]>('/api/users', s)
export const getServices   = (s?: AbortSignal) => get<Service[]>('/api/services', s)
export const getFiles      = (s?: AbortSignal) => get<FileEntry[]>('/api/files', s)
export const getDNS        = (s?: AbortSignal) => get<DNSEntry[]>('/api/dns', s)
export const getBlockchain = (s?: AbortSignal) => get<Block>('/api/blockchain', s)

/**
 * Bandwidth metrics. These routes are registered only when the node has a
 * bandwidth counter, so a 404 here is expected, not exceptional — callers
 * should treat it as "metrics unavailable".
 */
export const getMetrics     = (s?: AbortSignal) => get<Stats>('/api/metrics', s)
export const getPeerMetrics = (s?: AbortSignal) => get<PeerStats>('/api/metrics/peer', s)

/**
 * Delete a ledger entry. The UI must know each bucket's key semantics:
 * `machines` is keyed by IP address, `dns` by regex. Typed delete
 * endpoints are out of scope, so this coupling is preserved.
 */
export async function deleteLedgerKey(bucket: string, key: string): Promise<void> {
  const url = `/api/ledger/${encodeURIComponent(bucket)}/${encodeURIComponent(key)}`
  const res = await fetch(url, { method: 'DELETE' })
  if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => ''))
}
