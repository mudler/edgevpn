import { describe, it, expect, vi, afterEach } from 'vitest'
import { ApiError, getSummary, deleteLedgerKey } from './api'

afterEach(() => { vi.unstubAllGlobals() })

describe('getSummary', () => {
  it('returns parsed JSON on success', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ Machines: 7, NodeID: 'abc' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))
    const s = await getSummary()
    expect(s.Machines).toBe(7)
    expect(s.NodeID).toBe('abc')
  })

  it('throws ApiError carrying the status on failure', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      'boom', { status: 503 },
    )))
    await expect(getSummary()).rejects.toBeInstanceOf(ApiError)
    await expect(getSummary()).rejects.toMatchObject({ status: 503 })
  })
})

describe('deleteLedgerKey', () => {
  it('URL-encodes bucket and key so regexes and IPs survive', async () => {
    const spy = vi.fn(async () => new Response('{}', { status: 200 }))
    vi.stubGlobal('fetch', spy)
    await deleteLedgerKey('dns', 'foo.*\\.bar')
    expect(spy).toHaveBeenCalledWith(
      '/api/ledger/dns/foo.*%5C.bar',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })
})
