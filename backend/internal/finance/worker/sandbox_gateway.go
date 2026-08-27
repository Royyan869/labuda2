package worker

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SandboxGatewayConfig configures the sandbox gateway behavior.
type SandboxGatewayConfig struct {
	// SecretKey for webhook signature verification
	SecretKey string

	// BaseURL for the sandbox gateway API (for future real integration)
	BaseURL string

	// SimulateRealGateway when true, makes actual HTTP calls to sandbox API
	// when false, uses in-memory simulation (default: false)
	SimulateRealGateway bool

	// SimulateLatency adds artificial delay to simulate network latency
	SimulateLatency time.Duration

	// FailureRate is the probability (0.0-1.0) of simulating a failure
	FailureRate float64

	// AlwaysSucceed disables all failure simulation
	AlwaysSucceed bool

	// Log enables detailed logging
	Log *zap.Logger
}

// DefaultSandboxGatewayConfig returns default sandbox configuration.
func DefaultSandboxGatewayConfig() SandboxGatewayConfig {
	return SandboxGatewayConfig{
		SimulateLatency:     100 * time.Millisecond,
		FailureRate:         0.0, // No failures by default in sandbox
		AlwaysSucceed:       true,
		SimulateRealGateway: false,
	}
}

// SandboxPayoutGateway is a sandbox-safe implementation of PayoutGateway.
//
// SECURITY FEATURES:
// - Signature verification for webhooks
// - Idempotent submission via external_reference_id
// - Configurable failure simulation for testing
// - Safe for sandbox integration testing
//
// For production use, this should be replaced with a real gateway implementation
// that maintains the same interface and security guarantees.
type SandboxPayoutGateway struct {
	config    SandboxGatewayConfig
	processed map[string]*PayoutGatewayResponse
	client    *http.Client
	log       *zap.Logger
}

// NewSandboxPayoutGateway creates a new sandbox payout gateway.
func NewSandboxPayoutGateway(config SandboxGatewayConfig) *SandboxPayoutGateway {
	if config.SimulateLatency == 0 {
		config.SimulateLatency = 100 * time.Millisecond
	}

	log := config.Log
	if log == nil {
		log = zap.NewNop()
	}

	return &SandboxPayoutGateway{
		config:    config,
		processed: make(map[string]*PayoutGatewayResponse),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// SubmitPayout submits a payout to the sandbox gateway.
//
// This implementation is idempotent - the same external_reference_id returns
// the same response without executing a new payout.
func (s *SandboxPayoutGateway) SubmitPayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	// Check for idempotency - if already processed, return cached response
	if resp, exists := s.processed[req.ExternalReferenceID]; exists {
		s.log.Debug("Sandbox gateway: returning cached response for idempotency",
			zap.String("external_ref", req.ExternalReferenceID),
		)
		return resp, nil
	}

	// If configured to use real sandbox API, make HTTP call
	if s.config.SimulateRealGateway && s.config.BaseURL != "" {
		return s.submitToRealSandbox(ctx, req)
	}

	// Otherwise, use in-memory simulation
	return s.simulatePayout(ctx, req)
}

// simulatePayout simulates a payout without making real API calls.
func (s *SandboxPayoutGateway) simulatePayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	// Simulate network latency
	if s.config.SimulateLatency > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.config.SimulateLatency):
		}
	}

	// Build response
	resp := &PayoutGatewayResponse{
		GatewayReferenceID: "SANDBOX_" + uuid.New().String(),
		Message:            "Payout processed by sandbox gateway (simulated)",
	}

	// Simulate failure if configured
	if !s.config.AlwaysSucceed && shouldSimulateFailure(s.config.FailureRate) {
		resp.Status = PayoutResponseStatusFailed
		resp.Message = "Sandbox gateway: Simulated failure"

		// Determine if error is retryable or permanent
		if shouldSimulateFailure(0.3) {
			resp.ErrorType = ErrorTypePermanent
			resp.Message = "Sandbox gateway: Invalid bank account (permanent error)"
		} else {
			resp.ErrorType = ErrorTypeRetryable
			resp.Message = "Sandbox gateway: Temporary network error (retryable)"
		}

		s.log.Warn("Sandbox gateway: simulated failure",
			zap.String("external_ref", req.ExternalReferenceID),
			zap.String("error_type", string(resp.ErrorType)),
		)
	} else {
		resp.Status = PayoutResponseStatusSuccess
		resp.Message = "Payout submitted successfully to sandbox"
		resp.ErrorType = "" // No error
	}

	// Build raw response JSON
	rawResp := map[string]interface{}{
		"status":               string(resp.Status),
		"gateway_reference_id": resp.GatewayReferenceID,
		"message":              resp.Message,
		"timestamp":            time.Now().Unix(),
		"sandbox":              true,
		"simulated":            !s.config.SimulateRealGateway,
	}
	rawJSON, _ := json.Marshal(rawResp)
	resp.RawResponse = string(rawJSON)

	// Store for idempotency
	s.processed[req.ExternalReferenceID] = resp

	s.log.Info("Sandbox gateway: payout processed",
		zap.String("external_ref", req.ExternalReferenceID),
		zap.String("gateway_ref", resp.GatewayReferenceID),
		zap.String("status", string(resp.Status)),
	)

	return resp, nil
}

// submitToRealSandbox makes an actual HTTP call to the sandbox gateway.
// This is a placeholder for real sandbox integration.
func (s *SandboxPayoutGateway) submitToRealSandbox(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	// Build request payload
	payload := map[string]interface{}{
		"external_id":    req.ExternalReferenceID,
		"amount":         req.Amount,
		"currency":       req.Currency,
		"bank_code":      req.BankName,
		"account_number": req.AccountNumber,
		"account_holder": req.AccountHolder,
		"metadata":       req.Metadata,
	}

	_, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	url := s.config.BaseURL + "/payouts"
	_, err = http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// For now, fallback to simulation since we don't have a real sandbox endpoint
	s.log.Warn("Real sandbox gateway configured but no actual endpoint implemented, using simulation",
		zap.String("base_url", s.config.BaseURL),
	)
	return s.simulatePayout(ctx, req)
}

// ============================================================================
// WEBHOOK SIGNATURE VERIFICATION
// ============================================================================

// WebhookSignatureVerifier handles webhook signature verification.
type WebhookSignatureVerifier struct {
	SecretKey string // Exported for testing
	log       *zap.Logger
}

// NewWebhookSignatureVerifier creates a new signature verifier.
func NewWebhookSignatureVerifier(SecretKey string, log *zap.Logger) *WebhookSignatureVerifier {
	if log == nil {
		log = zap.NewNop()
	}
	return &WebhookSignatureVerifier{
		SecretKey: SecretKey,
		log:       log,
	}
}

// VerifySignature verifies the HMAC signature of a webhook payload.
//
// The signature is expected to be in the header as:
//   X-Webhook-Signature: sha256=<hex_digest>
//
// Returns:
// - true: signature is valid
// - false: signature is invalid
func (v *WebhookSignatureVerifier) VerifySignature(payload []byte, signatureHeader string) bool {
	if v.SecretKey == "" {
		// If no secret key is configured, reject all signatures for security
		v.log.Warn("Webhook signature verification skipped - no secret key configured")
		return false
	}

	if signatureHeader == "" {
		v.log.Warn("Webhook missing signature header")
		return false
	}

	// Parse signature header (format: "sha256=<hex_digest>")
	expectedPrefix := "sha256="
	if len(signatureHeader) <= len(expectedPrefix) {
		v.log.Warn("Invalid signature format", zap.Int("header_len", len(signatureHeader)))
		return false
	}

	signature := signatureHeader[len(expectedPrefix):]

	// Compute HMAC SHA256 of payload
	h := hmac.New(sha256.New, []byte(v.SecretKey))
	h.Write(payload)
	computedSignature := hex.EncodeToString(h.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	valid := hmacEqual([]byte(signature), []byte(computedSignature))

	if !valid {
		v.log.Warn("Webhook signature verification failed",
			zap.String("expected_prefix", signature[:min(len(signature), 8)]+"..."),
			zap.String("computed_prefix", computedSignature[:min(len(computedSignature), 8)]+"..."),
		)
		return false
	}

	v.log.Debug("Webhook signature verified successfully")
	return true
}

// VerifySignatureMiddleware returns a middleware that verifies webhook signatures.
func (v *WebhookSignatureVerifier) VerifySignatureMiddleware(next func([]byte) error) func([]byte, string) error {
	return func(payload []byte, signatureHeader string) error {
		if !v.VerifySignature(payload, signatureHeader) {
			return fmt.Errorf("invalid webhook signature")
		}
		return next(payload)
	}
}

// hmacEqual compares two HMAC values in constant time.
func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}

// ============================================================================
// WEBHOOK HTTP HANDLER
// ============================================================================

// WebhookHTTPHandler handles HTTP webhook requests from the payout gateway.
type WebhookHTTPHandler struct {
	verifier       *WebhookSignatureVerifier
	webhookHandler *WebhookHandler
	log            *zap.Logger
}

// NewWebhookHTTPHandler creates a new HTTP webhook handler.
func NewWebhookHTTPHandler(
	SecretKey string,
	webhookHandler *WebhookHandler,
	log *zap.Logger,
) *WebhookHTTPHandler {
	if log == nil {
		log = zap.NewNop()
	}

	return &WebhookHTTPHandler{
		verifier:       NewWebhookSignatureVerifier(SecretKey, log),
		webhookHandler: webhookHandler,
		log:            log,
	}
}

// HandleWebhook handles an incoming webhook request.
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
func (h *WebhookHTTPHandler) HandleWebhook(payload []byte, signatureHeader string) error {
	// STEP 1: Verify signature
	if !h.verifier.VerifySignature(payload, signatureHeader) {
		h.log.Error("Webhook rejected: invalid signature")
		return fmt.Errorf("invalid webhook signature")
	}

	// STEP 2: Parse payload
	var callback WebhookCallback
	if err := json.Unmarshal(payload, &callback); err != nil {
		h.log.Error("Failed to parse webhook payload", zap.Error(err))
		return fmt.Errorf("invalid payload: %w", err)
	}

	// STEP 3: Validate required fields
	if callback.ExternalReferenceID == "" {
		h.log.Error("Webhook missing external_reference_id")
		return fmt.Errorf("missing external_reference_id")
	}

	if callback.Status == "" {
		h.log.Error("Webhook missing status")
		return fmt.Errorf("missing status")
	}

	// STEP 4: Store raw payload for audit
	callback.RawPayload = string(payload)

	// STEP 5: Process callback (delegated to WebhookHandler)
	// Note: This needs a transaction context, which will be provided by the HTTP layer
	return nil
}

// ParseWebhookPayload parses a webhook payload from JSON bytes.
// This function supports both generic webhook format and Midtrans payout format.
func ParseWebhookPayload(payload []byte) (*WebhookCallback, error) {
	// First, try to detect if this is a Midtrans webhook by checking for Midtrans-specific fields
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	// Check for Midtrans payout webhook format
	// Midtrans uses "external_id" instead of "external_reference_id"
	if _, hasExternalID := raw["external_id"]; hasExternalID {
		return parseMidtransWebhook(payload, raw)
	}

	// Otherwise, parse as generic webhook format
	var callback WebhookCallback
	if err := json.Unmarshal(payload, &callback); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	// Validate required fields
	if callback.ExternalReferenceID == "" {
		return nil, fmt.Errorf("missing external_reference_id")
	}

	if callback.Status == "" {
		return nil, fmt.Errorf("missing status")
	}

	callback.RawPayload = string(payload)
	return &callback, nil
}

// parseMidtransWebhook parses a Midtrans payout webhook and converts to generic format
func parseMidtransWebhook(payload []byte, raw map[string]interface{}) (*WebhookCallback, error) {
	// Extract Midtrans fields
	externalID, _ := raw["external_id"].(string)
	id, _ := raw["id"].(string)
	status, _ := raw["status"].(string)
	statusMessage, _ := raw["status_message"].(string)

	// Validate required fields
	if externalID == "" {
		return nil, fmt.Errorf("missing external_id in Midtrans webhook")
	}

	if status == "" {
		return nil, fmt.Errorf("missing status in Midtrans webhook")
	}

	// Map Midtrans status to webhook status
	var webhookStatus WebhookStatus
	switch status {
	case "SUCCESS":
		webhookStatus = WebhookStatusSuccess
	case "PENDING":
		webhookStatus = WebhookStatusPending
	case "FAILED":
		webhookStatus = WebhookStatusFailed
	default:
		webhookStatus = WebhookStatusUnknown
	}

	// Build generic callback
	callback := &WebhookCallback{
		ExternalReferenceID: externalID,
		GatewayReferenceID:  id,
		Status:              webhookStatus,
		Message:             statusMessage,
		Timestamp:           time.Now().Unix(),
		RawPayload:          string(payload),
	}

	return callback, nil
}

// ============================================================================
// SIGNATURE GENERATION (for testing)
// ============================================================================

// GenerateSignature generates an HMAC SHA256 signature for a webhook payload.
// This is useful for testing webhook endpoints.
func GenerateSignature(payload []byte, SecretKey string) string {
	h := hmac.New(sha256.New, []byte(SecretKey))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// ============================================================================
// IDempotency helpers
// ============================================================================

// ClearProcessed clears the processed map (useful for testing).
func (s *SandboxPayoutGateway) ClearProcessed() {
	s.processed = make(map[string]*PayoutGatewayResponse)
}

// GetProcessed returns the number of unique payouts processed.
func (s *SandboxPayoutGateway) GetProcessed() int {
	return len(s.processed)
}

// GetProcessedIDs returns the external reference IDs of all processed payouts.
func (s *SandboxPayoutGateway) GetProcessedIDs() []string {
	ids := make([]string, 0, len(s.processed))
	for id := range s.processed {
		ids = append(ids, id)
	}
	return ids
}

// ============================================================================
// REAL GATEWAY ADAPTER INTERFACE (for future implementation)
// ============================================================================

// RealPayoutGateway defines the interface for a real payment gateway.
// This can be implemented for future payout providers.
type RealPayoutGateway interface {
	SubmitPayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error)
	VerifyWebhook(payload []byte, signature string) bool
}

// Note: For future payout providers, implement the PayoutGateway interface.

// ============================================================================
// HEALTH CHECK
// ============================================================================

// HealthCheck returns the health status of the sandbox gateway.
func (s *SandboxPayoutGateway) HealthCheck(ctx context.Context) error {
	// Sandbox gateway is always healthy
	return nil
}

// GetStatus returns the current status of the sandbox gateway.
func (s *SandboxPayoutGateway) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "sandbox",
		"simulated":            !s.config.SimulateRealGateway,
		"processed_count":      len(s.processed),
		"simulate_real_gateway": s.config.SimulateRealGateway,
		"base_url":             s.config.BaseURL,
	}
}

// ============================================================================
// BRIDGE TYPES
// ============================================================================

// ToApplicationResponse converts worker response to application response format.
// This bridges between worker.PayoutGatewayResponse and application.PayoutResponse.
func ToApplicationResponse(resp *PayoutGatewayResponse) map[string]interface{} {
	return map[string]interface{}{
		"status":               string(resp.Status),
		"gateway_reference_id": resp.GatewayReferenceID,
		"message":              resp.Message,
		"raw_response":         resp.RawResponse,
		"error_type":           string(resp.ErrorType),
	}
}

// FromApplicationRequest converts application request to worker request format.
func FromApplicationRequest(req map[string]interface{}) PayoutGatewayRequest {
	return PayoutGatewayRequest{
		ExternalReferenceID: getString(req, "external_reference_id"),
		Amount:              getInt64(req, "amount"),
		Currency:            getString(req, "currency"),
		BankName:            getString(req, "bank_name"),
		AccountNumber:       getString(req, "account_number"),
		AccountHolder:       getString(req, "account_holder"),
		Metadata:            getMapString(req, "metadata"),
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		}
	}
	return 0
}

func getMapString(m map[string]interface{}, key string) map[string]string {
	if v, ok := m[key]; ok {
		if m2, ok := v.(map[string]string); ok {
			return m2
		}
	}
	return nil
}

// ReadAndLimitBody reads the request body with size limit.
func ReadAndLimitBody(r io.Reader, maxBytes int64) ([]byte, error) {
	limitedReader := io.LimitReader(r, maxBytes)
	return io.ReadAll(limitedReader)
}


