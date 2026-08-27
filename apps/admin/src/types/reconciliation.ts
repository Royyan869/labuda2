// Reconciliation result types — read-only visibility into
// ReconciliationWorkerV2 output (backend: reconciliation_results table).
//
// Reconciliation is verification-only: these types have no mutation/repair
// shape because no such endpoint exists.

export type ReconciliationSeverity = 'passed' | 'low' | 'medium' | 'high' | 'critical'

export type ReconciliationAction = 'none' | 'logged' | 'alerted' | 'auto_repaired' | 'escalated'

export interface ReconciliationResult {
  id: string
  checked_at: string
  severity: ReconciliationSeverity
  action_taken: ReconciliationAction
  auto_repaired: boolean
  total_accounts: number
  mismatched_accounts: number
  details: Record<string, unknown>
  created_at: string
}

export interface ReconciliationListResponse {
  results: ReconciliationResult[]
  total: number
  limit: number
  offset: number
}

export const reconciliationSeverityLabels: Record<ReconciliationSeverity, string> = {
  passed: 'Passed',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
}
