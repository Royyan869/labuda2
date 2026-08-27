package midtrans

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

// ============================================================================
// MIDTRANS IRIS PAYOUT CLIENT
// ============================================================================
//
// Handles OUTBOUND payouts (platform → seller) via Midtrans Iris.
// Iris is Midtrans's dedicated disbursement product — separate from the
// Core API / Snap that handles INCOMING payments (buyer → platform).
//
// TASK 58 RUNTIME EVIDENCE:
// - api.sandbox.midtrans.com/v2/disbursements → HTTP 404 (wrong URL)
// - app.sandbox.midtrans.com/iris/api/v1/payouts → HTTP 401 (correct URL, needs Iris key)
// - app.sandbox.midtrans.com/iris/api/v1/balance → HTTP 401 (confirms Iris base URL)
//
// IRIS CREDENTIALS:
// - Iris uses SEPARATE credentials from Core API / Snap.
// - Mid-server-* keys are REJECTED by Iris (HTTP 401).
// - Operator key: creates payouts (MIDTRANS_IRIS_OPERATOR_KEY)
// - Approver key: approves queued payouts (MIDTRANS_IRIS_APPROVER_KEY)
//
// IRIS API:
// - Sandbox base: https://app.sandbox.midtrans.com/iris/api/v1
// - Production base: https://app.midtrans.com/iris/api/v1
//   (production base assumed from sandbox URL pattern — NOT runtime-verified)
// - Payout creation:  POST /payouts   (operator key)
// - Payout approval:  POST /payouts/approve  (approver key)
// - Status check:     GET  /payouts/{external_id}
// - Balance:          GET  /balance
//
// ============================================================================

const (
	// Midtrans Iris sandbox base — runtime-proven reachable (TASK 58)
	sandboxIrisBase = "https://app.sandbox.midtrans.com/iris/api/v1"

	// Midtrans Iris production base — assumed from sandbox URL pattern.
	// NOT runtime-verified. Production gate requires PAYOUT_ENABLE_PRODUCTION=true.
	productionIrisBase = "https://app.midtrans.com/iris/api/v1"

	sandboxPayoutURL    = sandboxIrisBase + "/payouts"
	productionPayoutURL = productionIrisBase + "/payouts"

	sandboxPayoutStatusURL    = sandboxIrisBase
	productionPayoutStatusURL = productionIrisBase
)

// PayoutCircuitBreaker implements circuit breaker for payout API calls
// Payouts have different failure modes than payment collection
type PayoutCircuitBreaker struct {
	mu           sync.RWMutex
	state        CircuitState
	failures     int
	lastFailTime time.Time
	halfOpenCount int
	openUntil    time.Time
}

// newPayoutCircuitBreaker creates a new circuit breaker for payouts
func newPayoutCircuitBreaker() *PayoutCircuitBreaker {
	return &PayoutCircuitBreaker{
		state:    CircuitClosed,
		failures: 0,
	}
}

// allowRequest returns true if request should proceed
func (cb *PayoutCircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	if cb.state == CircuitOpen {
		if now.After(cb.openUntil) {
			cb.state = CircuitHalfOpen
			cb.halfOpenCount = 0
			return true
		}
		return false
	}

	if cb.state == CircuitHalfOpen {
		if cb.halfOpenCount < halfOpenAttempts {
			cb.halfOpenCount++
			return true
		}
		return false
	}

	return true
}

// onSuccess records a successful payout request
func (cb *PayoutCircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
	}
	cb.failures = 0
	cb.halfOpenCount = 0
}

// onFailure records a failed payout request
func (cb *PayoutCircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= maxFailures {
		cb.state = CircuitOpen
		cb.openUntil = time.Now().Add(openTimeout)
	}
}

// getState returns the current circuit state
func (cb *PayoutCircuitBreaker) getState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// ============================================================================
// IRIS PAYOUT REQUEST / RESPONSE TYPES
// ============================================================================
//
// Field names match Midtrans Iris API (task 59 spec).
// Amount is a string in Iris (not a number).
// Payouts are submitted as an array (Iris batch format).
//
// NOTE: Iris status values are UNVERIFIED at runtime (blocked by missing Iris
// credentials in TASK 58). Status values below are from Iris documentation:
//   "queued"     — submitted, awaiting approval
//   "processed"  — approved and disbursed
//   "failed"     — disbursement failed
// Status mapping to internal values is in MapToGatewayStatus().
//
// ============================================================================

// IrisPayoutItem is a single payout in an Iris batch request
type IrisPayoutItem struct {
	BeneficiaryName    string `json:"beneficiary_name"`
	BeneficiaryAccount string `json:"beneficiary_account"`
	BeneficiaryBank    string `json:"beneficiary_bank"`
	// Amount is a string in Iris API (not a number)
	Amount             string `json:"amount"`
	Notes              string `json:"notes,omitempty"`
	ExternalID         string `json:"external_id,omitempty"`
	BeneficiaryEmail   string `json:"beneficiary_email,omitempty"`
	PhoneNumber        string `json:"phone_number,omitempty"`
}

// IrisCreatePayoutRequest is POST /payouts body
type IrisCreatePayoutRequest struct {
	Payouts []IrisPayoutItem `json:"payouts"`
}

// IrisPayoutResponseItem is one payout record in Iris response
type IrisPayoutResponseItem struct {
	Status             string `json:"status"`
	ExternalID         string `json:"external_id"`
	Amount             string `json:"amount"`
	BeneficiaryName    string `json:"beneficiary_name"`
	BeneficiaryAccount string `json:"beneficiary_account"`
	BeneficiaryBank    string `json:"beneficiary_bank"`
	Notes              string `json:"notes,omitempty"`
	ReferenceNo        string `json:"reference_no,omitempty"`
	FailedReason       string `json:"failed_reason,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// IrisCreatePayoutResponse is POST /payouts response body
type IrisCreatePayoutResponse struct {
	Payouts []IrisPayoutResponseItem `json:"payouts"`
}

// PayoutRequest is the legacy alias kept for gateway adapter compatibility.
// New code should build IrisCreatePayoutRequest directly.
type PayoutRequest = IrisPayoutItem

// PayoutResponse wraps a single Iris payout result for gateway adapter use.
type PayoutResponse struct {
	// Status — Iris values: "queued", "processed", "failed" (UNVERIFIED at runtime)
	Status     string `json:"status"`
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	// Amount is a string (Iris format)
	Amount        string   `json:"amount"`
	BeneficiaryBank string `json:"beneficiary_bank"`
	ReferenceNo   string   `json:"reference_no,omitempty"`
	FailedReason  string   `json:"failed_reason,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	// ErrorMessages preserved for error classification
	ErrorMessages []string `json:"error_messages,omitempty"`
}

// PayoutStatusResponse is GET /payouts/{external_id} response
type PayoutStatusResponse struct {
	Status             string `json:"status"`
	ExternalID         string `json:"external_id"`
	Amount             string `json:"amount"`
	BeneficiaryName    string `json:"beneficiary_name"`
	BeneficiaryAccount string `json:"beneficiary_account"`
	BeneficiaryBank    string `json:"beneficiary_bank"`
	ReferenceNo        string `json:"reference_no,omitempty"`
	FailedReason       string `json:"failed_reason,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// PayoutClient is the Midtrans Iris payout client
type PayoutClient struct {
	irisOperatorKey string
	isProduction    bool
	httpClient      *http.Client
	log             *logger.Logger
	cb              *PayoutCircuitBreaker
}

// PayoutClientConfig configures the Iris payout client
type PayoutClientConfig struct {
	// IrisOperatorKey is the Midtrans Iris operator key (creates payouts).
	// Obtained from the Iris dashboard — NOT the same as Mid-server-* Core API key.
	IrisOperatorKey string
	IsProduction    bool
}

// NewPayoutClient creates a Midtrans Iris payout client.
func NewPayoutClient(cfg *PayoutClientConfig, log *logger.Logger) *PayoutClient {
	if cfg == nil {
		cfg = &PayoutClientConfig{}
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}

	return &PayoutClient{
		irisOperatorKey: cfg.IrisOperatorKey,
		isProduction:    cfg.IsProduction,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		log: log,
		cb:  newPayoutCircuitBreaker(),
	}
}

// SubmitPayout submits a single payout to Midtrans Iris POST /payouts.
// The item is wrapped in the Iris batch envelope automatically.
func (c *PayoutClient) SubmitPayout(ctx context.Context, req *IrisPayoutItem) (*PayoutResponse, error) {
	if c.irisOperatorKey == "" {
		return nil, fmt.Errorf("Midtrans Iris credentials missing: MIDTRANS_IRIS_OPERATOR_KEY not set")
	}

	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans Iris payout circuit breaker is open",
			zap.String("state", c.cb.getState().String()),
		)
		return nil, fmt.Errorf("midtrans Iris payout circuit breaker is %s - service unavailable", c.cb.getState())
	}

	// Iris expects batch format: {"payouts": [...]}
	batchReq := IrisCreatePayoutRequest{Payouts: []IrisPayoutItem{*req}}

	jsonData, err := json.Marshal(batchReq)
	if err != nil {
		return nil, fmt.Errorf("marshal Iris payout request: %w", err)
	}

	c.log.Info("Submitting payout to Midtrans Iris",
		zap.String("external_id", req.ExternalID),
		zap.String("amount", req.Amount),
		zap.String("beneficiary_bank", req.BeneficiaryBank),
		zap.String("beneficiary_account", maskAccountNumber(req.BeneficiaryAccount)),
	)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.getPayoutURL(), bytes.NewBuffer(jsonData))
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("create Iris payout request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.irisOperatorKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("send Iris payout request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("read Iris payout response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.cb.onFailure()
		c.log.Error("Midtrans Iris payout API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
			zap.String("external_id", req.ExternalID),
		)
		return nil, fmt.Errorf("midtrans Iris payout API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var batchResp IrisCreatePayoutResponse
	if err := json.Unmarshal(body, &batchResp); err != nil {
		return nil, fmt.Errorf("unmarshal Iris payout response: %w", err)
	}

	if len(batchResp.Payouts) == 0 {
		return nil, fmt.Errorf("Iris payout response contained no payout items")
	}

	c.cb.onSuccess()

	item := batchResp.Payouts[0]
	payoutResp := &PayoutResponse{
		Status:          item.Status,
		ExternalID:      item.ExternalID,
		Amount:          item.Amount,
		BeneficiaryBank: item.BeneficiaryBank,
		ReferenceNo:     item.ReferenceNo,
		FailedReason:    item.FailedReason,
		Notes:           item.Notes,
		CreatedAt:       item.CreatedAt,
	}

	c.log.Info("Payout submitted to Midtrans Iris",
		zap.String("external_id", payoutResp.ExternalID),
		zap.String("reference_no", payoutResp.ReferenceNo),
		zap.String("status", payoutResp.Status),
	)

	return payoutResp, nil
}

// GetPayoutStatus checks payout status via GET /payouts/{external_id}.
func (c *PayoutClient) GetPayoutStatus(ctx context.Context, externalID string) (*PayoutStatusResponse, error) {
	if c.irisOperatorKey == "" {
		return nil, fmt.Errorf("Midtrans Iris credentials missing: MIDTRANS_IRIS_OPERATOR_KEY not set")
	}

	if !c.cb.allowRequest() {
		return nil, fmt.Errorf("midtrans Iris payout circuit breaker is %s - service unavailable", c.cb.getState())
	}

	// /disbursements path returned 404 in TASK 58; /payouts/{id} is the correct path
	url := fmt.Sprintf("%s/payouts/%s", c.getPayoutStatusURL(), externalID)

	c.log.Debug("Checking Iris payout status",
		zap.String("external_id", externalID),
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("create Iris status request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.irisOperatorKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("send Iris status request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("read Iris status response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.onFailure()
		return nil, fmt.Errorf("midtrans Iris status API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var statusResp PayoutStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("unmarshal Iris status response: %w", err)
	}

	c.cb.onSuccess()
	return &statusResp, nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// getPayoutURL returns the payout API URL based on environment
func (c *PayoutClient) getPayoutURL() string {
	if c.isProduction {
		return productionPayoutURL
	}
	return sandboxPayoutURL
}

// getPayoutStatusURL returns the payout status API URL based on environment
func (c *PayoutClient) getPayoutStatusURL() string {
	if c.isProduction {
		return productionPayoutStatusURL
	}
	return sandboxPayoutStatusURL
}

// maskAccountNumber masks an account number for logging
func maskAccountNumber(accountNumber string) string {
	if len(accountNumber) <= 4 {
		return "****"
	}
	return accountNumber[:2] + "****" + accountNumber[len(accountNumber)-2:]
}

// ============================================================================
// STATUS MAPPING
// ============================================================================
//
// Iris status values (UNVERIFIED at runtime — Iris credentials unavailable in TASK 58):
//   "queued"     → PENDING  (submitted, awaiting approval)
//   "processed"  → SUCCESS  (approved and disbursed)
//   "failed"     → FAILED
//
// These values come from Iris documentation and must be confirmed when
// Iris credentials are available and a real payout is submitted.
//
// ============================================================================

// IsPending returns true if the payout is queued/pending
func (r *PayoutResponse) IsPending() bool {
	return r.Status == "queued"
}

// IsSuccess returns true if the payout was processed
func (r *PayoutResponse) IsSuccess() bool {
	return r.Status == "processed"
}

// IsFailed returns true if the payout failed
func (r *PayoutResponse) IsFailed() bool {
	return r.Status == "failed"
}

// MapToGatewayStatus maps Iris status strings to internal gateway status.
// Iris values: "queued", "processed", "failed" (UNVERIFIED at runtime).
func (r *PayoutResponse) MapToGatewayStatus() string {
	switch r.Status {
	case "processed":
		return "SUCCESS"
	case "queued":
		return "PENDING"
	case "failed":
		return "FAILED"
	default:
		return "FAILED"
	}
}

// GetErrorType returns whether the failure is retryable.
// Uses FailedReason field (Iris uses failed_reason, not error_messages).
func (r *PayoutResponse) GetErrorType() string {
	if r.FailedReason != "" && isPermanentError(r.FailedReason) {
		return "PERMANENT"
	}
	for _, msg := range r.ErrorMessages {
		if isPermanentError(msg) {
			return "PERMANENT"
		}
	}
	return "RETRYABLE"
}

// isPermanentError checks if an error message indicates a permanent failure
func isPermanentError(msg string) bool {
	permanentPatterns := []string{
		"invalid account",
		"account not found",
		"account closed",
		"account number",
		"bank code",
		"validation",
		"unauthorized",
	}

	msgLower := toLowerPayout(msg)
	for _, pattern := range permanentPatterns {
		if containsPayout(msgLower, pattern) {
			return true
		}
	}
	return false
}

func toLowerPayout(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func containsPayout(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstringPayout(s, substr) >= 0
}

func indexOfSubstringPayout(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// ============================================================================
// BANK CODE MAPPING
// ============================================================================

// MidtransBankCodes maps internal bank codes to Iris beneficiary_bank values.
// Bank code strings are the same in Iris; only the field name changed (bank_code → beneficiary_bank).
var MidtransBankCodes = map[string]string{
	// Bank codes - must match Midtrans expected values
	"bca"         : "bca",
	"mandiri"     : "mandiri",
	"bri"         : "bri",
	"bni"         : "bni",
	"cimb"        : "cimb",
	"permata"     : "permata",
	"danamon"     : "danamon",
	"muamalat"    : "muamalat",
	"syariah"     : "bsi",
	"bsi"         : "bsi",
	"jago"        : "jago",
	"ocbc"        : "ocbc",
	"btn"         : "btn",
}

// NormalizeBankCode normalizes a bank code to Midtrans format
func NormalizeBankCode(bankCode string) string {
	normalized := toLowerPayout(bankCode)
	if code, ok := MidtransBankCodes[normalized]; ok {
		return code
	}
	return normalized
}

// IsValidBankCode checks if a bank code is supported by Midtrans
func IsValidBankCode(bankCode string) bool {
	normalized := NormalizeBankCode(bankCode)
	_, ok := MidtransBankCodes[normalized]
	return ok
}

// GetSupportedBankCodes returns a list of supported bank codes
func GetSupportedBankCodes() []string {
	codes := make([]string, 0, len(MidtransBankCodes))
	for code := range MidtransBankCodes {
		codes = append(codes, code)
	}
	return codes
}
