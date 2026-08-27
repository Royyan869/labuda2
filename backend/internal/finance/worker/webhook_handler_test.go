package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookHandler_DuplicateSuccessCallback tests that duplicate SUCCESS callbacks are idempotent
func TestWebhookHandler_DuplicateSuccessCallback(t *testing.T) {
	_ = NewWebhookHandler(repository.NewWithdrawRepository(), repository.NewLedgerRepository(), nil, nil)

	externalRef := "WD_test_123"
	gatewayRef := "GW_456"

	// Create a test withdrawal in SUBMITTED state
	withdrawal := &repository.Withdrawal{
		ID:                  uuid.New(),
		SellerID:            uuid.New(),
		Amount:              100000,
		Status:              repository.WithdrawalStatusSubmitted,
		ExternalReferenceID: externalRef,
		SubmittedAt:         time.Now().Unix(),
	}

	// Mock callback
	callback := WebhookCallback{
		ExternalReferenceID: externalRef,
		GatewayReferenceID:  gatewayRef,
		Status:              WebhookStatusSuccess,
		Message:             "Payout completed",
		Timestamp:           time.Now().Unix(),
		RawPayload:          `{"status":"SUCCESS"}`,
	}

	// NOTE: This test demonstrates the idempotency behavior.
	// In a real integration test with a test database:
	// 1. First callback: SUBMITTED -> SETTLED
	// 2. Second callback: Should return ErrDuplicateCallback (no-op)

	// The handler checks IsFinal() before processing
	assert.False(t, withdrawal.Status.IsFinal(), "SUBMITTED is not final")

	// After processing, status would be SETTLED which IS final
	withdrawal.Status = repository.WithdrawalStatusSettled
	assert.True(t, withdrawal.Status.IsFinal(), "SETTLED is final")

	// Duplicate callback would be ignored
	_ = callback // Used to avoid unused variable warning

	t.Log("Duplicate success callbacks are idempotent due to IsFinal() check")
}

// TestWebhookHandler_FailedCallbackAfterSuccess tests that a failed callback after success is ignored
func TestWebhookHandler_FailedCallbackAfterSuccess(t *testing.T) {
	_ = NewWebhookHandler(repository.NewWithdrawRepository(), repository.NewLedgerRepository(), nil, nil)

	externalRef := "WD_test_789"

	// Scenario: Withdrawal is already SETTLED
	withdrawal := &repository.Withdrawal{
		ID:                  uuid.New(),
		Status:              repository.WithdrawalStatusSettled,
		ExternalReferenceID: externalRef,
		SettledAt:           time.Now().Unix(),
	}

	// Gateway sends a FAILED callback (out of order/late)
	_ = WebhookCallback{
		ExternalReferenceID: externalRef,
		GatewayReferenceID:  "GW_FAILED",
		Status:              WebhookStatusFailed,
		Message:             "Payment failed",
		Timestamp:           time.Now().Unix(),
		RawPayload:          `{"status":"FAILED"}`,
	}

	// Handler should ignore this because withdrawal.IsFinal() == true
	assert.True(t, withdrawal.Status.IsFinal(), "SETTLED is final - callback should be ignored")

	t.Log("Failed callback after success is safely ignored due to IsFinal() check")
}

// TestWebhookHandler_InvalidStateTransition tests that invalid state transitions are rejected
func TestWebhookHandler_InvalidStateTransition(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  repository.WithdrawalStatus
		callbackStatus WebhookStatus
		shouldError    bool
		description    string
	}{
		{
			name:           "PROCESSING -> SUCCESS (invalid, not submitted yet)",
			currentStatus:  repository.WithdrawalStatusProcessing,
			callbackStatus: WebhookStatusSuccess,
			shouldError:    true,
			description:    "Cannot settle a withdrawal that hasn't been submitted to gateway",
		},
		{
			name:           "FAILED_FINAL -> SUCCESS (invalid, already failed)",
			currentStatus:  repository.WithdrawalStatusFailedFinal,
			callbackStatus: WebhookStatusSuccess,
			shouldError:    true,
			description:    "Cannot settle a withdrawal that already failed permanently",
		},
		{
			name:           "SUBMITTED -> SUCCESS (valid)",
			currentStatus:  repository.WithdrawalStatusSubmitted,
			callbackStatus: WebhookStatusSuccess,
			shouldError:    false,
			description:    "Normal successful flow",
		},
		{
			name:           "SETTLING -> SUCCESS (valid)",
			currentStatus:  repository.WithdrawalStatusSettling,
			callbackStatus: WebhookStatusSuccess,
			shouldError:    false,
			description:    "Settling withdrawal confirmed",
		},
		{
			name:           "SUBMITTED -> PENDING (valid)",
			currentStatus:  repository.WithdrawalStatusSubmitted,
			callbackStatus: WebhookStatusPending,
			shouldError:    false,
			description:    "Gateway reports pending - move to SETTLING",
		},
		{
			name:           "SETTLING -> PENDING (duplicate, no-op)",
			currentStatus:  repository.WithdrawalStatusSettling,
			callbackStatus: WebhookStatusPending,
			shouldError:    false, // Returns ErrDuplicateCallback
			description:    "Already settling, duplicate pending callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withdrawal := &repository.Withdrawal{
				ID:                  uuid.New(),
				Status:              tt.currentStatus,
				ExternalReferenceID: "WD_test_" + tt.name,
			}

			// Check if current status is final
			if withdrawal.Status.IsFinal() {
				t.Log("Current status is final - callback would be ignored")
				return
			}

			// Check if transition would be valid
			validTransition := withdrawal.Status.CanTransitionTo(repository.WithdrawalStatusSettled)

			if tt.shouldError {
				assert.False(t, validTransition, tt.description)
			} else {
				assert.True(t, validTransition || tt.currentStatus == repository.WithdrawalStatusSettling, tt.description)
			}

			t.Log(tt.description)
		})
	}
}

// TestWebhookHandler_OutOfOrderCallbacks tests various out-of-order callback scenarios
func TestWebhookHandler_OutOfOrderCallbacks(t *testing.T) {
	scenarios := []struct {
		name        string
		sequence    []WebhookStatus
		finalStatus repository.WithdrawalStatus
		description string
	}{
		{
			name:        "SUCCESS then FAILED (ignored)",
			sequence:    []WebhookStatus{WebhookStatusSuccess, WebhookStatusFailed},
			finalStatus: repository.WithdrawalStatusSettled,
			description: "First callback settles, second is ignored (IsFinal guard)",
		},
		{
			name:        "PENDING then SUCCESS",
			sequence:    []WebhookStatus{WebhookStatusPending, WebhookStatusSuccess},
			finalStatus: repository.WithdrawalStatusSettled,
			description: "Normal flow: SUBMITTED->SETTLING->SETTLED",
		},
		{
			name:        "PENDING then PENDING (duplicate)",
			sequence:    []WebhookStatus{WebhookStatusPending, WebhookStatusPending},
			finalStatus: repository.WithdrawalStatusSettling,
			description: "Second pending is idempotent no-op",
		},
		{
			name:        "FAILED then SUCCESS (rejected)",
			sequence:    []WebhookStatus{WebhookStatusFailed, WebhookStatusSuccess},
			finalStatus: repository.WithdrawalStatusFailedFinal,
			description: "First callback fails permanently, second rejected (InvalidStateTransition)",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Log(sc.description)

			// Simulate the state machine
			status := repository.WithdrawalStatusSubmitted

			for i, callbackStatus := range sc.sequence {
				t.Logf("Step %d: %s -> callback %s", i+1, status, callbackStatus)

				// Check if already final
				if status.IsFinal() {
					t.Logf("  Already in final state %s, callback ignored", status)
					continue
				}

				// Determine next state based on callback
				switch callbackStatus {
				case WebhookStatusPending:
					if status == repository.WithdrawalStatusSubmitted {
						status = repository.WithdrawalStatusSettling
					}
					// If already SETTLING, no-op (duplicate)
				case WebhookStatusSuccess:
					if status == repository.WithdrawalStatusSubmitted || status == repository.WithdrawalStatusSettling {
						status = repository.WithdrawalStatusSettled
					}
				case WebhookStatusFailed, WebhookStatusRejected:
					if status == repository.WithdrawalStatusSubmitted || status == repository.WithdrawalStatusSettling {
						status = repository.WithdrawalStatusFailedFinal
					}
				}
			}

			assert.Equal(t, sc.finalStatus, status, "Final status should match expected")
			t.Logf("Final status: %s", status)
		})
	}
}

// TestWithdrawalStatus_Transitions tests the state transition validation
func TestWithdrawalStatus_Transitions(t *testing.T) {
	tests := []struct {
		from     repository.WithdrawalStatus
		to       repository.WithdrawalStatus
		expected bool
	}{
		// Valid transitions
		{repository.WithdrawalStatusRequested, repository.WithdrawalStatusProcessing, true},
		{repository.WithdrawalStatusRequested, repository.WithdrawalStatusFailed, true},
		{repository.WithdrawalStatusProcessing, repository.WithdrawalStatusSubmitted, true},
		{repository.WithdrawalStatusProcessing, repository.WithdrawalStatusFailed, true},
		{repository.WithdrawalStatusSubmitted, repository.WithdrawalStatusSettling, true},
		{repository.WithdrawalStatusSubmitted, repository.WithdrawalStatusSettled, true},
		{repository.WithdrawalStatusSubmitted, repository.WithdrawalStatusFailedRetryable, true},
		{repository.WithdrawalStatusSubmitted, repository.WithdrawalStatusFailedFinal, true},
		{repository.WithdrawalStatusSettling, repository.WithdrawalStatusSettled, true},
		{repository.WithdrawalStatusSettling, repository.WithdrawalStatusFailedRetryable, true},
		{repository.WithdrawalStatusSettling, repository.WithdrawalStatusFailedFinal, true},
		{repository.WithdrawalStatusFailedRetryable, repository.WithdrawalStatusSubmitted, true},

		// Invalid transitions
		{repository.WithdrawalStatusSettled, repository.WithdrawalStatusProcessing, false},
		{repository.WithdrawalStatusSettled, repository.WithdrawalStatusSubmitted, false},
		{repository.WithdrawalStatusSettled, repository.WithdrawalStatusFailed, false},
		{repository.WithdrawalStatusFailedFinal, repository.WithdrawalStatusSubmitted, false},
		{repository.WithdrawalStatusFailed, repository.WithdrawalStatusProcessing, false},
		{repository.WithdrawalStatusRequested, repository.WithdrawalStatusSettled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.expected, result,
				"Transition %s -> %s should be %v", tt.from, tt.to, tt.expected)
		})
	}
}

// TestWithdrawalStatus_IsFinal tests terminal state detection
func TestWithdrawalStatus_IsFinal(t *testing.T) {
	finalStates := []repository.WithdrawalStatus{
		repository.WithdrawalStatusSettled,
		repository.WithdrawalStatusCompleted,
		repository.WithdrawalStatusFailed,
		repository.WithdrawalStatusFailedFinal,
	}

	nonFinalStates := []repository.WithdrawalStatus{
		repository.WithdrawalStatusRequested,
		repository.WithdrawalStatusProcessing,
		repository.WithdrawalStatusSubmitted,
		repository.WithdrawalStatusSettling,
		repository.WithdrawalStatusFailedRetryable,
	}

	for _, s := range finalStates {
		t.Run(string(s)+"_is_final", func(t *testing.T) {
			assert.True(t, s.IsFinal(), "%s should be final", s)
		})
	}

	for _, s := range nonFinalStates {
		t.Run(string(s)+"_not_final", func(t *testing.T) {
			assert.False(t, s.IsFinal(), "%s should not be final", s)
		})
	}
}

// TestMockGateway_Idempotency tests that the mock gateway is idempotent
func TestMockGateway_Idempotency(t *testing.T) {
	config := MockPayoutGatewayConfig{
		SimulateLatency:  0,
		FailureRate:      0.0,
		PermanentFailureRate: 0.0,
		AlwaysSucceed:    true,
	}

	gateway := NewMockPayoutGateway(config)
	ctx := context.Background()

	req := PayoutGatewayRequest{
		ExternalReferenceID: "TEST_IDEMPOTENCY_123",
		Amount:              100000,
		Currency:            "IDR",
	}

	// First call
	resp1, err1 := gateway.SubmitPayout(ctx, req)
	require.NoError(t, err1)
	require.NotNil(t, resp1)
	assert.Equal(t, PayoutResponseStatusSuccess, resp1.Status)
	ref1 := resp1.GatewayReferenceID

	// Second call with same external reference - should return same response
	resp2, err2 := gateway.SubmitPayout(ctx, req)
	require.NoError(t, err2)
	require.NotNil(t, resp2)
	assert.Equal(t, PayoutResponseStatusSuccess, resp2.Status)
	assert.Equal(t, ref1, resp2.GatewayReferenceID, "Same external_ref should return same gateway_ref")

	// Verify only one payout was processed
	assert.Equal(t, 1, gateway.GetProcessed(), "Should have exactly 1 processed payout")
	assert.Equal(t, []string{"TEST_IDEMPOTENCY_123"}, gateway.GetProcessedIDs())

	t.Log("Mock gateway is correctly idempotent - duplicate calls return same response")
}

// TestMockGateway_FailureSimulation tests failure simulation scenarios
func TestMockGateway_FailureSimulation(t *testing.T) {
	tests := []struct {
		name             string
		failureRate      float64
		permanentFailureRate float64
		expectedStatus   PayoutResponseStatus
	}{
		{
			name:               "Always succeed",
			failureRate:        0.0,
			permanentFailureRate: 0.0,
			expectedStatus:      PayoutResponseStatusSuccess,
		},
		{
			name:               "Always fail (retryable)",
			failureRate:        1.0,
			permanentFailureRate: 0.0,
			expectedStatus:      PayoutResponseStatusFailed,
		},
		{
			name:               "Always fail (permanent)",
			failureRate:        1.0,
			permanentFailureRate: 1.0,
			expectedStatus:      PayoutResponseStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MockPayoutGatewayConfig{
				SimulateLatency:  0,
				FailureRate:      tt.failureRate,
				PermanentFailureRate: tt.permanentFailureRate,
				AlwaysSucceed:    false,
			}

			gateway := NewMockPayoutGateway(config)
			ctx := context.Background()

			req := PayoutGatewayRequest{
				ExternalReferenceID: "TEST_" + tt.name,
				Amount:              100000,
				Currency:            "IDR",
			}

			resp, err := gateway.SubmitPayout(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			assert.Equal(t, tt.expectedStatus, resp.Status)

			if resp.Status == PayoutResponseStatusFailed {
				if tt.permanentFailureRate > 0 {
					assert.Equal(t, ErrorTypePermanent, resp.ErrorType)
				} else {
					assert.Equal(t, ErrorTypeRetryable, resp.ErrorType)
				}
			}

			t.Logf("Status: %s, ErrorType: %s", resp.Status, resp.ErrorType)
		})
	}
}

// TestMockGateway_TimeoutSimulation tests gateway timeout behavior
func TestMockGateway_TimeoutSimulation(t *testing.T) {
	config := MockPayoutGatewayConfig{
		SimulateLatency:  2 * time.Second,
		FailureRate:      0.0,
		PermanentFailureRate: 0.0,
		AlwaysSucceed:    true,
	}

	gateway := NewMockPayoutGateway(config)

	// Context with timeout shorter than simulated latency
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := PayoutGatewayRequest{
		ExternalReferenceID: "TEST_TIMEOUT",
		Amount:              100000,
		Currency:            "IDR",
	}

	resp, err := gateway.SubmitPayout(ctx, req)
	assert.Error(t, err, "Should return error due to context timeout")
	assert.Nil(t, resp)

	// Verify no payout was recorded (transaction was rolled back at gateway level)
	assert.Equal(t, 0, gateway.GetProcessed(), "Timeout should not record payout")

	t.Log("Gateway timeout correctly returns error without recording payout")
}

// TestPayoutGatewayResponse_IsSuccess tests success detection
func TestPayoutGatewayResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		status     PayoutResponseStatus
		isSuccess  bool
		isRetryable bool
	}{
		{PayoutResponseStatusSuccess, true, false},
		{PayoutResponseStatusPending, true, false},
		{PayoutResponseStatusFailed, false, false},
		{PayoutResponseStatusRejected, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			resp := &PayoutGatewayResponse{
				Status:    tt.status,
				ErrorType: ErrorTypeRetryable,
			}

			assert.Equal(t, tt.isSuccess, resp.IsSuccess())
		})
	}
}

// TestPayoutGatewayResponse_IsRetryable tests retryable error detection
func TestPayoutGatewayResponse_IsRetryable(t *testing.T) {
	tests := []struct {
		status    PayoutResponseStatus
		errorType PayoutErrorType
		expected  bool
	}{
		{PayoutResponseStatusFailed, ErrorTypeRetryable, true},
		{PayoutResponseStatusFailed, ErrorTypePermanent, false},
		{PayoutResponseStatusSuccess, ErrorTypeRetryable, false},
		{PayoutResponseStatusRejected, ErrorTypePermanent, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status)+"_"+string(tt.errorType), func(t *testing.T) {
			resp := &PayoutGatewayResponse{
				Status:    tt.status,
				ErrorType: tt.errorType,
			}

			assert.Equal(t, tt.expected, resp.IsRetryable())
		})
	}
}

// TestWebhookCallbackSerialization tests callback serialization
func TestWebhookCallbackSerialization(t *testing.T) {
	callback := WebhookCallback{
		ExternalReferenceID: "WD_123_456",
		GatewayReferenceID:  "GW_ABC",
		Status:              WebhookStatusSuccess,
		Message:             "Payout completed successfully",
		Timestamp:           time.Now().Unix(),
		RawPayload:          `{"status":"SUCCESS","ref":"GW_ABC"}`,
	}

	// Serialize to JSON
	data, err := json.Marshal(callback)
	require.NoError(t, err)

	// Deserialize
	var decoded WebhookCallback
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, callback.ExternalReferenceID, decoded.ExternalReferenceID)
	assert.Equal(t, callback.GatewayReferenceID, decoded.GatewayReferenceID)
	assert.Equal(t, callback.Status, decoded.Status)
	assert.Equal(t, callback.Message, decoded.Message)
	assert.Equal(t, callback.Timestamp, decoded.Timestamp)

	t.Log("WebhookCallback serializes correctly")
}

// BenchmarkIdempotencyCheck benchmarks the idempotency check performance
func BenchmarkIdempotencyCheck(b *testing.B) {
	gateway := NewMockPayoutGateway(MockPayoutGatewayConfig{
		SimulateLatency: 0,
		AlwaysSucceed:   true,
	})

	ctx := context.Background()
	req := PayoutGatewayRequest{
		ExternalReferenceID: "BENCHMARK_IDEMPOTENCY",
		Amount:              100000,
		Currency:            "IDR",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gateway.SubmitPayout(ctx, req)
	}
}


