// Package worker provides background workers for periodic tasks.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

const (
	// DefaultWithdrawalMonitoringInterval is how often the worker checks for stuck withdrawals
	DefaultWithdrawalMonitoringInterval = 30 * time.Minute

	// DefaultWithdrawalStuckThreshold is how long a withdrawal can be in PROCESSING/REQUESTED before alerting
	DefaultWithdrawalStuckThreshold = 24 * time.Hour

	// WithdrawalMonitoringQueryLimit is the maximum number of stuck withdrawals to return per status
	// This prevents full table scans and limits memory usage during abnormal spikes
	WithdrawalMonitoringQueryLimit = 1000

	// WithdrawalMonitoringAbnormalThreshold is the count that triggers a warning about abnormal spike
	WithdrawalMonitoringAbnormalThreshold = 100
)

// Withdrawal status constants from schema
const (
	WithdrawalStatusRequested  = "REQUESTED"
	WithdrawalStatusProcessing = "PROCESSING"
	WithdrawalStatusCompleted  = "COMPLETED"
	WithdrawalStatusFailed     = "FAILED"
)

// StuckWithdrawal represents a withdrawal that has been in a pending state too long.
type StuckWithdrawal struct {
	ID              uuid.UUID `json:"id"`
	SellerID        uuid.UUID `json:"seller_id"`
	Amount          int64     `json:"amount"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	HoursSinceState float64   `json:"hours_since_state"`
}

// WithdrawalMonitoringConfig holds worker configuration.
type WithdrawalMonitoringConfig struct {
	Interval      time.Duration // How often to run monitoring checks
	StuckThreshold time.Duration // How long before considering a withdrawal "stuck"
}

// DefaultWithdrawalMonitoringConfig returns default configuration.
func DefaultWithdrawalMonitoringConfig() WithdrawalMonitoringConfig {
	return WithdrawalMonitoringConfig{
		Interval:      DefaultWithdrawalMonitoringInterval,
		StuckThreshold: DefaultWithdrawalStuckThreshold,
	}
}

// WithdrawalMonitoringWorker monitors withdrawals stuck in PROCESSING or REQUESTED state.
//
// This is a READ-ONLY monitoring worker that:
// - Scans withdrawals stuck in PROCESSING or REQUESTED for too long
// - Logs critical alerts when stuck withdrawals are found
// - Provides hooks for admin dashboard notifications
// - Does NOT mutate ledger or withdrawal state automatically
//
// Alert conditions:
// - status = 'PROCESSING' AND updated_at < NOW() - threshold
// - status = 'REQUESTED' AND updated_at < NOW() - threshold
type WithdrawalMonitoringWorker struct {
	db    Transactor
	log   *zap.Logger
	cfg   WithdrawalMonitoringConfig

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	shutdownCtx context.Context
	cancelFn    context.CancelFunc
}

// AdminAlertNotifier is an optional interface for sending alerts to admin dashboard.
type AdminAlertNotifier interface {
	// NotifyWithdrawalStuck sends an alert about stuck withdrawals to admin dashboard
	NotifyWithdrawalStuck(ctx context.Context, stuck []StuckWithdrawal) error
}

// NewWithdrawalMonitoringWorker creates a new withdrawal monitoring worker.
func NewWithdrawalMonitoringWorker(
	db Transactor,
	log *zap.Logger,
	cfg WithdrawalMonitoringConfig,
) *WithdrawalMonitoringWorker {
	if log == nil {
		log = zap.NewNop()
	}

	if cfg.Interval == 0 {
		cfg.Interval = DefaultWithdrawalMonitoringInterval
	}
	if cfg.StuckThreshold == 0 {
		cfg.StuckThreshold = DefaultWithdrawalStuckThreshold
	}

	return &WithdrawalMonitoringWorker{
		db:  db,
		log: log,
		cfg: cfg,
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic withdrawal monitoring in the background.
func (w *WithdrawalMonitoringWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Warn("Withdrawal monitoring worker already running")
		return
	}

	w.running = true
	w.shutdownCtx, w.cancelFn = context.WithCancel(context.Background())
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.run()

	w.log.Info("Withdrawal monitoring worker started",
		zap.Duration("interval", w.cfg.Interval),
		zap.Duration("stuck_threshold", w.cfg.StuckThreshold),
	)
}

// Stop gracefully shuts down the worker.
func (w *WithdrawalMonitoringWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.log.Info("Stopping withdrawal monitoring worker...")

	w.cancelFn()
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.log.Info("Withdrawal monitoring worker stopped gracefully")
	case <-time.After(10 * time.Second):
		w.log.Warn("Withdrawal monitoring worker shutdown timeout")
	}

	w.running = false
}

// IsRunning returns true if the worker is currently running.
func (w *WithdrawalMonitoringWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// run is the main worker loop.
func (w *WithdrawalMonitoringWorker) run() {
	defer w.wg.Done()

	w.runOnce()

	for {
		select {
		case <-w.shutdownCtx.Done():
			w.log.Info("Worker shutdown requested")
			return

		case <-time.After(w.cfg.Interval):
			w.runOnce()

		case <-w.stopCh:
			return
		}
	}
}

// RunOnce executes a single monitoring check.
// Can be called manually for on-demand verification.
func (w *WithdrawalMonitoringWorker) RunOnce() ([]StuckWithdrawal, error) {
	return w.runOnce()
}

// runOnce executes the withdrawal monitoring check.
// Returns any stuck withdrawals found.
func (w *WithdrawalMonitoringWorker) runOnce() ([]StuckWithdrawal, error) {
	ctx := context.Background()

	w.log.Debug("Starting withdrawal monitoring check")

	var allStuck []StuckWithdrawal

	// Use read-only transaction for consistency
	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Check for stuck PROCESSING withdrawals
		processingStuck, err := w.scanStuckWithdrawals(ctx, tx, WithdrawalStatusProcessing)
		if err != nil {
			w.log.Error("Failed to scan processing withdrawals", zap.Error(err))
			// Continue to check REQUESTED status
		} else {
			allStuck = append(allStuck, processingStuck...)
		}

		// Check for stuck REQUESTED withdrawals
		requestedStuck, err := w.scanStuckWithdrawals(ctx, tx, WithdrawalStatusRequested)
		if err != nil {
			w.log.Error("Failed to scan requested withdrawals", zap.Error(err))
		} else {
			allStuck = append(allStuck, requestedStuck...)
		}

		return nil
	})

	if len(allStuck) > 0 {
		w.logCriticalAlerts(allStuck)
	} else {
		w.log.Debug("Withdrawal monitoring check passed - no stuck withdrawals found")
	}

	return allStuck, err
}

// scanStuckWithdrawals scans for withdrawals stuck in a specific status.
func (w *WithdrawalMonitoringWorker) scanStuckWithdrawals(
	ctx context.Context,
	tx db.Tx,
	status string,
) ([]StuckWithdrawal, error) {
	query := `
		SELECT id, seller_id, amount, status, created_at, updated_at,
		       EXTRACT(EPOCH FROM (NOW() - updated_at)) / 3600 as hours_since_update
		FROM withdrawals
		WHERE status = $1
		  AND updated_at < NOW() - INTERVAL '1 second' * $2
		ORDER BY updated_at ASC
		LIMIT ` + fmt.Sprint(WithdrawalMonitoringQueryLimit) + `;
	`

	rows, err := tx.Query(ctx, query, status, int(w.cfg.StuckThreshold.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("failed to query stuck withdrawals: %w", err)
	}
	defer rows.Close()

	var stuck []StuckWithdrawal
	for rows.Next() {
		var s StuckWithdrawal
		if err := rows.Scan(
			&s.ID, &s.SellerID, &s.Amount, &s.Status,
			&s.CreatedAt, &s.UpdatedAt, &s.HoursSinceState,
		); err != nil {
			w.log.Error("Failed to scan stuck withdrawal row", zap.Error(err))
			continue
		}
		stuck = append(stuck, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stuck withdrawals: %w", err)
	}

	// Log warning if count exceeds abnormal threshold (may indicate a systemic issue)
	if len(stuck) >= WithdrawalMonitoringAbnormalThreshold {
		w.log.Warn("Abnormal spike in stuck withdrawals detected",
			zap.String("status", status),
			zap.Int("count", len(stuck)),
			zap.Int("abnormal_threshold", WithdrawalMonitoringAbnormalThreshold),
			zap.Duration("stuck_threshold", w.cfg.StuckThreshold),
		)
	}

	return stuck, nil
}

// logCriticalAlerts logs detailed critical alerts for stuck withdrawals.
func (w *WithdrawalMonitoringWorker) logCriticalAlerts(stuck []StuckWithdrawal) {
	// Group by status for better logging
	byStatus := make(map[string][]StuckWithdrawal)
	for _, s := range stuck {
		byStatus[s.Status] = append(byStatus[s.Status], s)
	}

	// Log summary by status
	for status, items := range byStatus {
		fields := []zap.Field{
			zap.String("status", status),
			zap.Int("count", len(items)),
			zap.Duration("stuck_threshold", w.cfg.StuckThreshold),
		}
		// Add warning if we hit the query limit (there may be more stuck withdrawals)
		if len(items) >= WithdrawalMonitoringQueryLimit {
			fields = append(fields, zap.Bool("query_limit_reached", true))
			fields = append(fields, zap.Int("query_limit", WithdrawalMonitoringQueryLimit))
		}
		w.log.Error("CRITICAL: Stuck withdrawals detected", fields...)

		// Log individual items (limit to first 10 to avoid log spam)
		for i, item := range items {
			if i >= 10 {
				w.log.Error("... additional stuck withdrawals omitted",
					zap.Int("omitted_count", len(items)-10),
				)
				break
			}

			w.log.Error("Stuck withdrawal details",
				zap.String("withdrawal_id", item.ID.String()),
				zap.String("seller_id", item.SellerID.String()),
				zap.Int64("amount", item.Amount),
				zap.String("status", item.Status),
				zap.Float64("hours_since_update", item.HoursSinceState),
				zap.Time("created_at", item.CreatedAt),
				zap.Time("updated_at", item.UpdatedAt),
			)
		}
	}

	// Log total summary
	w.log.Error("Withdrawal monitoring check completed with CRITICAL issues",
		zap.Int("total_stuck", len(stuck)),
	)
}

// NotifyAdminDashboard sends alerts to the admin dashboard.
// This is an optional operation that requires an AdminAlertNotifier.
// If notifier is nil, the method is a no-op (logs only, no panic).
func (w *WithdrawalMonitoringWorker) NotifyAdminDashboard(
	notifier AdminAlertNotifier,
) error {
	ctx := context.Background()

	stuck, err := w.runOnce()
	if err != nil {
		return fmt.Errorf("failed to scan for stuck withdrawals: %w", err)
	}

	if len(stuck) == 0 {
		return nil
	}

	if notifier == nil {
		w.log.Warn("NotifyAdminDashboard: notifier is nil, stuck withdrawals logged but not dispatched to dashboard",
			zap.Int("stuck_count", len(stuck)),
		)
		return nil
	}

	if err := notifier.NotifyWithdrawalStuck(ctx, stuck); err != nil {
		w.log.Error("Failed to notify admin dashboard of stuck withdrawals",
			zap.Error(err),
			zap.Int("stuck_count", len(stuck)),
		)
		return fmt.Errorf("failed to notify admin dashboard: %w", err)
	}

	w.log.Info("Admin dashboard notified of stuck withdrawals",
		zap.Int("stuck_count", len(stuck)),
	)

	return nil
}

// GetStuckWithdrawals returns the current list of stuck withdrawals.
// This is a convenience method for external callers.
func (w *WithdrawalMonitoringWorker) GetStuckWithdrawals(ctx context.Context) ([]StuckWithdrawal, error) {
	var allStuck []StuckWithdrawal

	err := w.db.WithTx(ctx, func(tx db.Tx) error {
		// Check PROCESSING
		processingStuck, err := w.scanStuckWithdrawals(ctx, tx, WithdrawalStatusProcessing)
		if err != nil {
			return err
		}
		allStuck = append(allStuck, processingStuck...)

		// Check REQUESTED
		requestedStuck, err := w.scanStuckWithdrawals(ctx, tx, WithdrawalStatusRequested)
		if err != nil {
			return err
		}
		allStuck = append(allStuck, requestedStuck...)

		return nil
	})

	return allStuck, err
}

// HealthCheck returns the current health status of withdrawals.
// Returns an error if there are withdrawals stuck beyond the threshold.
func (w *WithdrawalMonitoringWorker) HealthCheck(ctx context.Context) error {
	stuck, err := w.GetStuckWithdrawals(ctx)
	if err != nil {
		return fmt.Errorf("failed to check withdrawal health: %w", err)
	}

	if len(stuck) > 0 {
		return fmt.Errorf("found %d stuck withdrawals (threshold: %v)",
			len(stuck), w.cfg.StuckThreshold)
	}

	return nil
}

// WithdrawalMonitoringStats represents statistics about the withdrawal monitoring.
type WithdrawalMonitoringStats struct {
	TotalStuckProcessing int `json:"total_stuck_processing"`
	TotalStuckRequested  int `json:"total_stuck_requested"`
	TotalStuck           int `json:"total_stuck"`
	LongestStuckHours    float64 `json:"longest_stuck_hours"`
	TotalAmountStuck     int64 `json:"total_amount_stuck"`
	CheckedAt            time.Time `json:"checked_at"`
}

// GetStats returns statistics about stuck withdrawals.
func (w *WithdrawalMonitoringWorker) GetStats(ctx context.Context) (WithdrawalMonitoringStats, error) {
	stats := WithdrawalMonitoringStats{
		CheckedAt: time.Now(),
	}

	stuck, err := w.GetStuckWithdrawals(ctx)
	if err != nil {
		return stats, err
	}

	stats.TotalStuck = len(stuck)

	for _, s := range stuck {
		if s.Status == WithdrawalStatusProcessing {
			stats.TotalStuckProcessing++
		} else if s.Status == WithdrawalStatusRequested {
			stats.TotalStuckRequested++
		}

		if s.HoursSinceState > stats.LongestStuckHours {
			stats.LongestStuckHours = s.HoursSinceState
		}

		stats.TotalAmountStuck += s.Amount
	}

	return stats, nil
}


