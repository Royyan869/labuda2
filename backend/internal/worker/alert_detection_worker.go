package worker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultAlertDetectionPollInterval is how often the worker runs anomaly detection
	DefaultAlertDetectionPollInterval = 5 * time.Minute

	// DefaultAlertCleanupInterval is how often the worker cleans up old resolved alerts
	DefaultAlertCleanupInterval = 24 * time.Hour

	// DefaultAlertRetentionDays is how long resolved alerts are kept
	DefaultAlertRetentionDays = 90
)

// AlertDetectionConfig holds worker configuration
type AlertDetectionConfig struct {
	PollInterval      time.Duration // How often to run detection
	CleanupInterval   time.Duration // How often to clean up old alerts
	RetentionDays     int           // How long to keep resolved alerts
}

// DefaultAlertDetectionConfig returns default configuration
func DefaultAlertDetectionConfig() AlertDetectionConfig {
	return AlertDetectionConfig{
		PollInterval:    DefaultAlertDetectionPollInterval,
		CleanupInterval: DefaultAlertCleanupInterval,
		RetentionDays:   DefaultAlertRetentionDays,
	}
}

// DetectionRule defines a rule for detecting anomalies.
type DetectionRule interface {
	// Name returns the rule name
	Name() string
	// Detect runs the detection rule and returns true if an anomaly is found
	Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error)
}

// AnomalyFinding represents the result of a detection rule.
type AnomalyFinding struct {
	AlertType  alertentity.AlertType
	Severity   alertentity.AlertSeverity
	EntityType string
	EntityID   uuid.UUID
	Message    string
	Metadata   alertentity.AlertMetadata
	GroupKey   *string
}

// AlertDetectionWorker detects anomalies and creates alerts.
//
// BUSINESS RULES:
// - Scans audit_events for patterns indicating anomalies
// - Creates alerts via AlertService with duplicate prevention
// - Cleans up old resolved alerts periodically
//
// DETECTION RULES:
// 1. payment_failure_spike: Detects sudden increase in payment failures
// 2. payment_stuck: Detects payments stuck in pending state
// 3. dispute_spike: Detects sudden increase in disputes
// 4. seller_risk: Detects sellers with high risk metrics
// 5. coins_anomaly: Detects unusual coin activity
// 6. withdrawal_anomaly: Detects suspicious withdrawal patterns
type AlertDetectionWorker struct {
	db              db.Transactor
	alertService    *application.AlertService
	log             *zap.Logger
	pollInterval    time.Duration
	cleanupInterval time.Duration
	retentionDays   int
	workerID        string // Unique identifier for this worker instance

	metrics WorkerLivenessRecorder // optional sink; nil = no-op

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Context for shutdown
	shutdownCtx context.Context
	cancelFn    context.CancelFunc

	// Detection rules
	rules []DetectionRule
}

// NewAlertDetectionWorker creates a new alert detection worker.
func NewAlertDetectionWorker(
	db db.Transactor,
	alertService *application.AlertService,
	log *zap.Logger,
	cfg AlertDetectionConfig,
) *AlertDetectionWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultAlertDetectionPollInterval
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = DefaultAlertCleanupInterval
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = DefaultAlertRetentionDays
	}

	// Generate unique worker ID for logging
	workerID := uuid.New().String()[:8]

	w := &AlertDetectionWorker{
		db:              db,
		alertService:    alertService,
		log:             log,
		pollInterval:    cfg.PollInterval,
		cleanupInterval: cfg.CleanupInterval,
		retentionDays:   cfg.RetentionDays,
		workerID:        workerID,
		stopCh:          make(chan struct{}),
	}

	// Initialize detection rules
	w.rules = []DetectionRule{
		NewPaymentFailureSpikeRule(db, log),
		NewPaymentStuckRule(db, log),
		NewDisputeSpikeRule(db, log),
		NewSellerRiskRule(db, log),
		NewCoinsAnomalyRule(db, log),
		NewWithdrawalAnomalyRule(db, log),
		NewSubscriptionOrphanedPaymentRule(db, log),
		NewSubscriptionConversionRateRule(db, log),
		NewSubscriptionLifecycleRule(db, log),
		NewStaleDisputeFreezeRule(db, log), // O2: stale freeze operator visibility
		NewOutboxDLQSpikeRule(db, log),     // B23: outbox dead-letter spike detection
		NewOutboxStuckRule(db, log),          // B23: outbox stuck-processing detection
		NewSellerNonShipmentRule(db, log),    // Seller non-shipment alert (observational)
		NewEscrowStuckRule(db, log),          // FIX-2: escrow holding age detection
		NewOrderPaidStuckRule(db, log),       // FIX-3: order paid age detection
		NewOrderShippedStuckRule(db, log),    // FIX-3: order shipped age detection
		NewDisputeOpenStuckRule(db, log),     // FIX-3: dispute under_review age detection
	}

	return w
}

// SetMetricsRecorder attaches an optional liveness sink. Must be called before
// Start(). The recorder is sink-only and never influences detection decisions.
func (w *AlertDetectionWorker) SetMetricsRecorder(r WorkerLivenessRecorder) {
	w.metrics = r
}

// Start begins processing anomaly detection in the background.
func (w *AlertDetectionWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Alert detection worker already running",
			zap.String("worker_id", w.workerID),
		)
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameAlertDetection, true)
	}

	w.wg.Add(1)
	go w.run()

	w.log.Info("Alert detection worker started",
		zap.String("worker_id", w.workerID),
		zap.Duration("poll_interval", w.pollInterval),
		zap.Duration("cleanup_interval", w.cleanupInterval),
		zap.Int("retention_days", w.retentionDays),
		zap.Int("rules", len(w.rules)),
	)
}

// Stop gracefully shuts down the worker.
func (w *AlertDetectionWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping alert detection worker...",
		zap.String("worker_id", w.workerID),
	)

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Alert detection worker stopped gracefully",
			zap.String("worker_id", w.workerID),
		)
	case <-time.After(10 * time.Second):
		w.log.Warn("Alert detection worker shutdown timeout",
			zap.String("worker_id", w.workerID),
		)
	}

	w.running = false
	if w.metrics != nil {
		w.metrics.SetWorkerRunning(WorkerNameAlertDetection, false)
	}
}

// IsRunning returns true if the worker is currently running.
func (w *AlertDetectionWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *AlertDetectionWorker) run() {
	defer w.wg.Done()

	// Initial processing on startup
	w.processDetectionRules()

	// Start cleanup ticker
	cleanupTicker := time.NewTicker(w.cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested",
				zap.String("worker_id", w.workerID),
			)
			return

		case <-time.After(w.pollInterval):
			w.processDetectionRules()

		case <-cleanupTicker.C:
			w.processCleanup()

		case <-w.stopCh:
			return
		}
	}
}

// processDetectionRules runs all detection rules and creates alerts for anomalies.
func (w *AlertDetectionWorker) processDetectionRules() {
	start := time.Now()
	ctx := context.Background()

	w.log.Debug("Running anomaly detection rules",
		zap.String("worker_id", w.workerID),
		zap.Int("rules_count", len(w.rules)),
	)

	var alertsCreated, alertsUpdated int

	for _, rule := range w.rules {
		ruleStart := time.Now()

		detected, finding, err := w.runRule(ctx, rule)
		if err != nil {
			w.log.Error("worker_error",
				zap.String("worker", "alert_detection"),
				zap.String("worker_id", w.workerID),
				zap.String("rule", rule.Name()),
				zap.Error(err),
			)
			continue
		}

		ruleDuration := time.Since(ruleStart)

		if detected {
			result, err := w.alertService.CreateAlert(
				ctx,
				finding.AlertType,
				finding.Severity,
				finding.EntityType,
				finding.EntityID,
				finding.Message,
				finding.Metadata,
				finding.GroupKey,
			)
			if err != nil {
				w.log.Error("Failed to create alert",
					zap.String("worker_id", w.workerID),
					zap.String("rule", rule.Name()),
					zap.Error(err),
				)
				continue
			}

			if result.Created {
				alertsCreated++
			} else {
				alertsUpdated++
			}

			w.log.Info("Anomaly detected",
				zap.String("worker_id", w.workerID),
				zap.String("rule", rule.Name()),
				zap.String("alert_type", string(finding.AlertType)),
				zap.Bool("created", result.Created),
				zap.Duration("detection_time", ruleDuration),
			)
		}
	}

	duration := time.Since(start)

	w.log.Info("worker_run",
		zap.String("worker", "alert_detection"),
		zap.String("worker_id", w.workerID),
		zap.Int("rules_checked", len(w.rules)),
		zap.Int("alerts_created", alertsCreated),
		zap.Int("alerts_updated", alertsUpdated),
		zap.Int("duration_ms", int(duration.Milliseconds())),
	)

	// Heartbeat after full sweep completes. Intentionally placed here (not in
	// a defer) so per-rule errors that cause early-return do NOT advance the
	// heartbeat. Staleness of this timestamp distinguishes a stuck sweep from
	// an idle one.
	if w.metrics != nil {
		w.metrics.RecordWorkerHeartbeat(WorkerNameAlertDetection)
	}
}

// runRule runs a single detection rule within a transaction.
func (w *AlertDetectionWorker) runRule(ctx context.Context, rule DetectionRule) (bool, *AnomalyFinding, error) {
	var detected bool
	var finding *AnomalyFinding

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		detected, finding, err = rule.Detect(ctx, tx)
		return err
	})

	return detected, finding, err
}

// processCleanup cleans up old resolved alerts.
func (w *AlertDetectionWorker) processCleanup() {
	ctx := context.Background()

	deleted, err := w.alertService.CleanupOldAlerts(ctx, w.retentionDays)
	if err != nil {
		w.log.Error("worker_error",
			zap.String("worker", "alert_detection"),
			zap.String("worker_id", w.workerID),
			zap.String("task", "cleanup"),
			zap.Error(err),
		)
		return
	}

	w.log.Info("Alert cleanup completed",
		zap.String("worker_id", w.workerID),
		zap.Int("deleted", deleted),
		zap.Int("retention_days", w.retentionDays),
	)
}

// ManualProcess triggers immediate processing of all detection rules.
// Useful for testing or manual intervention.
func (w *AlertDetectionWorker) ManualProcess(ctx context.Context) error {
	w.processDetectionRules()
	return nil
}

// ManualCleanup triggers immediate cleanup of old alerts.
func (w *AlertDetectionWorker) ManualCleanup(ctx context.Context) error {
	w.processCleanup()
	return nil
}


