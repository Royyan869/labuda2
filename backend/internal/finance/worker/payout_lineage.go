package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PayoutLineage is the canonical trace of a single payout operation.
//
// Lineage chain: WithdrawalID → ExternalReferenceID → GatewayReferenceID
//                → LedgerReferenceIDs → ReconciliationNote
//
// This struct is the single operator-readable trace path. It is read-only
// and safe to expose via admin endpoints or structured log.
type PayoutLineage struct {
	WithdrawalID        uuid.UUID      `json:"withdrawal_id"`
	SellerID            uuid.UUID      `json:"seller_id"`
	AmountCents         int64          `json:"amount_cents"`
	Status              string         `json:"status"`
	ExternalReferenceID string         `json:"external_reference_id"`
	GatewayReferenceID  string         `json:"gateway_reference_id"`
	LedgerReferenceIDs  []string       `json:"ledger_reference_ids"`
	Timeline            []LineageEvent `json:"timeline"`
	ReconciliationNote  string         `json:"reconciliation_note,omitempty"`
	TracedAt            time.Time      `json:"traced_at"`
}

// LineageEvent is a timestamped event in the payout lifecycle.
type LineageEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Details   string    `json:"details,omitempty"`
}

// PayoutLineageTracer builds read-only lineage traces for payouts.
// Safe for concurrent use.
type PayoutLineageTracer struct {
	withdrawRepo *repository.WithdrawRepository
	db           Transactor
	log          *zap.Logger
}

// NewPayoutLineageTracer creates a new tracer.
func NewPayoutLineageTracer(
	withdrawRepo *repository.WithdrawRepository,
	db Transactor,
	log *zap.Logger,
) *PayoutLineageTracer {
	if log == nil {
		log = zap.NewNop()
	}
	return &PayoutLineageTracer{
		withdrawRepo: withdrawRepo,
		db:           db,
		log:          log,
	}
}

// Trace builds a complete lineage for the given withdrawal_id.
// This is a read-only operation — it never modifies state.
func (t *PayoutLineageTracer) Trace(ctx context.Context, withdrawalID uuid.UUID) (*PayoutLineage, error) {
	var w *repository.Withdrawal

	if err := t.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		w, err = t.withdrawRepo.GetByID(ctx, tx, withdrawalID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("lineage: get withdrawal %s: %w", withdrawalID, err)
	}

	gatewayRefID := extractGatewayRefID(w.GatewayResponse)
	timeline := t.buildTimeline(w)

	ledgerRefs, err := t.queryLedgerRefs(ctx, withdrawalID)
	if err != nil {
		t.log.Warn("Lineage: ledger reference query failed",
			zap.String("withdrawal_id", withdrawalID.String()),
			zap.Error(err),
		)
		ledgerRefs = []string{"query_error: " + err.Error()}
	}

	lineage := &PayoutLineage{
		WithdrawalID:        w.ID,
		SellerID:            w.SellerID,
		AmountCents:         w.Amount,
		Status:              string(w.Status),
		ExternalReferenceID: w.ExternalReferenceID,
		GatewayReferenceID:  gatewayRefID,
		LedgerReferenceIDs:  ledgerRefs,
		Timeline:            timeline,
		ReconciliationNote:  buildReconcNote(w),
		TracedAt:            time.Now(),
	}

	t.log.Info("PAYOUT_LINEAGE_TRACED",
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.String("external_ref", w.ExternalReferenceID),
		zap.String("gateway_ref", gatewayRefID),
		zap.Int("ledger_refs_count", len(ledgerRefs)),
		zap.String("status", string(w.Status)),
	)

	return lineage, nil
}

// LogLineage emits the full lineage object as a single structured log entry.
func (t *PayoutLineageTracer) LogLineage(lineage *PayoutLineage) {
	data, _ := json.Marshal(lineage)
	t.log.Info("PAYOUT_LINEAGE_FULL",
		zap.String("withdrawal_id", lineage.WithdrawalID.String()),
		zap.String("lineage_json", string(data)),
	)
}

// buildTimeline constructs the lifecycle event list from withdrawal timestamps.
func (t *PayoutLineageTracer) buildTimeline(w *repository.Withdrawal) []LineageEvent {
	var events []LineageEvent

	if w.CreatedAt > 0 {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.CreatedAt, 0),
			EventType: "WITHDRAWAL_REQUESTED",
		})
	}
	if w.SubmittedAt > 0 {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.SubmittedAt, 0),
			EventType: "SUBMITTED_TO_GATEWAY",
			Details:   "external_ref=" + w.ExternalReferenceID,
		})
	}
	if w.SettledAt > 0 {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.SettledAt, 0),
			EventType: "SETTLED",
		})
	}
	if w.Status == repository.WithdrawalStatusPilotBlocked {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.UpdatedAt, 0),
			EventType: "PILOT_BLOCKED",
			Details:   "seller_id=" + w.SellerID.String(),
		})
	}
	if w.Status == repository.WithdrawalStatusFailedFinal || w.Status == repository.WithdrawalStatusFailed {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.UpdatedAt, 0),
			EventType: "FAILED",
			Details:   w.FailureReason,
		})
	}
	if w.RetryCount > 0 {
		events = append(events, LineageEvent{
			Timestamp: time.Unix(w.UpdatedAt, 0),
			EventType: "RETRY_ATTEMPTED",
			Details:   fmt.Sprintf("retry_count=%d", w.RetryCount),
		})
	}
	return events
}

// queryLedgerRefs looks up ledger transaction IDs that reference this withdrawal.
func (t *PayoutLineageTracer) queryLedgerRefs(ctx context.Context, withdrawalID uuid.UUID) ([]string, error) {
	var refs []string
	err := t.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT lt.id::text, lt.reference_type
			FROM ledger_transactions lt
			WHERE lt.reference_id = $1
			  AND lt.reference_type IN ('payout_requested', 'payout_completed', 'payout_failed')
			ORDER BY lt.created_at ASC
		`, withdrawalID)
		if err != nil {
			return fmt.Errorf("query ledger_transactions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, refType string
			if err := rows.Scan(&id, &refType); err != nil {
				return fmt.Errorf("scan ledger row: %w", err)
			}
			refs = append(refs, fmt.Sprintf("ledger_tx:%s[%s]", id, refType))
		}
		return rows.Err()
	})
	return refs, err
}

// buildReconcNote returns an operator-readable note for the current status.
func buildReconcNote(w *repository.Withdrawal) string {
	switch w.Status {
	case repository.WithdrawalStatusSettled, repository.WithdrawalStatusCompleted:
		return "Payout completed. No action required."
	case repository.WithdrawalStatusPilotBlocked:
		return "Blocked by pilot whitelist. Add seller to PAYOUT_PILOT_WHITELIST or disable pilot mode."
	case repository.WithdrawalStatusFailedFinal:
		if w.FailureReason != "" {
			return "Permanent failure: " + w.FailureReason + ". Manual review required."
		}
		return "Permanent failure. Manual review required."
	case repository.WithdrawalStatusFailedRetryable:
		return fmt.Sprintf("Retryable failure (attempt %d of 5). Worker will retry automatically.", w.RetryCount)
	case repository.WithdrawalStatusSubmitted:
		return "Awaiting gateway confirmation. Check gateway_reference_id status."
	case repository.WithdrawalStatusSettling:
		return "Settling. Awaiting final bank confirmation."
	case repository.WithdrawalStatusProcessing:
		return "In worker queue. Next poll will submit."
	default:
		return "Status: " + string(w.Status)
	}
}


