// REC-6 SLICE 2: GATEWAY REFUND DISPATCH EXECUTOR.
//
// Business rule (REC-6): if the payment gateway captured money from the buyer
// AFTER the Labuda order became invalid for payment finalization (expired,
// cancelled), the platform must automatically refund the buyer in full.
//
// Slice 1 (closed) proved the INTENT half: all three producers (payment
// webhook, discovery worker, orphan-webhook recovery worker) converge on
// CreateRefundIntentForInvalidOrder (same package), which persists exactly one
// refund row per payment with:
//
//	status         = system_refunded       (decision axis: platform decided)
//	gateway_status = unsubmitted           (settlement axis: not yet dispatched)
//	amount         = payment.gross_amount  (full captured amount)
//	reviewed_by    = NULL                  (no human reviewer; migration 000100)
//	gateway_idempotency_key = rec6:payment:<payment_id>
//
// Slice 2 (this file) implements the DISPATCH half: a single executor reads
// those intents and submits them to the gateway using the existing
// RefundWithKey capability, moving the settlement axis to
// gateway_status='pending' — REQUESTED/SUBMITTED at the gateway.
//
// SLICE 2 IS NOT FINAL CONFIRMATION. 'pending' means the gateway accepted the
// refund request; it does NOT mean the buyer has the money back. Confirmation
// stays with the async webhook path
// (RefundService.HandleGatewayRefundAck → gateway_status='succeeded').
//
// HARD INVARIANTS enforced here:
//   - The decision row is NEVER rewritten by dispatch: status stays
//     system_refunded, reviewed_by stays NULL, amounts stay payment.gross_amount.
//     Only refunds.gateway_* columns may move.
//   - No escrow read/write, no ledger mutation, no seller payable, no order
//     finalization. Slice-2 bookkeeping lives entirely in refunds.gateway_*
//     plus one outbox event per outcome.
//   - Exactly ONE dispatch authority (DispatchPendingRec6Refunds). Re-running
//     it is always safe: terminal rows (succeeded/failed) are skipped, the
//     gateway call carries the deterministic merchant refund key
//     rec6:payment:<payment_id>, and concurrent workers claim disjoint rows
//     via SELECT ... FOR UPDATE SKIP LOCKED inside a short transaction.
//   - The HTTP gateway call happens OUTSIDE any DB transaction or row lock:
//     (1) short tx — claim row; (2) HTTP with no locks; (3) short tx —
//     persist outcome. A crash between (1) and (3) leaves the row durable and
//     retryable; it can never invent a confirmed state.

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// rec6DispatchReason is the canonical reason string sent to the gateway.
const rec6DispatchReason = "order invalid for payment finalization; captured funds must be returned to buyer"

// Rec6DispatchClaim is one refund intent claimed for gateway dispatch.
// Exactly one row per payment exists by construction: the unique partial index
// idx_refunds_gateway_idempotency_key admits only one rec6:payment:<payment_id>
// row, so two workers can never surface distinct intents for one payment.
type Rec6DispatchClaim struct {
	RefundID        uuid.UUID
	OrderID         uuid.UUID
	PaymentID       uuid.UUID // parsed from gateway_idempotency_key (rec6:payment:<uuid>)
	Amount          int64     // payment.gross_amount persisted at intent time
	IdempotencyKey  string
	GatewayAttempts int
}

// DefaultRec6DispatchRecoveryGrace is how old a claimed-but-unresolved intent
// (gateway_status='pending', gateway_requested_at IS NULL) must be before the
// next scan may reclaim it. The grace only spaces out concurrent workers; it
// is NOT a safety mechanism — re-dispatch is always safe because the merchant
// refund key rec6:payment:<payment_id> makes the gateway call idempotent.
const DefaultRec6DispatchRecoveryGrace = 2 * time.Minute

// DispatchPendingRec6Refunds is THE single REC-6 refund dispatch authority.
//
// Per claimed refund it executes: (1) claim in a short tx, (2) RefundWithKey
// HTTP with no locks held, (3) persist the outcome in its own short tx.
//
// Failure semantics: a per-refund dispatch failure never aborts the batch; the
// error is persisted on the refund row, the next invocation retries the SAME
// deterministic key (the gateway deduplicates it), and the returned failure
// count lets monitoring alert on repeated failures.
//
// Idempotency: re-running this method can neither create a second intent,
// double-dispatch a succeeded refund, nor fabricate a confirmed state from a
// successful submission.
func (s *RefundService) DispatchPendingRec6Refunds(ctx context.Context, limit int) (int, error) {
	if s.gatewayClient == nil {
		return 0, ErrGatewayClientNotConfigured
	}
	if s.outboxRepo == nil {
		return 0, fmt.Errorf("rec6 refund dispatch: outbox repository not wired")
	}
	if limit <= 0 {
		limit = 1
	}

	claims, err := s.claimRec6DispatchBatch(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("rec6 refund dispatch: claim batch: %w", err)
	}

	failures := 0
	for _, claim := range claims {
		if derr := s.dispatchOneRec6Refund(ctx, claim); derr != nil {
			failures++
			s.gatewayLog().Error("rec6_refund_dispatch_failed",
				zap.String("refund_id", claim.RefundID.String()),
				zap.String("payment_id", claim.PaymentID.String()),
				zap.String("idempotency_key", claim.IdempotencyKey),
				zap.Error(derr),
			)
		}
	}
	return failures, nil
}

// rec6DispatchRecoveryGrace is the reclaim threshold for crash-orphaned
// claims (see DefaultRec6DispatchRecoveryGrace). Test-overridable.
var rec6DispatchRecoveryGrace = DefaultRec6DispatchRecoveryGrace

// claimRec6DispatchBatch selects and claims dispatchable REC-6 refund intents
// in ONE short transaction. SKIP LOCKED gives concurrent workers disjoint
// batches; every claimed row transitions to gateway_status='pending' and the
// claim commits BEFORE any HTTP call, so the gateway submission happens with
// zero locks held.
//
// Why mark 'pending' at claim time: the refunds_gateway_status_check constraint
// admits only unsubmitted/pending/succeeded/failed, so the in-progress marker
// reuses 'pending'. This is safe and honest: 'pending' is NOT a confirmed
// state (confirmation is exclusively 'succeeded' via webhook, and the entity
// guard MarkGatewayAckSucceeded accepts pending → succeeded), and the outcome
// persistence below always reconciles a claimed row to its real outcome. A
// crash mid-dispatch leaves a durable, retryable intent — never a false
// confirmation.
//
// NOTE: only refunds rows are read here (no join), so the batch transaction is
// a single short statement over the partial index.
func (s *RefundService) claimRec6DispatchBatch(ctx context.Context, limit int) ([]Rec6DispatchClaim, error) {
	type candidate struct {
		id, orderID uuid.UUID
		paymentID   uuid.UUID
		amount      int64
		attempts    int
		key         string
	}

	var claims []Rec6DispatchClaim
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Two selection arms:
		//   A. fresh work — intents never submitted (unsubmitted/failed)
		//   B. crash recovery — claimed ('pending') but never observed by the
		//      gateway (gateway_requested_at IS NULL) and older than the grace:
		//      the previous worker crashed between claim and outcome. Re-sending
		//  the SAME deterministic key is safe either way.
		//
		// Phase 1 — SELECT only. The result set is fully drained and closed
		// BEFORE any write: pgx executes statements on one connection per tx,
		// and issuing the claim UPDATE while the cursor is still open fails
		// with a busy-connection error (and would abort the whole batch).
		rows, err := tx.Query(ctx, `
			SELECT id, order_id, requested_amount, final_refund_amount,
			       gateway_attempts, gateway_idempotency_key
			FROM refunds
			WHERE status = 'system_refunded'
			  AND reason = 'gateway_captured_after_order_invalid'
			  AND gateway_idempotency_key LIKE 'rec6:payment:%'
			  AND (
			        gateway_status IN ('unsubmitted', 'failed')
			        OR (gateway_status = 'pending'
			            AND gateway_requested_at IS NULL
			            AND updated_at < $2)
			      )
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		`, limit, time.Now().Add(-rec6DispatchRecoveryGrace))
		if err != nil {
			return fmt.Errorf("select dispatchable rec6 refunds: %w", err)
		}

		var candidates []candidate
		err = func() error {
			defer rows.Close()
			for rows.Next() {
				var c candidate
				var requestedAmt, finalAmt *int64
				if err := rows.Scan(&c.id, &c.orderID, &requestedAmt, &finalAmt, &c.attempts, &c.key); err != nil {
					return fmt.Errorf("scan dispatchable rec6 refund: %w", err)
				}

				// The payment identity is embedded in the deterministic key.
				paymentID, perr := uuid.Parse(strings.TrimPrefix(c.key, "rec6:payment:"))
				if perr != nil {
					return fmt.Errorf("rec6 refund dispatch: refund %s carries malformed key %q", c.id, c.key)
				}
				// Defense-in-depth: never dispatch with a non-canonical key.
				if c.key != Rec6IdempotencyKey(paymentID) {
					return fmt.Errorf(
						"rec6 refund dispatch: refund %s carries non-canonical key %q (expected %q)",
						c.id, c.key, Rec6IdempotencyKey(paymentID))
				}
				c.paymentID = paymentID

				// Amount authority: requested_amount == payment.gross_amount is set
				// at intent time; final_refund_amount mirrors it on this row shape.
				amount := int64(0)
				if requestedAmt != nil {
					amount = *requestedAmt
				}
				if finalAmt != nil && *finalAmt > 0 {
					amount = *finalAmt
				}
				if amount <= 0 {
					return fmt.Errorf("rec6 refund dispatch: refund %s has non-positive amount %d", c.id, amount)
				}
				c.amount = amount

				candidates = append(candidates, c)
			}
			return rows.Err()
		}()
		if err != nil {
			return err
		}

		// Phase 2 — claim each candidate: mark submission in progress and bump
		// attempts. Committed with the tx; the HTTP call below holds no locks.
		now := time.Now()
		for _, c := range candidates {
			if _, err := tx.Exec(ctx, `
				UPDATE refunds
				SET gateway_attempts = gateway_attempts + 1,
				    gateway_status = 'pending',
				    updated_at = $2
				WHERE id = $1
			`, c.id, now); err != nil {
				return fmt.Errorf("claim rec6 refund %s: %w", c.id, err)
			}

			claims = append(claims, Rec6DispatchClaim{
				RefundID:        c.id,
				OrderID:         c.orderID,
				PaymentID:       c.paymentID,
				Amount:          c.amount,
				IdempotencyKey:  c.key,
				GatewayAttempts: c.attempts,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// dispatchOneRec6Refund drives one claimed intent:
//
//	(2) resolve payments.midtrans_order_id in its own short tx (indexed PK
//	    lookup, no refund lock held),
//	(3) RefundWithKey HTTP call with the merchant refund key — NO tx, NO locks,
//	(4) persist the outcome in its own short tx.
//
// Any step-2 error is persisted as a retryable dispatch failure instead of
// aborting the batch.
func (s *RefundService) dispatchOneRec6Refund(ctx context.Context, claim Rec6DispatchClaim) error {
	midtransOrderID, err := s.resolveRec6MidtransOrderID(ctx, claim.PaymentID)
	if err != nil {
		return s.persistRec6DispatchFailure(ctx, claim,
			fmt.Sprintf("resolve midtrans_order_id: %v", err), time.Now())
	}

	// HTTP gateway submission — deliberately OUTSIDE every DB transaction.
	resp, herr := s.gatewayClient.RefundWithKey(
		ctx, midtransOrderID, claim.IdempotencyKey, claim.Amount, rec6DispatchReason,
	)
	now := time.Now()
	if herr != nil {
		return s.persistRec6DispatchFailure(ctx, claim, herr.Error(), now)
	}
	if perr := s.persistRec6DispatchSuccess(ctx, claim, resp, now); perr != nil {
		// The gateway may hold the refund while we failed to persist it.
		// Compensate: mark failed so the next scan retries the same key
		// instead of trusting a stale claim marker.
		_ = s.persistRec6DispatchFailure(ctx, claim,
			fmt.Sprintf("persist dispatch success: %v", perr), time.Now())
		return perr
	}
	return nil
}

// resolveRec6MidtransOrderID reads payments.midtrans_order_id in a short tx.
// The refund row was already claimed; this lookup holds no refund lock.
func (s *RefundService) resolveRec6MidtransOrderID(ctx context.Context, paymentID uuid.UUID) (string, error) {
	var midtransOrderID string
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT midtrans_order_id FROM payments WHERE id = $1`, paymentID,
		).Scan(&midtransOrderID)
	})
	if err != nil {
		return "", fmt.Errorf("lookup payment %s: %w", paymentID, err)
	}
	if strings.TrimSpace(midtransOrderID) == "" {
		return "", fmt.Errorf("payment %s has empty midtrans_order_id", paymentID)
	}
	return midtransOrderID, nil
}

// persistRec6DispatchSuccess records the REQUESTED/SUBMITTED state:
// gateway_status='pending' + gateway_requested_at + gateway_refund_id.
//
// The WHERE guard (gateway_status='pending') means only the worker that
// claimed the row can finalize it; a concurrent actor that moved the row (a
// webhook ack that already confirmed 'succeeded') is respected — never
// overwritten. Idempotent re-submissions keep the first requested_at.
//
// Touches ONLY refunds.gateway_* columns: no decision-axis rewrite (status
// stays system_refunded, reviewed_by stays NULL, amounts stay gross), no
// escrow, no ledger, no order, no seller payable.
func (s *RefundService) persistRec6DispatchSuccess(
	ctx context.Context, claim Rec6DispatchClaim, resp *midtrans.RefundResponse, now time.Time,
) error {
	gatewayRefundID := ""
	if resp != nil {
		if resp.RefundChargeID != "" {
			gatewayRefundID = resp.RefundChargeID
		} else if resp.TransactionID != "" {
			gatewayRefundID = resp.TransactionID
		}
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE refunds
			SET gateway_status = 'pending',
			    gateway_refund_id = COALESCE(NULLIF($2, ''), gateway_refund_id),
			    gateway_requested_at = COALESCE(gateway_requested_at, $3),
			    last_gateway_error = NULL,
			    updated_at = $3
			WHERE id = $1
			  AND gateway_status = 'pending'
		`, claim.RefundID, gatewayRefundID, now)
		if err != nil {
			return fmt.Errorf("persist rec6 dispatch success: %w", err)
		}
		if tag.RowsAffected() == 0 {
			// Claimed row moved on (terminal) or vanished between claim and
			// outcome. Never overwrite that state with a stale one.
			s.gatewayLog().Warn("rec6_dispatch_result_skipped_state_moved",
				zap.String("refund_id", claim.RefundID.String()))
			return nil
		}
		return s.emitRec6DispatchOutbox(ctx, tx, claim, "money.refund_requested", nil)
	})
}

// persistRec6DispatchFailure records gateway_status='failed' plus the error.
// A failed dispatch is retryable: the next scan re-claims the row and re-sends
// the SAME deterministic key, so the gateway deduplicates instead of refunding
// twice. Never touches the decision axis.
func (s *RefundService) persistRec6DispatchFailure(
	ctx context.Context, claim Rec6DispatchClaim, errMsg string, now time.Time,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE refunds
			SET gateway_status = 'failed',
			    last_gateway_error = $2,
			    updated_at = $3
			WHERE id = $1
			  AND gateway_status = 'pending'
		`, claim.RefundID, errMsg, now)
		if err != nil {
			return fmt.Errorf("persist rec6 dispatch failure: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil // state moved on; never regress it
		}
		return s.emitRec6DispatchOutbox(ctx, tx, claim, "money.refund_failed", &errMsg)
	})
}

// emitRec6DispatchOutbox emits the observability event for a dispatch outcome.
// The outbox row is written in the SAME tx as the state change so the audit
// trail and the persisted status can never disagree.
func (s *RefundService) emitRec6DispatchOutbox(
	ctx context.Context, tx db.Tx, claim Rec6DispatchClaim, eventType string, errMsg *string,
) error {
	payload := map[string]interface{}{
		"refund_id":               claim.RefundID,
		"order_id":                claim.OrderID,
		"payment_id":              claim.PaymentID,
		"amount":                  claim.Amount,
		"gateway_idempotency_key": claim.IdempotencyKey,
		"source":                  "rec6_dispatch",
	}
	if errMsg != nil {
		payload["error"] = *errMsg
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("rec6 dispatch outbox marshal: %w", err)
	}
	return s.outboxRepo.InsertEvent(ctx, tx, eventType, claim.RefundID, bytes)
}
