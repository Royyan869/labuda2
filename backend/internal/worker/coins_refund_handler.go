package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsRepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// CoinsRefundRequiredHandler handles coins.refund_required events.
//
// CRITICAL PRINCIPLE: This is the SINGLE ENTRY POINT for all coins refunds.
// All refund logic MUST flow through this handler to ensure:
// 1. Idempotency (no double refunds)
// 2. Transaction-based refunds (not order.snapshot based)
// 3. Traceability (all refunds logged)
// 4. Failure recovery (coins.refund_failed event on error)
//
// This replaces direct calls to RefundCoinsInternal() across the codebase.
type CoinsRefundRequiredHandler struct {
	db           *dbpkg.DB
	coinsService *coinsApp.CoinsService
	log          *zap.Logger
	// poisonedOrderIDs is a corpus-generation knob: when non-empty, Handle
	// returns an error for any event whose order_id is in the set. The
	// returned error feeds the outbox worker's canonical retry / dead-letter
	// machinery — there is NO direct UPDATE of outbox.status. Populated from
	// COINS_REFUND_POISON_ORDER_IDS at handler construction; empty in
	// production.
	poisonedOrderIDs map[uuid.UUID]struct{}
}

// NewCoinsRefundRequiredHandler creates a new coins refund handler.
//
// The handler reads COINS_REFUND_POISON_ORDER_IDS once at construction. The
// value is a comma-separated list of order UUIDs; matching events fail with
// a deterministic error so the outbox worker can exhaust retries and reach
// dead_letter through its canonical path. Used by Phase 1B corpus
// generation to produce D9 drift signal; default empty disables the path.
func NewCoinsRefundRequiredHandler(db *dbpkg.DB, coinsService *coinsApp.CoinsService, log *zap.Logger) *CoinsRefundRequiredHandler {
	if log == nil {
		log = zap.NewNop()
	}
	poisoned := loadCoinsRefundPoisonList(log)
	return &CoinsRefundRequiredHandler{
		db:               db,
		coinsService:     coinsService,
		log:              log,
		poisonedOrderIDs: poisoned,
	}
}

// loadCoinsRefundPoisonList parses COINS_REFUND_POISON_ORDER_IDS. Invalid
// UUIDs are skipped with a structured warn log; the variable is meant for
// dev/test runs so silent skipping of malformed entries is safer than a
// fatal startup error.
func loadCoinsRefundPoisonList(log *zap.Logger) map[uuid.UUID]struct{} {
	raw := strings.TrimSpace(os.Getenv("COINS_REFUND_POISON_ORDER_IDS"))
	if raw == "" {
		return nil
	}
	out := make(map[uuid.UUID]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			log.Warn("coins_refund_poison_invalid_uuid",
				zap.String("value", part),
				zap.Error(err),
			)
			continue
		}
		out[id] = struct{}{}
	}
	if len(out) > 0 {
		ids := make([]string, 0, len(out))
		for id := range out {
			ids = append(ids, id.String())
		}
		log.Warn("coins_refund_poison_list_loaded",
			zap.Int("count", len(ids)),
			zap.Strings("order_ids", ids),
		)
	}
	return out
}

// CoinsRefundRequiredEvent represents the payload of a coins.refund_required event.
//
// CRITICAL: Amount is NOT included - it MUST be resolved from coins_transactions
// to ensure refunds are based on actual spend, not order snapshot.
//
// REFUND ECONOMICS REBASE: when CoinDelta is present (gateway refund ack
// path), the exact proportional restoration floor(K * cumProductRefund / PD)
// has already been computed by the refund pipeline. The handler restores
// EXACTLY that amount and each event creates its own refund_earn row keyed
// by the outbox event ID, so multiple partial restorations per order are
// each recorded exactly once. When coin_delta is ABSENT (legacy emitters),
// the handler falls back to full-K restoration from the spend transaction.
type CoinsRefundRequiredEvent struct {
	OrderID   uuid.UUID `json:"order_id"`
	UserID    uuid.UUID `json:"user_id"`
	Reason    string    `json:"reason"`    // "payment_failed" | "payment_expired" | "order_expired" | "order_cancelled" | "money_refunded" | "money_partially_refunded" | "money_refund_succeeded"
	Source    string    `json:"source"`    // "payment_webhook" | "order_expire_worker" | "finance" | "gateway_refund_ack"
	CoinDelta int64     `json:"coin_delta"` // Proportional coins to restore (ack-computed); absent/0 = none in the delta path
}

// Handle processes the coins.refund_required event.
//
// IDEMPOTENCY GUARANTEE:
// - Pre-check: isAlreadyRefunded() fast path via FindEarnByReference
// - Hard guard: idx_coins_transactions_unique_reference UNIQUE index on
//   (user_id, reference_type, reference_id) prevents any double-insert at the DB level
// - Safe to retry: duplicate detected = idempotent success, no error returned
//
// TRANSACTION-BASED REFUND:
// - Finds original spend transaction from coins_transactions
// - Refunds ONLY if spend exists and not already refunded
// - Does NOT use order.coins_used snapshot (can be stale / not persisted to orders)
func (h *CoinsRefundRequiredHandler) Handle(ctx context.Context, event platformevent.OutboxEvent) error {
	// Parse event payload
	var refundEvent CoinsRefundRequiredEvent
	if err := json.Unmarshal(event.Payload, &refundEvent); err != nil {
		h.log.Error("Failed to parse coins.refund_required event payload",
			zap.String("event_id", event.ID.String()),
			zap.Error(err),
		)
		// Don't retry on parsing errors - payload is invalid
		return nil
	}

	// Corpus-generation poison gate. The error returned here is treated as
	// any other handler failure by the outbox worker: it increments
	// retry_count and reschedules. After MaxAttempts the canonical path in
	// OutboxWorker.handleFailureInTx moves the row to dead_letter — that is
	// the runtime-truth path the D9 detector validates against.
	if _, poisoned := h.poisonedOrderIDs[refundEvent.OrderID]; poisoned {
		h.log.Warn("coins_refund_poison_applied",
			zap.String("event_id", event.ID.String()),
			zap.String("order_id", refundEvent.OrderID.String()),
			zap.String("event_type", event.EventType),
		)
		return fmt.Errorf("coins_refund_poison: deterministic failure for order=%s", refundEvent.OrderID)
	}

	h.log.Info("Processing coins.refund_required event",
		zap.String("event_id", event.ID.String()),
		zap.String("order_id", refundEvent.OrderID.String()),
		zap.String("user_id", refundEvent.UserID.String()),
		zap.String("reason", refundEvent.Reason),
		zap.String("source", refundEvent.Source),
	)

	// coin_delta presence distinguishes the gateway-ack path (delta is
	// authoritative; zero means nothing to restore) from the legacy emitters
	// (no delta → full-K restoration from the spend transaction).
	hasCoinDelta := jsonPayloadHasKey(event.Payload, "coin_delta")

	// Process refund in a new transaction
	// This ensures the refund is atomic and retryable
	err := h.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return h.processRefund(ctx, tx, refundEvent, event.ID, hasCoinDelta)
	})

	if err != nil {
		h.log.Error("Failed to process coins refund",
			zap.String("event_id", event.ID.String()),
			zap.String("order_id", refundEvent.OrderID.String()),
			zap.String("user_id", refundEvent.UserID.String()),
			zap.Error(err),
		)
		// Return error to trigger retry
		return fmt.Errorf("coins refund failed: %w", err)
	}

	return nil
}

// processRefund executes the refund logic within a transaction.
//
// eventID is the outbox event ID: it uniquely identifies each emission and
// becomes the refund_earn reference for delta restorations, so multiple
// partial restorations per order each create their own earn row while a
// replay of the same event stays idempotent.
func (h *CoinsRefundRequiredHandler) processRefund(
	ctx context.Context,
	tx dbpkg.Tx,
	event CoinsRefundRequiredEvent,
	eventID uuid.UUID,
	hasCoinDelta bool,
) error {
	// ====================================================================
	// PATH A (NEW): GATEWAY ACK COMPUTED THE PROPORTIONAL DELTA
	// ====================================================================
	// The gateway refund ack pipeline computes the exact proportional coin
	// restoration floor(K * cumProductRefund / PD) and emits it as
	// coin_delta. Restore EXACTLY that amount; a zero delta means there are
	// no coins to restore for this event (no-op). Each event creates its
	// own refund_earn transaction keyed by the outbox event ID, so the
	// UNIQUE (user_id, reference_type, reference_id) constraint still
	// guarantees idempotency per emission.
	if hasCoinDelta {
		if event.CoinDelta <= 0 {
			h.log.Info("Coins refund delta is zero - nothing to restore",
				zap.String("event_id", eventID.String()),
				zap.String("order_id", event.OrderID.String()),
				zap.String("user_id", event.UserID.String()),
			)
			_ = h.markCoinsRefundedAt(ctx, tx, event.OrderID)
			return nil // Zero delta = no coins to restore for this event
		}

		if err := h.coinsService.RefundCoinsWithDelta(ctx, tx, event.UserID, eventID, event.CoinDelta); err != nil {
			if coinsRepo.IsDuplicateTransaction(err) {
				h.log.Info("Coins delta refund already exists (duplicate constraint), treating as success",
					zap.String("event_id", eventID.String()),
					zap.String("order_id", event.OrderID.String()),
					zap.String("user_id", event.UserID.String()),
				)
				return nil // Idempotent - treat as success
			}
			return fmt.Errorf("failed to execute delta coin refund: %w", err)
		}

		h.log.Info("Coins refund processed (ack delta)",
			zap.String("event_id", eventID.String()),
			zap.String("order_id", event.OrderID.String()),
			zap.String("user_id", event.UserID.String()),
			zap.Int64("amount_refunded", event.CoinDelta),
			zap.String("reason", event.Reason),
			zap.String("source", event.Source),
		)

		if err := h.markCoinsRefundedAt(ctx, tx, event.OrderID); err != nil {
			h.log.Warn("Failed to mark coins_refunded_at (non-critical)",
				zap.String("order_id", event.OrderID.String()),
				zap.Error(err),
			)
		}
		return nil
	}

	// ====================================================================
	// PATH B (LEGACY): FULL-K RESTORATION FROM SPEND TRANSACTION
	// ====================================================================
	// Legacy emitters (order expire / overdue cancel / money refunded) do
	// not carry coin_delta: resolve the spend from coins_transactions and
	// restore the full amount spent, as before.
	// ====================================================================
	// CRITICAL: PARTIAL REFUND HANDLING
	// ====================================================================
	// For partial refunds, we do NOT refund coins.
	// Partial refunds are edge cases (e.g., dispute resolution) where
	// only a portion of money is returned - coins should stay with the buyer.
	// Only full refunds should return coins.
	if event.Reason == "money_partially_refunded" {
		h.log.Info("Partial refund detected - coins NOT refunded (edge case)",
			zap.String("order_id", event.OrderID.String()),
			zap.String("user_id", event.UserID.String()),
			zap.String("reason", event.Reason),
		)
		// Update order.coins_refunded_at to indicate we processed this
		// (even though we didn't actually refund coins)
		if err := h.markCoinsRefundedAt(ctx, tx, event.OrderID); err != nil {
			h.log.Warn("Failed to mark coins_refunded_at for partial refund",
				zap.String("order_id", event.OrderID.String()),
				zap.Error(err),
			)
			// Non-critical - continue
		}
		return nil // Skip - partial refund, no coins refunded
	}

	// ====================================================================
	// STEP 1: FIND ORIGINAL SPEND TRANSACTION (SINGLE SOURCE OF TRUTH)
	// ====================================================================
	// We ONLY trust coins_transactions, NOT order.coins_used snapshot.
	// This ensures refunds are based on actual coins spent.
	//
	// If no spend transaction exists, nothing to refund (exit gracefully).
	spendTx, err := h.findSpendTransaction(ctx, tx, event.UserID, event.OrderID)
	if err != nil {
		h.log.Error("Failed to find spend transaction",
			zap.String("order_id", event.OrderID.String()),
			zap.String("user_id", event.UserID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to find spend transaction: %w", err)
	}

	if spendTx == nil {
		// No spend transaction found - no coins were used
		h.log.Info("No spend transaction found, skipping coins refund",
			zap.String("order_id", event.OrderID.String()),
			zap.String("user_id", event.UserID.String()),
		)
		return nil // Skip - no error, nothing to refund
	}

	// ====================================================================
	// STEP 2: CHECK IF ALREADY REFUNDED (IDEMPOTENCY)
	// ====================================================================
	// Look for existing refund_earn transaction for this order.
	// If found, refund already processed - skip (idempotent).
	alreadyRefunded, err := h.isAlreadyRefunded(ctx, tx, event.UserID, event.OrderID)
	if err != nil {
		return fmt.Errorf("failed to check refund status: %w", err)
	}

	if alreadyRefunded {
		h.log.Info("Coins already refunded, skipping (idempotent)",
			zap.String("order_id", event.OrderID.String()),
			zap.String("user_id", event.UserID.String()),
		)
		return nil // Skip - already refunded
	}

	// ====================================================================
	// STEP 3: EXECUTE REFUND (ATOMIC)
	// ====================================================================
	// Use RefundCoinsInternal which uses INSERT-FIRST pattern.
	// Database UNIQUE constraint ensures idempotency.
	amountToRefund := spendTx.Amount

	if err := h.coinsService.RefundCoinsInternal(ctx, tx, event.UserID, event.OrderID); err != nil {
		// Check if duplicate (idempotent success)
		if coinsRepo.IsDuplicateTransaction(err) {
			h.log.Info("Refund already exists (duplicate constraint), treating as success",
				zap.String("order_id", event.OrderID.String()),
				zap.String("user_id", event.UserID.String()),
			)
			return nil // Idempotent - treat as success
		}

		return fmt.Errorf("failed to execute refund: %w", err)
	}

	h.log.Info("Coins refund processed successfully",
		zap.String("order_id", event.OrderID.String()),
		zap.String("user_id", event.UserID.String()),
		zap.Int64("amount_refunded", amountToRefund),
		zap.String("reason", event.Reason),
		zap.String("source", event.Source),
	)

	// ====================================================================
	// STEP 4: UPDATE OBSERVABILITY (coins_refunded_at)
	// ====================================================================
	// Mark the order with coins_refunded_at timestamp for observability.
	// This allows ops to track when coins were refunded for each order.
	if err := h.markCoinsRefundedAt(ctx, tx, event.OrderID); err != nil {
		h.log.Warn("Failed to mark coins_refunded_at (non-critical)",
			zap.String("order_id", event.OrderID.String()),
			zap.Error(err),
		)
		// Non-critical - refund already succeeded, don't fail on this
	}

	return nil
}

// jsonPayloadHasKey reports whether the JSON payload contains the given key.
// Used to distinguish new-format events (coin_delta present, even when zero)
// from legacy-format events that predate the delta path.
func jsonPayloadHasKey(payload []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// findSpendTransaction finds the original spend transaction for an order.
// Returns nil if no spend transaction exists (not an error).
func (h *CoinsRefundRequiredHandler) findSpendTransaction(
	ctx context.Context,
	tx dbpkg.Tx,
	userID uuid.UUID,
	orderID uuid.UUID,
) (*SpendTransaction, error) {
	// Query coins_transactions for the spend transaction
	// reference_type = 'order_spend' and reference_id = order_id
	query := `
		SELECT id, user_id, type, amount, reference_type, reference_id, created_at
		FROM coins_transactions
		WHERE user_id = $1
		  AND reference_type = 'order_spend'
		  AND reference_id = $2
		  AND type = 'spend'
		LIMIT 1
	`

	var txRecord SpendTransaction
	err := tx.QueryRow(ctx, query, userID, orderID).Scan(
		&txRecord.ID,
		&txRecord.UserID,
		&txRecord.Type,
		&txRecord.Amount,
		&txRecord.ReferenceType,
		&txRecord.ReferenceID,
		&txRecord.CreatedAt,
	)

	if err != nil {
		// No rows = no spend transaction (return nil, not error)
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	return &txRecord, nil
}

// isAlreadyRefunded checks if a refund transaction already exists.
// Returns true if refund_earn transaction exists for this order.
func (h *CoinsRefundRequiredHandler) isAlreadyRefunded(
	ctx context.Context,
	tx dbpkg.Tx,
	userID uuid.UUID,
	orderID uuid.UUID,
) (bool, error) {
	// Query coins_transactions for refund_earn transaction
	query := `
		SELECT 1
		FROM coins_transactions
		WHERE user_id = $1
		  AND reference_type = 'refund_earn'
		  AND reference_id = $2
		  AND type = 'earn'
		LIMIT 1
	`

	var exists int
	err := tx.QueryRow(ctx, query, userID, orderID).Scan(&exists)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil // No refund exists
		}
		return false, err
	}

	return exists == 1, nil
}

// SpendTransaction represents a coins spend transaction.
type SpendTransaction struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Type          string
	Amount        int64
	ReferenceType string
	ReferenceID   uuid.UUID
	CreatedAt     string
}

// markCoinsRefundedAt sets the coins_refunded_at timestamp on an order.
// This provides observability for when coins were refunded.
// If the column doesn't exist (migration not run), this is a non-critical error.
func (h *CoinsRefundRequiredHandler) markCoinsRefundedAt(
	ctx context.Context,
	tx dbpkg.Tx,
	orderID uuid.UUID,
) error {
	// Try to update coins_refunded_at - if column doesn't exist, fail gracefully
	query := `UPDATE orders SET coins_refunded_at = NOW() WHERE id = $1`
	_, err := tx.Exec(ctx, query, orderID)
	return err
}


