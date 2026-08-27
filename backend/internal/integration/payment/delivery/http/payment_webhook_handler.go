package http

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/integration/payment/application"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// PaymentWebhookHandler handles Midtrans webhook notifications
type PaymentWebhookHandler struct {
	webhookService *application.PaymentWebhookService
	log            *zap.Logger
}

// NewPaymentWebhookHandler creates a new PaymentWebhookHandler
func NewPaymentWebhookHandler(
	webhookService *application.PaymentWebhookService,
	log *zap.Logger,
) *PaymentWebhookHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PaymentWebhookHandler{
		webhookService: webhookService,
		log:            log,
	}
}

// HandleMidtransWebhook handles POST /webhooks/payment/midtrans
// This is the callback endpoint for Midtrans payment notifications
//
// The webhook:
// 1. Receives notification from Midtrans
// 2. Validates signature
// 3. Checks idempotency
// 4. Updates payment status
// 5. Creates payment escrow ledger entries (DR ESCROW, CR GATEWAY_CLEARING)
//
// Transaction handling:
// - Always returns 200 OK to Midtrans to prevent retries
// - Uses idempotency keys to prevent double processing
// - FOR UPDATE locks payment row to prevent race conditions
func (h *PaymentWebhookHandler) HandleMidtransWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Get client IP for logging
	clientIP := c.ClientIP()

	// Parse webhook payload
	var notification midtrans.NotificationPayload
	if err := c.ShouldBindJSON(&notification); err != nil {
		h.log.Warn("Invalid webhook payload",
			zap.String("client_ip", clientIP),
			zap.Error(err),
		)
		// Return 200 OK to prevent Midtrans retry (malformed request)
		response.Success(c, gin.H{"status": "ignored", "reason": "invalid payload"})
		return
	}

	h.log.Info("Received Midtrans webhook",
		zap.String("order_id", notification.OrderID),
		zap.String("transaction_id", notification.TransactionID),
		zap.String("transaction_status", notification.TransactionStatus),
		zap.String("client_ip", clientIP),
	)

	// Process webhook
	if err := h.webhookService.HandleWebhook(ctx, &notification, clientIP); err != nil {
		h.log.Error("Webhook processing failed",
			zap.String("order_id", notification.OrderID),
			zap.String("transaction_id", notification.TransactionID),
			zap.Error(err),
		)
		// Still return 200 OK - webhook is recorded as failed
		// Midtrans will not retry
	}

	// Always return 200 OK to prevent Midtrans retry
	// Failed webhooks are recorded for manual reconciliation
	response.Success(c, gin.H{"status": "received"})
}

// HandleMidtransWebhookRaw handles POST /webhooks/payment/midtrans/raw
// Alternative endpoint that returns the raw processing result
// Useful for debugging and monitoring
func (h *PaymentWebhookHandler) HandleMidtransWebhookRaw(c *gin.Context) {
	ctx := c.Request.Context()
	clientIP := c.ClientIP()

	var notification midtrans.NotificationPayload
	if err := c.ShouldBindJSON(&notification); err != nil {
		response.BadRequest(c, "invalid payload")
		return
	}

	err := h.webhookService.HandleWebhook(ctx, &notification, clientIP)
	if err != nil {
		response.RespondWithError(c, h.log, err)
		return
	}

	response.Success(c, gin.H{
		"status":             "success",
		"order_id":           notification.OrderID,
		"transaction_id":     notification.TransactionID,
		"transaction_status": notification.TransactionStatus,
	})
}

// HandleMidtransWebhookDevReplay replays a verified Midtrans success payload
// through the canonical webhook path. This endpoint is dev-only and exists
// to repair local delivery when the public notification URL is stale or
// unreachable.
func (h *PaymentWebhookHandler) HandleMidtransWebhookDevReplay(c *gin.Context) {
	ctx := c.Request.Context()
	clientIP := c.ClientIP()

	paymentIDStr := c.Param("payment_id")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		response.BadRequest(c, "invalid payment_id")
		return
	}

	payload, err := h.webhookService.ReplayVerifiedWebhookFromGateway(ctx, paymentID, clientIP)
	if err != nil {
		h.log.Warn("dev webhook replay failed",
			zap.String("payment_id", paymentIDStr),
			zap.Error(err),
		)
		response.BadRequest(c, fmt.Sprintf("webhook replay failed: %v", err))
		return
	}

	response.Success(c, gin.H{
		"status":             "replayed",
		"payment_id":         paymentIDStr,
		"midtrans_order_id":  payload.OrderID,
		"transaction_id":     payload.TransactionID,
		"transaction_status": payload.TransactionStatus,
		"fraud_status":       payload.FraudStatus,
		"gross_amount":       payload.GrossAmount,
		"safe_to_activate":   true,
	})
}


