import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { getReconciliationResults, getLatestReconciliationResult } from '@/lib/api/reconciliation'
import {
  reconciliationSeverityLabels,
  type ReconciliationResult,
  type ReconciliationSeverity,
} from '@/types/reconciliation'
import { FinanceSummaryPanel } from '@/components/finance/FinanceSummaryPanel'
import { RefreshCw, AlertTriangle, History, CheckCircle2, ChevronDown, ChevronRight } from 'lucide-react'

const PAGE_SIZE = 50

const severityBadgeVariant: Record<Exclude<ReconciliationSeverity, 'passed'>, 'default' | 'info' | 'warning' | 'error'> = {
  low: 'default',
  medium: 'info',
  high: 'warning',
  critical: 'error',
}

function SeverityBadge({ severity }: { severity: ReconciliationSeverity }) {
  if (severity === 'passed') {
    return <Badge variant="success">Passed</Badge>
  }
  return <Badge variant={severityBadgeVariant[severity]}>{reconciliationSeverityLabels[severity]}</Badge>
}

function LatestRunCard() {
  const [latest, setLatest] = useState<ReconciliationResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)

  const fetchLatest = useCallback(async () => {
    setLoading(true)
    setNotFound(false)
    try {
      const result = await getLatestReconciliationResult()
      setLatest(result)
    } catch {
      // No results yet (fresh environment) — this is expected, not an error.
      setNotFound(true)
      setLatest(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLatest()
  }, [fetchLatest])

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-4">
          <div className={`p-2 rounded-lg ${latest?.severity === 'passed' ? 'bg-green-500' : notFound ? 'bg-gray-400' : 'bg-amber-500'}`}>
            {latest?.severity === 'passed' ? (
              <CheckCircle2 className="h-5 w-5 text-white" />
            ) : (
              <AlertTriangle className="h-5 w-5 text-white" />
            )}
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-gray-900">
              {loading
                ? 'Checking last reconciliation run...'
                : notFound
                ? 'No reconciliation runs yet'
                : `Last run: ${latest ? new Date(latest.checked_at).toLocaleString() : ''}`}
            </p>
            <p className="text-xs text-gray-500 mt-0.5">
              {!loading && !notFound && latest && (
                <>Worker is alive — most recent run was {reconciliationSeverityLabels[latest.severity].toLowerCase()}</>
              )}
              {!loading && notFound && 'The reconciliation worker has not persisted a result yet.'}
            </p>
          </div>
          {!loading && latest && <SeverityBadge severity={latest.severity} />}
        </div>
      </CardContent>
    </Card>
  )
}

export function FinanceReconciliationPage() {
  const [results, setResults] = useState<ReconciliationResult[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [severityFilter, setSeverityFilter] = useState<ReconciliationSeverity | ''>('')
  const [fromFilter, setFromFilter] = useState('')
  const [toFilter, setToFilter] = useState('')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  const fetchResults = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getReconciliationResults({
        severity: severityFilter || undefined,
        date_from: fromFilter ? new Date(fromFilter).toISOString() : undefined,
        date_to: toFilter ? new Date(toFilter).toISOString() : undefined,
        limit: PAGE_SIZE,
        offset,
      })
      setResults(response?.results ?? [])
      setTotal(response?.total ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch reconciliation results')
    } finally {
      setLoading(false)
    }
  }, [severityFilter, fromFilter, toFilter, offset])

  useEffect(() => {
    fetchResults()
  }, [fetchResults])

  const resetFilters = () => {
    setSeverityFilter('')
    setFromFilter('')
    setToFilter('')
    setOffset(0)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Reconciliation</h1>
          <p className="text-gray-600 mt-1">
            Ledger/account balance verification history (read-only — reconciliation never auto-repairs)
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchResults} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <FinanceSummaryPanel />

      <LatestRunCard />

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-4 flex-wrap">
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Severity</label>
              <select
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={severityFilter}
                onChange={(e) => { setSeverityFilter(e.target.value as ReconciliationSeverity | ''); setOffset(0) }}
              >
                <option value="">All</option>
                <option value="passed">Passed</option>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="critical">Critical</option>
              </select>
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
            {(severityFilter || fromFilter || toFilter) && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={resetFilters}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto text-sm text-gray-500">
              {total} run{total !== 1 ? 's' : ''}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <Card>
          <CardContent className="p-8 text-center">
            <AlertTriangle className="h-10 w-10 text-red-400 mx-auto mb-3" />
            <p className="text-gray-900 font-medium">Failed to load reconciliation results</p>
            <p className="text-gray-600 text-sm mt-1">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchResults} className="mt-4">
              Retry
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Loading State */}
      {loading && results.length === 0 && !error && (
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
      {!loading && !error && results.length === 0 && (
        <Card>
          <CardContent className="p-12 text-center">
            <History className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-gray-900">No Reconciliation Runs</h2>
            <p className="text-gray-600 mt-1">No runs match the current filters.</p>
          </CardContent>
        </Card>
      )}

      {/* Results Table */}
      {results.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Reconciliation Runs</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-gray-200">
              {results.map((r) => {
                const expanded = expandedId === r.id
                const hasDetails = r.details && Object.keys(r.details).length > 0
                return (
                  <div key={r.id} className="px-6 py-4">
                    <div className="flex items-start gap-4">
                      <div className="flex flex-col gap-1.5 min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <SeverityBadge severity={r.severity} />
                          <span className="text-xs text-gray-500">action: {r.action_taken}</span>
                          {r.auto_repaired && <Badge variant="warning">auto_repaired (historical)</Badge>}
                        </div>
                        <p className="text-sm text-gray-900">
                          {new Date(r.checked_at).toLocaleString()} — {r.mismatched_accounts}/{r.total_accounts} accounts mismatched
                        </p>
                        {hasDetails && (
                          <div>
                            <button
                              onClick={() => setExpandedId(expanded ? null : r.id)}
                              className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800"
                            >
                              {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                              {expanded ? 'Hide' : 'Show'} details
                            </button>
                            {expanded && (
                              <pre className="mt-2 text-xs bg-gray-50 border border-gray-200 rounded p-2 overflow-auto max-h-64 text-gray-700">
                                {JSON.stringify(r.details, null, 2)}
                              </pre>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })}
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
