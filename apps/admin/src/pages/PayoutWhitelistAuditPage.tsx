import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { getWhitelistAudit } from '@/lib/api'
import type { WhitelistAuditRow, WhitelistAuditAction } from '@/types/finance'
import { whitelistActionLabels, whitelistActionVariants } from '@/types/finance'
import { RefreshCw, AlertTriangle, ClipboardCheck } from 'lucide-react'

const PAGE_SIZE = 50

export function PayoutWhitelistAuditPage() {
  const [rows, setRows] = useState<WhitelistAuditRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [count, setCount] = useState(0)
  const [offset, setOffset] = useState(0)
  const [sellerIdFilter, setSellerIdFilter] = useState('')

  const hasMore = count >= PAGE_SIZE
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  const fetchAudit = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getWhitelistAudit({
        seller_id: sellerIdFilter || undefined,
        limit: PAGE_SIZE,
        offset,
      })
      setRows(response?.audit_log ?? [])
      setCount(response?.count ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch whitelist audit')
    } finally {
      setLoading(false)
    }
  }, [sellerIdFilter, offset])

  useEffect(() => {
    fetchAudit()
  }, [fetchAudit])

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Payout Whitelist Audit</h1>
          <p className="text-gray-600 mt-1">Payout pilot whitelist change log (read-only)</p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchAudit} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-4 flex-wrap">
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Seller ID</label>
              <input
                type="text"
                placeholder="UUID"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm w-72 font-mono"
                value={sellerIdFilter}
                onChange={(e) => { setSellerIdFilter(e.target.value); setOffset(0) }}
              />
            </div>
            {sellerIdFilter && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={() => { setSellerIdFilter(''); setOffset(0) }}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto text-sm text-gray-500">
              {count} record{count !== 1 ? 's' : ''} on this page
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <Card>
          <CardContent className="p-8 text-center">
            <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
            <p className="text-gray-900 font-medium">Failed to load whitelist audit</p>
            <p className="text-gray-600 text-sm mt-1">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchAudit} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Loading State */}
      {loading && rows.length === 0 && !error && (
        <Card>
          <CardContent className="p-8">
            <div className="space-y-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="animate-pulse flex items-center gap-4">
                  <div className="h-4 bg-gray-200 rounded w-32" />
                  <div className="h-4 bg-gray-200 rounded flex-1" />
                  <div className="h-6 w-20 bg-gray-200 rounded-full" />
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!loading && !error && rows.length === 0 && (
        <Card>
          <CardContent className="p-12 text-center">
            <ClipboardCheck className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-gray-900">No Audit Records</h2>
            <p className="text-gray-600 mt-1">No whitelist audit records match the current filter.</p>
          </CardContent>
        </Card>
      )}

      {/* Audit Table */}
      {rows.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Whitelist Audit Log</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 bg-gray-50">
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Action</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Seller ID</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Actor</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Source</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Reason</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {rows.map((row) => {
                    const actionKey = row.action as WhitelistAuditAction
                    return (
                      <tr key={row.id} className="hover:bg-gray-50">
                        <td className="px-4 py-3">
                          <Badge variant={whitelistActionVariants[actionKey] ?? 'info'}>
                            {whitelistActionLabels[actionKey] ?? row.action}
                          </Badge>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-gray-700">
                          {row.seller_id ? `${row.seller_id.slice(0, 8)}...` : '-'}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-gray-700">
                          {row.actor_id}
                        </td>
                        <td className="px-4 py-3 text-gray-700">
                          {row.source}
                        </td>
                        <td className="px-4 py-3 text-gray-600 max-w-[300px] truncate" title={row.reason}>
                          {row.reason || '-'}
                        </td>
                        <td className="px-4 py-3 text-xs text-gray-600 whitespace-nowrap">
                          {new Date(row.created_at).toLocaleString()}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Pagination */}
      {(offset > 0 || hasMore) && (
        <div className="flex items-center justify-between">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
            disabled={offset <= 0}
          >
            Previous
          </Button>
          <span className="text-sm text-gray-600">
            Page {currentPage}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setOffset((o) => o + PAGE_SIZE)}
            disabled={!hasMore}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
