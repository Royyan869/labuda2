package http

import (
	"github.com/gin-gonic/gin"
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
		// The failure was already recorded durably (REC-1): HandleWebhook writes
		// payment_webhook_events.status='failed' in an independent transaction
		// after the processing transaction rolls back.
	}

	// Always return 200 OK to prevent Midtrans retry.
	//
	// Midtrans does not retry a 200, so a processing failure is NOT recovered by
	// redelivery — it is surfaced through the durable record above for operator
	// reconciliation. Changing this status would change gateway retry behaviour
	// and is deliberately out of REC-1's scope.
	response.Success(c, gin.H{"status": "received"})
}
