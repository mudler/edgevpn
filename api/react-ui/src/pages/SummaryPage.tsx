import { getMetrics, getSummary, getUsers } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { bytesToSize, formatRate, truncateID } from '../lib/format'
import type { User } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'
import Tile from '../components/Tile'

const COLUMNS: Column<User>[] = [
  { key: 'peer', header: 'Peer ID',
    render: (u) => <span title={u.PeerID}>{truncateID(u.PeerID, 8)}</span>,
    sortValue: (u) => u.PeerID },
  { key: 'ts', header: 'Last seen', render: (u) => u.Timestamp, sortValue: (u) => u.Timestamp },
]

export default function SummaryPage() {
  const summary = usePolling((s) => getSummary(s), 1500)
  const users = usePolling((s) => getUsers(s), 1500)
  // Metrics routes are registered only when the node has a bandwidth
  // counter, so absence is normal rather than an error.
  const metrics = usePolling((s) => getMetrics(s), 1500)

  const s = summary.data

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Network</h2>
        {summary.error && <p className="ev-error">Cannot reach the node: {summary.error.message}</p>}
        <div className="ev-tiles">
          <Tile label="Machines" value={s?.Machines ?? '—'} />
          <Tile label="On chain" value={s?.OnChainNodes ?? '—'} />
          <Tile label="Peers" value={s?.Peers ?? '—'} />
          <Tile label="Users" value={s?.Users ?? '—'} />
          <Tile label="Services" value={s?.Services ?? '—'} />
          <Tile label="Files" value={s?.Files ?? '—'} />
          <Tile label="Block" value={s?.BlockChain ?? '—'} />
        </div>
        {s?.NodeID && (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            node<span className="slash">/</span>
            <span title={s.NodeID}>{truncateID(s.NodeID, 10)}</span>
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Bandwidth</h2>
        {metrics.data ? (
          <div className="ev-tiles">
            <Tile label="Rate in" value={formatRate(metrics.data.RateIn)} />
            <Tile label="Rate out" value={formatRate(metrics.data.RateOut)} />
            <Tile label="Total in" value={bytesToSize(metrics.data.TotalIn)} />
            <Tile label="Total out" value={bytesToSize(metrics.data.TotalOut)} />
          </div>
        ) : (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            Bandwidth metrics are not enabled on this node.
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Connected users</h2>
        <DataTable
          columns={COLUMNS}
          rows={users.data ?? []}
          rowKey={(u) => u.PeerID}
          emptyText="No users announced yet"
        />
      </section>
    </>
  )
}
