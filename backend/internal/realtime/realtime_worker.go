package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultPollInterval is how often the worker checks for pending events
	DefaultPollInterval = 5 * time.Second

	// DefaultBatchSize is max events to process per batch
	DefaultBatchSize = 50
)

// Worker processes outbox events for realtime delivery.
//
// Similar to OutboxWorker but focused on chat.message.sent events.
// Implements the same Worker interface for consistency.
//
// PROCESSING FLOW (1 event = 1 transaction):
// 1. Fetch batch of event IDs with SKIP LOCKED
// 2. For each event:
//   - Start new transaction
//   - Mark as processing
//   - Parse and dispatch to hub
//   - Mark as succeeded
//   - Commit transaction
//
// SAFETY:
// - Each event processed in its own transaction
// - FOR UPDATE SKIP LOCKED for concurrent worker support
// - Never blocks on slow clients (buffered channel)
// - No panic propagation
type Worker struct {
	db           *dbpkg.DB
	outboxRepo   OutboxRepository
	dispatcher   *Dispatcher
	log          *zap.Logger
	pollInterval time.Duration
	batchSize    int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	workerID string
}

// WorkerConfig holds worker configuration.
type WorkerConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// DefaultWorkerConfig returns default configuration.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval: DefaultPollInterval,
		BatchSize:    DefaultBatchSize,
	}
}

// OutboxRepository defines the interface for outbox operations.
// This is a minimal interface for the realtime worker.
type OutboxRepository interface {
	FetchPendingBatch(ctx context.Context, tx dbpkg.Tx, limit int) ([]Event, error)
	MarkProcessing(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error
	MarkSucceeded(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID) error
	MarkFailedWithRetry(ctx context.Context, tx dbpkg.Tx, eventID uuid.UUID, retryCount int, nextAttemptAt time.Time) error
}

// Event represents an outbox event.
// This matches the repository.Event structure from the outbox package.
type Event struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       []byte
	Status        string
	RetryCount    int
	NextAttemptAt time.Time
}

// NewWorker creates a new realtime worker.
func NewWorker(
	db *dbpkg.DB,
	outboxRepo OutboxRepository,
	hub *Hub,
	statusChecker auth.AccountStatusChecker,
	log *zap.Logger,
	cfg WorkerConfig,
) *Worker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = DefaultBatchSize
	}

	workerID := fmt.Sprintf("realtime-%s", uuid.New().String()[:8])

	return &Worker{
		db:           db,
		outboxRepo:   outboxRepo,
		dispatcher:   NewDispatcherWithRoomResolver(hub, statusChecker, NewDBChatMessageRoomResolver(db), log),
		log:          log,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
		stopCh:       make(chan struct{}),
		workerID:     workerID,
	}
}

// Start begins processing outbox events in the background.
func (w *Worker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Realtime worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Realtime worker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.String("worker_id", w.workerID),
	)
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping realtime worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Realtime worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Realtime worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running.
func (w *Worker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *Worker) run() {
	defer w.wg.Done()

	// Process immediately on start
	w.processBatch()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.pollInterval):
			w.processBatch()

		case <-w.stopCh:
			return
		}
	}
}

// processBatch processes a batch of pending outbox events.
func (w *Worker) processBatch() {
	ctx := context.Background()

	// Fetch events in a short transaction
	var events []Event
	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		var err error
		events, err = w.outboxRepo.FetchPendingBatch(ctx, tx, w.batchSize)
		return err
	})

	if err != nil {
		w.log.Error("Failed to fetch pending events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	w.log.Debug("Processing realtime batch",
		zap.Int("count", len(events)),
		zap.String("worker_id", w.workerID),
	)

	// Process each event in its own transaction
	var succeeded, failed int
	for _, event := range events {
		// Process only chat realtime events that the realtime dispatcher knows
		// how to deliver to websocket recipients.
		if event.EventType != EventTypeChatMessageSent &&
			event.EventType != EventTypeModerationChatHidden &&
			event.EventType != EventTypeModerationChatRestored &&
			event.EventType != EventTypeChatRoomCreated &&
			event.EventType != EventTypeChatRoomUpdated {
			continue
		}

		if w.processSingleEvent(ctx, event) {
			succeeded++
		} else {
			failed++
		}
	}

	if succeeded > 0 || failed > 0 {
		w.log.Info("Realtime batch processed",
			zap.Int("processed", succeeded+failed),
			zap.Int("succeeded", succeeded),
			zap.Int("failed", failed),
		)
	}
}

// processSingleEvent processes a single event in its own transaction.
// Returns true if successful, false otherwise.
func (w *Worker) processSingleEvent(ctx context.Context, event Event) bool {
	err := w.db.WithTx(ctx, func(tx dbpkg.Tx) error {
		return w.processEventInTx(ctx, tx, event)
	})

	if err != nil {
		w.log.Error("Transaction failed for event",
			zap.String("event_id", event.ID.String()),
			zap.String("event_type", event.EventType),
			zap.Error(err),
		)
		return false
	}

	return true
}

// processEventInTx processes an event within a single transaction.
func (w *Worker) processEventInTx(
	ctx context.Context,
	tx dbpkg.Tx,
	event Event,
) error {
	// Step 1: Mark as processing
	if err := w.outboxRepo.MarkProcessing(ctx, tx, event.ID); err != nil {
		// Event already being processed - skip
		return nil
	}

	// Step 2: Dispatch to hub (non-blocking, won't fail on slow clients)
	if err := w.dispatcher.Dispatch(event.EventType, event.Payload); err != nil {
		// Dispatch failed - schedule retry
		// This is likely a payload parse error, so retry with backoff
		newRetryCount := event.RetryCount + 1
		nextAttemptAt := time.Now().Add(time.Duration(newRetryCount) * time.Second)
		return w.outboxRepo.MarkFailedWithRetry(ctx, tx, event.ID, newRetryCount, nextAttemptAt)
	}

	// Step 3: Mark as succeeded
	if err := w.outboxRepo.MarkSucceeded(ctx, tx, event.ID); err != nil {
		return fmt.Errorf("mark succeeded failed: %w", err)
	}

	w.log.Debug("Event processed successfully",
		zap.String("event_id", event.ID.String()),
		zap.String("event_type", event.EventType),
	)

	return nil
}

// ManualProcess triggers immediate processing of pending events.
// Useful for testing or manual intervention.
func (w *Worker) ManualProcess(ctx context.Context) error {
	w.processBatch()
	return nil
}


