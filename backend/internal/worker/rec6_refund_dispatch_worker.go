package worker

// REC-6 SLICE 2: refund dispatch worker.
//
// Sole runtime actor that drives canonical REC-6 refund intents (created by
// RefundService.CreateRefundIntentForInvalidOrder) to the payment gateway.
// The worker holds NO business logic of its own: it periodically invokes the
// single dispatch authority, RefundService.DispatchPendingRec6Refunds, which
// claims intents with SKIP LOCKED, calls the existing RefundWithKey gateway
// capability outside any DB transaction, and persists
// gateway_status='pending' (requested/submitted — NOT confirmed).
//
// Idempotent by construction: re-running can neither create a second intent,
// double-dispatch a succeeded refund, nor fabricate a confirmed state. All
// financial effects (ledger, escrow, order, seller payable) remain untouched
// by this slice.

import (
	"context"
	"errors"
	"sync"
	"time"

	refundapp "github.com/labuda/backend/internal/finance/refund/application"
	"go.uber.org/zap"
)

const (
	// DefaultRec6RefundDispatchInterval is how often the worker scans for
	// dispatchable REC-6 refund intents.
	DefaultRec6RefundDispatchInterval = 30 * time.Second

	// DefaultRec6RefundDispatchBatchSize caps gateway submissions per tick so
	// one saturated tick cannot monopolize the gateway client.
	DefaultRec6RefundDispatchBatchSize = 50
)

// Rec6RefundDispatcher is the canonical REC-6 refund dispatch authority,
// implemented by *refundapp.RefundService.DispatchPendingRec6Refunds.
type Rec6RefundDispatcher interface {
	DispatchPendingRec6Refunds(ctx context.Context, limit int) (int, error)
}

// Rec6RefundDispatchConfig holds worker configuration.
type Rec6RefundDispatchConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// DefaultRec6RefundDispatchConfig returns the canonical Slice-2 configuration.
func DefaultRec6RefundDispatchConfig() Rec6RefundDispatchConfig {
	return Rec6RefundDispatchConfig{
		PollInterval: DefaultRec6RefundDispatchInterval,
		BatchSize:    DefaultRec6RefundDispatchBatchSize,
	}
}

// Rec6RefundDispatchWorker periodically dispatches pending REC-6 refund
// intents to the gateway through the RefundService dispatch authority.
type Rec6RefundDispatchWorker struct {
	dispatcher   Rec6RefundDispatcher
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewRec6RefundDispatchWorker creates the Slice-2 dispatch worker.
// dispatcher and db are mandatory dependencies (fail fast: a nil dispatcher
// is a composition error, matching the REC-6 Slice-1 producer policy).
func NewRec6RefundDispatchWorker(
	dispatcher Rec6RefundDispatcher,
	log *zap.Logger,
	cfg Rec6RefundDispatchConfig,
) *Rec6RefundDispatchWorker {
	if log == nil {
		log = zap.NewNop()
	}
	return &Rec6RefundDispatchWorker{
		dispatcher:   dispatcher,
		log:          log,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
	}
}

// Start launches the periodic dispatch loop.
func (w *Rec6RefundDispatchWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		w.log.Warn("rec6_refund_dispatch_worker already running")
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.wg.Add(1)
	go w.run()
	w.log.Info("rec6_refund_dispatch_worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
	)
}

// Stop gracefully shuts the worker down.
func (w *Rec6RefundDispatchWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.log.Info("rec6_refund_dispatch_worker stopping...")
	close(w.stopCh)
	w.mu.Unlock()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		w.log.Info("rec6_refund_dispatch_worker stopped")
	case <-time.After(10 * time.Second):
		w.log.Warn("rec6_refund_dispatch_worker shutdown timeout")
	}
}

// ScanOnce performs exactly one dispatch cycle (also the test entry point).
// A gateway-client-not-configured error is expected while the gateway client
// is not wired and is downgraded to a warning; every other cycle error is
// logged as an error. Per-refund dispatch failures are already counted and
// persisted inside the dispatch authority.
func (w *Rec6RefundDispatchWorker) ScanOnce() {
	failures, err := w.dispatcher.DispatchPendingRec6Refunds(context.Background(), w.batchSize)
	if err != nil {		if errors.Is(err, refundapp.ErrGatewayClientNotConfigured) {
			w.log.Warn("rec6_refund_dispatch_gateway_not_configured")
			return
		}
		w.log.Error("rec6_refund_dispatch_cycle_failed", zap.Error(err))
		return
	}
	if failures > 0 {
		w.log.Warn("rec6_refund_dispatch_cycle_had_failures", zap.Int("failures", failures))
	}
}

// run is the polling loop.
func (w *Rec6RefundDispatchWorker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		w.ScanOnce()
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
		}
	}
}
