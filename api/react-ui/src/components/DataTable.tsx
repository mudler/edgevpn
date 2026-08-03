import { useMemo, useState, type ReactNode } from 'react'
import './DataTable.css'

export type Column<T> = {
  key: string
  header: string
  render: (row: T) => ReactNode
  /** Omit to make the column unsortable. */
  sortValue?: (row: T) => string | number
}

type Props<T> = {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  emptyText?: string
  pageSize?: number
}

export default function DataTable<T>({
  columns, rows, rowKey, emptyText = 'Nothing here yet', pageSize = 10,
}: Props<T>) {
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [asc, setAsc] = useState(true)
  const [page, setPage] = useState(0)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return rows
    // Search across every column's sortable value, falling back to the
    // row key so peer IDs remain findable.
    return rows.filter((r) =>
      columns.some((c) => c.sortValue
        ? String(c.sortValue(r)).toLowerCase().includes(q)
        : false) || rowKey(r).toLowerCase().includes(q),
    )
  }, [rows, query, columns, rowKey])

  const sorted = useMemo(() => {
    const col = columns.find((c) => c.key === sortKey)
    if (!col?.sortValue) return filtered
    const get = col.sortValue
    return [...filtered].sort((a, b) => {
      const av = get(a), bv = get(b)
      if (av === bv) return 0
      return (av < bv ? -1 : 1) * (asc ? 1 : -1)
    })
  }, [filtered, sortKey, asc, columns])

  const pages = Math.max(1, Math.ceil(sorted.length / pageSize))
  const current = Math.min(page, pages - 1)
  const visible = sorted.slice(current * pageSize, current * pageSize + pageSize)

  function toggleSort(key: string) {
    if (sortKey === key) setAsc(!asc)
    else { setSortKey(key); setAsc(true) }
    setPage(0)
  }

  return (
    <div className="ev-table-wrap">
      <div className="ev-table-tools">
        <input
          className="ev-search"
          type="search"
          placeholder="Filter…"
          value={query}
          onChange={(e) => { setQuery(e.target.value); setPage(0) }}
          aria-label="Filter table"
        />
        <span className="ev-count tabular">{sorted.length}</span>
      </div>

      <div className="ev-scroller">
        <table className="ev-table">
          <thead>
            <tr>
              {columns.map((c) => (
                <th key={c.key} scope="col">
                  {c.sortValue ? (
                    <button type="button" className="ev-sort" onClick={() => toggleSort(c.key)}>
                      {c.header}
                      {sortKey === c.key && <span aria-hidden="true">{asc ? ' ↑' : ' ↓'}</span>}
                    </button>
                  ) : c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 && (
              <tr><td colSpan={columns.length} className="ev-empty">{emptyText}</td></tr>
            )}
            {visible.map((row) => (
              <tr key={rowKey(row)}>
                {columns.map((c) => <td key={c.key}>{c.render(row)}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="ev-pager">
          <button type="button" onClick={() => setPage(Math.max(0, current - 1))}
                  disabled={current === 0}>Prev</button>
          <span className="tabular">{current + 1}<span className="slash">/</span>{pages}</span>
          <button type="button" onClick={() => setPage(Math.min(pages - 1, current + 1))}
                  disabled={current >= pages - 1}>Next</button>
        </div>
      )}
    </div>
  )
}
