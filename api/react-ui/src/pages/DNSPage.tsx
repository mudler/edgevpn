import { useState } from 'react'
import { deleteLedgerKey, getDNS } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import type { DNSEntry } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'

export default function DNSPage() {
  const dns = usePolling((s) => getDNS(s), 1500)
  const [busy, setBusy] = useState<string | null>(null)
  const [err, setErr] = useState<string | null>(null)

  async function forget(e: DNSEntry) {
    setBusy(e.Regex)
    setErr(null)
    try {
      await deleteLedgerKey('dns', e.Regex)
      dns.refetch()
    } catch (ex) {
      setErr(`Could not remove ${e.Regex}: ${(ex as Error).message}`)
    } finally {
      setBusy(null)
    }
  }

  const columns: Column<DNSEntry>[] = [
    { key: 'regex', header: 'Match', render: (e) => e.Regex, sortValue: (e) => e.Regex },
    { key: 'records', header: 'Records',
      render: (e) => (
        <span>
          {Object.entries(e.Records ?? {}).map(([type, value], i) => (
            <span key={type}>
              {i > 0 && <span className="slash">/</span>}
              <span style={{ color: 'var(--ev-faint)' }}>{type}</span> {value}
            </span>
          ))}
        </span>
      ) },
    { key: 'act', header: '',
      render: (e) => (
        <button type="button" className="ev-sort" disabled={busy === e.Regex}
                onClick={() => void forget(e)}
                aria-label={`Remove DNS entry ${e.Regex}`}>
          {busy === e.Regex ? '…' : 'remove'}
        </button>
      ) },
  ]

  return (
    <section className="ev-panel">
      <h2 className="ev-panel-title">DNS</h2>
      {dns.error && <p className="ev-error">Cannot reach the node: {dns.error.message}</p>}
      {err && <p className="ev-error">{err}</p>}
      <DataTable columns={columns} rows={dns.data ?? []}
                 rowKey={(e) => e.Regex} emptyText="No DNS entries announced" />
    </section>
  )
}
