import { useMemo } from 'react'
import { getMachines, getNodes, getPeerMetrics, getPeerstore, getSummary } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { formatRate, truncateID } from '../lib/format'
import type { PeerRow } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'
import PeerGraph, { plottedPeers } from '../components/PeerGraph'
import Pill from '../components/Pill'

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
  const drawn = plottedPeers(rows).length
  const hidden = rows.length - drawn

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Topology</h2>
        <PeerGraph peers={rows} selfId={summary.data?.NodeID ?? ''} />
        <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
          This node and the {drawn} {drawn === 1 ? 'peer' : 'peers'} it is connected to
          {' '}or shares a VPN address with. Edge width is live per-peer bandwidth.
          {hidden > 0 && (hidden === 1
            ? ' 1 address-book entry from the DHT is not drawn — it is listed in the table below.'
            : ` ${hidden} address-book entries from the DHT are not drawn — they are listed in the table below.`)}
          {' '}Links between other peers are not shown either — no endpoint reports them.
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
