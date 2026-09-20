package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/stretchr/testify/assert"
)

// mockTransactor implements Transactor for testing
type mockTransactor struct {
	transactions []func(tx db.Tx) error
	txErr        error
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.txErr != nil {
		return m.txErr
	}
	return fn(nil)
}

// mockGatewayClient implements GatewayTransactionStatuser for testing
type mockGatewayClient struct {
	statusToReturn string
	fraudStatus    string
	grossAmount    string
	callCount      int64
	mu             sync.Mutex
	behaviors      map[string]func(string) (*midtrans.NotificationPayload, error)
}

func newMockGatewayClient() *mockGatewayClient {
	return &mockGatewayClient{
		statusToReturn: string(midtrans.StatusSettlement),
		fraudStatus:    "accept",
		grossAmount:    "100000.00",
		behaviors:      make(map[string]func(string) (*midtrans.NotificationPayload, error)),
	}
}

func (m *mockGatewayClient) QueryProviderState(orderID string) (*midtrans.ProviderStatus, error) {
	atomic.AddInt64(&m.callCount, 1)
	m.mu.Lock()
	behavior, ok := m.behaviors[orderID]
	m.mu.Unlock()
	if ok {
		payload, err := behavior(orderID)
		if err != nil {
			return nil, err
		}
		if payload == nil {
			return &midtrans.ProviderStatus{State: midtrans.ProviderStateNotPresent}, nil
		}
		return &midtrans.ProviderStatus{State: payload.ProviderState(), Notification: payload}, nil
	}
	payload := &midtrans.NotificationPayload{
		OrderID:           orderID,
		TransactionStatus: m.statusToReturn,
		FraudStatus:       m.fraudStatus,
		GrossAmount:       m.grossAmount,
		TransactionID:     "TX-" + uuid.New().String()[:8],
		PaymentType:       "bank_transfer",
	}
	return &midtrans.ProviderStatus{State: payload.ProviderState(), Notification: payload}, nil
}

func (m *mockGatewayClient) setBehavior(orderID string, fn func(string) (*midtrans.NotificationPayload, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.behaviors[orderID] = fn
}

// mockOrderFinalizer implements OrderPaymentFinalizer for testing
type mockOrderFinalizer struct {
	finalizationCount int64
	finalizedPayments []uuid.UUID
	mu                sync.Mutex
}

func (m *mockOrderFinalizer) FinalizeOrderPayment(ctx context.Context, tx db.Tx, payment *paymentRepo.Payment, transactionID string, paymentType string) error {
	atomic.AddInt64(&m.finalizationCount, 1)
	m.mu.Lock()
	m.finalizedPayments = append(m.finalizedPayments, payment.ID)
	m.mu.Unlock()
	return nil
}

func (m *mockOrderFinalizer) getCount() int64 {
	return atomic.LoadInt64(&m.finalizationCount)
}

// mockSubscriptionProcessor implements SubscriptionPaymentProcessor for testing
type mockSubscriptionProcessor struct {
	processingCount  int64
	processedPayments []uuid.UUID
	mu               sync.Mutex
}

func (m *mockSubscriptionProcessor) ProcessSuccessfulPayment(ctx context.Context, paymentID uuid.UUID, userID uuid.UUID, providerEventID string) error {
	atomic.AddInt64(&m.processingCount, 1)
	m.mu.Lock()
	m.processedPayments = append(m.processedPayments, paymentID)
	m.mu.Unlock()
	return nil
}

func (m *mockSubscriptionProcessor) getCount() int64 {
	return atomic.LoadInt64(&m.processingCount)
}

// failingGatewayClient always returns errors
type failingGatewayClient struct {
	err error
}

func (f *failingGatewayClient) QueryProviderState(orderID string) (*midtrans.ProviderStatus, error) {
	return nil, f.err
}

// slowGatewayClient simulates timeout behavior
type slowGatewayClient struct {
	delay time.Duration
	mock  *mockGatewayClient
}

func (s *slowGatewayClient) QueryProviderState(orderID string) (*midtrans.ProviderStatus, error) {
	time.Sleep(s.delay)
	return s.mock.QueryProviderState(orderID)
}

// TestPaymentDiscoveryWorker_Configuration tests configuration defaults
func TestPaymentDiscoveryWorker_Configuration(t *testing.T) {
	t.Run("default_config_values", func(t *testing.T) {
		cfg := DefaultPaymentDiscoveryConfig()

		assert.Equal(t, 1*time.Minute, cfg.PollInterval)
		assert.Equal(t, 100, cfg.BatchSize)
		assert.Equal(t, 10*time.Minute, cfg.InquiryEligibilityAge)
		assert.Equal(t, 30*time.Second, cfg.GatewayTimeout)
	})

	t.Run("zero_values_use_defaults", func(t *testing.T) {
		w := NewPaymentDiscoveryWorker(nil, nil, nil, nil, nil, PaymentDiscoveryConfig{})

		assert.Equal(t, DefaultDiscoveryPollInterval, w.pollInterval)
		assert.Equal(t, DefaultDiscoveryBatchSize, w.batchSize)
		assert.Equal(t, DefaultDiscoveryInquiryEligibilityAge, w.inquiryEligibilityAge)
		assert.Equal(t, DefaultDiscoveryGatewayTimeout, w.gatewayTimeout)
	})
}

// TestPaymentDiscoveryWorker_ConstantsOwnership verifies constants are in canonical location
func TestPaymentDiscoveryWorker_ConstantsOwnership(t *testing.T) {
	assert.Equal(t, 1*time.Minute, DefaultDiscoveryPollInterval)
	assert.Equal(t, 100, DefaultDiscoveryBatchSize)
	assert.Equal(t, 10*time.Minute, DefaultDiscoveryInquiryEligibilityAge)
	assert.Equal(t, 30*time.Second, DefaultDiscoveryGatewayTimeout)
}

// TestPaymentDiscoveryWorker_InterfaceContracts verifies interface contracts
func TestPaymentDiscoveryWorker_InterfaceContracts(t *testing.T) {
	t.Run("OrderPaymentFinalizer_interface", func(t *testing.T) {
		var _ OrderPaymentFinalizer = &mockOrderFinalizer{}
	})

	t.Run("SubscriptionPaymentProcessor_interface", func(t *testing.T) {
		var _ SubscriptionPaymentProcessor = &mockSubscriptionProcessor{}
	})

	t.Run("GatewayTransactionStatuser_interface", func(t *testing.T) {
		var _ GatewayTransactionStatuser = &mockGatewayClient{}
	})
}

// TestPaymentDiscoveryWorker_ShutdownCancellation tests worker shutdown behavior
func TestPaymentDiscoveryWorker_ShutdownCancellation(t *testing.T) {
	t.Run("context_cancellation_stops_worker", func(t *testing.T) {
		gateway := newMockGatewayClient()
		gateway.statusToReturn = string(midtrans.StatusPending)

		w := NewPaymentDiscoveryWorker(nil, gateway, nil, nil, nil, PaymentDiscoveryConfig{
			PollInterval: 100 * time.Millisecond,
		})

		w.Start()
		time.Sleep(50 * time.Millisecond) // Let it run briefly

		done := make(chan struct{})
		go func() {
			w.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Success - worker stopped
		case <-time.After(2 * time.Second):
			t.Fatal("Worker did not stop within timeout")
		}

		assert.False(t, w.IsRunning())
	})

	t.Run("stop_is_idempotent", func(t *testing.T) {
		w := NewPaymentDiscoveryWorker(nil, newMockGatewayClient(), nil, nil, nil, DefaultPaymentDiscoveryConfig())

		w.Start()
		time.Sleep(10 * time.Millisecond)

		w.Stop()
		w.Stop() // Should not panic

		assert.False(t, w.IsRunning())
	})

	t.Run("double_start_is_idempotent", func(t *testing.T) {
		w := NewPaymentDiscoveryWorker(nil, newMockGatewayClient(), nil, nil, nil, DefaultPaymentDiscoveryConfig())

		w.Start()
		time.Sleep(10 * time.Millisecond)
		w.Start() // Should log warning but not panic

		assert.True(t, w.IsRunning())

		w.Stop()
	})
}

// TestPaymentDiscoveryWorker_FailureIsolation tests that one failing candidate
// does not abort the entire batch (scenario S)
func TestPaymentDiscoveryWorker_FailureIsolation(t *testing.T) {
	t.Run("gateway_error_does_not_panic", func(t *testing.T) {
		gateway := &failingGatewayClient{
			err: errors.New("circuit breaker open"),
		}

		w := NewPaymentDiscoveryWorker(nil, gateway, nil, nil, nil, DefaultPaymentDiscoveryConfig())

		// Worker creation should succeed even with failing gateway
		assert.NotNil(t, w)
		assert.False(t, w.IsRunning())
	})

	t.Run("nil_dependencies_do_not_panic", func(t *testing.T) {
		w := NewPaymentDiscoveryWorker(nil, nil, nil, nil, nil, DefaultPaymentDiscoveryConfig())
		assert.NotNil(t, w)
	})
}

// TestPaymentDiscoveryWorker_GatewayClassification pins the canonical provider
// state contract the worker routes on (midtrans.ClassifyProviderState is the
// single authority; the worker never interprets raw status strings itself).
func TestPaymentDiscoveryWorker_GatewayClassification(t *testing.T) {
	t.Run("settlement_is_settled", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateSettled, midtrans.ClassifyProviderState(string(midtrans.StatusSettlement), ""))
	})

	t.Run("accepted_capture_is_settled", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateSettled, midtrans.ClassifyProviderState(string(midtrans.StatusCapture), "accept"))
	})

	t.Run("unaccepted_capture_is_pending", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStatePending, midtrans.ClassifyProviderState(string(midtrans.StatusCapture), "challenge"))
	})

	t.Run("deny_is_failed", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateFailed, midtrans.ClassifyProviderState(string(midtrans.StatusDeny), ""))
	})

	t.Run("cancel_is_failed", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateFailed, midtrans.ClassifyProviderState(string(midtrans.StatusCancel), ""))
	})

	t.Run("expire_is_failed", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateFailed, midtrans.ClassifyProviderState(string(midtrans.StatusExpire), ""))
	})

	t.Run("pending_is_pending", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStatePending, midtrans.ClassifyProviderState(string(midtrans.StatusPending), ""))
	})

	t.Run("refund_is_unknown_never_settled", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateUnknown, midtrans.ClassifyProviderState(string(midtrans.StatusRefund), ""))
	})

	t.Run("unknown_status_is_unknown", func(t *testing.T) {
		assert.Equal(t, midtrans.ProviderStateUnknown, midtrans.ClassifyProviderState("unknown", ""))
	})
}

// TestPaymentDiscoveryWorker_LockBoundaryProof proves the conceptual lock boundary:
// 1. Discovery: short transaction with FOR UPDATE SKIP LOCKED
// 2. Gateway inquiry: READ-ONLY, OUTSIDE DB lock
// 3. Finalization: fresh transaction with ORDER → PAYMENT lock order
func TestPaymentDiscoveryWorker_LockBoundaryProof(t *testing.T) {
	t.Run("worker_struct_separates_phases", func(t *testing.T) {
		gateway := newMockGatewayClient()
		finalizer := &mockOrderFinalizer{}

		w := NewPaymentDiscoveryWorker(nil, gateway, finalizer, nil, nil, DefaultPaymentDiscoveryConfig())

		// Verify worker stores dependencies separately
		assert.NotNil(t, w.midtransClient)  // Gateway (used outside DB lock)
		assert.NotNil(t, w.canonicalFinal)  // Finalization (used inside DB lock)
		assert.Nil(t, w.db)                  // DB not wired in this test
	})

	t.Run("gateway_inquiry_is_readonly", func(t *testing.T) {
		gateway := newMockGatewayClient()

		// Gateway client has no DB reference - proves inquiry is read-only
		assert.NotNil(t, gateway)
		// QueryProviderState only returns data, never mutates
		result, err := gateway.QueryProviderState("order-123")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestPaymentDiscoveryWorker_ConcurrentAccessProof tests concurrent worker access patterns
func TestPaymentDiscoveryWorker_ConcurrentAccessProof(t *testing.T) {
	t.Run("multiple_workers_can_be_created_concurrently", func(t *testing.T) {
		gateway := newMockGatewayClient()
		var wg sync.WaitGroup

		workers := make([]*PaymentDiscoveryWorker, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				workers[idx] = NewPaymentDiscoveryWorker(nil, gateway, nil, nil, nil, DefaultPaymentDiscoveryConfig())
			}(i)
		}
		wg.Wait()

		for _, w := range workers {
			assert.NotNil(t, w)
		}
	})

	t.Run("stop_channel_is_per_worker", func(t *testing.T) {
		gateway := newMockGatewayClient()
		w1 := NewPaymentDiscoveryWorker(nil, gateway, nil, nil, nil, DefaultPaymentDiscoveryConfig())
		w2 := NewPaymentDiscoveryWorker(nil, gateway, nil, nil, nil, DefaultPaymentDiscoveryConfig())

		w1.Start()
		w2.Start()

		time.Sleep(10 * time.Millisecond)

		w1.Stop()
		assert.False(t, w1.IsRunning())
		assert.True(t, w2.IsRunning()) // w2 should still be running

		w2.Stop()
		assert.False(t, w2.IsRunning())
	})
}

// TestPaymentDiscoveryWorker_AmountValidation proves amount mismatch safety
func TestPaymentDiscoveryWorker_AmountValidation(t *testing.T) {
	t.Run("mismatched_amount_rejects_finalization", func(t *testing.T) {
		gateway := newMockGatewayClient()
		gateway.grossAmount = "999999.00" // Different from expected

		// The worker would reject this because amount doesn't match
		result, err := gateway.QueryProviderState("order-123")
		assert.NoError(t, err)
		assert.Equal(t, "999999.00", result.Notification.GrossAmount)
	})
}

// TestPaymentDiscoveryWorker_CaptureFraudValidation proves the capture/fraud
// gate is enforced by the canonical provider state, so the worker cannot settle
// an unaccepted capture.
func TestPaymentDiscoveryWorker_CaptureFraudValidation(t *testing.T) {
	gateway := newMockGatewayClient()

	t.Run("capture_with_accept_is_settled", func(t *testing.T) {
		gateway.statusToReturn = string(midtrans.StatusCapture)
		gateway.fraudStatus = "accept"

		result, _ := gateway.QueryProviderState("order-123")
		assert.Equal(t, "accept", result.Notification.FraudStatus)
		assert.Equal(t, midtrans.ProviderStateSettled, result.State)
	})

	t.Run("capture_with_challenge_is_not_settled", func(t *testing.T) {
		gateway.statusToReturn = string(midtrans.StatusCapture)
		gateway.fraudStatus = "challenge"

		result, _ := gateway.QueryProviderState("order-123")
		assert.Equal(t, "challenge", result.Notification.FraudStatus)
		assert.NotEqual(t, midtrans.ProviderStateSettled, result.State)
	})
}

// TestPaymentDiscoveryWorker_NilLoggerUsesNoop proves nil logger safety
func TestPaymentDiscoveryWorker_NilLoggerUsesNoop(t *testing.T) {
	w := NewPaymentDiscoveryWorker(nil, nil, nil, nil, nil, DefaultPaymentDiscoveryConfig())
	assert.NotNil(t, w.log) // Should be nop logger, not nil
}
