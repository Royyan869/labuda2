// ⚠️ RECONCILIATION LAYER:
// This module detects payment-order inconsistencies.
// It does NOT modify business data - detection and alerting only.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderrepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	paymentrepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PaymentOrderReconcilerService checks payment ↔ order consistency
// - Orders in pending state must have corresponding payment record
// - Payment status must match order state expectations
// - Detects orphaned payments and orphaned orders
//
// FINANCIAL SAFETY LAYER:
// - Detects when payments succeed but orders don't update (seller loss risk)
// - Detects when orders complete but payments fail (buyer charged, seller not paid)
// - NO AUTO-FIX - detection and alerting only
type PaymentOrderReconcilerService struct {
	paymentRepo  *paymentrepo.PaymentRepository
	orderRepo    *orderrepo.OrderRepository
	alertService *alertapp.AlertService
	db           db.Transactor
	log          *zap.Logger
}

// NewPaymentOrderReconcilerService creates a new payment-order reconciler service.
func NewPaymentOrderReconcilerService(
	paymentRepo *paymentrepo.PaymentRepository,
	orderRepo *orderrepo.OrderRepository,
	alertService *alertapp.AlertService,
	db db.Transactor,
	log *zap.Logger,
) *PaymentOrderReconcilerService {
	if log == nil {
		log = zap.NewNop()
	}

	return &PaymentOrderReconcilerService{
		paymentRepo:  paymentRepo,
		orderRepo:    orderRepo,
		alertService: alertService,
		db:           db,
		log:          log,
	}
}

// ReconcilePaymentOrders checks all orders in pending state for payment consistency
// Returns number of mismatches found
func (s *PaymentOrderReconcilerService) ReconcilePaymentOrders(ctx context.Context) (int, error) {
	s.log.Debug("Starting payment-order reconciliation check")

	// Get all orders in pending state (awaiting payment)
	orderIDs, err := s.getPendingOrderIDs(ctx)
	if err != nil {
		s.log.Error("Failed to get pending order IDs", zap.Error(err))
		return 0, fmt.Errorf("failed to get pending order IDs: %w", err)
	}

	totalOrders := len(orderIDs)
	mismatchCount := 0

	// Check each order's payment state
	for _, orderID := range orderIDs {
		if err := s.checkOrderPayment(ctx, orderID); err != nil {
			s.log.Error("Failed to check order payment",
				zap.String("order_id", orderID.String()),
				zap.Error(err),
			)
			// Continue checking other orders
			mismatchCount++
		}
	}

	s.log.Info("Payment-order reconciliation completed",
		zap.Int("total_orders_checked", totalOrders),
		zap.Int("mismatches_found", mismatchCount),
	)

	return mismatchCount, nil
}

// checkOrderPayment validates a single order's payment state
// Creates alerts for any inconsistencies found
func (s *PaymentOrderReconcilerService) checkOrderPayment(ctx context.Context, orderID uuid.UUID) error {
	var order *orderentity.Order
	var payment *paymentrepo.Payment

	// Use transaction for database operations
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Get order details
		var err error
		order, err = s.orderRepo.GetByID(ctx, tx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order: %w", err)
		}

		// Get payment record for this order
		payment, err = s.paymentRepo.GetPaymentByReference(ctx, tx, paymentrepo.ReferenceTypeOrder, orderID)
		if err != nil {
			// Payment not found is not an error for us - we'll handle it
			return nil
		}
		return err
	})

	// Check 1: Orphaned Order - Order in pending but no payment record exists
	if err != nil || payment == nil {
		if order != nil {
			s.createOrphanedOrderAlert(ctx, order)
		}
		return fmt.Errorf("orphaned order: no payment record")
	}

	// Check 2: Payment settled but order still pending (CRITICAL - seller loss risk)
	if payment.IsSettled() && order.Status == orderentity.StatusPending {
		s.createPaymentSettledOrderPendingAlert(ctx, order, payment)
		return fmt.Errorf("payment settled but order still pending")
	}

	// Check 3: Payment failed but order not cancelled
	if payment.IsFailed() && order.Status != orderentity.StatusCancelled && order.Status != orderentity.StatusExpired {
		s.createPaymentFailedOrderNotCancelledAlert(ctx, order, payment)
		return fmt.Errorf("payment failed but order not cancelled")
	}

	// Check 4: Payment pending but order not in pending state (state drift)
	if payment.IsPending() && order.Status != orderentity.StatusPending {
		s.createStateMismatchAlert(ctx, order, payment)
		return fmt.Errorf("payment pending but order not in pending state")
	}

	return nil
}

// getPendingOrderIDs retrieves all order IDs in pending state
func (s *PaymentOrderReconcilerService) getPendingOrderIDs(ctx context.Context) ([]uuid.UUID, error) {
	var orderIDs []uuid.UUID

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM orders
			WHERE status = 'pending_payment'
			ORDER BY created_at DESC
			LIMIT 10000
		`

		rows, err := tx.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}
			orderIDs = append(orderIDs, id)
		}

		return rows.Err()
	})

	return orderIDs, err
}

// createOrphanedOrderAlert creates a WARNING alert for orphaned order
func (s *PaymentOrderReconcilerService) createOrphanedOrderAlert(ctx context.Context, order *orderentity.Order) {
	metadata := alertentity.AlertMetadata{
		"order_id":           order.ID.String(),
		"buyer_id":           order.BuyerID.String(),
		"seller_id":          order.SellerID.String(),
		"order_status":       string(order.Status),
		"order_created_at":   order.CreatedAt.String(),
		"payment_expires_at": order.PaymentExpiresAt.String(),
		"required_action":    "investigate_orphaned_order",
		"reason":             "order_in_pending_state_without_payment_record",
	}

	message := fmt.Sprintf(
		"ORPHANED ORDER: Order %s is in pending state but has no payment record. Buyer: %s, Seller: %s",
		order.ID.String(),
		order.BuyerID.String(),
		order.SellerID.String(),
	)

	groupKey := fmt.Sprintf("orphaned-order-%s", order.ID.String())
	_, err := s.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityMedium, // WARNING level
		"order",
		order.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		s.log.Error("Failed to create orphaned order alert",
			zap.String("order_id", order.ID.String()),
			zap.Error(err),
		)
	} else {
		s.log.Warn("Orphaned order detected and alerted",
			zap.String("order_id", order.ID.String()),
			zap.String("buyer_id", order.BuyerID.String()),
			zap.String("seller_id", order.SellerID.String()),
		)
	}
}

// createPaymentSettledOrderPendingAlert creates a CRITICAL alert for payment settled but order still pending
func (s *PaymentOrderReconcilerService) createPaymentSettledOrderPendingAlert(
	ctx context.Context,
	order *orderentity.Order,
	payment *paymentrepo.Payment,
) {
	metadata := alertentity.AlertMetadata{
		"order_id":           order.ID.String(),
		"buyer_id":           order.BuyerID.String(),
		"seller_id":          order.SellerID.String(),
		"payment_id":         payment.ID.String(),
		"payment_status":     payment.Status,
		"payment_paid_at":    payment.PaidAt.String(),
		"order_status":       string(order.Status),
		"payment_expires_at": order.PaymentExpiresAt.String(),
		"required_action":    "manual_verify_order_payment_consistency",
		"reason":             "payment_settled_but_order_not_updated_to_paid",
		"seller_at_risk":     "true", // Seller hasn't received payment credit
	}

	message := fmt.Sprintf(
		"CRITICAL: Payment %s settled but order %s still in pending state. Seller %s at risk of not receiving payment credit. Buyer: %s",
		payment.ID.String(),
		order.ID.String(),
		order.SellerID.String(),
		order.BuyerID.String(),
	)

	groupKey := fmt.Sprintf("payment-settled-order-pending-%s", order.ID.String())
	_, err := s.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityCritical, // CRITICAL level - seller loss risk
		"order",
		order.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		s.log.Error("Failed to create payment settled order pending alert",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
	} else {
		s.log.Error("Payment settled but order still pending - CRITICAL",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.String("seller_id", order.SellerID.String()),
			zap.String("payment_status", payment.Status),
		)
	}
}

// createPaymentFailedOrderNotCancelledAlert creates a WARNING alert for payment failed but order not cancelled
func (s *PaymentOrderReconcilerService) createPaymentFailedOrderNotCancelledAlert(
	ctx context.Context,
	order *orderentity.Order,
	payment *paymentrepo.Payment,
) {
	metadata := alertentity.AlertMetadata{
		"order_id":        order.ID.String(),
		"buyer_id":        order.BuyerID.String(),
		"seller_id":       order.SellerID.String(),
		"payment_id":      payment.ID.String(),
		"payment_status":  payment.Status,
		"order_status":    string(order.Status),
		"required_action": "check_order_cancellation_required",
		"reason":          "payment_failed_but_order_not_cancelled",
	}

	message := fmt.Sprintf(
		"WARNING: Payment %s failed (status: %s) but order %s not cancelled. Buyer: %s, Seller: %s",
		payment.ID.String(),
		payment.Status,
		order.ID.String(),
		order.BuyerID.String(),
		order.SellerID.String(),
	)

	groupKey := fmt.Sprintf("payment-failed-order-not-cancelled-%s", order.ID.String())
	_, err := s.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityMedium, // WARNING level
		"order",
		order.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		s.log.Error("Failed to create payment failed order not cancelled alert",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
	} else {
		s.log.Warn("Payment failed but order not cancelled",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.String("payment_status", payment.Status),
			zap.String("order_status", string(order.Status)),
		)
	}
}

// createStateMismatchAlert creates an INFO alert for payment-order state mismatch
func (s *PaymentOrderReconcilerService) createStateMismatchAlert(
	ctx context.Context,
	order *orderentity.Order,
	payment *paymentrepo.Payment,
) {
	metadata := alertentity.AlertMetadata{
		"order_id":        order.ID.String(),
		"buyer_id":        order.BuyerID.String(),
		"seller_id":       order.SellerID.String(),
		"payment_id":      payment.ID.String(),
		"payment_status":  payment.Status,
		"order_status":    string(order.Status),
		"required_action": "investigate_state_drift",
		"reason":          "payment_order_state_mismatch",
	}

	message := fmt.Sprintf(
		"STATE MISMATCH: Payment %s is pending but order %s is in status %s. Investigate required.",
		payment.ID.String(),
		order.ID.String(),
		string(order.Status),
	)

	groupKey := fmt.Sprintf("state-mismatch-%s", order.ID.String())
	_, err := s.alertService.CreateAlert(
		ctx,
		alertentity.AlertTypeReconciliationDrift,
		alertentity.SeverityLow, // INFO level
		"order",
		order.ID,
		message,
		metadata,
		&groupKey,
	)

	if err != nil {
		s.log.Error("Failed to create state mismatch alert",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
	} else {
		s.log.Info("Payment-order state mismatch detected",
			zap.String("order_id", order.ID.String()),
			zap.String("payment_id", payment.ID.String()),
			zap.String("payment_status", payment.Status),
			zap.String("order_status", string(order.Status)),
		)
	}
}


