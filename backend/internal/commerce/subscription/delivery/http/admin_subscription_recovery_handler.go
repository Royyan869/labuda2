package http

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// paymentRow is the minimal payment fields needed for recovery validation.
type paymentRow struct {
	UserID        uuid.UUID
	Status        string
	ReferenceType string
}

// fetchPaymentForRecovery reads a payment row by ID within the given transaction.
func fetchPaymentForRecovery(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*paymentRow, error) {
	var p paymentRow
	err := tx.QueryRow(ctx,
		`SELECT user_id, status, reference_type FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&p.UserID, &p.Status, &p.ReferenceType)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// AdminSubscriptionRecoveryHandler handles manual recovery of settled subscription
// payments that have no corresponding seller_subscriptions row (webhook miss scenario).
//
// POST /admin/seller-subscriptions/recover/:payment_id
//   - Requires capability: seller.subscription.recover
//   - Validates: reference_type = 'subscription', status IN ('settlement', 'capture')
//   - Delegates to canonical ProcessSuccessfulPayment (idempotent)
//   - Audit logs every successful recovery
type AdminSubscriptionRecoveryHandler struct {
	paymentService *subscriptionApp.SellerSubscriptionPaymentService
	db             db.Transactor
	log            *zap.Logger
	auditLogger    audit.AdminAuditLogger
}

// NewAdminSubscriptionRecoveryHandler constructs the handler.
func NewAdminSubscriptionRecoveryHandler(
	paymentService *subscriptionApp.SellerSubscriptionPaymentService,
	database *db.DB,
	log *zap.Logger,
) *AdminSubscriptionRecoveryHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminSubscriptionRecoveryHandler{
		paymentService: paymentService,
		db:             database,
		log:            log,
		auditLogger:    audit.NewAdminAuditLoggerDB(database.Pool()),
	}
}

// Recover handles POST /api/v1/admin/seller-subscriptions/recover/:payment_id
//
// Recovers a subscription payment that was settled but never activated due to
// webhook delivery failure. The operation is idempotent: if the subscription
// already exists for this payment_id, it returns 200 with message "already active".
func (h *AdminSubscriptionRecoveryHandler) Recover(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapSellerSubscriptionRecover.String()) {
		response.Forbidden(c, "seller.subscription.recover capability required")
		return
	}

	paymentIDStr := c.Param("payment_id")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		response.BadRequest(c, "Invalid payment_id: must be a valid UUID")
		return
	}

	// Validate payment exists and is recoverable before delegating to service.
	var p *paymentRow
	validationErr := h.db.WithTx(ctx, func(tx db.Tx) error {
		row, err := fetchPaymentForRecovery(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		p = row
		return nil
	})
	if validationErr != nil {
		h.log.Warn("recovery validation failed: payment not found",
			zap.String("payment_id", paymentIDStr),
			zap.Error(validationErr),
		)
		response.NotFound(c, fmt.Sprintf("Payment %s not found", paymentIDStr))
		return
	}

	if p.ReferenceType != "subscription" {
		response.BadRequest(c, fmt.Sprintf(
			"Payment %s is not a subscription payment (reference_type=%s)",
			paymentIDStr, p.ReferenceType,
		))
		return
	}

	if p.Status != "settlement" && p.Status != "capture" {
		response.BadRequest(c, fmt.Sprintf(
			"Payment %s is not settled (status=%s); only settlement/capture payments can be recovered",
			paymentIDStr, p.Status,
		))
		return
	}

	// Delegate to canonical idempotent service.
	// ProcessSuccessfulPayment returns nil if subscription already exists (no-op).
	providerEventID := "admin_recovery_" + actor.ID.String()
	if err := h.paymentService.ProcessSuccessfulPayment(ctx, paymentID, p.UserID, providerEventID); err != nil {
		h.log.Error("subscription recovery failed",
			zap.String("payment_id", paymentIDStr),
			zap.String("user_id", p.UserID.String()),
			zap.String("admin_id", actor.ID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to recover subscription: "+err.Error())
		return
	}

	h.auditLogger.LogSafe(ctx, actor.ID,
		"subscription_payment_recovered", "payment", paymentID,
		map[string]interface{}{
			"payment_id": paymentIDStr,
			"user_id":    p.UserID.String(),
		},
	)

	h.log.Info("subscription payment recovered",
		zap.String("payment_id", paymentIDStr),
		zap.String("user_id", p.UserID.String()),
		zap.String("admin_id", actor.ID.String()),
	)

	response.Success(c, gin.H{
		"message":    "Subscription payment recovered successfully",
		"payment_id": paymentIDStr,
		"user_id":    p.UserID.String(),
	})
}


