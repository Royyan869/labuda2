// ⚠️ RECONCILIATION LAYER:
// This module detects escrow inconsistencies.
// It does NOT modify business data - detection and alerting only.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	walletentity "github.com/labuda/backend/internal/core/wallet/entity"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// EscrowToleranceAmount is the acceptable rounding difference in rupiah units
// for escrow integrity checks. Differences within this tolerance are NOT flagged.
const EscrowToleranceAmount int64 = 100

// TimingGraceMinutes excludes orders created within this window to avoid
// false positives from in-flight transactions.
const TimingGraceMinutes = 5

// holdingOrderRow is a lightweight struct for per-order checks.
// Uses total_before_coins_amount directly from the DB (the canonical
// buyer-funded escrow base = PD + S) rather than recomputing from entity
// fields. orders.escrow_amount is NOT authoritative (never persisted).
type holdingOrderRow struct {
	ID           uuid.UUID
	BuyerID      uuid.UUID
	SellerID     uuid.UUID
	EscrowAmount int64 // canonical buyer-funded escrow base = total_before_coins_amount
}

// escrowLookupService is the narrow dependency needed by this checker.
//
// The gateway-funded model keeps the escrow row as the canonical record for an
// order's held funds. The checker only needs to fetch the escrow row for a
// given order; it must not inspect buyer-wallet balances.
type escrowLookupService interface {
	GetEscrowForOrder(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*walletentity.Escrow, error)
}

// EscrowIntegrityChecker verifies order escrow rows match the canonical escrow
// record in the gateway-funded model.
//
// - Order.total_before_coins_amount (the canonical buyer-funded escrow base =
//   PD + S) should match the corresponding escrows.amount
// - Sum of all holding order total_before_coins_amounts should equal total
//   holding escrows
//
// FINANCIAL SAFETY LAYER:
// - Detects when order says holding but the escrow row is missing/mismatched
// - Detects global escrow imbalance (systemic issue)
// - NO AUTO-FIX - detection and alerting only
type EscrowIntegrityChecker struct {
	escrowLookup escrowLookupService
	alertService *alertapp.AlertService
	db           db.Transactor
	log          *zap.Logger
	shadowMode   bool
}

// NewEscrowIntegrityChecker creates a new escrow integrity checker.
// When shadowMode is true, the checker logs findings but does NOT create alerts.
func NewEscrowIntegrityChecker(
	escrowLookup escrowLookupService,
	alertService *alertapp.AlertService,
	db db.Transactor,
	log *zap.Logger,
	shadowMode bool,
) *EscrowIntegrityChecker {
	if log == nil {
		log = zap.NewNop()
	}

	return &EscrowIntegrityChecker{
		escrowLookup: escrowLookup,
		alertService: alertService,
		db:           db,
		log:          log,
		shadowMode:   shadowMode,
	}
}

// CheckEscrowIntegrity validates all escrow amounts.
// Returns number of mismatches found.
func (c *EscrowIntegrityChecker) CheckEscrowIntegrity(ctx context.Context) (int, error) {
	c.log.Debug("Starting escrow integrity check", zap.Bool("shadow_mode", c.shadowMode))

	if c.db == nil {
		return 0, fmt.Errorf("db transactor not configured")
	}

	var (
		totalOrders           int
		perOrderMismatchCount int
		globalMismatch        bool
		totalMismatches       int
	)

	err := c.db.WithTx(ctx, func(tx db.Tx) error {
		// Get all orders in holding state with their canonical buyer-funded
		// escrow base (total_before_coins_amount = PD + S) from DB.
		// Excludes orders created within the timing grace window to avoid in-flight false positives.
		holdingOrders, err := c.getHoldingOrders(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to get holding orders: %w", err)
		}

		totalOrders = len(holdingOrders)

		// Check each order's escrow amount against the canonical escrow row.
		for _, row := range holdingOrders {
			if err := c.checkOrderEscrow(ctx, tx, row); err != nil {
				c.log.Error("Failed to check order escrow",
					zap.String("order_id", row.ID.String()),
					zap.Error(err),
				)
				// Continue checking other orders
				perOrderMismatchCount++
			}
		}

		// Check global invariant: total order escrow = total holding escrows.
		globalMismatch, err = c.checkGlobalEscrowInvariant(ctx, tx)
		if err != nil {
			c.log.Error("Failed to check global escrow invariant", zap.Error(err))
			// Don't count this as a mismatch in the return value
			// since it's a separate check
		} else if globalMismatch {
			// Global mismatch is counted separately
			c.log.Error("Global escrow invariant violation detected")
		}

		totalMismatches = perOrderMismatchCount
		if globalMismatch {
			totalMismatches++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	c.log.Info("Escrow integrity check completed",
		zap.Int("total_orders_checked", totalOrders),
		zap.Int("per_order_mismatches", perOrderMismatchCount),
		zap.Bool("global_mismatch", globalMismatch),
		zap.Int("total_mismatches", totalMismatches),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	return totalMismatches, nil
}

// checkOrderEscrow validates a single order's escrow amount.
// Uses the canonical buyer-funded escrow base (total_before_coins_amount = PD + S)
// from the DB (not orders.escrow_amount, which is never persisted).
// Verifies that the canonical escrow row exists and matches the order amount.
func (c *EscrowIntegrityChecker) checkOrderEscrow(ctx context.Context, tx db.Tx, row holdingOrderRow) error {
	// Escrow amount should be positive
	if row.EscrowAmount <= 0 {
		c.emitInvalidEscrowAmountAlert(ctx, row)
		return fmt.Errorf("invalid escrow amount: %d", row.EscrowAmount)
	}

	if c.escrowLookup == nil {
		return fmt.Errorf("escrow lookup not configured")
	}

	escrow, err := c.escrowLookup.GetEscrowForOrder(ctx, tx, row.ID)
	if err != nil {
		return fmt.Errorf("failed to get escrow row: %w", err)
	}
	if escrow == nil {
		c.emitMissingEscrowAlert(ctx, row)
		return fmt.Errorf("escrow row not found")
	}

	if escrow.Status != walletentity.EscrowStatusHolding {
		c.emitEscrowStatusMismatchAlert(ctx, row, escrow.Status.String())
		return fmt.Errorf("escrow status mismatch: expected=holding got=%s", escrow.Status)
	}

	// Check if the canonical escrow amount matches this order's escrow amount.
	diff := escrow.Amount - row.EscrowAmount
	if diff < 0 {
		diff = -diff
	}
	if diff > EscrowToleranceAmount {
		c.emitEscrowAmountMismatchAlert(ctx, row, escrow.Amount)
		return fmt.Errorf("escrow amount mismatch: order=%d, escrow_row=%d", row.EscrowAmount, escrow.Amount)
	}

	return nil
}

// checkGlobalEscrowInvariant validates the global escrow invariant.
// SUM(orders.total_before_coins_amount WHERE escrow_status='holding') should
// equal SUM(escrows.amount WHERE status='holding') within tolerance.
func (c *EscrowIntegrityChecker) checkGlobalEscrowInvariant(ctx context.Context, tx db.Tx) (bool, error) {
	// Calculate total escrow from orders (using canonical total_before_coins_amount)
	totalOrderEscrow, err := c.getTotalOrderEscrow(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("failed to get total order escrow: %w", err)
	}

	// Calculate total holding escrow from canonical escrow rows.
	totalEscrowRows, err := c.getTotalHoldingEscrow(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("failed to get total holding escrow: %w", err)
	}

	// Check invariant with tolerance
	difference := totalEscrowRows - totalOrderEscrow
	if difference < 0 {
		difference = -difference
	}

	if difference > EscrowToleranceAmount {
		c.emitGlobalEscrowImbalanceAlert(ctx, totalOrderEscrow, totalEscrowRows)
		return true, nil // Mismatch detected
	}

	return false, nil // No mismatch (within tolerance)
}

// getHoldingOrders retrieves all orders in holding escrow state with their
// canonical buyer-funded escrow base (total_before_coins_amount = PD + S).
// orders.escrow_amount is NOT authoritative. Excludes orders within
// TimingGraceMinutes to avoid false positives from in-flight transactions.
func (c *EscrowIntegrityChecker) getHoldingOrders(ctx context.Context, tx db.Tx) ([]holdingOrderRow, error) {
	var orders []holdingOrderRow

	query := fmt.Sprintf(`
		SELECT id, buyer_id, seller_id, total_before_coins_amount
		FROM orders
		WHERE escrow_status = 'holding'
		  AND created_at < NOW() - INTERVAL '%d minutes'
		ORDER BY created_at DESC
		LIMIT 10000
	`, TimingGraceMinutes)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row holdingOrderRow
		if err := rows.Scan(&row.ID, &row.BuyerID, &row.SellerID, &row.EscrowAmount); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		orders = append(orders, row)
	}

	return orders, rows.Err()
}

// getTotalOrderEscrow calculates the sum of all canonical buyer-funded escrow
// bases (total_before_coins_amount = PD + S) for orders in holding state.
// orders.escrow_amount is NOT authoritative (never persisted).
// Excludes orders within the timing grace window.
func (c *EscrowIntegrityChecker) getTotalOrderEscrow(ctx context.Context, tx db.Tx) (int64, error) {
	var totalEscrow int64

	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(total_before_coins_amount), 0)
		FROM orders
		WHERE escrow_status = 'holding'
		  AND created_at < NOW() - INTERVAL '%d minutes'
	`, TimingGraceMinutes)

	if err := tx.QueryRow(ctx, query).Scan(&totalEscrow); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return totalEscrow, nil
}

// getTotalHoldingEscrow calculates sum of all holding escrows.amount values.
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

// emitInvalidEscrowAmountAlert creates a CRITICAL alert for invalid escrow amount.
// In shadow mode, only logs the finding.
func (c *EscrowIntegrityChecker) emitInvalidEscrowAmountAlert(ctx context.Context, row holdingOrderRow) {
	c.log.Error("Invalid escrow amount detected",
		zap.String("order_id", row.ID.String()),
		zap.Int64("escrow_amount", row.EscrowAmount),
		zap.String("buyer_id", row.BuyerID.String()),
		zap.String("seller_id", row.SellerID.String()),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	metadata := alertentity.AlertMetadata{
		"order_id":        row.ID.String(),
		"buyer_id":        row.BuyerID.String(),
		"seller_id":       row.SellerID.String(),
		"escrow_amount":   row.EscrowAmount,
		"required_action": "investigate_invalid_escrow_amount",
		"reason":          "escrow_amount_invalid_or_missing",
	}

	message := fmt.Sprintf(
		"CRITICAL: Order %s has invalid escrow amount (%d). Buyer: %s, Seller: %s",
		row.ID.String(),
		row.EscrowAmount,
		row.BuyerID.String(),
		row.SellerID.String(),
	)

	groupKey := fmt.Sprintf("invalid-escrow-amount-%s", row.ID.String())
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"order",
		row.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create invalid escrow amount alert",
			zap.String("order_id", row.ID.String()),
			zap.Error(err),
		)
	}
}

// emitMissingEscrowAlert creates a CRITICAL alert for a missing canonical escrow row.
// In shadow mode, only logs the finding.
func (c *EscrowIntegrityChecker) emitMissingEscrowAlert(ctx context.Context, row holdingOrderRow) {
	c.log.Error("Missing escrow row for order with escrow",
		zap.String("order_id", row.ID.String()),
		zap.String("buyer_id", row.BuyerID.String()),
		zap.Int64("escrow_amount", row.EscrowAmount),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	metadata := alertentity.AlertMetadata{
		"order_id":        row.ID.String(),
		"buyer_id":        row.BuyerID.String(),
		"seller_id":       row.SellerID.String(),
		"escrow_amount":   row.EscrowAmount,
		"required_action": "investigate_missing_escrow",
		"reason":          "escrow_row_not_found_for_order_with_escrow",
	}

	message := fmt.Sprintf(
		"CRITICAL: Order %s has escrow holding but no matching escrow row was found. Gateway-funded escrow mismatch risk.",
		row.ID.String(),
	)

	groupKey := fmt.Sprintf("missing-escrow-%s", row.ID.String())
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"order",
		row.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create missing escrow alert",
			zap.String("order_id", row.ID.String()),
			zap.Error(err),
		)
	}
}

// emitEscrowStatusMismatchAlert creates a CRITICAL alert when escrow status is not holding.
// In shadow mode, only logs the finding.
func (c *EscrowIntegrityChecker) emitEscrowStatusMismatchAlert(ctx context.Context, row holdingOrderRow, escrowStatus string) {
	c.log.Error("Escrow status mismatch detected",
		zap.String("order_id", row.ID.String()),
		zap.String("order_escrow_status", "holding"),
		zap.String("escrow_row_status", escrowStatus),
		zap.String("buyer_id", row.BuyerID.String()),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	metadata := alertentity.AlertMetadata{
		"order_id":        row.ID.String(),
		"buyer_id":        row.BuyerID.String(),
		"seller_id":       row.SellerID.String(),
		"expected_status": "holding",
		"actual_status":   escrowStatus,
		"required_action": "manual_verify_gateway_funded_escrow",
		"reason":          "order_escrow_status_does_not_match_escrow_row_status",
		"money_leak_risk": "true",
	}

	message := fmt.Sprintf(
		"CRITICAL: Order %s expects escrow status holding but escrow row is %s. Gateway-funded escrow mismatch risk. Buyer: %s, Seller: %s",
		row.ID.String(),
		escrowStatus,
		row.BuyerID.String(),
		row.SellerID.String(),
	)

	groupKey := fmt.Sprintf("escrow-status-mismatch-%s", row.ID.String())
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"order",
		row.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create escrow status mismatch alert",
			zap.String("order_id", row.ID.String()),
			zap.String("escrow_row_status", escrowStatus),
			zap.Error(err),
		)
	}
}

// emitEscrowAmountMismatchAlert creates a CRITICAL alert for escrow amount mismatch.
// In shadow mode, only logs the finding.
func (c *EscrowIntegrityChecker) emitEscrowAmountMismatchAlert(
	ctx context.Context,
	row holdingOrderRow,
	escrowRowAmount int64,
) {
	c.log.Error("Escrow amount mismatch detected",
		zap.String("order_id", row.ID.String()),
		zap.Int64("order_escrow", row.EscrowAmount),
		zap.Int64("escrow_row_amount", escrowRowAmount),
		zap.String("buyer_id", row.BuyerID.String()),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	metadata := alertentity.AlertMetadata{
		"order_id":            row.ID.String(),
		"buyer_id":            row.BuyerID.String(),
		"seller_id":           row.SellerID.String(),
		"order_escrow_amount": row.EscrowAmount,
		"escrow_row_amount":   escrowRowAmount,
		"required_action":     "manual_verify_gateway_funded_escrow",
		"reason":              "order_escrow_amount_does_not_match_escrow_row_amount",
		"money_leak_risk":     "true",
	}

	message := fmt.Sprintf(
		"CRITICAL: Order %s escrow amount (%d) does not match escrow row amount (%d). Gateway-funded escrow mismatch risk. Buyer: %s, Seller: %s",
		row.ID.String(),
		row.EscrowAmount,
		escrowRowAmount,
		row.BuyerID.String(),
		row.SellerID.String(),
	)

	groupKey := fmt.Sprintf("escrow-mismatch-%s", row.ID.String())
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"order",
		row.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create escrow amount mismatch alert",
			zap.String("order_id", row.ID.String()),
			zap.Int64("order_escrow", row.EscrowAmount),
			zap.Int64("escrow_row_amount", escrowRowAmount),
			zap.Error(err),
		)
	}
}

// emitGlobalEscrowImbalanceAlert creates a CRITICAL alert for global escrow imbalance.
// In shadow mode, only logs the finding.
func (c *EscrowIntegrityChecker) emitGlobalEscrowImbalanceAlert(
	ctx context.Context,
	totalOrderEscrow,
	totalEscrowRows int64,
) {
	c.log.Error("Global escrow invariant violation",
		zap.Int64("total_order_escrow", totalOrderEscrow),
		zap.Int64("total_escrow_rows", totalEscrowRows),
		zap.Int64("difference", totalEscrowRows-totalOrderEscrow),
		zap.Bool("shadow_mode", c.shadowMode),
	)

	if c.shadowMode {
		return
	}

	metadata := alertentity.AlertMetadata{
		"total_order_escrow": totalOrderEscrow,
		"total_escrow_rows":  totalEscrowRows,
		"difference":         totalEscrowRows - totalOrderEscrow,
		"required_action":    "emergency_investigate_systemic_escrow_imbalance",
		"reason":             "global_escrow_invariant_violation",
		"systemic_issue":     "true",
	}

	message := fmt.Sprintf(
		"CRITICAL: Global escrow invariant violated. Total order escrow (%d) != total holding escrow rows (%d). Difference: %d. SYSTEMIC ISSUE.",
		totalOrderEscrow,
		totalEscrowRows,
		totalEscrowRows-totalOrderEscrow,
	)

	groupKey := "global-escrow-imbalance"
	_, err := c.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical,
		"system",
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		c.log.Error("Failed to create global escrow imbalance alert",
			zap.Int64("total_order_escrow", totalOrderEscrow),
			zap.Int64("total_escrow_rows", totalEscrowRows),
			zap.Error(err),
		)
	}
}


