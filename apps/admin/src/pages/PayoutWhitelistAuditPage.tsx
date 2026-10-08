import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import { getWhitelistAudit } from '@/lib/api'
import type { WhitelistAuditRow, WhitelistAuditAction } from '@/types/finance'
import { whitelistActionLabels, whitelistActionVariants } from '@/types/finance'
import { RefreshCw, ClipboardCheck } from 'lucide-react'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, PageHeader } from '@/components/common'

const PAGE_SIZE = 50

/**
 * Payout Whitelist Audit.
 *
 * Read-only, append-only compliance log of every pilot whitelist mutation.
 * Navigation is keyset-based (`next_cursor` + `has_more`), not page-based:
 * the log is ordered (created_at DESC, id DESC) and grows by appends, so a
 * truthful total / page number would be neither stable nor useful.
 */
export function PayoutWhitelistAuditPage() {
  const [rows, setRows] = useState<WhitelistAuditRow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [sellerIdFilter, setSellerIdFilter] = useState('')

  const fetchPage = useCallback(
    async (cursor: string | null) => {
      setLoading(true)
      setError(null)
      try {
        const response = await getWhitelistAudit({
          seller_id: sellerIdFilter || undefined,
          limit: PAGE_SIZE,
          cursor,
        })
        const pageRows = response?.audit_log ?? []
        setRows((prev) => (cursor ? [...prev, ...pageRows] : pageRows))
        setHasMore(Boolean(response?.has_more))
        setNextCursor(response?.next_cursor ?? null)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch whitelist audit')
      } finally {
        setLoading(false)
      }
    },
    [sellerIdFilter]
  )

  // A filter change resets the keyset and reloads from the newest page.
  useEffect(() => {
    fetchPage(null)
  }, [fetchPage])

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Payout Whitelist Audit"
        description="Payout pilot whitelist change log (read-only)"
        actions={
          <Button variant="ghost" size="sm" onClick={() => fetchPage(null)} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        }
      />

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-end gap-4 flex-wrap">
            <Input
              type="text"
              label="Seller ID"
              size="compact"
              placeholder="UUID"
              className="w-72 font-mono"
              value={sellerIdFilter}
              onChange={(e) => setSellerIdFilter(e.target.value)}
            />
            {sellerIdFilter && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={() => setSellerIdFilter('')}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto type-secondary">
              {rows.length} record{rows.length !== 1 ? 's' : ''} loaded
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <AdminErrorState
          title="Failed to load whitelist audit"
          message={error}
          onRetry={() => fetchPage(null)}
        />
      )}

      {/* Loading State */}
      {loading && rows.length === 0 && !error && <AdminLoadingState />}

      {/* Empty State */}
      {!loading && !error && rows.length === 0 && (
        <Card>
          <CardContent>
            <AdminEmptyState
              icon={ClipboardCheck}
              title="No Audit Records"
              description={
                sellerIdFilter
                  ? 'No whitelist audit records match the current filter.'
                  : 'No whitelist audit records have been recorded yet.'
              }
              filtered={Boolean(sellerIdFilter)}
              onClearFilters={() => setSellerIdFilter('')}
            />
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
                  <tr className="border-b border-border bg-surface-muted">
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Action</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Seller ID</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Actor</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Source</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Reason</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {rows.map((row) => {
                    const actionKey = row.action as WhitelistAuditAction
                    return (
                      <tr key={row.id} className="hover:bg-surface-muted">
                        <td className="px-4 py-3">
                          <Badge variant={whitelistActionVariants[actionKey] ?? 'info'}>
                            {whitelistActionLabels[actionKey] ?? row.action}
                          </Badge>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-foreground">
                          {row.seller_id ? `${row.seller_id.slice(0, 8)}...` : '-'}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-foreground">
                          {row.actor_id}
                        </td>
                        <td className="px-4 py-3 text-foreground">
                          {row.source}
                        </td>
                        <td className="px-4 py-3 text-muted-foreground max-w-[300px] truncate" title={row.reason}>
                          {row.reason || '-'}
                        </td>
                        <td className="px-4 py-3 type-caption whitespace-nowrap">
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

      {/* Keyset continuation */}
      {rows.length > 0 && (
        <div className="flex items-center justify-between">
          <span className="type-secondary">
            {rows.length} record{rows.length !== 1 ? 's' : ''} loaded
          </span>
          {hasMore ? (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => fetchPage(nextCursor)}
              disabled={loading || !nextCursor}
            >
              {loading ? 'Loading…' : 'Load more'}
            </Button>
          ) : (
            <span className="type-secondary">End of log</span>
          )}
        </div>
      )}
    </div>
  )
}
