import { api } from './client'
import type { FinanceSummaryResponse } from '@/types/finance-summary'

// ============================================================================
// FINANCE/RECONCILIATION SUMMARY API (read-only) — PASS_18Z
//
// Aggregates existing ledger/alert/reconciliation data. Introduces no new
// accounting model or endpoint mutation — mirrors the sibling read-only
// finance endpoints in reconciliation.ts (raw JSON body, no {data:} envelope).
// ============================================================================

/**
 * Get the aggregate finance/reconciliation summary.
 * GET /api/v1/admin/finance/summary
 * Requires: finance.withdraw.read
 */
export async function getFinanceSummary() {
  return api.get<FinanceSummaryResponse>('/api/v1/admin/finance/summary')
}
