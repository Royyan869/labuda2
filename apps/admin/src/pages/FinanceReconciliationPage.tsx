import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { getReconciliationResults, getLatestReconciliationResult } from '@/lib/api/reconciliation'
import {
  reconciliationSeverityLabels,
  type ReconciliationResult,
  type ReconciliationSeverity,
} from '@/types/reconciliation'
import { FinanceSummaryPanel } from '@/components/finance/FinanceSummaryPanel'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
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
          <div className={`p-2 rounded-lg ${latest?.severity === 'passed' ? 'bg-success-bg' : notFound ? 'bg-surface-muted' : 'bg-warning-bg'}`}>
            {latest?.severity === 'passed' ? (
              <CheckCircle2 className="h-5 w-5 text-white" />
            ) : (
              <AlertTriangle className="h-5 w-5 text-white" />
            )}
          </div>
          <div className="flex-1">
            <p className="type-label">
              {loading
                ? 'Checking last reconciliation run...'
                : notFound
                ? 'No reconciliation runs yet'
                : `Last run: ${latest ? new Date(latest.checked_at).toLocaleString() : ''}`}
            </p>
            <p className="type-caption mt-0.5">
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
      <PageHeader
        title="Reconciliation"
        description="Ledger/account balance verification history (read-only — reconciliation never auto-repairs)"
        actions={
          <Button variant="ghost" size="sm" onClick={fetchResults} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        }
      />

      <FinanceSummaryPanel />

      <LatestRunCard />

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center gap-4 flex-wrap">
            <Select
              label="Severity"
              size="compact"
              value={severityFilter}
              onChange={(e) => { setSeverityFilter(e.target.value as ReconciliationSeverity | ''); setOffset(0) }}
            >
              <option value="">All</option>
              <option value="passed">Passed</option>
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="critical">Critical</option>
            </Select>
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
            {(severityFilter || fromFilter || toFilter) && (
              <div className="self-end">
                <Button variant="ghost" size="sm" onClick={resetFilters}>
                  Clear
                </Button>
              </div>
            )}
            <div className="ml-auto type-secondary">
              {total} run{total !== 1 ? 's' : ''}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <AdminErrorState
          title="Failed to load reconciliation results"
          message={error}
          onRetry={fetchResults}
        />
      )}

      {/* Loading State */}
      {loading && results.length === 0 && !error && <AdminLoadingState />}

      {/* Empty State */}
      {!loading && !error && results.length === 0 && (
        <Card>
          <CardContent>
            <AdminEmptyState
              icon={History}
              title="No Reconciliation Runs"
              description={
                severityFilter || fromFilter || toFilter
                  ? 'No runs match the current filters.'
                  : 'No reconciliation runs have been recorded yet.'
              }
              filtered={Boolean(severityFilter || fromFilter || toFilter)}
              onClearFilters={resetFilters}
            />
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
            <div className="divide-y divide-border">
              {results.map((r) => {
                const expanded = expandedId === r.id
                const hasDetails = r.details && Object.keys(r.details).length > 0
                return (
                  <div key={r.id} className="px-6 py-4">
                    <div className="flex items-start gap-4">
                      <div className="flex flex-col gap-1.5 min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <SeverityBadge severity={r.severity} />
                          <span className="type-caption">action: {r.action_taken}</span>
                          {r.auto_repaired && <Badge variant="warning">auto_repaired (historical)</Badge>}
                        </div>
                        <p className="type-body">
                          {new Date(r.checked_at).toLocaleString()} — {r.mismatched_accounts}/{r.total_accounts} accounts mismatched
                        </p>
                        {hasDetails && (
                          <div>
                            <button
                              onClick={() => setExpandedId(expanded ? null : r.id)}
                              className="flex items-center gap-1 text-xs text-info hover:text-info"
                            >
                              {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                              {expanded ? 'Hide' : 'Show'} details
                            </button>
                            {expanded && (
                              <pre className="mt-2 text-xs bg-surface-muted border border-border rounded p-2 overflow-auto max-h-64 text-foreground">
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
