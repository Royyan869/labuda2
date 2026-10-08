import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { getFinanceSummary } from '@/lib/api'
import { ACCOUNT_LABELS, type FinanceSummaryResponse } from '@/types/finance-summary'
import { RefreshCw, ShieldAlert, Info, Wallet, TrendingUp, Bell } from 'lucide-react'
import { AdminErrorState } from '@/components/common'

// ============================================================================
// FinanceSummaryPanel (PASS_18Z)
//
// Pure presentation of backend-computed numbers — no accounting logic lives
// here. Formats what GET /api/v1/admin/finance/summary already decided.
// ============================================================================

function formatIdr(amount: number): string {
  return `Rp ${amount.toLocaleString('id-ID')}`
}

/** Read-only balance card for one system account. */
function AccountBalanceCard({ accountType, balance, highlight }: { accountType: string; balance: number; highlight?: string }) {
  return (
    <div className="border border-border rounded-lg p-4">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
        {ACCOUNT_LABELS[accountType] ?? accountType}
      </p>
      <p className="text-xl font-mono font-semibold text-foreground mt-1">{formatIdr(balance)}</p>
      {highlight && <p className="type-caption mt-1">{highlight}</p>}
    </div>
  )
}

export function FinanceSummaryPanel() {
  const [summary, setSummary] = useState<FinanceSummaryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchSummary = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await getFinanceSummary()
      setSummary(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch finance summary')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSummary()
  }, [fetchSummary])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-foreground">Finance Summary</h2>
          <p className="type-secondary mt-0.5">
            Aggregate ledger balances and revenue — answers "how much is where"
            without a DB query.
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchSummary} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {error && (
        <AdminErrorState
          title="Failed to load finance summary"
          message={error}
          onRetry={fetchSummary}
        />
      )}

      {loading && !summary && !error && (
        <Card role="status" aria-busy="true">
          <CardContent className="p-6">
            <span className="sr-only">Loading finance summary…</span>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4" aria-hidden="true">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="animate-pulse h-20 bg-surface-muted rounded-lg" />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {summary && (
        <>
          {/* Honesty banner: internal vs external reconciliation */}
          <div className="flex items-start gap-3 p-3 bg-warning-bg border border-warning rounded-lg">
            <ShieldAlert className="h-5 w-5 text-warning mt-0.5 flex-shrink-0" />
            <div className="text-sm text-warning space-y-1">
              <p className="font-semibold">Internal ledger consistency only.</p>
              <p>Midtrans settlement/bank reconciliation is not implemented yet.</p>
              <p>Non-zero Gateway Clearing can be normal for paid orders not yet released.</p>
            </div>
          </div>

          {/* Account balances */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Wallet className="h-4 w-4" /> Account Balances
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <AccountBalanceCard
                  accountType="GATEWAY_CLEARING"
                  balance={summary.gateway_clearing.balance_rupiah}
                  highlight={summary.gateway_clearing.is_zero ? 'Zero' : 'Non-zero — see note below'}
                />
                {Object.entries(summary.system_account_balances)
                  .filter(([type]) => type !== 'GATEWAY_CLEARING')
                  .map(([type, balance]) => (
                    <AccountBalanceCard key={type} accountType={type} balance={balance} />
                  ))}
                {(summary.aggregate_user_account_balances ?? []).map((agg) => (
                  <AccountBalanceCard
                    key={agg.account_type}
                    accountType={agg.account_type}
                    balance={agg.total_balance_rupiah}
                    highlight={`across ${agg.account_count} account${agg.account_count === 1 ? '' : 's'}`}
                  />
                ))}
              </div>
              <p className="type-caption mt-3">{summary.gateway_clearing.note}</p>
            </CardContent>
          </Card>

          {/* Revenue breakdown */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <TrendingUp className="h-4 w-4" /> Platform Revenue Breakdown
              </CardTitle>
            </CardHeader>
            <CardContent>
              {summary.revenue_breakdown.available ? (
                <>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <AccountBalanceCard accountType="Buyer Payment Fee Revenue" balance={summary.revenue_breakdown.buyer_payment_fee_revenue_rupiah} />
                    <AccountBalanceCard accountType="Commission Revenue" balance={summary.revenue_breakdown.commission_revenue_rupiah} />
                    <AccountBalanceCard accountType="Other Revenue" balance={summary.revenue_breakdown.other_revenue_rupiah} />
                    <AccountBalanceCard accountType="Total Platform Revenue" balance={summary.revenue_breakdown.total_platform_revenue_rupiah} />
                  </div>
                  {summary.revenue_breakdown.other_revenue_reference_types && summary.revenue_breakdown.other_revenue_reference_types.length > 0 && (
                    <p className="type-caption mt-2">
                      Other revenue sources: {summary.revenue_breakdown.other_revenue_reference_types.join(', ')}
                    </p>
                  )}
                </>
              ) : (
                <div className="flex items-center gap-2 type-secondary">
                  <Info className="h-4 w-4" />
                  Breakdown not distinguishable from current ledger data.
                </div>
              )}
              <p className="type-caption mt-3">{summary.revenue_breakdown.note}</p>
            </CardContent>
          </Card>

          {/* Finance alerts */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Bell className="h-4 w-4" /> Finance Alerts
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-4 flex-wrap">
                <Badge variant={summary.finance_alerts.unresolved_critical_total > 0 ? 'error' : 'default'}>
                  {summary.finance_alerts.unresolved_critical_total} unresolved critical
                </Badge>
                <Badge variant={summary.finance_alerts.unresolved_total > 0 ? 'warning' : 'success'}>
                  {summary.finance_alerts.unresolved_total} unresolved total
                </Badge>
                <Badge variant={summary.finance_alerts.payment_captured_after_expiry_count > 0 ? 'error' : 'default'}>
                  {summary.finance_alerts.payment_captured_after_expiry_count} payment_captured_after_expiry
                </Badge>
              </div>
              {summary.finance_alerts.unresolved_by_type && Object.keys(summary.finance_alerts.unresolved_by_type).length > 0 && (
                <ul className="mt-3 type-body space-y-1">
                  {Object.entries(summary.finance_alerts.unresolved_by_type).map(([type, count]) => (
                    <li key={type} className="flex justify-between border-b border-border py-1">
                      <span className="font-mono text-xs">{type}</span>
                      <span className="font-medium">{count}</span>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>

          {/* Reconciliation status: internal vs external */}
          <Card>
            <CardHeader>
              <CardTitle>Reconciliation Status</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="type-label">Internal ledger consistency:</span>
                  {summary.internal_reconciliation.available ? (
                    <Badge variant={summary.internal_reconciliation.severity === 'passed' ? 'success' : 'warning'}>
                      {summary.internal_reconciliation.severity}
                      {' — '}
                      {summary.internal_reconciliation.mismatched_accounts}/{summary.internal_reconciliation.total_accounts} mismatched
                    </Badge>
                  ) : (
                    <Badge variant="default">no runs yet</Badge>
                  )}
                </div>
                {summary.internal_reconciliation.available && summary.internal_reconciliation.last_checked_at && (
                  <p className="type-caption mt-1">
                    Last checked: {new Date(summary.internal_reconciliation.last_checked_at).toLocaleString()}
                  </p>
                )}
                <p className="type-caption mt-1">{summary.internal_reconciliation.note}</p>
              </div>

              <div>
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="type-label">External Midtrans settlement reconciliation:</span>
                  <Badge variant="default">Not Implemented</Badge>
                </div>
                <div className="flex items-center gap-2 flex-wrap mt-1">
                  <span className="type-label">Bank statement reconciliation:</span>
                  <Badge variant="default">Not Implemented</Badge>
                </div>
                <p className="type-caption mt-1">{summary.external_reconciliation.note}</p>
              </div>
            </CardContent>
          </Card>

          <p className="type-caption text-right">
            Generated at {new Date(summary.generated_at).toLocaleString()}
          </p>
        </>
      )}
    </div>
  )
}
