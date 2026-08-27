package http

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/internal/finance/worker"
	"github.com/labuda/backend/internal/platform/response"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/database"
	"go.uber.org/zap"
)

const (
	// MaxWebhookPayloadSize is the maximum size of a webhook payload (1MB)
	MaxWebhookPayloadSize = 1 << 20
	// WebhookSignatureHeader is the header containing the HMAC signature
	WebhookSignatureHeader = "X-Webhook-Signature"
)

// PayoutWebhookHandler handles HTTP webhook requests from payout gateways.
//
// SECURITY FEATURES:
// - HMAC SHA256 signature verification
// - Payload size limits
// - Idempotent callback processing
// - Comprehensive audit logging
//
// This handler works with the sandbox gateway for testing and can be
// extended to work with real payment gateways in the future.
type PayoutWebhookHandler struct {
	withdrawRepo   *repository.WithdrawRepository
	db             *database.DB
	verifier       *worker.WebhookSignatureVerifier
	webhookHandler *worker.WebhookHandler
	log            *zap.Logger

	// Gateway configuration for health check observability
	gatewayProvider   string // Provider name (e.g., "sandbox", "midtrans_payout")
	isProductionMode  bool   // True if production payouts are enabled
	isSandboxMode     bool   // True if running in sandbox mode
}

// NewPayoutWebhookHandler creates a new payout webhook handler.
func NewPayoutWebhookHandler(
	withdrawRepo *repository.WithdrawRepository,
	db *database.DB,
	secretKey string,
	log *zap.Logger,
	outboxRepo *outboxrepo.OutboxRepository,
) *PayoutWebhookHandler {
	return newPayoutWebhookHandlerWithConfig(withdrawRepo, db, secretKey, log, "sandbox", false, true, outboxRepo)
}

// NewPayoutWebhookHandlerWithConfig creates a new payout webhook handler with explicit gateway configuration.
// This constructor should be used when gateway configuration is available for observability.
func NewPayoutWebhookHandlerWithConfig(
	withdrawRepo *repository.WithdrawRepository,
	db *database.DB,
	secretKey string,
	log *zap.Logger,
	gatewayProvider string,
	isProductionMode bool,
	isSandboxMode bool,
	outboxRepo *outboxrepo.OutboxRepository,
) *PayoutWebhookHandler {
	return newPayoutWebhookHandlerWithConfig(withdrawRepo, db, secretKey, log, gatewayProvider, isProductionMode, isSandboxMode, outboxRepo)
}

// newPayoutWebhookHandlerWithConfig is the internal constructor that handles all initialization.
func newPayoutWebhookHandlerWithConfig(
	withdrawRepo *repository.WithdrawRepository,
	db *database.DB,
	secretKey string,
	log *zap.Logger,
	gatewayProvider string,
	isProductionMode bool,
	isSandboxMode bool,
	outboxRepo *outboxrepo.OutboxRepository,
) *PayoutWebhookHandler {
	if log == nil {
		log = zap.NewNop()
	}

	// Initialize ledger repository for finality accounting
	ledgerRepo := repository.NewLedgerRepository()

	return &PayoutWebhookHandler{
		withdrawRepo:     withdrawRepo,
		db:               db,
		verifier:         worker.NewWebhookSignatureVerifier(secretKey, log),
		webhookHandler:   worker.NewWebhookHandler(withdrawRepo, ledgerRepo, outboxRepo, log),
		log:              log,
		gatewayProvider:  gatewayProvider,
		isProductionMode: isProductionMode,
		isSandboxMode:    isSandboxMode,
	}
}

// HandlePayoutWebhook handles incoming webhook requests from payout gateways.
//
// Expected request format:
//   Content-Type: application/json
//   X-Webhook-Signature: sha256=<hex_digest>
//
// Request body:
//   {
//     "external_reference_id": "WD_xxx",
//     "gateway_reference_id": "GW_xxx",
//     "status": "SUCCESS|PENDING|FAILED|REJECTED",
//     "message": "...",
//     "timestamp": 1234567890
//   }
//
// This endpoint does NOT require authentication (called by gateway).
// Security is enforced via signature verification.
//
// ROUTE: POST /webhooks/payout
// NO AUTH REQUIRED (gateway callback)
func (h *PayoutWebhookHandler) HandlePayoutWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// STEP 1: Read request body with size limit
	payload, err := readLimitedBody(c.Request, MaxWebhookPayloadSize)
	if err != nil {
		h.log.Error("Failed to read webhook payload", zap.Error(err))
		response.BadRequest(c, "Invalid request body")
		return
	}

	// STEP 2: Extract signature from header
	signature := c.GetHeader(WebhookSignatureHeader)

	// CRITICAL SECURITY: Reject webhooks without signature
	// This is a fail-closed approach - if signature verification is not properly
	// configured, we reject ALL webhooks rather than accepting unverified callbacks
	// that could forge payout settlements.
	if signature == "" {
		h.log.Error("Webhook rejected: missing signature header",
			zap.Bool("has_verifier", h.verifier != nil),
			zap.Bool("has_secret_key", h.verifier != nil && h.verifier.SecretKey != ""),
			zap.Bool("is_sandbox", h.isSandboxMode),
		)
		response.Unauthorized(c, "Missing signature header - webhook signature verification required")
		return
	}

	// CRITICAL SECURITY: Reject if verifier is not properly configured
	if h.verifier == nil || h.verifier.SecretKey == "" {
		h.log.Error("Webhook rejected: signature verification not configured",
			zap.Bool("has_verifier", h.verifier != nil),
			zap.Bool("is_sandbox", h.isSandboxMode),
			zap.String("gateway_provider", h.gatewayProvider),
		)
		response.Unauthorized(c, "Webhook signature verification not configured - cannot accept callbacks")
		return
	}

	// STEP 3: Verify signature (always required after above checks)
	if !h.verifier.VerifySignature(payload, signature) {
		h.log.Error("Webhook signature verification failed",
			zap.String("gateway_provider", h.gatewayProvider),
		)
		response.Unauthorized(c, "Invalid signature")
		return
	}
	h.log.Info("Webhook signature verified successfully",
		zap.String("gateway_provider", h.gatewayProvider),
	)

	// STEP 4: Parse webhook payload
	callback, err := worker.ParseWebhookPayload(payload)
	if err != nil {
		h.log.Error("Failed to parse webhook payload", zap.Error(err))
		response.BadRequest(c, "Invalid payload format")
		return
	}

	// STEP 5: Process callback within a transaction
	err = h.db.Pgx().WithTx(ctx, func(tx db.Tx) error {
		return h.webhookHandler.HandleCallback(ctx, tx, *callback)
	})

	// Handle specific error cases
	if err != nil {
		// Idempotent callback - not an error
		if err == worker.ErrDuplicateCallback {
			h.log.Info("Duplicate webhook callback ignored",
				zap.String("external_ref", callback.ExternalReferenceID),
			)
			// Return 200 OK for idempotent callbacks
			response.Success(c, gin.H{
				"message":   "Callback ignored (already processed)",
				"idempotent": true,
			})
			return
		}

		// Invalid state transition
		if _, ok := err.(*worker.InvalidStateTransitionError); ok {
			h.log.Warn("Webhook rejected: invalid state transition",
				zap.String("external_ref", callback.ExternalReferenceID),
				zap.Error(err),
			)
			// Return 200 OK with warning (we acknowledged, but didn't apply)
			response.Success(c, gin.H{
				"message": "Callback acknowledged but not applied (invalid state transition)",
				"warning":  err.Error(),
			})
			return
		}

		// Other errors
		h.log.Error("Failed to process webhook callback",
			zap.String("external_ref", callback.ExternalReferenceID),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to process callback")
		return
	}

	// STEP 6: Success response
	h.log.Info("Webhook callback processed successfully",
		zap.String("external_ref", callback.ExternalReferenceID),
		zap.String("status", string(callback.Status)),
	)

	response.Success(c, gin.H{
		"message":             "Webhook processed successfully",
		"external_reference_id": callback.ExternalReferenceID,
	})
}

// HandleHealthCheck returns the health status of the payout webhook handler.
// ROUTE: GET /webhooks/payout/health
//
// OBSERVABILITY: Exposes gateway configuration for production safety awareness.
// This is critical for operators to know whether payouts are running in sandbox
// or production mode, and whether signature verification is properly configured.
func (h *PayoutWebhookHandler) HandleHealthCheck(c *gin.Context) {
	// Determine the actual mode for display
	mode := "sandbox"
	if h.isProductionMode {
		mode = "production"
	}

	// Determine signature verification status
	signatureVerificationStatus := "not_configured"
	if h.verifier != nil && h.verifier.SecretKey != "" {
		signatureVerificationStatus = "configured"
	}

	// Determine overall webhook readiness
	webhookReady := h.verifier != nil && h.verifier.SecretKey != ""

	status := map[string]interface{}{
		"status":                       "healthy",
		"webhook_configured":          h.verifier != nil,
		"signature_verification":      signatureVerificationStatus,
		"signature_required":          true, // Always required for security
		"webhook_ready_for_callbacks": webhookReady,
		// Gateway observability - CRITICAL for production safety
		"gateway": map[string]interface{}{
			"provider":            h.gatewayProvider,
			"mode":                mode,
			"is_sandbox":          h.isSandboxMode,
			"is_production":       h.isProductionMode,
			"experimental_status": getGatewayExperimentalStatus(h.gatewayProvider),
		},
		// Security warnings for operators
		"security_warnings": getSecurityWarnings(h.verifier, h.gatewayProvider, h.isProductionMode),
	}

	response.Success(c, status)
}

// getGatewayExperimentalStatus returns the experimental status of a gateway provider
func getGatewayExperimentalStatus(provider string) string {
	switch provider {
	case "sandbox":
		return "verified" // Sandbox is our internal implementation
	case "midtrans_payout":
		return "unverified" // Midtrans payout is not yet verified
	default:
		return "unknown"
	}
}

// getSecurityWarnings returns security warnings based on current configuration
func getSecurityWarnings(verifier *worker.WebhookSignatureVerifier, gatewayProvider string, isProductionMode bool) []string {
	warnings := []string{}

	if verifier == nil || verifier.SecretKey == "" {
		warnings = append(warnings, "Webhook signature verification is not configured - callbacks will be REJECTED")
	}

	if gatewayProvider == "midtrans_payout" {
		warnings = append(warnings, "Midtrans payout integration is EXPERIMENTAL - provider assumptions not yet verified")
	}

	if isProductionMode && gatewayProvider == "midtrans_payout" {
		warnings = append(warnings, "CRITICAL: Midtrans payout should NOT be used in production until verified")
	}

	if len(warnings) == 0 {
		return nil
	}
	return warnings
}

// ============================================================================
// OBSERVABILITY ENDPOINTS
// ============================================================================

// GetWebhookStats returns statistics about webhook processing.
// ROUTE: GET /api/v1/admin/payouts/webhooks/stats (admin only)
func (h *PayoutWebhookHandler) GetWebhookStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get status counts
	statusCounts, err := h.withdrawRepo.GetStatusCounts(ctx, nil)
	if err != nil {
		h.log.Error("Failed to get status counts", zap.Error(err))
		response.InternalServerError(c, "Failed to get statistics")
		return
	}

	response.Success(c, gin.H{
		"status_counts": statusCounts,
	})
}

// ============================================================================
// HELPERS
// ============================================================================

// readLimitedBody reads the request body with a size limit.
func readLimitedBody(r *http.Request, maxBytes int64) ([]byte, error) {
	// Limit reader to maxBytes
	limitedReader := io.LimitReader(r.Body, maxBytes)

	// Read the payload
	payload, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	// Check if we hit the limit
	if len(payload) == int(maxBytes) {
		// Read one more byte to check if there's more data
		oneMore := make([]byte, 1)
		n, _ := r.Body.Read(oneMore)
		if n > 0 {
			return nil, &http.MaxBytesError{}
		}
	}

	return payload, nil
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// SANDBOX TESTING UTILS
// ============================================================================

// MockWebhookTestHandler allows testing webhooks without a real gateway.
// This is a development-only endpoint for testing the webhook flow.
// ROUTE: POST /api/v1/admin/payouts/webhooks/test (admin only, dev mode only)
func (h *PayoutWebhookHandler) MockWebhookTestHandler(c *gin.Context) {
	var req struct {
		ExternalReferenceID string `json:"external_reference_id" binding:"required"`
		Status              string `json:"status" binding:"required,oneof=SUCCESS PENDING FAILED REJECTED"`
		Message             string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Create mock callback
	callback := worker.WebhookCallback{
		ExternalReferenceID: req.ExternalReferenceID,
		GatewayReferenceID:   "MOCK_TEST_" + req.ExternalReferenceID,
		Status:               worker.WebhookStatus(req.Status),
		Message:              req.Message,
		Timestamp:            0,
		RawPayload:           "{}",
	}

	// Process the callback
	ctx := c.Request.Context()
	err := h.db.Pgx().WithTx(ctx, func(tx db.Tx) error {
		return h.webhookHandler.HandleCallback(ctx, tx, callback)
	})

	if err != nil {
		if err == worker.ErrDuplicateCallback {
			response.Success(c, gin.H{
				"message":    "Duplicate callback (idempotent)",
				"idempotent": true,
			})
			return
		}
		h.log.Error("Mock webhook test failed", zap.Error(err))
		response.InternalServerError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message":             "Mock webhook processed successfully",
		"external_reference_id": req.ExternalReferenceID,
		"status":              req.Status,
	})
}

// GenerateTestSignature generates a signature for testing webhook endpoints.
// ROUTE: POST /api/v1/admin/payouts/webhooks/sign (admin only, dev mode only)
func (h *PayoutWebhookHandler) GenerateTestSignature(c *gin.Context) {
	var req struct {
		Payload string `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Generate signature
	var secretKey string
	if h.verifier != nil {
		secretKey = h.verifier.SecretKey
	}
	signature := worker.GenerateSignature([]byte(req.Payload), secretKey)

	response.Success(c, gin.H{
		"signature": signature,
		"header":    WebhookSignatureHeader + ": " + signature,
	})
}


