import { useMemo } from 'react'
import { getMachines, getNodes, getPeerMetrics, getPeerstore, getSummary } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { formatRate, truncateID } from '../lib/format'
import DataTable, { type Column } from '../components/DataTable'
import PeerGraph from '../components/PeerGraph'
import Pill from '../components/Pill'

export type PeerRow = {
  id: string
  /** True only when a source that actually reports liveness says so. */
  online: boolean
  /**
   * True when the peer announced a VPN machine entry, i.e. holds an address on
   * this network. Narrower than "on the ledger": peers reported by /api/nodes
   * are on the ledger too, in the healthcheck bucket.
   */
  known: boolean
  rateIn: number
  rateOut: number
}

/**
 * Merge the three peer sources into one view.
 *
 * /api/nodes reports real liveness — a current session or a healthcheck
 * announced on the ledger in the last 10 minutes. /api/peerstore is a bare
 * address book and always reports Online:false, so its entries contribute
 * identity only. /api/machines tells us which peers hold a VPN address.
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
      : <Pill tone="warn">address book</Pill>,
    sortValue: (p) => (p.online ? 1 : 0) },
  { key: 'ledger', header: 'VPN machine',
    render: (p) => (p.known ? 'yes' : '—'), sortValue: (p) => (p.known ? 1 : 0) },
  { key: 'in', header: 'Rate in',
    render: (p) => (p.rateIn ? formatRate(p.rateIn) : '—'), sortValue: (p) => p.rateIn },
  { key: 'out', header: 'Rate out',
    render: (p) => (p.rateOut ? formatRate(p.rateOut) : '—'), sortValue: (p) => p.rateOut },
]

export default function PeersPage() {
  const { rows, error } = usePeerRows()
  const summary = usePolling((s) => getSummary(s), 5500)

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Topology</h2>
        <PeerGraph peers={rows} selfId={summary.data?.NodeID ?? ''} />
        <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
          This node and its direct peers. Edge width is live per-peer bandwidth.
          Links between other peers are not shown — no endpoint reports them.
        </p>
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Peers</h2>
        {error && <p className="ev-error">{error.message}</p>}
        <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
          <b style={{ color: 'var(--ev-muted)' }}>connected</b> peers are ones this node
          {' '}reports as live: a current session, or a healthcheck announced on the ledger
          {' '}in the last 10 minutes.
          {' '}<b style={{ color: 'var(--ev-muted)' }}>address book</b> peers come from this
          {' '}node&apos;s peerstore, which carries no liveness at all.
          {' '}<b style={{ color: 'var(--ev-muted)' }}>VPN machine</b> marks the peers that
          {' '}also announced a machine entry, i.e. hold an address on this network.
        </p>
        <DataTable columns={COLUMNS} rows={rows} rowKey={(p) => p.id}
                   emptyText="No peers discovered yet" />
      </section>
    </>
  )
}
