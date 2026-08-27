package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

const (
	// Sandbox URLs
	sandboxSnapURL    = "https://app.sandbox.midtrans.com/snap/v1/transactions"
	sandboxCoreAPIURL = "https://api.sandbox.midtrans.com/v2"

	// Production URLs
	productionSnapURL    = "https://app.midtrans.com/snap/v1/transactions"
	productionCoreAPIURL = "https://api.midtrans.com/v2"
)

// P0-11: Circuit breaker constants
const (
	maxFailures      = 5                // Open circuit after 5 consecutive failures
	openTimeout      = 30 * time.Second // Try recovery after 30 seconds
	halfOpenAttempts = 1                // Allow 1 request in half-open state
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Circuit is open, reject requests
	CircuitHalfOpen                     // Testing if service has recovered
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// circuitBreaker implements a minimal circuit breaker pattern
// P0-11: Prevents cascading failures when Midtrans is down
type circuitBreaker struct {
	mu            sync.RWMutex
	state         CircuitState
	failures      int
	lastFailTime  time.Time
	halfOpenCount int
	openUntil     time.Time
}

// newCircuitBreaker creates a new circuit breaker in closed state
func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		state:    CircuitClosed,
		failures: 0,
	}
}

// allowRequest returns true if request should proceed
func (cb *circuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	// If circuit is open, check if timeout has passed
	if cb.state == CircuitOpen {
		if now.After(cb.openUntil) {
			// Transition to half-open
			cb.state = CircuitHalfOpen
			cb.halfOpenCount = 0
			return true
		}
		return false // Fail fast
	}

	// In half-open state, allow limited requests
	if cb.state == CircuitHalfOpen {
		if cb.halfOpenCount < halfOpenAttempts {
			cb.halfOpenCount++
			return true
		}
		return false
	}

	// Closed state - allow all requests
	return true
}

// onSuccess records a successful request
func (cb *circuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Reset on success
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
	}
	cb.failures = 0
	cb.halfOpenCount = 0
}

// onFailure records a failed request
func (cb *circuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	// Open circuit after threshold
	if cb.failures >= maxFailures {
		cb.state = CircuitOpen
		cb.openUntil = time.Now().Add(openTimeout)
	}
}

// getState returns the current circuit state
func (cb *circuitBreaker) getState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// TransactionStatus represents Midtrans transaction statuses
type TransactionStatus string

const (
	StatusCapture       TransactionStatus = "capture"
	StatusSettlement    TransactionStatus = "settlement"
	StatusPending       TransactionStatus = "pending"
	StatusDeny          TransactionStatus = "deny"
	StatusCancel        TransactionStatus = "cancel"
	StatusExpire        TransactionStatus = "expire"
	StatusRefund        TransactionStatus = "refund"
	StatusPartialRefund TransactionStatus = "partial_refund"
)

// Client represents the Midtrans API client
type Client struct {
	serverKey       string
	clientKey       string
	isProduction    bool
	notificationURL string
	httpClient      *http.Client
	log             *logger.Logger
	cb              *circuitBreaker // P0-11: Circuit breaker
}

// NewClient creates a new Midtrans client
// P0-13: Configures HTTP connection pool for optimal performance
func NewClient(cfg *config.MidtransConfig, log *logger.Logger) *Client {
	// P0-13: Configure HTTP transport with connection pooling
	// These settings optimize for multiple concurrent requests to Midtrans API
	transport := &http.Transport{
		// MaxIdleConns controls the maximum number of idle connections across all hosts
		// Default Go default is 100, we set higher for better concurrency
		MaxIdleConns: 100,

		// MaxIdleConnsPerHost controls the maximum number of idle connections per host
		// Default is 2, which is too low for API clients
		MaxIdleConnsPerHost: 20,

		// MaxConnsPerHost limits the total number of connections per host
		// 0 means no limit (relies on MaxIdleConnsPerHost)
		MaxConnsPerHost: 0,

		// IdleConnTimeout is the maximum time an idle connection remains usable
		// 90 seconds is standard for HTTPS APIs
		IdleConnTimeout: 90 * time.Second,

		// TLSHandshakeTimeout is the timeout for TLS handshake
		// 10 seconds is standard and prevents hanging on TLS
		TLSHandshakeTimeout: 10 * time.Second,

		// ExpectContinueTimeout controls the timeout for 100-continue
		ExpectContinueTimeout: 1 * time.Second,

		// Force HTTP/2 for better performance with HTTPS endpoints
		// Midtrans supports HTTP/2
		ForceAttemptHTTP2: true,
	}

	return &Client{
		serverKey:       cfg.ServerKey,
		clientKey:       cfg.ClientKey,
		isProduction:    cfg.Environment == "production",
		notificationURL: cfg.NotificationURL,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		log: log,
		cb:  newCircuitBreaker(), // P0-11: Initialize circuit breaker
	}
}

// SnapRequest represents a Snap transaction request
type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	CustomerDetails    *CustomerDetails   `json:"customer_details,omitempty"`
	ItemDetails        []ItemDetail       `json:"item_details,omitempty"`
	Callbacks          *Callbacks         `json:"callbacks,omitempty"`
	Expiry             *Expiry            `json:"expiry,omitempty"`
	// EnabledPayments restricts the Snap payment page to the given channel
	// codes (e.g. "bca_va", "other_qris", "credit_card"). Omitted/empty
	// means Midtrans shows all enabled channels for the merchant account.
	EnabledPayments []string `json:"enabled_payments,omitempty"`
}

// TransactionDetails contains the core transaction info
type TransactionDetails struct {
	OrderID     string  `json:"order_id"`
	GrossAmount float64 `json:"gross_amount"`
}

// CustomerDetails contains customer information
type CustomerDetails struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// ItemDetail represents an item in the transaction
type ItemDetail struct {
	ID       string  `json:"id,omitempty"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Name     string  `json:"name"`
}

// Callbacks contains callback URLs
type Callbacks struct {
	Finish string `json:"finish,omitempty"`
}

// Expiry contains expiry settings
type Expiry struct {
	StartTime string `json:"start_time,omitempty"` // Format: "2025-01-01 00:00:00 +0700"
	Unit      string `json:"unit"`                 // "minute", "hour", "day"
	Duration  int    `json:"duration"`
}

// SnapResponse represents the response from Snap API
type SnapResponse struct {
	Token         string   `json:"token"`
	RedirectURL   string   `json:"redirect_url"`
	ErrorMessages []string `json:"error_messages,omitempty"`
}

// SnapCreateErrorClass classifies a CreateSnapTransaction failure using only
// evidence from the provider response surface.
type SnapCreateErrorClass string

const (
	SnapCreateErrorClassDefinitiveNoTransactionCreated  SnapCreateErrorClass = "definitive_no_transaction_created"
	SnapCreateErrorClassExistingTransactionPossible     SnapCreateErrorClass = "existing_transaction_possible"
	SnapCreateErrorClassTransientOrUncertain            SnapCreateErrorClass = "transient_or_uncertain"
	SnapCreateErrorClassConfigurationOrProgrammingError SnapCreateErrorClass = "configuration_or_programming_error"
)

// APIError captures a non-2xx Midtrans HTTP response with the raw status code
// and body. Callers can distinguish terminal provider refusals from uncertain
// transport/system failures without guessing from a plain string.
type APIError struct {
	Operation  string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return "midtrans api error"
	}
	return fmt.Sprintf("midtrans %s error: status %d, body: %s", e.Operation, e.StatusCode, e.Body)
}

// CreateSnapClassification returns the exact classification for a Snap create
// failure. Only validation failures that conclusively prove the provider could
// not have created the transaction are marked definitive.
func (e *APIError) CreateSnapClassification() SnapCreateErrorClass {
	if e == nil {
		return SnapCreateErrorClassTransientOrUncertain
	}

	body := strings.ToLower(e.Body)
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return SnapCreateErrorClassConfigurationOrProgrammingError
	case http.StatusConflict:
		return SnapCreateErrorClassExistingTransactionPossible
	case 406:
		return SnapCreateErrorClassExistingTransactionPossible
	case http.StatusBadRequest:
		switch {
		case strings.Contains(body, "order_id has been paid and utilized"),
			strings.Contains(body, "order id has already been utilized"),
			strings.Contains(body, "duplicate order id"):
			return SnapCreateErrorClassExistingTransactionPossible
		case strings.Contains(body, "invalid json"),
			strings.Contains(body, "validation error"),
			strings.Contains(body, "gross_amount is not equal to the sum of item_details"),
			strings.Contains(body, "missing mandatory field"),
			strings.Contains(body, "invalid format"),
			strings.Contains(body, "syntax error"):
			return SnapCreateErrorClassDefinitiveNoTransactionCreated
		default:
			return SnapCreateErrorClassTransientOrUncertain
		}
	case http.StatusTooManyRequests:
		return SnapCreateErrorClassTransientOrUncertain
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return SnapCreateErrorClassTransientOrUncertain
	default:
		if e.StatusCode >= 500 && e.StatusCode < 600 {
			return SnapCreateErrorClassTransientOrUncertain
		}
		return SnapCreateErrorClassTransientOrUncertain
	}
}

// DefinitiveRefusal reports whether the HTTP response conclusively rejected
// the Snap create request with evidence that the transaction could not have
// been created.
func (e *APIError) DefinitiveRefusal() bool {
	if e == nil {
		return false
	}
	return e.CreateSnapClassification() == SnapCreateErrorClassDefinitiveNoTransactionCreated
}

// NotificationPayload represents Midtrans webhook notification.
//
// Refund-specific fields (RefundKey, RefundAmount, RefundChargeID) are
// populated only on refund / partial_refund notifications. They are
// omitempty so payment notifications stay byte-identical on the wire.
type NotificationPayload struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusMessage     string `json:"status_message"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	MerchantID        string `json:"merchant_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
	RefundKey         string `json:"refund_key,omitempty"`
	RefundAmount      string `json:"refund_amount,omitempty"`
	RefundChargeID    string `json:"refund_chargeback_id,omitempty"`
}

// CreateSnapTransaction creates a new Snap transaction
func (c *Client) CreateSnapTransaction(req *SnapRequest) (*SnapResponse, error) {
	// P0-11: Circuit breaker - fail fast if circuit is open
	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans circuit breaker is open - failing fast",
			zap.String("state", c.cb.getState().String()),
		)
		return nil, fmt.Errorf("midtrans circuit breaker is %s - service unavailable", c.cb.getState())
	}

	url := c.getSnapURL()

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.log.Debug("Creating Snap transaction",
		zap.String("order_id", req.TransactionDetails.OrderID),
		zap.Float64("gross_amount", req.TransactionDetails.GrossAmount),
	)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.serverKey, "")
	// Per-request override of the merchant-level notification URL.
	// Only set when configured; absent header means Midtrans uses the dashboard value.
	if c.notificationURL != "" {
		httpReq.Header.Set("Notification-Url", c.notificationURL)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// P0-11: Record failure on network error
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// P0-11: Record failure on read error
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// P0-11: Record failure on API error
		c.cb.onFailure()
		c.log.Error("Midtrans API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
			zap.String("circuit_state", c.cb.getState().String()),
		)
		return nil, &APIError{
			Operation:  "create_snap",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var snapResp SnapResponse
	if err := json.Unmarshal(body, &snapResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// P0-11: Record success
	c.cb.onSuccess()

	c.log.Info("Snap transaction created",
		zap.String("order_id", req.TransactionDetails.OrderID),
	)

	return &snapResp, nil
}

// GetTransactionStatus gets the status of a transaction
func (c *Client) GetTransactionStatus(orderID string) (*NotificationPayload, error) {
	// P0-11: Circuit breaker - fail fast if circuit is open
	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans circuit breaker is open - failing fast",
			zap.String("state", c.cb.getState().String()),
		)
		return nil, fmt.Errorf("midtrans circuit breaker is %s - service unavailable", c.cb.getState())
	}

	url := fmt.Sprintf("%s/%s/status", c.getCoreAPIURL(), orderID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.serverKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// P0-11: Record failure on network error
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// P0-11: Record failure on read error
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// P0-11: Record failure on API error
		c.cb.onFailure()
		return nil, &APIError{
			Operation:  "get_transaction_status",
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var status NotificationPayload
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// P0-11: Record success
	c.cb.onSuccess()

	return &status, nil
}

// VerifySignature verifies the signature from Midtrans webhook
func (c *Client) VerifySignature(notification *NotificationPayload) bool {
	return c.BuildWebhookSignature(notification) == notification.SignatureKey
}

// BuildWebhookSignature computes the Midtrans payment webhook signature for a
// notification payload without revealing the server key to callers.
func (c *Client) BuildWebhookSignature(notification *NotificationPayload) string {
	if notification == nil {
		return ""
	}
	// Signature = SHA512(order_id + status_code + gross_amount + server_key)
	data := notification.OrderID + notification.StatusCode + notification.GrossAmount + c.serverKey
	hash := sha512.Sum512([]byte(data))
	return hex.EncodeToString(hash[:])
}

// IsTransactionSuccess checks if the transaction status indicates success
func (c *Client) IsTransactionSuccess(status string) bool {
	return status == string(StatusCapture) || status == string(StatusSettlement)
}

// IsTransactionPending checks if the transaction is pending
func (c *Client) IsTransactionPending(status string) bool {
	return status == string(StatusPending)
}

// IsTransactionFailed checks if the transaction failed
func (c *Client) IsTransactionFailed(status string) bool {
	return status == string(StatusDeny) ||
		status == string(StatusCancel) ||
		status == string(StatusExpire)
}

// Helper methods

func (c *Client) getSnapURL() string {
	if c.isProduction {
		return productionSnapURL
	}
	return sandboxSnapURL
}

func (c *Client) getCoreAPIURL() string {
	if c.isProduction {
		return productionCoreAPIURL
	}
	return sandboxCoreAPIURL
}

// GetClientKey returns the client key for frontend use
func (c *Client) GetClientKey() string {
	return c.clientKey
}

// IsProduction returns whether the client is in production mode
func (c *Client) IsProduction() bool {
	return c.isProduction
}

// RefundRequest represents a refund request to Midtrans API.
//
// RefundKey is the merchant-issued idempotency key. Midtrans uses it to
// deduplicate refund attempts at the gateway side: re-sending the same
// (order_id, refund_key) pair returns the original refund response instead
// of triggering a second refund. Required for the gateway-aware refund
// orchestration (TASK 33 / Phase 1).
type RefundRequest struct {
	RefundKey string  `json:"refund_key,omitempty"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason,omitempty"`
}

// RefundResponse represents the response from Midtrans refund API.
//
// RefundKey echoes the merchant's idempotency key. RefundChargeID is the
// gateway-issued refund identifier — orchestration stores it on the refund
// row so subsequent webhook acks can be resolved back to a specific refund.
type RefundResponse struct {
	StatusCode     string `json:"status_code"`
	StatusMessage  string `json:"status_message"`
	TransactionID  string `json:"transaction_id"`
	OrderID        string `json:"order_id"`
	RefundKey      string `json:"refund_key"`
	RefundChargeID string `json:"refund_chargeback_id"`
	Amount         string `json:"refund_amount"`
}

// Refund processes a refund for a transaction via Midtrans Core API
// POST /v2/{order_id}/refund
func (c *Client) Refund(
	ctx context.Context,
	orderID string,
	amount int64,
	reason string,
) error {
	// P0-11: Circuit breaker - fail fast if circuit is open
	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans circuit breaker is open - failing fast",
			zap.String("state", c.cb.getState().String()),
		)
		return fmt.Errorf("midtrans circuit breaker is %s - service unavailable", c.cb.getState())
	}

	url := fmt.Sprintf("%s/%s/refund", c.getCoreAPIURL(), orderID)

	// amount is a Rupiah integer (Labuda's canonical money unit, PASS_18H/
	// PASS_18J) — sent to Midtrans as-is, with no /100 or *100 scaling.
	reqBody := RefundRequest{
		Amount: float64(amount),
		Reason: reason,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal refund request: %w", err)
	}

	c.log.Info("Processing Midtrans refund",
		zap.String("order_id", orderID),
		zap.Int64("amount", amount),
		zap.String("reason", reason),
	)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.cb.onFailure()
		return fmt.Errorf("failed to create refund request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.serverKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.onFailure()
		return fmt.Errorf("failed to send refund request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.onFailure()
		return fmt.Errorf("failed to read refund response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.cb.onFailure()
		c.log.Error("Midtrans refund API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
			zap.String("order_id", orderID),
		)
		return fmt.Errorf("midtrans refund API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(body, &refundResp); err != nil {
		return fmt.Errorf("failed to unmarshal refund response: %w", err)
	}

	// P0-11: Record success
	c.cb.onSuccess()

	c.log.Info("Midtrans refund processed successfully",
		zap.String("order_id", orderID),
		zap.String("status_code", refundResp.StatusCode),
		zap.String("status_message", refundResp.StatusMessage),
	)

	return nil
}

// RefundWithKey is the gateway-aware refund call used by the canonical
// refund orchestration pipeline (TASK 33 / Phase 1).
//
// Differences vs Refund:
//   - Sends a merchant-issued idempotency key (refund_key) so the gateway
//     can deduplicate retried refund attempts and re-sending the same key
//     returns the prior result instead of triggering a duplicate refund.
//   - Returns the parsed RefundResponse so callers can persist the gateway-
//     side refund identifier and reconcile against the async webhook ack.
//   - Endpoint is the asynchronous Core API endpoint (POST /v2/{order_id}/refund),
//     matching the existing Refund implementation. Settlement of the refund
//     is acknowledged later via webhook.
//
// Side effects: only the outgoing HTTP call. No database, no ledger, no wallet.
func (c *Client) RefundWithKey(
	ctx context.Context,
	orderID string,
	refundKey string,
	amount int64,
	reason string,
) (*RefundResponse, error) {
	if !c.cb.allowRequest() {
		c.log.Warn("Midtrans circuit breaker is open - failing fast",
			zap.String("state", c.cb.getState().String()),
		)
		return nil, fmt.Errorf("midtrans circuit breaker is %s - service unavailable", c.cb.getState())
	}

	url := fmt.Sprintf("%s/%s/refund", c.getCoreAPIURL(), orderID)

	// amount is a Rupiah integer (Labuda's canonical money unit, PASS_18H/
	// PASS_18J) — sent to Midtrans as-is, with no /100 or *100 scaling.
	reqBody := RefundRequest{
		RefundKey: refundKey,
		Amount:    float64(amount),
		Reason:    reason,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refund request: %w", err)
	}

	c.log.Info("midtrans_refund_request",
		zap.String("order_id", orderID),
		zap.String("refund_key", refundKey),
		zap.Int64("amount", amount),
		zap.String("reason", reason),
	)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to create refund request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.serverKey, "")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to send refund request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.onFailure()
		return nil, fmt.Errorf("failed to read refund response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.cb.onFailure()
		c.log.Error("midtrans_refund_http_error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
			zap.String("order_id", orderID),
			zap.String("refund_key", refundKey),
		)
		return nil, fmt.Errorf("midtrans refund API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(body, &refundResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal refund response: %w", err)
	}

	c.cb.onSuccess()

	c.log.Info("midtrans_refund_response",
		zap.String("order_id", orderID),
		zap.String("refund_key", refundKey),
		zap.String("status_code", refundResp.StatusCode),
		zap.String("status_message", refundResp.StatusMessage),
		zap.String("transaction_id", refundResp.TransactionID),
	)

	return &refundResp, nil
}

// IsRefundNotification reports whether the webhook transaction_status
// indicates a refund acknowledgement (full or partial). Used by the
// payment webhook dispatcher to route refund acks to RefundService.
func (c *Client) IsRefundNotification(status string) bool {
	return status == string(StatusRefund) || status == "partial_refund"
}
