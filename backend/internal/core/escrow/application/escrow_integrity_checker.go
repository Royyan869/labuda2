// ⚠️ RECONCILIATION LAYER:
// This module detects escrow inconsistencies.
// It does NOT modify business data - detection and alerting only.
//
// CANONICAL ESCROW MODEL (Owner-locked):
//
//	unpaid order         => NO escrow row
//	settled payment      => EXACTLY ONE escrow row (created in the settlement tx)
//	escrows              => SOLE authority for escrow existence/amount/state
//
// The old invariant compared sum(orders.escrow_status='holding' projection)
// against sum(escrows.amount) — two representations of the same state. Orders
// no longer persist an escrow projection; every check below consumes the
// canonical escrows table joined with payments/orders.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	alertapp "github.com/hishumi/backend/internal/platform/alert/application"
	alertentity "github.com/hishumi/backend/internal/platform/alert/entity"
	"github.com/hishumi/backend/pkg/db"
	"go.uber.org/zap"
)

// EscrowToleranceAmount is the acceptable rounding difference in rupiah units
// for escrow amount integrity checks. Differences within this tolerance are
// NOT flagged.
const EscrowToleranceAmount int64 = 100

// EscrowIntegrityChecker verifies the canonical escrow invariants against the
// escrows table (sole authority), payments (settlement authority), and orders
// (lifecycle + canonical amount base).
//
// Checks:
//  1. SETTLEMENT CONSISTENCY — every order with a settled payment
//     (settlement/capture) has exactly one escrow row.
//  2. UNPAID PURITY — no escrow row exists for an order without a settled
//     payment, or whose order status is pre-payment/terminal-unpaid
//     (pending_payment, expired, cancelled, cancelled_timeout).
//  3. AMOUNT INTEGRITY — escrow.amount equals orders.total_before_coins_amount
//     (canonical buyer-funded base PD+S) within tolerance.
//  4. UNIQUENESS — at most one escrow row per order (defense-in-depth over
//     the DB UNIQUE(order_id) constraint).
//  5. STATE INTEGRITY — a holding escrow never sits on a terminal order
//     (completed, refunded, cancelled, cancelled_timeout, expired).
//
// FINANCIAL SAFETY LAYER:
//   - NO AUTO-FIX - detection and alerting only
//   - Shadow mode logs findings without creating alerts
type EscrowIntegrityChecker struct {
	alertService *alertapp.AlertService
	db           db.Transactor
	log          *zap.Logger
	shadowMode   bool
}

// NewEscrowIntegrityChecker creates a new escrow integrity checker.
// When shadowMode is true, the checker logs findings but does NOT create alerts.
func NewEscrowIntegrityChecker(
	alertService *alertapp.AlertService,
	database db.Transactor,
	log *zap.Logger,
	shadowMode bool,
) *EscrowIntegrityChecker {
	if log == nil {
		log = zap.NewNop()
	}

	return &EscrowIntegrityChecker{
		alertService: alertService,
		db:           database,
		log:          log,
		shadowMode:   shadowMode,
	}
}

// CheckEscrowIntegrity validates all canonical escrow invariants.
// Returns number of mismatches found.
func (c *EscrowIntegrityChecker) CheckEscrowIntegrity(ctx context.Context) (int, error) {
	c.log.Debug("Starting escrow integrity check", zap.Bool("shadow_mode", c.shadowMode))

	if c.db == nil {
		return 0, fmt.Errorf("db transactor not configured")
	}

	var totalMismatches int

	err := c.db.WithTx(ctx, func(tx db.Tx) error {
		missing, err := c.checkSettledOrdersHaveEscrow(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed settled-escrow check: %w", err)
		}
		totalMismatches += missing

		phantom, err := c.checkNoEscrowForUnpaidOrders(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed unpaid-purity check: %w", err)
		}
		totalMismatches += phantom

		amount, err := c.checkEscrowAmountIntegrity(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed amount-integrity check: %w", err)
		}
		totalMismatches += amount

		unique, err := c.checkEscrowUniqueness(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed uniqueness check: %w", err)
		}
		totalMismatches += unique

		state, err := c.checkHoldingOnTerminalOrders(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed state-integrity check: %w", err)
		}
		totalMismatches += state

		// Observability only — the canonical total of held funds.
		if total, terr := c.getTotalHoldingEscrow(ctx, tx); terr == nil {
			c.log.Info("canonical_holding_escrow_total", zap.Int64("total_holding_escrow", total))
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	c.log.Info("Escrow integrity check completed",
		zap.Int("total_mismatches", totalMismatches),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	return totalMismatches, nil
}

// checkSettledOrdersHaveEscrow: every order with a settled payment must have
// exactly one escrow row (the settlement transaction creates it atomically).
func (c *EscrowIntegrityChecker) checkSettledOrdersHaveEscrow(ctx context.Context, tx db.Tx) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT p.reference_id, o.buyer_id, o.seller_id, o.total_before_coins_amount
		FROM payments p
		JOIN orders o ON o.id = p.reference_id
		LEFT JOIN escrows e ON e.order_id = p.reference_id
		WHERE p.reference_type = 'order'
		  AND p.status IN ('settlement', 'capture')
		  AND e.id IS NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var orderID, buyerID, sellerID uuid.UUID
		var amount int64
		if err := rows.Scan(&orderID, &buyerID, &sellerID, &amount); err != nil {
			return count, fmt.Errorf("scan failed: %w", err)
		}
		c.emitAlert(ctx, "missing-escrow-"+orderID.String(),
			"order", orderID,
			fmt.Sprintf("CRITICAL: Order %s has a settled payment but no escrow row. Settlement invariant broken — money settled without escrow.", orderID),
			alertentity.AlertMetadata{
				"order_id":        orderID.String(),
				"buyer_id":        buyerID.String(),
				"seller_id":       sellerID.String(),
				"escrow_amount":   amount,
				"required_action": "investigate_missing_escrow_for_settled_payment",
				"reason":          "settled_payment_without_escrow_row",
			})
		count++
	}
	return count, rows.Err()
}

// checkNoEscrowForUnpaidOrders: an escrow row may only exist on an order that
// was settled and is in a post-payment lifecycle. Unpaid/terminal-unpaid
// orders must have NO escrow.
func (c *EscrowIntegrityChecker) checkNoEscrowForUnpaidOrders(ctx context.Context, tx db.Tx) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT e.order_id, o.buyer_id, o.seller_id, e.amount, e.status, o.status
		FROM escrows e
		JOIN orders o ON o.id = e.order_id
		LEFT JOIN payments p
		  ON p.reference_type = 'order' AND p.reference_id = e.order_id
		 AND p.status IN ('settlement', 'capture')
		WHERE p.id IS NULL
		   OR o.status IN ('pending_payment', 'expired', 'cancelled', 'cancelled_timeout')
	`)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var orderID, buyerID, sellerID uuid.UUID
		var amount int64
		var escrowStatus, orderStatus string
		if err := rows.Scan(&orderID, &buyerID, &sellerID, &amount, &escrowStatus, &orderStatus); err != nil {
			return count, fmt.Errorf("scan failed: %w", err)
		}
		c.emitAlert(ctx, "phantom-escrow-"+orderID.String(),
			"order", orderID,
			fmt.Sprintf("CRITICAL: Order %s (status=%s) has an escrow row (status=%s amount=%d) without a settled payment. Phantom escrow — funds were never settled.", orderID, orderStatus, escrowStatus, amount),
			alertentity.AlertMetadata{
				"order_id":        orderID.String(),
				"buyer_id":        buyerID.String(),
				"seller_id":       sellerID.String(),
				"escrow_amount":   amount,
				"escrow_status":   escrowStatus,
				"order_status":    orderStatus,
				"required_action": "investigate_phantom_escrow_row",
				"reason":          "escrow_row_without_settled_payment",
			})
		count++
	}
	return count, rows.Err()
}

// checkEscrowAmountIntegrity: escrow.amount must equal the canonical
// order-side buyer-funded base (total_before_coins_amount = PD+S) within
// tolerance.
func (c *EscrowIntegrityChecker) checkEscrowAmountIntegrity(ctx context.Context, tx db.Tx) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT e.order_id, o.buyer_id, o.seller_id, e.amount, o.total_before_coins_amount
		FROM escrows e
		JOIN orders o ON o.id = e.order_id
		WHERE ABS(e.amount - o.total_before_coins_amount) > $1
	`, EscrowToleranceAmount)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var orderID, buyerID, sellerID uuid.UUID
		var escrowAmount, orderBase int64
		if err := rows.Scan(&orderID, &buyerID, &sellerID, &escrowAmount, &orderBase); err != nil {
			return count, fmt.Errorf("scan failed: %w", err)
		}
		c.emitAlert(ctx, "escrow-amount-mismatch-"+orderID.String(),
			"order", orderID,
			fmt.Sprintf("CRITICAL: Order %s escrow amount (%d) does not match canonical order base total_before_coins_amount (%d).", orderID, escrowAmount, orderBase),
			alertentity.AlertMetadata{
				"order_id":              orderID.String(),
				"buyer_id":              buyerID.String(),
				"seller_id":             sellerID.String(),
				"escrow_amount":         escrowAmount,
				"order_base_amount":     orderBase,
				"required_action":       "manual_verify_escrow_amount",
				"reason":                "escrow_amount_does_not_match_order_base",
				"money_leak_risk":       "true",
			})
		count++
	}
	return count, rows.Err()
}

// checkEscrowUniqueness: defense-in-depth over UNIQUE(order_id) — there must
// never be multiple escrow rows for one order.
func (c *EscrowIntegrityChecker) checkEscrowUniqueness(ctx context.Context, tx db.Tx) (int, error) {
	var dupCount int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(n - 1), 0) FROM (
			SELECT COUNT(*) AS n FROM escrows GROUP BY order_id HAVING COUNT(*) > 1
		) d
	`).Scan(&dupCount); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	if dupCount > 0 {
		c.emitAlert(ctx, "duplicate-escrow-rows", "system",
			uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			fmt.Sprintf("CRITICAL: %d duplicate escrow rows detected (multiple escrows per order). UNIQUE(order_id) constraint violated.", dupCount),
			alertentity.AlertMetadata{
				"duplicate_count":  dupCount,
				"required_action": "investigate_duplicate_escrow_rows",
				"reason":          "multiple_escrow_rows_per_order",
				"systemic_issue":  "true",
			})
		return dupCount, nil
	}
	return 0, nil
}

// checkHoldingOnTerminalOrders: a holding escrow on a terminal order status
// means the release/refund never flipped the canonical row.
func (c *EscrowIntegrityChecker) checkHoldingOnTerminalOrders(ctx context.Context, tx db.Tx) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT e.order_id, o.buyer_id, o.seller_id, e.amount, o.status
		FROM escrows e
		JOIN orders o ON o.id = e.order_id
		WHERE e.status = 'holding'
		  AND o.status IN ('completed', 'refunded', 'cancelled', 'cancelled_timeout', 'expired')
	`)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var orderID, buyerID, sellerID uuid.UUID
		var amount int64
		var orderStatus string
		if err := rows.Scan(&orderID, &buyerID, &sellerID, &amount, &orderStatus); err != nil {
			return count, fmt.Errorf("scan failed: %w", err)
		}
		c.emitAlert(ctx, "holding-escrow-terminal-order-"+orderID.String(),
			"order", orderID,
			fmt.Sprintf("CRITICAL: Order %s is terminal (status=%s) but its escrow row is still holding (amount=%d). Release/refund never flipped the canonical escrow.", orderID, orderStatus, amount),
			alertentity.AlertMetadata{
				"order_id":        orderID.String(),
				"buyer_id":        buyerID.String(),
				"seller_id":       sellerID.String(),
				"escrow_amount":   amount,
				"order_status":    orderStatus,
				"required_action": "investigate_stuck_holding_escrow",
				"reason":          "holding_escrow_on_terminal_order",
				"money_leak_risk": "true",
			})
		count++
	}
	return count, rows.Err()
}

// getTotalHoldingEscrow: canonical total of held funds (observability only).
func (c *EscrowIntegrityChecker) getTotalHoldingEscrow(ctx context.Context, tx db.Tx) (int64, error) {
	var totalEscrow int64
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM escrows
		WHERE status = 'holding'
	`
	if err := tx.QueryRow(ctx, query).Scan(&totalEscrow); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}
	return totalEscrow, nil
}

// emitAlert creates a CRITICAL reconciliation_drift alert (or logs it in
// shadow mode).
func (c *EscrowIntegrityChecker) emitAlert(ctx context.Context, groupKey string, entityType string, entityID uuid.UUID, message string, metadata alertentity.AlertMetadata) {
	c.log.Error("Escrow integrity violation",
		zap.String("group_key", groupKey),
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID.String()),
		zap.String("message", message),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		entityType,
		entityID,
		message,
		metadata,
		&groupKey,
	)
	if err != nil {
		c.log.Error("Failed to create escrow integrity alert",
			zap.String("group_key", groupKey),
			zap.Error(err),
		)
	}
}
