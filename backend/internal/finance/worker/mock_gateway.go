package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockPayoutGatewayConfig configures the mock gateway behavior.
type MockPayoutGatewayConfig struct {
	// SimulateLatency adds artificial delay to simulate network latency
	SimulateLatency time.Duration

	// FailureRate is the probability (0.0-1.0) of simulating a failure
	FailureRate float64

	// PermanentFailureRate is the probability (0.0-1.0) that a failure is permanent
	PermanentFailureRate float64

	// AlwaysSucceed disables all failure simulation
	AlwaysSucceed bool
}

// DefaultMockPayoutGatewayConfig returns default configuration.
func DefaultMockPayoutGatewayConfig() MockPayoutGatewayConfig {
	return MockPayoutGatewayConfig{
		SimulateLatency:       100 * time.Millisecond,
		FailureRate:           0.1, // 10% failure rate
		PermanentFailureRate:  0.3, // 30% of failures are permanent
		AlwaysSucceed:         false,
	}
}

// MockPayoutGateway is a mock implementation of PayoutGateway for testing.
// It simulates various success/failure scenarios without calling real APIs.
//
// IMPORTANT: This mock is idempotent - the same external_reference_id will
// return the same response without executing a new payout.
type MockPayoutGateway struct {
	mu        sync.Mutex
	config    MockPayoutGatewayConfig
	processed map[string]*PayoutGatewayResponse
}

// NewMockPayoutGateway creates a new mock payout gateway.
func NewMockPayoutGateway(config MockPayoutGatewayConfig) *MockPayoutGateway {
	if config.SimulateLatency == 0 {
		config.SimulateLatency = 100 * time.Millisecond
	}
	return &MockPayoutGateway{
		config:    config,
		processed: make(map[string]*PayoutGatewayResponse),
	}
}

// SubmitPayout simulates submitting a payout to a payment gateway.
// This implementation is idempotent - the same external_reference_id returns
// the same response without executing a new payout.
func (m *MockPayoutGateway) SubmitPayout(ctx context.Context, req PayoutGatewayRequest) (*PayoutGatewayResponse, error) {
	// Check for idempotency - if already processed, return cached response
	m.mu.Lock()
	if resp, exists := m.processed[req.ExternalReferenceID]; exists {
		m.mu.Unlock()
		return resp, nil
	}
	// Keep the lock during processing to ensure idempotency
	// Unlock is deferred after storing the response

	// Simulate network latency
	if m.config.SimulateLatency > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.config.SimulateLatency):
		}
	}

	// Build response
	resp := &PayoutGatewayResponse{
		GatewayReferenceID: "MOCK_" + uuid.New().String(),
		Message:            "Payout processed by mock gateway",
	}

	// Simulate failure if configured
	if !m.config.AlwaysSucceed && shouldSimulateFailure(m.config.FailureRate) {
		resp.Status = PayoutResponseStatusFailed
		resp.Message = "Mock gateway: Simulated failure"

		// Determine if error is retryable or permanent
		if shouldSimulateFailure(m.config.PermanentFailureRate) {
			resp.ErrorType = ErrorTypePermanent
			resp.Message = "Mock gateway: Invalid bank account (permanent error)"
		} else {
			resp.ErrorType = ErrorTypeRetryable
			resp.Message = "Mock gateway: Temporary network error (retryable)"
		}
	} else {
		resp.Status = PayoutResponseStatusSuccess
		resp.Message = "Payout submitted successfully"
		resp.ErrorType = "" // No error
	}

	// Build raw response JSON
	rawResp := map[string]interface{}{
		"status":               string(resp.Status),
		"gateway_reference_id": resp.GatewayReferenceID,
		"message":              resp.Message,
		"timestamp":            time.Now().Unix(),
		"mock":                 true,
	}
	rawJSON, _ := json.Marshal(rawResp)
	resp.RawResponse = string(rawJSON)

	// Store for idempotency
	m.processed[req.ExternalReferenceID] = resp
	m.mu.Unlock()

	return resp, nil
}

// shouldSimulateFailure returns true based on the given rate.
func shouldSimulateFailure(rate float64) bool {
	// Simple deterministic simulation based on time
	// This ensures consistent behavior within the same second
	now := time.Now().UnixNano()
	return (now % 100) < int64(rate*100)
}

// ClearProcessed clears the processed map (useful for testing).
func (m *MockPayoutGateway) ClearProcessed() {
	m.processed = make(map[string]*PayoutGatewayResponse)
}

// GetProcessed returns the number of unique payouts processed.
func (m *MockPayoutGateway) GetProcessed() int {
	return len(m.processed)
}

// GetProcessedIDs returns the external reference IDs of all processed payouts.
func (m *MockPayoutGateway) GetProcessedIDs() []string {
	ids := make([]string, 0, len(m.processed))
	for id := range m.processed {
		ids = append(ids, id)
	}
	return ids
}


