import { useState } from 'react'
import { getBlockchain } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import DataTable, { type Column } from '../components/DataTable'
import Tile from '../components/Tile'

type BucketRow = { bucket: string; keys: number }

export default function BlockchainPage() {
  const block = usePolling((s) => getBlockchain(s), 5500)
  const [open, setOpen] = useState<string | null>(null)

  const storage = block.data?.Storage ?? {}
  const rows: BucketRow[] = Object.entries(storage)
    .map(([bucket, entries]) => ({ bucket, keys: Object.keys(entries ?? {}).length }))

  const columns: Column<BucketRow>[] = [
    { key: 'bucket', header: 'Bucket', render: (r) => r.bucket, sortValue: (r) => r.bucket },
    { key: 'keys', header: 'Keys', render: (r) => r.keys, sortValue: (r) => r.keys },
    { key: 'act', header: '',
      render: (r) => (
        <button type="button" className="ev-sort"
                onClick={() => setOpen(open === r.bucket ? null : r.bucket)}>
          {open === r.bucket ? 'hide' : 'inspect'}
        </button>
      ) },
  ]

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Last block</h2>
        {block.error && <p className="ev-error">Cannot reach the node: {block.error.message}</p>}
        <div className="ev-tiles">
          <Tile label="Index" value={block.data?.Index ?? '—'} />
          <Tile label="Buckets" value={rows.length} />
        </div>
        {block.data && (
          <p style={{ margin: 0, color: 'var(--ev-faint)', fontSize: 'var(--ev-step--1)' }}>
            {block.data.Timestamp}
            <span className="slash">/</span>
            <span title={block.data.Hash}>{truncateID(block.data.Hash, 10)}</span>
            <span className="slash">/</span>
            <span title={block.data.PrevHash}>prev {truncateID(block.data.PrevHash, 6)}</span>
          </p>
        )}
      </section>

      <section className="ev-panel">
        <h2 className="ev-panel-title">Storage</h2>
        <DataTable columns={columns} rows={rows} rowKey={(r) => r.bucket}
                   emptyText="Block carries no storage" />
        {open && (
          <pre className="ev-scroller" style={{
            margin: 0, background: 'var(--ev-code)', border: '1px solid var(--ev-rule)',
            borderRadius: 'var(--ev-radius)', padding: 'var(--ev-3)',
            fontSize: 'var(--ev-step--1)', maxHeight: '40vh', overflow: 'auto',
          }}>
            {JSON.stringify(storage[open], null, 2)}
          </pre>
        )}
      </section>
    </>
  )
}
