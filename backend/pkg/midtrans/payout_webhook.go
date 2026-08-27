package midtrans

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ============================================================================
// MIDTRANS PAYOUT WEBHOOK HANDLING
// ============================================================================
//
// Midtrans sends webhook callbacks for disbursement status changes.
// This module provides parsing and status mapping for Midtrans payout webhooks.
//
// MIDTRANS DISBURSEMENT WEBHOOK FORMAT (simplified):
// {
//   "id": "disbursement-id",
//   "external_id": "our-external-ref",
//   "amount": 100000,
//   "status": "PENDING|SUCCESS|FAILED",
//   "bank_code": "bca",
//   "account_number": "1234567890",
//   "transaction_time": "2025-01-01 10:00:00",
//   "completed_time": "2025-01-01 10:05:00",
//   "receipt_url": "...",
//   "status_message": "..."
// }
//
// ============================================================================

// PayoutWebhookPayload represents a Midtrans payout/disbursement webhook callback
type PayoutWebhookPayload struct {
	// ID is Midtrans's internal reference for the disbursement
	ID string `json:"id"`

	// ExternalID is our reference (echoed back from submission)
	ExternalID string `json:"external_id"`

	// Amount is the disbursement amount in IDR
	Amount float64 `json:"amount"`

	// Status of the disbursement
	// Possible values: "PENDING", "SUCCESS", "FAILED"
	Status string `json:"status"`

	// BankCode is the recipient bank code
	BankCode string `json:"bank_code"`

	// AccountNumber is the recipient account number
	AccountNumber string `json:"account_number"`

	// AccountHolderName is the recipient account holder name (optional)
	AccountHolderName string `json:"account_holder_name,omitempty"`

	// TransactionTime is when the disbursement was initiated
	TransactionTime string `json:"transaction_time"`

	// CompletedTime is when the disbursement was completed (if applicable)
	CompletedTime string `json:"completed_time,omitempty"`

	// ReceiptURL contains the disbursement receipt (if applicable)
	ReceiptURL string `json:"receipt_url,omitempty"`

	// Fee charged by Midtrans (if available)
	Fee float64 `json:"fee,omitempty"`

	// StatusMessage contains additional information
	StatusMessage string `json:"status_message,omitempty"`

	// BCA reference number (if applicable)
	BCAReferenceNumber string `json:"bca_reference_number,omitempty"`
}

// PayoutWebhookVerifier handles signature verification for Midtrans payout webhooks
//
// NOTE: Midtrans uses webhook signature for payment notifications, but for
// disbursements, they may use a different mechanism. This verifier provides
// a framework that can be adapted based on actual Midtrans documentation.
type PayoutWebhookVerifier struct {
	secretKey string
	log       *zap.Logger
}

// NewPayoutWebhookVerifier creates a new webhook verifier for payouts
func NewPayoutWebhookVerifier(secretKey string, log *zap.Logger) *PayoutWebhookVerifier {
	if log == nil {
		log = zap.NewNop()
	}
	return &PayoutWebhookVerifier{
		secretKey: secretKey,
		log:       log,
	}
}

// VerifySignature verifies the webhook signature
//
// CRITICAL SECURITY: This method implements FAIL-CLOSED behavior for payout webhooks.
// Midtrans disbursement webhooks control money movement, so we must NOT accept
// unverified callbacks that could forge settlement status.
//
// CURRENT STATUS: EXPERIMENTAL - Midtrans payout webhook signature mechanism is not
// yet verified. Until the exact signature algorithm is confirmed, this method:
// - Rejects all webhooks when signature verification is not properly configured
// - Rejects all webhooks with empty signature headers
// - Implements HMAC SHA256 verification as a reasonable default (needs confirmation)
//
// ACTION REQUIRED: Verify Midtrans disbursement webhook signature mechanism from
// official Midtrans documentation and update this implementation accordingly.
func (v *PayoutWebhookVerifier) VerifySignature(payload []byte, signatureHeader string) bool {
	// CRITICAL: If no secret key configured, reject ALL webhooks
	// This prevents accepting unverified callbacks that could forge settlements
	if v.secretKey == "" {
		v.log.Error("Midtrans payout webhook REJECTED - no secret key configured for signature verification",
			zap.String("security_status", "fail_closed"),
		)
		return false
	}

	// CRITICAL: Reject webhooks without signature header
	// Midtrans MUST provide signature verification for payout webhooks
	// If they don't, we need IP whitelisting or other verification methods
	if signatureHeader == "" {
		v.log.Error("Midtrans payout webhook REJECTED - missing signature header",
			zap.String("security_status", "fail_closed"),
			zap.String("reason", "signature_header_required"),
		)
		return false
	}

	// Parse signature header (format: "sha256=<hex_digest>")
	expectedPrefix := "sha256="
	if len(signatureHeader) <= len(expectedPrefix) {
		v.log.Error("Midtrans payout webhook REJECTED - invalid signature format",
			zap.Int("header_len", len(signatureHeader)),
		)
		return false
	}

	signature := signatureHeader[len(expectedPrefix):]

	// Compute HMAC SHA256 of payload
	// NOTE: This is the standard algorithm for payment webhooks.
	// For disbursements, verify this matches Midtrans documentation.
	h := hmac.New(sha256.New, []byte(v.secretKey))
	h.Write(payload)
	computedSignature := hex.EncodeToString(h.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	valid := hmacEqualMidtrans([]byte(signature), []byte(computedSignature))

	if !valid {
		v.log.Error("Midtrans payout webhook REJECTED - signature verification failed",
			zap.String("provided_signature", signature[:min(len(signature), 8)]+"..."),
			zap.String("computed_signature", computedSignature[:min(len(computedSignature), 8)]+"..."),
		)
		return false
	}

	v.log.Debug("Midtrans payout webhook signature verified")
	return true
}

// hmacEqualMidtrans compares two HMAC values in constant time
func hmacEqualMidtrans(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// STATUS MAPPING
// ============================================================================

// WebhookStatus represents the internal webhook status after mapping
type WebhookStatus string

const (
	WebhookStatusSuccess  WebhookStatus = "SUCCESS"
	WebhookStatusPending  WebhookStatus = "PENDING"
	WebhookStatusFailed   WebhookStatus = "FAILED"
	WebhookStatusRejected WebhookStatus = "REJECTED"
	WebhookStatusUnknown  WebhookStatus = "UNKNOWN"
)

// MapToInternalStatus maps Midtrans status to internal webhook status
func (p *PayoutWebhookPayload) MapToInternalStatus() WebhookStatus {
	switch p.Status {
	case "SUCCESS":
		return WebhookStatusSuccess
	case "PENDING":
		return WebhookStatusPending
	case "FAILED":
		return WebhookStatusFailed
	default:
		return WebhookStatusUnknown
	}
}

// IsRetryable returns whether a failed payout is retryable
func (p *PayoutWebhookPayload) IsRetryable() bool {
	// Most Midtrans payout failures are not retryable via webhook
	// They require fixing the bank account and creating a new payout
	return false
}

// GetFailureReason returns a human-readable failure reason
func (p *PayoutWebhookPayload) GetFailureReason() string {
	if p.StatusMessage != "" {
		return p.StatusMessage
	}

	switch p.Status {
	case "FAILED":
		return "Disbursement failed"
	case "PENDING":
		return "Disbursement is pending"
	case "SUCCESS":
		return "Disbursement completed"
	default:
		return fmt.Sprintf("Unknown status: %s", p.Status)
	}
}

// GetTimestamp returns the webhook timestamp as Unix time
func (p *PayoutWebhookPayload) GetTimestamp() int64 {
	// Try to parse the completed time first
	if p.CompletedTime != "" {
		if t, err := parseMidtransTimestamp(p.CompletedTime); err == nil {
			return t
		}
	}

	// Fall back to transaction time
	if p.TransactionTime != "" {
		if t, err := parseMidtransTimestamp(p.TransactionTime); err == nil {
			return t
		}
	}

	// Default to current time
	return time.Now().Unix()
}

// parseMidtransTimestamp parses Midtrans timestamp format
// Format: "2025-01-01 10:00:00"
func parseMidtransTimestamp(ts string) (int64, error) {
	// Midtrans uses a simple datetime format
	layout := "2006-01-02 15:04:05"
	t, err := time.Parse(layout, ts)
	if err != nil {
		return 0, fmt.Errorf("parse timestamp: %w", err)
	}
	return t.Unix(), nil
}

// ============================================================================
// PARSING
// ============================================================================

// ParsePayoutWebhook parses a Midtrans payout webhook payload
func ParsePayoutWebhook(payload []byte) (*PayoutWebhookPayload, error) {
	var webhook PayoutWebhookPayload
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil, fmt.Errorf("parse Midtrans payout webhook: %w", err)
	}

	// Validate required fields
	if webhook.ID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	if webhook.ExternalID == "" {
		return nil, fmt.Errorf("missing required field: external_id")
	}

	if webhook.Status == "" {
		return nil, fmt.Errorf("missing required field: status")
	}

	return &webhook, nil
}

// ============================================================================
// GATEWAY CALLBACK ADAPTER
// ============================================================================

// GatewayCallback represents the generic callback format expected by the worker
type GatewayCallback struct {
	ExternalReferenceID string
	GatewayReferenceID string
	Status              WebhookStatus
	Message             string
	Timestamp           int64
	RawPayload          string
}

// ToGatewayCallback converts Midtrans webhook to generic gateway callback
func (p *PayoutWebhookPayload) ToGatewayCallback(rawPayload []byte) GatewayCallback {
	return GatewayCallback{
		ExternalReferenceID: p.ExternalID,
		GatewayReferenceID:  p.ID,
		Status:              p.MapToInternalStatus(),
		Message:             p.GetFailureReason(),
		Timestamp:           p.GetTimestamp(),
		RawPayload:          string(rawPayload),
	}
}

// ParseAndConvert parses a Midtrans webhook and converts to generic callback
func ParseAndConvert(payload []byte) (*GatewayCallback, error) {
	webhook, err := ParsePayoutWebhook(payload)
	if err != nil {
		return nil, err
	}

	callback := webhook.ToGatewayCallback(payload)
	return &callback, nil
}

// ============================================================================
// WEBHOOK RESPONSE
// ============================================================================

// WebhookResponse represents the response to send back to Midtrans
type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SuccessResponse returns a success response for Midtrans webhook
func SuccessResponse() WebhookResponse {
	return WebhookResponse{
		Status:  "success",
		Message: "Webhook received",
	}
}

// ErrorResponse returns an error response for Midtrans webhook
func ErrorResponse(message string) WebhookResponse {
	return WebhookResponse{
		Status:  "error",
		Message: message,
	}
}
