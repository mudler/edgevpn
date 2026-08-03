import { getFiles, getServices } from '../lib/api'
import { usePolling } from '../hooks/usePolling'
import { truncateID } from '../lib/format'
import type { FileEntry, Service } from '../types/api'
import DataTable, { type Column } from '../components/DataTable'

const SERVICE_COLUMNS: Column<Service>[] = [
  { key: 'name', header: 'Name', render: (s) => s.Name, sortValue: (s) => s.Name },
  { key: 'peer', header: 'Served by',
    render: (s) => <span title={s.PeerID}>{truncateID(s.PeerID, 8)}</span>,
    sortValue: (s) => s.PeerID },
]

const FILE_COLUMNS: Column<FileEntry>[] = [
  { key: 'name', header: 'Name', render: (f) => f.Name, sortValue: (f) => f.Name },
  { key: 'peer', header: 'Shared by',
    render: (f) => <span title={f.PeerID}>{truncateID(f.PeerID, 8)}</span>,
    sortValue: (f) => f.PeerID },
]

export default function ServicesPage() {
  const services = usePolling((s) => getServices(s), 1500)
  const files = usePolling((s) => getFiles(s), 1500)

  return (
    <>
      <section className="ev-panel">
        <h2 className="ev-panel-title">TCP tunnels</h2>
        {services.error && (
          <p className="ev-error">Cannot reach the node: {services.error.message}</p>
        )}
        <DataTable columns={SERVICE_COLUMNS} rows={services.data ?? []}
                   rowKey={(s) => `${s.PeerID}/${s.Name}`}
                   emptyText="No services advertised" />
      </section>
      <section className="ev-panel">
        <h2 className="ev-panel-title">Files</h2>
        {files.error && <p className="ev-error">Cannot reach the node: {files.error.message}</p>}
        <DataTable columns={FILE_COLUMNS} rows={files.data ?? []}
                   rowKey={(f) => `${f.PeerID}/${f.Name}`}
                   emptyText="No files shared" />
      </section>
    </>
  )
}
