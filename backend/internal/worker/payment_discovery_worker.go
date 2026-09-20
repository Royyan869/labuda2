package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	refundentity "github.com/labuda/backend/internal/finance/refund/entity"
	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

// OrderPaymentFinalizer abstracts the canonical order payment finalization service.
// Implemented by paymentApp.CanonicalFinalizationService.
type OrderPaymentFinalizer interface {
	FinalizeOrderPayment(ctx context.Context, tx db.Tx, payment *paymentRepo.Payment, transactionID string, paymentType string) error
}

// SubscriptionPaymentProcessor abstracts the canonical subscription payment service.
// Implemented by subscriptionApp.SellerSubscriptionPaymentService.
type SubscriptionPaymentProcessor interface {
	ProcessSuccessfulPayment(ctx context.Context, paymentID uuid.UUID, userID uuid.UUID, providerEventID string) error
}

const (
	// DefaultDiscoveryPollInterval is how often the worker scans for stale pending payments.
	DefaultDiscoveryPollInterval = 1 * time.Minute

	// DefaultDiscoveryBatchSize is max candidate payments per scan cycle.
	DefaultDiscoveryBatchSize = 100

	// DefaultDiscoveryInquiryEligibilityAge is how old a pending payment must be
	// before gateway inquiry is attempted. This ensures the normal webhook path
	// has time to process before the scanner intervenes.
	DefaultDiscoveryInquiryEligibilityAge = 10 * time.Minute

	// DefaultDiscoveryGatewayTimeout is the timeout for a single gateway inquiry call.
	DefaultDiscoveryGatewayTimeout = 30 * time.Second
)

// GatewayTransactionStatuser abstracts the canonical gateway inquiry capability.
// Implemented by midtrans.Client.QueryProviderState.
//
// The worker receives the CANONICAL provider state and never re-derives it from
// raw gateway status strings: one provider response must mean the same thing to
// every settle-capable consumer.
type GatewayTransactionStatuser interface {
	QueryProviderState(orderID string) (*midtrans.ProviderStatus, error)
}

// PaymentDiscoveryConfig holds worker configuration.
type PaymentDiscoveryConfig struct {
	PollInterval          time.Duration
	BatchSize             int
	InquiryEligibilityAge time.Duration
	GatewayTimeout        time.Duration
}

// DefaultPaymentDiscoveryConfig returns the canonical REC-5 configuration.
func DefaultPaymentDiscoveryConfig() PaymentDiscoveryConfig {
	return PaymentDiscoveryConfig{
		PollInterval:          DefaultDiscoveryPollInterval,
		BatchSize:             DefaultDiscoveryBatchSize,
		InquiryEligibilityAge: DefaultDiscoveryInquiryEligibilityAge,
		GatewayTimeout:        DefaultDiscoveryGatewayTimeout,
	}
}

// Rec6RefundIntentCreator abstracts the REC-6 canonical refund-intent authority.
// Implemented by *refundapp.RefundService.CreateRefundIntentForInvalidOrder.
type Rec6RefundIntentCreator interface {
	CreateRefundIntentForInvalidOrder(ctx context.Context, tx db.Tx, input refundapp.Rec6RefundIntentInput) (*refundentity.Refund, error)
}

// PaymentDiscoveryWorker implements REC-5 payment-centric discovery.
//
// It scans for pending payments that have not received a webhook signal
// and queries the gateway for truth. On gateway settlement/capture it
// routes to the canonical domain finalization authority.
//
// DESIGN INVARIANTS:
//   - Gateway inquiry is READ-ONLY (no DB mutation during HTTP call)
//   - Canonical lock order: ORDER → PAYMENT
//   - Re-reads current state after gateway inquiry before any mutation
//   - Does NOT settle expired/cancelled orders (routes to REC-6 refund intent)
//   - Uses canonical CanonicalFinalizationService for order payments
//   - Uses canonical SellerSubscriptionPaymentService for subscription payments
type PaymentDiscoveryWorker struct {
	db                    Transactor
	paymentRepo           *paymentRepo.PaymentRepository
	midtransClient        GatewayTransactionStatuser
	canonicalFinal        OrderPaymentFinalizer
	subscriptionPaySvc    SubscriptionPaymentProcessor
	rec6RefundCreator     Rec6RefundIntentCreator
	log                   *zap.Logger
	pollInterval          time.Duration
	batchSize             int
	inquiryEligibilityAge time.Duration
	gatewayTimeout        time.Duration

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// NewPaymentDiscoveryWorker creates the REC-5 payment discovery worker.
func NewPaymentDiscoveryWorker(
	db Transactor,
	midtransClient GatewayTransactionStatuser,
	canonicalFinal OrderPaymentFinalizer,
	subscriptionPaySvc SubscriptionPaymentProcessor,
	log *zap.Logger,
	cfg PaymentDiscoveryConfig,
) *PaymentDiscoveryWorker {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultDiscoveryPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultDiscoveryBatchSize
	}
	if cfg.InquiryEligibilityAge == 0 {
		cfg.InquiryEligibilityAge = DefaultDiscoveryInquiryEligibilityAge
	}
	if cfg.GatewayTimeout == 0 {
		cfg.GatewayTimeout = DefaultDiscoveryGatewayTimeout
	}

	return &PaymentDiscoveryWorker{
		db:                    db,
		paymentRepo:           paymentRepo.NewPaymentRepository(),
		midtransClient:        midtransClient,
		canonicalFinal:        canonicalFinal,
		subscriptionPaySvc:    subscriptionPaySvc,
		log:                   log,
		pollInterval:          cfg.PollInterval,
		batchSize:             cfg.BatchSize,
		inquiryEligibilityAge: cfg.InquiryEligibilityAge,
		gatewayTimeout:        cfg.GatewayTimeout,
		stopCh:                make(chan struct{}),
	}
}

// SetRec6RefundCreator wires the REC-6 canonical refund-intent authority.
//
// MANDATORY: production composition always supplies *refundapp.RefundService.
// The worker fails fast if it is asked to create a refund intent while unset;
// there is deliberately no evidence-only fallback.
func (w *PaymentDiscoveryWorker) SetRec6RefundCreator(creator Rec6RefundIntentCreator) {
	w.rec6RefundCreator = creator
}

// Start begins the periodic discovery scan.
func (w *PaymentDiscoveryWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("payment_discovery_worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("payment_discovery_worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Duration("inquiry_eligibility_age", w.inquiryEligibilityAge),
	)
}

// Stop gracefully shuts down the worker.
func (w *PaymentDiscoveryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("payment_discovery_worker stopping...")
	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("payment_discovery_worker stopped")
	case <-time.After(10 * time.Second):
		w.log.Warn("payment_discovery_worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns whether the worker is active.
func (w *PaymentDiscoveryWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *PaymentDiscoveryWorker) run() {
	defer w.wg.Done()

	// Run immediately on start
	w.scanOnceInternal()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("payment_discovery_worker shutdown requested")
			return
		case <-time.After(w.pollInterval):
			w.scanOnceInternal()
		case <-w.stopCh:
			return
		}
	}
}

// ScanOnce performs one discovery cycle. Exported for integration testing.
func (w *PaymentDiscoveryWorker) ScanOnce() {
	w.scanOnceInternal()
}

func (w *PaymentDiscoveryWorker) scanOnceInternal() {
	ctx := w.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}

	// Guard: DB is required for discovery
	if w.db == nil {
		w.log.Debug("payment_discovery_db_not_wired")
		return
	}

	// Phase 1: Fetch candidate payment IDs (short transaction, SKIP LOCKED)
	candidates, err := w.findCandidates(ctx)
	if err != nil {
		w.log.Error("payment_discovery_find_candidates_failed", zap.Error(err))
		return
	}

	if len(candidates) == 0 {
		return
	}

	w.log.Info("payment_discovery_candidates_found", zap.Int("count", len(candidates)))

	// Phase 2: Process each candidate in its own transaction
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			w.log.Info("payment_discovery_worker_shutdown_mid_batch")
			return
		}
		if err := w.processCandidate(ctx, candidate); err != nil {
			w.log.Error("payment_discovery_candidate_failed",
				zap.String("payment_id", candidate.String()),
				zap.Error(err),
			)
		}
	}
}

// findCandidates retrieves payment IDs eligible for gateway inquiry.
//
// Eligibility predicate:
//   - status = 'pending'
//   - created_at < NOW() - inquiry_eligibility_age
//   - expired_at > NOW() (still inside validity window — not yet expired)
//
// FOR UPDATE SKIP LOCKED only avoids two workers claiming the SAME row inside
// this short discovery transaction. The lock is RELEASED when that transaction
// commits — before the gateway inquiry — so it does NOT provide single-flight
// across the whole workflow: a second worker may legitimately rediscover the
// same payment. Exactly-once settlement is enforced later, by the finalization
// transaction (ORDER → PAYMENT row locks + re-read + the conditional
// pending→settlement transition), never by this SELECT.
func (w *PaymentDiscoveryWorker) findCandidates(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		query := `
			SELECT id
			FROM payments
			WHERE status = $1
			  AND created_at < NOW() - $2::interval
			  AND expired_at > NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		`

		eligibilityInterval := fmt.Sprintf("%d seconds", int(w.inquiryEligibilityAge.Seconds()))

		rows, err := tx.Query(ctx, query,
			paymentRepo.PaymentStatusPending,
			eligibilityInterval,
			w.batchSize,
		)
		if err != nil {
			return fmt.Errorf("query candidates: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan candidate: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})

	return ids, err
}

// processCandidate handles one payment through the full discovery→inquiry→finalization cycle.
//
// CRITICAL: The gateway inquiry is READ-ONLY and happens OUTSIDE any DB lock.
// After the inquiry, a fresh transaction re-locks ORDER → PAYMENT and re-reads
// current state before any mutation.
func (w *PaymentDiscoveryWorker) processCandidate(ctx context.Context, paymentID uuid.UUID) error {
	// Step 1: Load payment (read-only, no lock) to get midtrans_order_id
	var midtransOrderID string
	var referenceType string
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := w.paymentRepo.GetByID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("load payment: %w", err)
		}
		if payment == nil {
			return fmt.Errorf("payment not found: %s", paymentID)
		}
		// Re-check eligibility (payment may have been processed since discovery)
		if payment.Status != paymentRepo.PaymentStatusPending {
			return nil // Already processed, skip silently
		}
		if payment.ExpiredAt.Before(time.Now()) {
			return nil // Expired, skip (REC-6 territory)
		}
		midtransOrderID = payment.MidtransOrderID
		referenceType = payment.ReferenceType
		return nil
	})
	if err != nil {
		return err
	}

	if midtransOrderID == "" {
		return fmt.Errorf("payment %s has no midtrans_order_id", paymentID)
	}

	// Step 2: Gateway inquiry (READ-ONLY, outside DB transaction)
	_, cancel := context.WithTimeout(ctx, w.gatewayTimeout)
	defer cancel()

	providerStatus, err := w.midtransClient.QueryProviderState(midtransOrderID)
	if err != nil {
		// Gateway error/timeout/circuit breaker → no mutation, retry later
		w.log.Debug("payment_discovery_gateway_inquiry_failed",
			zap.String("payment_id", paymentID.String()),
			zap.String("midtrans_order_id", midtransOrderID),
			zap.Error(err),
		)
		return nil // Non-mutating retry state
	}
	if providerStatus == nil {
		return nil
	}

	// Step 3: Route on the canonical provider state
	switch providerStatus.State {
	case midtrans.ProviderStateSettled:
		return w.handleProviderSettled(ctx, paymentID, referenceType, providerStatus)
	case midtrans.ProviderStateFailed:
		return w.handleGatewayFailure(ctx, paymentID, midtransOrderID, providerStatus.Notification)
	case midtrans.ProviderStatePending, midtrans.ProviderStateNotPresent:
		// The gateway is reachable and the money is not ours yet — including the
		// case where the customer has not opened the payment page at all.
		w.log.Debug("payment_discovery_gateway_not_settled",
			zap.String("payment_id", paymentID.String()),
			zap.String("provider_state", string(providerStatus.State)),
		)
		return nil
	default:
		w.log.Debug("payment_discovery_gateway_state_unknown",
			zap.String("payment_id", paymentID.String()),
			zap.String("provider_state", string(providerStatus.State)),
		)
		return nil
	}
}

// handleProviderSettled processes a canonical SETTLED provider state.
//
// CRITICAL: Re-locks ORDER → PAYMENT, re-reads current state, checks validity
// before calling canonical finalization.
func (w *PaymentDiscoveryWorker) handleProviderSettled(
	ctx context.Context,
	paymentID uuid.UUID,
	referenceType string,
	providerStatus *midtrans.ProviderStatus,
) error {
	gatewayStatus := providerStatus.Notification
	if gatewayStatus == nil {
		return nil
	}

	// Provider state SETTLED already enforces the capture/fraud gate and the
	// settled status set (midtrans.ClassifyProviderState — the single authority).

	// Validate amount
	expectedAmount := int64(0)
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := w.paymentRepo.GetByID(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		if payment != nil {
			expectedAmount = payment.GrossAmount.Int64()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("load payment for amount check: %w", err)
	}

	// Parse gateway amount
	gatewayAmount := midtrans.ParseGrossAmount(gatewayStatus.GrossAmount)
	if gatewayAmount != expectedAmount {
		w.log.Warn("payment_discovery_amount_mismatch",
			zap.String("payment_id", paymentID.String()),
			zap.Int64("expected", expectedAmount),
			zap.Int64("gateway", gatewayAmount),
		)
		return nil // Amount mismatch, do not finalize
	}

	// Route to canonical finalization based on reference_type
	switch referenceType {
	case paymentRepo.ReferenceTypeOrder:
		return w.finalizeOrderPayment(ctx, paymentID, gatewayStatus)
	case paymentRepo.ReferenceTypeSubscription:
		return w.finalizeSubscriptionPayment(ctx, paymentID, gatewayStatus)
	default:
		w.log.Warn("payment_discovery_unknown_reference_type",
			zap.String("payment_id", paymentID.String()),
			zap.String("reference_type", referenceType),
		)
		return nil
	}
}

// finalizeOrderPayment routes to CanonicalFinalizationService.
//
// CRITICAL PATTERN:
// 1. Lock ORDER → PAYMENT (canonical lock order)
// 2. Re-read payment state (still pending?)
// 3. Re-read order state (still valid? not expired/cancelled?)
// 4. Call CanonicalFinalizationService.FinalizeOrderPayment
func (w *PaymentDiscoveryWorker) finalizeOrderPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	gatewayStatus *midtrans.NotificationPayload,
) error {
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		// Step 1: Load payment without lock first (to get reference_id for order lock)
		payment, err := w.paymentRepo.GetByID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("load payment: %w", err)
		}
		if payment == nil {
			return fmt.Errorf("payment not found: %s", paymentID)
		}

		// Re-check: must still be pending
		if payment.Status != paymentRepo.PaymentStatusPending {
			w.log.Debug("payment_discovery_payment_no_longer_pending",
				zap.String("payment_id", paymentID.String()),
				zap.String("status", payment.Status),
			)
			return nil // Already processed
		}

		// Step 2: Lock ORDER first (canonical lock order: ORDER → PAYMENT)
		if payment.ReferenceType == paymentRepo.ReferenceTypeOrder && payment.ReferenceID != nil {
			// Use a raw query to avoid importing order application package
			var orderID uuid.UUID
			var orderStatus string
			var orderSellerID uuid.UUID
			var orderExpiresAt time.Time
			orderErr := tx.QueryRow(ctx, `
				SELECT id, status, seller_id, payment_expires_at
				FROM orders WHERE id = $1
				FOR UPDATE
			`, *payment.ReferenceID).Scan(&orderID, &orderStatus, &orderSellerID, &orderExpiresAt)
			if orderErr != nil {
				return fmt.Errorf("lock order: %w", orderErr)
			}

			// CRITICAL: Check order validity — REC-6 boundary.
			// Single canonical authority shared with webhook + orphan recovery.
			if orderentity.IsInvalidForPaymentFinalization(orderentity.Status(orderStatus), orderExpiresAt) {
				w.log.Warn("payment_discovery_order_invalid_rec6",
					zap.String("payment_id", paymentID.String()),
					zap.String("order_id", orderID.String()),
					zap.String("order_status", orderStatus),
				)
				// REC-6: create refund intent, do NOT settle
				return w.createRec6RefundIntent(ctx, tx, payment, orderID, orderSellerID, gatewayStatus)
			}
		}

		// Step 3: Lock payment (after order)
		payment, err = w.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("lock payment: %w", err)
		}

		// Re-check payment state after lock
		if payment.Status != paymentRepo.PaymentStatusPending {
			w.log.Debug("payment_discovery_payment_changed_after_lock",
				zap.String("payment_id", paymentID.String()),
				zap.String("status", payment.Status),
			)
			return nil
		}

		// Step 4: Call canonical finalization
		if w.canonicalFinal == nil {
			return fmt.Errorf("CRITICAL: CanonicalFinalizationService not wired")
		}

		return w.canonicalFinal.FinalizeOrderPayment(
			ctx, tx, payment,
			gatewayStatus.TransactionID,
			gatewayStatus.PaymentType,
		)
	})
}

// finalizeSubscriptionPayment routes to SellerSubscriptionPaymentService.
//
// CRITICAL: ProcessSuccessfulPaymentTx requires the payment to ALREADY be settled.
// The scanner must first settle the payment, then call the subscription processor.
func (w *PaymentDiscoveryWorker) finalizeSubscriptionPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	gatewayStatus *midtrans.NotificationPayload,
) error {
	if w.subscriptionPaySvc == nil {
		return fmt.Errorf("CRITICAL: SellerSubscriptionPaymentService not wired")
	}

	// Step 1: Settle the payment FIRST (required before subscription activation)
	var userID uuid.UUID
	var paymentSettled bool
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := w.paymentRepo.GetByID(ctx, tx, paymentID)
		if err != nil {
			return err
		}
		if payment == nil {
			return fmt.Errorf("payment not found: %s", paymentID)
		}
		if payment.Status != paymentRepo.PaymentStatusPending {
			// Already processed (settled or failed by webhook/other path)
			paymentSettled = payment.IsSettled()
			userID = payment.UserID
			return nil
		}
		userID = payment.UserID

		// Settle the payment using canonical settlement service
		settlementSvc := paymentRepo.NewPaymentSettlementService()
		if err := settlementSvc.SettlePaymentByID(ctx, tx, paymentID, gatewayStatus.TransactionID, gatewayStatus.PaymentType); err != nil {
			return fmt.Errorf("settle subscription payment: %w", err)
		}
		paymentSettled = true
		return nil
	})
	if err != nil {
		return err
	}

	if !paymentSettled {
		return nil // Already processed
	}

	// Step 2: ProcessSuccessfulPayment opens its own transaction with canonical locking
	return w.subscriptionPaySvc.ProcessSuccessfulPayment(
		ctx, paymentID, userID, gatewayStatus.TransactionID,
	)
}

// handleGatewayFailure processes gateway deny/cancel/expire.
func (w *PaymentDiscoveryWorker) handleGatewayFailure(
	ctx context.Context,
	paymentID uuid.UUID,
	midtransOrderID string,
	gatewayStatus *midtrans.NotificationPayload,
) error {
	if gatewayStatus == nil {
		return nil
	}
	// Load payment and delegate to settlement service
	return w.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := w.paymentRepo.GetByID(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("load payment: %w", err)
		}
		if payment == nil {
			return nil
		}

		// Re-check: must still be pending
		if payment.Status != paymentRepo.PaymentStatusPending {
			return nil
		}

		settlementSvc := paymentRepo.NewPaymentSettlementService()
		return settlementSvc.FailPayment(ctx, tx, midtransOrderID, gatewayStatus.TransactionStatus)
	})
}

// createRec6RefundIntent creates a canonical REC-6 refund intent via the
// RefundService (the canonical refund-intent authority).
//
// FAIL FAST: the creator is a mandatory dependency; a missing creator is a
// composition error and must never degrade into an evidence-only write.
func (w *PaymentDiscoveryWorker) createRec6RefundIntent(
	ctx context.Context,
	tx db.Tx,
	payment *paymentRepo.Payment,
	orderID uuid.UUID,
	orderSellerID uuid.UUID,
	gatewayStatus *midtrans.NotificationPayload,
) error {
	if w.rec6RefundCreator == nil {
		return fmt.Errorf("CRITICAL: REC-6 refund intent creator not wired")
	}

	result, err := w.rec6RefundCreator.CreateRefundIntentForInvalidOrder(ctx, tx, refundapp.Rec6RefundIntentInput{
		PaymentID:              payment.ID,
		OrderID:                orderID,
		BuyerID:                payment.UserID,
		SellerID:               orderSellerID,
		GrossAmount:            payment.GrossAmount.Int64(),
		PaymentMidtransOrderID: payment.MidtransOrderID,
	})
	if err != nil {
		w.log.Error("rec6_refund_intent_create_failed",
			zap.String("payment_id", payment.ID.String()),
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		return err
	}

	w.log.Info("rec6_refund_intent_created",
		zap.String("payment_id", payment.ID.String()),
		zap.String("order_id", orderID.String()),
		zap.String("refund_id", result.ID.String()),
		zap.Int64("gross_amount", payment.GrossAmount.Int64()),
	)

	return nil
}
