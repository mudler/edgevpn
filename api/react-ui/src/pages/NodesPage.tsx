import { useState } from 'react'
import { deleteLedgerKey, getMachines } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import type { Machine } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'
import Pill from '../components/Pill'

export default function NodesPage() {
  const machines = usePolling((s) => getMachines(s), 1500)
  const [busy, setBusy] = useState<string | null>(null)
  const [err, setErr] = useState<string | null>(null)

  async function forget(m: Machine) {
    setBusy(m.Address)
    setErr(null)
    try {
      await deleteLedgerKey('machines', m.Address)
      machines.refetch()
    } catch (e) {
      setErr(`Could not remove ${m.Address}: ${(e as Error).message}`)
    } finally {
      setBusy(null)
    }
  }

  const columns: Column<Machine>[] = [
    { key: 'addr', header: 'Address', render: (m) => m.Address, sortValue: (m) => m.Address },
    { key: 'host', header: 'Hostname', render: (m) => m.Hostname, sortValue: (m) => m.Hostname },
    { key: 'peer', header: 'Peer ID',
      render: (m) => <span title={m.PeerID}>{truncateID(m.PeerID, 6)}</span>,
      sortValue: (m) => m.PeerID },
    { key: 'os', header: 'Platform', render: (m) => `${m.OS}/${m.Arch}`, sortValue: (m) => m.OS },
    { key: 'ver', header: 'Version', render: (m) => m.Version, sortValue: (m) => m.Version },
    { key: 'state', header: 'State',
      render: (m) => m.Online
        ? <Pill tone="ok">{m.Connected ? 'direct' : 'online'}</Pill>
        : <Pill tone="warn">stale</Pill>,
      sortValue: (m) => (m.Online ? 1 : 0) },
    { key: 'act', header: '',
      render: (m) => (
        <button type="button" className="ev-sort" disabled={busy === m.Address}
                onClick={() => void forget(m)}
                aria-label={`Remove ${m.Address} from the ledger`}>
          {busy === m.Address ? '…' : 'remove'}
        </button>
      ) },
  ]

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">Nodes</h2>
      {machines.error && (
        <p className="ev-error">Cannot reach the node: {machines.error.message}</p>
      )}
      {err && <p className="ev-error">{err}</p>}
      <DataTable columns={columns} rows={machines.data ?? []}
                 rowKey={(m) => m.Address} emptyText="No machines on the ledger yet" />
    </section>
  )
}
