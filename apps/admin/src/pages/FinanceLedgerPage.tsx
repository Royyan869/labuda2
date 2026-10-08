import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import { getLedgerTransactions } from '@/lib/api'
import type { LedgerTransaction } from '@/types/finance'
import { RefreshCw, BookOpen } from 'lucide-react'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'

const PAGE_SIZE = 50

export function FinanceLedgerPage() {
  const [transactions, setTransactions] = useState<LedgerTransaction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [referenceTypeFilter, setReferenceTypeFilter] = useState('')
  const [fromFilter, setFromFilter] = useState('')
  const [toFilter, setToFilter] = useState('')

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  const fetchLedger = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getLedgerTransactions({
        from: fromFilter || undefined,
        to: toFilter || undefined,
        reference_type: referenceTypeFilter || undefined,
        limit: PAGE_SIZE,
        offset,
      })
      setTransactions(response?.transactions ?? [])
      setTotal(response?.total ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch ledger')
    } finally {
      setLoading(false)
    }
  }, [referenceTypeFilter, fromFilter, toFilter, offset])

  useEffect(() => {
    fetchLedger()
  }, [fetchLedger])

  const resetFilters = () => {
    setReferenceTypeFilter('')
    setFromFilter('')
    setToFilter('')
    setOffset(0)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Finance Ledger"
        description="Ledger transactions (read-only)"
        actions={
          <Button variant="ghost" size="sm" onClick={fetchLedger} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        }
      />

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-4 flex-wrap">
            <Input
              type="text"
              label="Reference Type"
              size="compact"
              placeholder="e.g. ORDER"
              className="w-40"
              value={referenceTypeFilter}
              onChange={(e) => { setReferenceTypeFilter(e.target.value); setOffset(0) }}
            />
            <Input
              type="date"
              label="From"
              size="compact"
              value={fromFilter}
              onChange={(e) => { setFromFilter(e.target.value); setOffset(0) }}
            />
            <Input
              type="date"
              label="To"
              size="compact"
              value={toFilter}
              onChange={(e) => { setToFilter(e.target.value); setOffset(0) }}
            />
            {(referenceTypeFilter || fromFilter || toFilter) && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={resetFilters}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto type-secondary">
              {total} transaction{total !== 1 ? 's' : ''}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <AdminErrorState
          title="Failed to load ledger"
          message={error}
          onRetry={fetchLedger}
        />
      )}

      {/* Loading State */}
      {loading && transactions.length === 0 && !error && <AdminLoadingState />}

      {/* Empty State */}
      {!loading && !error && transactions.length === 0 && (
        <Card>
          <CardContent>
            <AdminEmptyState
              icon={BookOpen}
              title="No Ledger Transactions"
              description={
                referenceTypeFilter || fromFilter || toFilter
                  ? 'No transactions match the current filters.'
                  : 'No ledger transactions have been recorded yet.'
              }
              filtered={Boolean(referenceTypeFilter || fromFilter || toFilter)}
              onClearFilters={resetFilters}
            />
          </CardContent>
        </Card>
      )}

      {/* Transactions Table */}
      {transactions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Ledger Transactions</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-surface-muted">
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Transaction ID</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Reference</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Idempotency Key</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Entries</th>
                    <th className="px-4 py-3 text-left font-medium text-muted-foreground">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {transactions.map((tx) => (
                    <tr key={tx.id} className="hover:bg-surface-muted align-top">
                      <td className="px-4 py-3 font-mono text-xs text-foreground">
                        {tx.id.slice(0, 8)}...
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant="info">{tx.reference_type}</Badge>
                        {tx.reference_id && (
                          <div className="font-mono type-caption mt-1">
                            {tx.reference_id.slice(0, 8)}...
                          </div>
                        )}
                        {tx.order_id && (
                          <div className="type-caption mt-0.5">
                            order: {tx.order_id.slice(0, 8)}...
                          </div>
                        )}
                        {tx.payment_id && (
                          <div className="type-caption mt-0.5">
                            payment: {tx.payment_id.slice(0, 8)}...
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3 font-mono type-caption max-w-[200px] truncate" title={tx.idempotency_key}>
                        {tx.idempotency_key}
                      </td>
                      <td className="px-4 py-3">
                        <div className="space-y-1">
                          {/* Defensive guard: the backend wire contract sends
                              entries: [] (never null), but a legacy/cached
                              response or proxy must never crash this page
                              again ("Cannot read properties of null
                              (reading 'map')"). */}
                          {(tx.entries ?? []).map((entry) => (
                            <div key={entry.id} className="flex items-center gap-2 text-xs">
                              <Badge variant={entry.entry_type === 'debit' ? 'error' : 'success'}>
                                {entry.entry_type === 'debit' ? 'DR' : 'CR'}
                              </Badge>
                              <span className="text-foreground">{entry.account_type}</span>
                              <span className="font-medium text-foreground">
                                Rp {entry.amount.toLocaleString()}
                              </span>
                              <span className="text-muted-foreground" title="Balance after">
                                (bal: {entry.balance_after.toLocaleString()})
                              </span>
                            </div>
                          ))}
                        </div>
                      </td>
                      <td className="px-4 py-3 type-caption whitespace-nowrap">
                        {new Date(tx.created_at).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <AdminPagination
          page={currentPage}
          totalPages={totalPages}
          onPageChange={(newPage) => setOffset((newPage - 1) * PAGE_SIZE)}
          disabled={loading}
        />
      )}
    </div>
  )
}
