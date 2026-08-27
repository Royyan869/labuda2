import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { getLedgerTransactions } from '@/lib/api'
import type { LedgerTransaction } from '@/types/finance'
import { RefreshCw, AlertTriangle, BookOpen } from 'lucide-react'

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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Finance Ledger</h1>
          <p className="text-gray-600 mt-1">Ledger transactions (read-only)</p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchLedger} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-4 flex-wrap">
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Reference Type</label>
              <input
                type="text"
                placeholder="e.g. ORDER"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm w-40"
                value={referenceTypeFilter}
                onChange={(e) => { setReferenceTypeFilter(e.target.value); setOffset(0) }}
              />
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">From</label>
              <input
                type="date"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={fromFilter}
                onChange={(e) => { setFromFilter(e.target.value); setOffset(0) }}
              />
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">To</label>
              <input
                type="date"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={toFilter}
                onChange={(e) => { setToFilter(e.target.value); setOffset(0) }}
              />
            </div>
            {(referenceTypeFilter || fromFilter || toFilter) && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={resetFilters}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto text-sm text-gray-500">
              {total} transaction{total !== 1 ? 's' : ''}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <Card>
          <CardContent className="p-8 text-center">
            <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
            <p className="text-gray-900 font-medium">Failed to load ledger</p>
            <p className="text-gray-600 text-sm mt-1">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchLedger} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Loading State */}
      {loading && transactions.length === 0 && !error && (
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
      {!loading && !error && transactions.length === 0 && (
        <Card>
          <CardContent className="p-12 text-center">
            <BookOpen className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-gray-900">No Ledger Transactions</h2>
            <p className="text-gray-600 mt-1">No transactions match the current filters.</p>
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
                  <tr className="border-b border-gray-200 bg-gray-50">
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Transaction ID</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Reference</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Idempotency Key</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Entries</th>
                    <th className="px-4 py-3 text-left font-medium text-gray-600">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {transactions.map((tx) => (
                    <tr key={tx.id} className="hover:bg-gray-50 align-top">
                      <td className="px-4 py-3 font-mono text-xs text-gray-700">
                        {tx.id.slice(0, 8)}...
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant="info">{tx.reference_type}</Badge>
                        {tx.reference_id && (
                          <div className="font-mono text-xs text-gray-500 mt-1">
                            {tx.reference_id.slice(0, 8)}...
                          </div>
                        )}
                        {tx.order_id && (
                          <div className="text-xs text-gray-500 mt-0.5">
                            order: {tx.order_id.slice(0, 8)}...
                          </div>
                        )}
                        {tx.payment_id && (
                          <div className="text-xs text-gray-500 mt-0.5">
                            payment: {tx.payment_id.slice(0, 8)}...
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-gray-600 max-w-[200px] truncate" title={tx.idempotency_key}>
                        {tx.idempotency_key}
                      </td>
                      <td className="px-4 py-3">
                        <div className="space-y-1">
                          {tx.entries.map((entry) => (
                            <div key={entry.id} className="flex items-center gap-2 text-xs">
                              <Badge variant={entry.entry_type === 'debit' ? 'error' : 'success'}>
                                {entry.entry_type === 'debit' ? 'DR' : 'CR'}
                              </Badge>
                              <span className="text-gray-700">{entry.account_type}</span>
                              <span className="font-medium text-gray-900">
                                Rp {entry.amount.toLocaleString()}
                              </span>
                              <span className="text-gray-400" title="Balance after">
                                (bal: {entry.balance_after.toLocaleString()})
                              </span>
                            </div>
                          ))}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-xs text-gray-600 whitespace-nowrap">
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
            Page {currentPage} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setOffset((o) => o + PAGE_SIZE)}
            disabled={currentPage >= totalPages}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
