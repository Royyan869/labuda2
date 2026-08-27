package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReconciliationResult represents a single reconciliation check result.
//
// DESIGN PRINCIPLES:
// - Persistent log of all reconciliation runs
// - Stores mismatch details for analysis
// - Tracks severity and required actions
// - Supports both detection and auto-repair outcomes
type ReconciliationResult struct {
	ID                   uuid.UUID        `json:"id"`
	CheckedAt            time.Time        `json:"checked_at"`
	TotalAccounts        int              `json:"total_accounts"`
	MismatchedAccounts   int              `json:"mismatched_accounts"`
	Severity             ReconcileSeverity `json:"severity"`
	Details              ReconcileDetails `json:"details"`
	ActionTaken           ReconcileAction  `json:"action_taken"`
	AutoRepaired          bool             `json:"auto_repaired"`
	DoubleCheckPassed     bool             `json:"double_check_passed"`
	CreatedAt            time.Time        `json:"created_at"`
}

// ReconcileSeverity defines the severity level of a reconciliation issue.
type ReconcileSeverity string

const (
	// SeverityReconcilePassed means no issues found
	SeverityReconcilePassed ReconcileSeverity = "passed"
	// SeverityReconcileLow means minor discrepancies, no action needed
	SeverityReconcileLow ReconcileSeverity = "low"
	// SeverityReconcileMedium means mismatches detected, monitoring needed
	SeverityReconcileMedium ReconcileSeverity = "medium"
	// SeverityReconcileHigh means significant drift, intervention needed
	SeverityReconcileHigh ReconcileSeverity = "high"
	// SeverityReconcileCritical means critical system invariant violated
	SeverityReconcileCritical ReconcileSeverity = "critical"
)

// ReconcileAction defines the action taken after reconciliation.
type ReconcileAction string

const (
	// ActionNone means no action needed (passed)
	ActionNone ReconcileAction = "none"
	// ActionLogged means issue logged only
	ActionLogged ReconcileAction = "logged"
	// ActionAlerted means alert created
	ActionAlerted ReconcileAction = "alerted"
	// ActionAutoRepaired is retained for historical reconciliation_results rows
	// that were written before the auto-repair surface was eliminated. No new
	// code path produces this value — reconciliation is verification only
	// (RUNTIME-INVARIANTS §7.1, ADR-002). Corrections flow through canonical
	// FinanceService methods with attributable operator.
	ActionAutoRepaired ReconcileAction = "auto_repaired"
	// ActionEscalated means escalated for manual intervention
	ActionEscalated ReconcileAction = "escalated"
)

// ReconcileDetails contains detailed information about reconciliation results.
type ReconcileDetails map[string]interface{}

// AccountMismatch represents a single account balance mismatch.
type AccountMismatch struct {
	AccountID         uuid.UUID `json:"account_id"`
	AccountType       string    `json:"account_type"`
	StoredBalance     int64     `json:"stored_balance"`
	CalculatedBalance int64     `json:"calculated_balance"`
	Difference        int64     `json:"difference"`
	IsCritical        bool      `json:"is_critical"`
	OwnerID           *uuid.UUID `json:"owner_id,omitempty"`
}

// TransactionImbalance represents an unbalanced transaction.
type TransactionImbalance struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	SumAmount     int64     `json:"sum_amount"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   uuid.UUID `json:"reference_id"`
}

// NewReconciliationResult creates a new reconciliation result.
func NewReconciliationResult(
	checkedAt time.Time,
	totalAccounts int,
	mismatchedAccounts int,
	severity ReconcileSeverity,
	details ReconcileDetails,
) *ReconciliationResult {
	now := time.Now()
	return &ReconciliationResult{
		ID:                 uuid.New(),
		CheckedAt:          checkedAt,
		TotalAccounts:      totalAccounts,
		MismatchedAccounts: mismatchedAccounts,
		Severity:           severity,
		Details:            details,
		ActionTaken:        ActionLogged,
		AutoRepaired:       false,
		CreatedAt:          now,
	}
}

// WithAction updates the action taken and returns the result for chaining.
func (r *ReconciliationResult) WithAction(action ReconcileAction) *ReconciliationResult {
	r.ActionTaken = action
	return r
}

// WithDoubleCheck marks the double-check verification status.
func (r *ReconciliationResult) WithDoubleCheck(passed bool) *ReconciliationResult {
	r.DoubleCheckPassed = passed
	return r
}

// IsCritical returns true if the severity is critical.
func (r *ReconciliationResult) IsCritical() bool {
	return r.Severity == SeverityReconcileCritical
}

// NeedsEscalation returns true if this result needs manual intervention.
func (r *ReconciliationResult) NeedsEscalation() bool {
	return r.Severity == SeverityReconcileCritical || r.Severity == SeverityReconcileHigh
}

// GetAccountMismatches extracts account mismatches from details.
func (r *ReconciliationResult) GetAccountMismatches() []AccountMismatch {
	mismatches, ok := r.Details["account_mismatches"].([]AccountMismatch)
	if !ok {
		return nil
	}
	return mismatches
}

// GetTransactionImbalances extracts transaction imbalances from details.
func (r *ReconciliationResult) GetTransactionImbalances() []TransactionImbalance {
	imbalances, ok := r.Details["transaction_imbalances"].([]TransactionImbalance)
	if !ok {
		return nil
	}
	return imbalances
}

// ToJSON converts details to JSON bytes.
func (d ReconcileDetails) ToJSON() []byte {
	if d == nil {
		return []byte("{}")
	}
	data, _ := json.Marshal(d)
	return data
}

// FromJSON parses JSON bytes into ReconcileDetails.
func ReconcileDetailsFromJSON(data []byte) ReconcileDetails {
	if len(data) == 0 {
		return ReconcileDetails{}
	}
	var result ReconcileDetails
	_ = json.Unmarshal(data, &result)
	return result
}

// Helper function for absolute value
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// CriticalAccountTypes defines account types that are considered critical.
// A drift on these accounts is always reported at CRITICAL severity and
// escalated for manual review by an attributable operator.
var CriticalAccountTypes = map[string]bool{
	"ESCROW":             true,
	"WITHDRAWAL_PENDING": true,
	"WITHDRAWAL_COMMITTED": true,
	"PLATFORM_BANK":      true,
	"GATEWAY_CLEARING":   true,
}

// IsCriticalAccountType returns true if the account type is critical.
func IsCriticalAccountType(accountType string) bool {
	return CriticalAccountTypes[accountType]
}


