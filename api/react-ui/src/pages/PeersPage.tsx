import { useMemo } from 'react'
import { getMachines, getNodes, getPeerMetrics, getPeerstore } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { formatRate, truncateID } from '../lib/format'
import DataTable, { type Column } from '../components/DataTable'
import Pill from '../components/Pill'

export type PeerRow = {
  id: string
  /** True only when a source that actually reports liveness says so. */
  online: boolean
  /** True when the peer is on the ledger, i.e. more than a peerstore entry. */
  known: boolean
  rateIn: number
  rateOut: number
}

/**
 * Merge the three peer sources into one view.
 *
 * /api/nodes reports real liveness. /api/peerstore is a bare address book
 * and always reports Online:false, so its entries contribute identity only.
 * /api/machines tells us which peers are on the ledger.
 */
export function usePeerRows(): { rows: PeerRow[]; error: Error | null } {
  const nodes = usePolling((s) => getNodes(s), 1500)
  const store = usePolling((s) => getPeerstore(s), 1500)
  const machines = usePolling((s) => getMachines(s), 1500)
  const metrics = usePolling((s) => getPeerMetrics(s), 1500)

  const rows = useMemo(() => {
    const byId = new Map<string, PeerRow>()
    const onLedger = new Set((machines.data ?? []).map((m) => m.PeerID))

    for (const p of store.data ?? []) {
      byId.set(p.ID, { id: p.ID, online: false, known: onLedger.has(p.ID), rateIn: 0, rateOut: 0 })
    }
    for (const p of nodes.data ?? []) {
      const existing = byId.get(p.ID)
      byId.set(p.ID, {
        id: p.ID,
        online: p.Online,
        known: onLedger.has(p.ID) || (existing?.known ?? false),
        rateIn: 0, rateOut: 0,
      })
    }
    for (const [id, stats] of Object.entries(metrics.data ?? {})) {
      const row = byId.get(id)
      if (row) { row.rateIn = stats.RateIn; row.rateOut = stats.RateOut }
    }
    return [...byId.values()]
  }, [nodes.data, store.data, machines.data, metrics.data])

  return { rows, error: nodes.error ?? store.error }
}

const COLUMNS: Column<PeerRow>[] = [
  { key: 'id', header: 'Peer ID',
    render: (p) => <span title={p.id}>{truncateID(p.id, 8)}</span>,
    sortValue: (p) => p.id },
  { key: 'state', header: 'State',
    render: (p) => p.online
      ? <Pill tone="ok">connected</Pill>
      : <Pill tone="warn">known</Pill>,
    sortValue: (p) => (p.online ? 1 : 0) },
  { key: 'ledger', header: 'On ledger',
    render: (p) => (p.known ? 'yes' : '—'), sortValue: (p) => (p.known ? 1 : 0) },
  { key: 'in', header: 'Rate in',
    render: (p) => (p.rateIn ? formatRate(p.rateIn) : '—'), sortValue: (p) => p.rateIn },
  { key: 'out', header: 'Rate out',
    render: (p) => (p.rateOut ? formatRate(p.rateOut) : '—'), sortValue: (p) => p.rateOut },
]

export default function PeersPage() {
  const { rows, error } = usePeerRows()

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">Peers</h2>
      {error && <p className="ev-error">{error.message}</p>}
      <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
        <b style={{ color: 'var(--ev-muted)' }}>connected</b> peers have a live session.
        {' '}<b style={{ color: 'var(--ev-muted)' }}>known</b> peers are address-book entries
        {' '}whose liveness this node does not track.
      </p>
      <DataTable columns={COLUMNS} rows={rows} rowKey={(p) => p.id}
                 emptyText="No peers discovered yet" />
    </section>
  )
}
