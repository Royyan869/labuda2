package worker

import (
	"context"
	"sync"
	"time"

	contractApp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"go.uber.org/zap"
)

const (
	// DefaultPromotionFinalizationInterval is how often the worker scans for
	// contracts whose planned delivery window has reached planned_finish.
	DefaultPromotionFinalizationInterval = 1 * time.Minute

	// DefaultPromotionFinalizationBatchSize is the max contracts finalized
	// per cycle. Finalization of each contract is its own transaction, so a
	// large backlog drains a batch per cycle without long-lived transactions.
	DefaultPromotionFinalizationBatchSize = 100
)

// PromotionFinalizationWorker triggers AUTOMATIC finalization at planned
// finish (Owner business truth: planned-finish completion finalizes the
// contract and releases unused allocation to the seller's Promote Balance —
// the seller must never have to finalize manually just to recover funds).
//
// AUTHORITY: this worker is an ORCHESTRATION TRIGGER ONLY. It contains NO
// finalization logic, NO ledger writes and NO release logic of its own — it
// only observes which contracts are due and delegates each one to the single
// canonical PromotionContractService.FinalizeBySystem boundary (which shares
// the exact same finalization core as the seller stop path).
//
// CONCURRENCY: the contract row lock (FOR UPDATE) inside the canonical
// boundary serializes concurrent triggers. If two workers/processes observe
// the same contract, exactly one finalizes it; the second observes the
// finalized status and is a no-op. The ledger release idempotency key
// (promotion_allocation_release_<contract_id>) guarantees the remaining
// allocation is released EXACTLY ONCE regardless of process count.
type PromotionFinalizationWorker struct {
	finalizer ContractFinalizer
	logger    *zap.Logger
	interval  time.Duration
	batchSize int

	mu          sync.RWMutex
	running     bool
	shutdownCtx context.Context
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// ContractFinalizer is the orchestration surface the worker consumes. It is
// implemented by the canonical PromotionContractService (FinalizeDueContracts
// delegates every contract to the single canonical finalization boundary).
type ContractFinalizer interface {
	FinalizeDueContracts(ctx context.Context, limit int) (int, error)
}

// compile-time proof the canonical service satisfies the orchestration surface.
var _ ContractFinalizer = (*contractApp.PromotionContractService)(nil)

// NewPromotionFinalizationWorker wires the planned-finish finalization worker.
func NewPromotionFinalizationWorker(
	finalizer ContractFinalizer,
	logger *zap.Logger,
) *PromotionFinalizationWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PromotionFinalizationWorker{
		finalizer: finalizer,
		logger:    logger,
		interval:  DefaultPromotionFinalizationInterval,
		batchSize: DefaultPromotionFinalizationBatchSize,
	}
}

// Start begins the finalization loop.
func (w *PromotionFinalizationWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.wg.Add(1)
	go w.loop()
}

// Stop gracefully stops the loop.
func (w *PromotionFinalizationWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.cancelFn()
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *PromotionFinalizationWorker) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.shutdownCtx.Done():
			return
		case <-ticker.C:
			w.runOnce()
		}
	}
}

func (w *PromotionFinalizationWorker) runOnce() {
	ctx, cancel := context.WithTimeout(w.shutdownCtx, w.interval)
	defer cancel()

	finalized, err := w.finalizer.FinalizeDueContracts(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("promotion_planned_finish_cycle_failed", zap.Error(err))
		return
	}
	if finalized > 0 {
		w.logger.Info("promotion_planned_finish_cycle_completed",
			zap.Int("finalized_count", finalized),
		)
	}
}
