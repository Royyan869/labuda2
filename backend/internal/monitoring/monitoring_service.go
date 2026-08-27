package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MonitoringService performs read-only production monitoring checks.
// All checks are SELECT queries only - no mutations allowed.
type MonitoringService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewMonitoringService creates a new monitoring service
func NewMonitoringService(db *pgxpool.Pool, logger *zap.Logger) *MonitoringService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MonitoringService{
		db:     db,
		logger: logger,
	}
}

// CheckResult represents the result of a single monitoring check
type CheckResult struct {
	Name    string
	Status  string // "OK", "WARNING", "ERROR"
	Message string
	Count   int
	Details []string
}

// RunChecks executes all monitoring checks and returns results
func (s *MonitoringService) RunChecks(ctx context.Context) []CheckResult {
	s.logger.Debug("Starting monitoring checks")

	var results []CheckResult

	// Check A: Ledger Imbalance Check
	results = append(results, s.checkLedgerImbalance(ctx))

	// Check B: Escrow Stuck Check
	results = append(results, s.checkEscrowStuck(ctx))

	// Check C: Withdrawal Stuck Check
	results = append(results, s.checkWithdrawalStuck(ctx))

	// Check D: Auction Settlement Stuck
	results = append(results, s.checkAuctionSettlementStuck(ctx))

	return results
}

// checkLedgerImbalance verifies ledger balance (double-entry invariant)
// Query: SUM(total_debit) - SUM(total_credit) should equal 0
func (s *MonitoringService) checkLedgerImbalance(ctx context.Context) CheckResult {
	const query = `
		SELECT COALESCE(SUM(total_debit), 0) - COALESCE(SUM(total_credit), 0) as balance
		FROM ledger_transactions;
	`

	var balance int64
	err := s.db.QueryRow(ctx, query).Scan(&balance)
	if err != nil {
		s.logger.Error("Ledger imbalance check query failed", zap.Error(err))
		return CheckResult{
			Name:    "Ledger Imbalance",
			Status:  "ERROR",
			Message: fmt.Sprintf("Query failed: %v", err),
		}
	}

	if balance != 0 {
		s.logger.Error("CRITICAL: Ledger imbalance detected",
			zap.Int64("balance", balance),
		)
		return CheckResult{
			Name:    "Ledger Imbalance",
			Status:  "ERROR",
			Message: "ALERT: Ledger is not balanced!",
			Count:   1,
			Details: []string{fmt.Sprintf("Balance difference: %d", balance)},
		}
	}

	s.logger.Debug("Ledger imbalance check passed", zap.Int64("balance", balance))
	return CheckResult{
		Name:    "Ledger Imbalance",
		Status:  "OK",
		Message: "Ledger is balanced",
	}
}

// checkEscrowStuck finds orders that are shipped but past auto_release_at
// Query: orders with status='shipped', auto_release_at < NOW(), escrow_status='holding'
func (s *MonitoringService) checkEscrowStuck(ctx context.Context) CheckResult {
	const query = `
		SELECT id, buyer_id, seller_id, auto_release_at
		FROM orders
		WHERE status = 'shipped'
		  AND auto_release_at < NOW()
		  AND escrow_status = 'holding'
		LIMIT 100;
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		s.logger.Error("Escrow stuck check query failed", zap.Error(err))
		return CheckResult{
			Name:    "Escrow Stuck",
			Status:  "ERROR",
			Message: fmt.Sprintf("Query failed: %v", err),
		}
	}
	defer rows.Close()

	type stuckEscrow struct {
		ID          string
		BuyerID     string
		SellerID    string
		AutoRelease string
	}

	var stuckItems []stuckEscrow
	for rows.Next() {
		var item struct {
			ID          string
			BuyerID     string
			SellerID    string
			AutoRelease string
		}
		if err := rows.Scan(&item.ID, &item.BuyerID, &item.SellerID, &item.AutoRelease); err != nil {
			s.logger.Error("Failed to scan escrow stuck row", zap.Error(err))
			continue
		}
		stuckItems = append(stuckItems, item)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating escrow stuck results", zap.Error(err))
	}

	if len(stuckItems) > 0 {
		s.logger.Warn("Escrow stuck items detected",
			zap.Int("count", len(stuckItems)),
		)

		details := make([]string, min(len(stuckItems), 10))
		for i, item := range stuckItems {
			if i >= 10 {
				break
			}
			details[i] = fmt.Sprintf("OrderID: %s, AutoReleaseAt: %s", item.ID, item.AutoRelease)
		}

		return CheckResult{
			Name:    "Escrow Stuck",
			Status:  "WARNING",
			Message: "Orders past auto-release time",
			Count:   len(stuckItems),
			Details: details,
		}
	}

	s.logger.Debug("Escrow stuck check passed")
	return CheckResult{
		Name:    "Escrow Stuck",
		Status:  "OK",
		Message: "No stuck escrow orders",
	}
}

// checkWithdrawalStuck finds withdrawals stuck in processing status
// Query: withdrawals with status='processing' and updated_at < NOW() - 24 hours
func (s *MonitoringService) checkWithdrawalStuck(ctx context.Context) CheckResult {
	const query = `
		SELECT id, seller_id, amount, updated_at
		FROM withdrawals
		WHERE status = 'PROCESSING'
		  AND updated_at < NOW() - INTERVAL '24 hours'
		LIMIT 100;
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		s.logger.Error("Withdrawal stuck check query failed", zap.Error(err))
		return CheckResult{
			Name:    "Withdrawal Stuck",
			Status:  "ERROR",
			Message: fmt.Sprintf("Query failed: %v", err),
		}
	}
	defer rows.Close()

	type stuckWithdrawal struct {
		ID        string
		SellerID  string
		Amount    int64
		UpdatedAt string
	}

	var stuckItems []stuckWithdrawal
	for rows.Next() {
		var item struct {
			ID        string
			SellerID  string
			Amount    int64
			UpdatedAt string
		}
		if err := rows.Scan(&item.ID, &item.SellerID, &item.Amount, &item.UpdatedAt); err != nil {
			s.logger.Error("Failed to scan withdrawal stuck row", zap.Error(err))
			continue
		}
		stuckItems = append(stuckItems, item)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating withdrawal stuck results", zap.Error(err))
	}

	if len(stuckItems) > 0 {
		s.logger.Warn("Withdrawal stuck items detected",
			zap.Int("count", len(stuckItems)),
		)

		details := make([]string, min(len(stuckItems), 10))
		for i, item := range stuckItems {
			if i >= 10 {
				break
			}
			details[i] = fmt.Sprintf("WithdrawalID: %s, Amount: %d, UpdatedAt: %s",
				item.ID, item.Amount, item.UpdatedAt)
		}

		return CheckResult{
			Name:    "Withdrawal Stuck",
			Status:  "WARNING",
			Message: "Withdrawals in processing > 24 hours",
			Count:   len(stuckItems),
			Details: details,
		}
	}

	s.logger.Debug("Withdrawal stuck check passed")
	return CheckResult{
		Name:    "Withdrawal Stuck",
		Status:  "OK",
		Message: "No stuck withdrawals",
	}
}

// checkAuctionSettlementStuck finds ended auctions without orders
// Query: auctions with status='ended', order_id IS NULL, and has bids
func (s *MonitoringService) checkAuctionSettlementStuck(ctx context.Context) CheckResult {
	const query = `
		SELECT a.id, a.seller_id, a.ended_at
		FROM auctions a
		WHERE a.status = 'ended'
		  AND a.order_id IS NULL
		  AND EXISTS (
		    SELECT 1 FROM auction_bids WHERE auction_id = a.id
		  )
		LIMIT 100;
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		s.logger.Error("Auction settlement stuck check query failed", zap.Error(err))
		return CheckResult{
			Name:    "Auction Settlement Stuck",
			Status:  "ERROR",
			Message: fmt.Sprintf("Query failed: %v", err),
		}
	}
	defer rows.Close()

	type stuckAuction struct {
		ID       string
		SellerID string
		EndedAt  string
	}

	var stuckItems []stuckAuction
	for rows.Next() {
		var item struct {
			ID       string
			SellerID string
			EndedAt  string
		}
		if err := rows.Scan(&item.ID, &item.SellerID, &item.EndedAt); err != nil {
			s.logger.Error("Failed to scan auction stuck row", zap.Error(err))
			continue
		}
		stuckItems = append(stuckItems, item)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating auction stuck results", zap.Error(err))
	}

	if len(stuckItems) > 0 {
		s.logger.Error("Auction settlement stuck items detected",
			zap.Int("count", len(stuckItems)),
		)

		details := make([]string, min(len(stuckItems), 10))
		for i, item := range stuckItems {
			if i >= 10 {
				break
			}
			details[i] = fmt.Sprintf("AuctionID: %s, EndedAt: %s", item.ID, item.EndedAt)
		}

		return CheckResult{
			Name:    "Auction Settlement Stuck",
			Status:  "ERROR",
			Message: "Ended auctions without trade settlement",
			Count:   len(stuckItems),
			Details: details,
		}
	}

	s.logger.Debug("Auction settlement stuck check passed")
	return CheckResult{
		Name:    "Auction Settlement Stuck",
		Status:  "OK",
		Message: "No stuck auction settlements",
	}
}

// SystemHealthStatus represents the overall system health status
type SystemHealthStatus struct {
	LedgerBalanced           bool      `json:"ledger_balanced"`
	LedgerImbalanceValue     int64     `json:"ledger_imbalance_value"`
	EscrowStuckCount         int       `json:"escrow_stuck_count"`
	WithdrawalStuckCount     int       `json:"withdrawal_stuck_count"`
	AuctionStuckCount        int       `json:"auction_stuck_count"`
	OutboxPendingCount       int       `json:"outbox_pending_count"`
	OutboxStuckCount         int       `json:"outbox_stuck_count"`
	OutboxDeadLetterCount    int       `json:"outbox_dead_letter_count"`
	OutboxLagSeconds         float64   `json:"outbox_lag_seconds"`

	// Subscription Health Metrics
	OrphanedPaymentCount              int     `json:"orphaned_payment_count"`
	PaymentSubscriptionConversionRate float64 `json:"payment_subscription_conversion_rate"`
	ActiveSubscriptionCount           int     `json:"active_subscription_count"`
	ExpiringSubscriptionCount         int     `json:"expiring_subscription_count"`
	ExpiredSubscriptionCount          int     `json:"expired_subscription_count"`

	RealtimeActiveConnections int      `json:"realtime_active_connections"`
	Goroutines               int       `json:"goroutines"`
	LastCheckedAt            time.Time `json:"last_checked_at"`
}

// GetSystemHealth returns the current system health status
// Read-only queries only - no mutations
func (s *MonitoringService) GetSystemHealth(ctx context.Context) (SystemHealthStatus, error) {
	status := SystemHealthStatus{
		LastCheckedAt: time.Now(),
	}

	// 1. Ledger Balance Check
	const ledgerQuery = `
		SELECT COALESCE(SUM(total_debit), 0) - COALESCE(SUM(total_credit), 0) as balance
		FROM ledger_transactions;
	`
	err := s.db.QueryRow(ctx, ledgerQuery).Scan(&status.LedgerImbalanceValue)
	if err != nil {
		status.LedgerBalanced = false
		return status, fmt.Errorf("ledger check failed: %w", err)
	}
	status.LedgerBalanced = (status.LedgerImbalanceValue == 0)

	// 2. Escrow Stuck Count
	const escrowQuery = `
		SELECT COUNT(*)
		FROM orders
		WHERE status = 'shipped'
		  AND auto_release_at < NOW()
		  AND escrow_status = 'holding';
	`
	err = s.db.QueryRow(ctx, escrowQuery).Scan(&status.EscrowStuckCount)
	if err != nil {
		return status, fmt.Errorf("escrow check failed: %w", err)
	}

	// 3. Withdrawal Stuck Count
	const withdrawalQuery = `
		SELECT COUNT(*)
		FROM withdrawals
		WHERE status = 'PROCESSING'
		  AND updated_at < NOW() - INTERVAL '24 hours';
	`
	err = s.db.QueryRow(ctx, withdrawalQuery).Scan(&status.WithdrawalStuckCount)
	if err != nil {
		return status, fmt.Errorf("withdrawal check failed: %w", err)
	}

	// 4. Auction Stuck Count
	const auctionQuery = `
		SELECT COUNT(*)
		FROM auctions a
		WHERE a.status = 'ended'
		  AND a.order_id IS NULL
		  AND EXISTS (
		    SELECT 1 FROM auction_bids WHERE auction_id = a.id
		  );
	`
	err = s.db.QueryRow(ctx, auctionQuery).Scan(&status.AuctionStuckCount)
	if err != nil {
		return status, fmt.Errorf("auction check failed: %w", err)
	}

	// 5. Outbox Pending Events Count
	const outboxQuery = `
		SELECT COUNT(*)
		FROM outbox
		WHERE status = 'pending';
	`
	err = s.db.QueryRow(ctx, outboxQuery).Scan(&status.OutboxPendingCount)
	if err != nil {
		return status, fmt.Errorf("outbox pending check failed: %w", err)
	}

	// 6. Outbox Stuck Events Count (events in 'processing' for > 5 minutes)
	const outboxStuckQuery = `
		SELECT COUNT(*)
		FROM outbox
		WHERE status = 'processing'
		  AND updated_at < NOW() - INTERVAL '5 minutes';
	`
	err = s.db.QueryRow(ctx, outboxStuckQuery).Scan(&status.OutboxStuckCount)
	if err != nil {
		return status, fmt.Errorf("outbox stuck check failed: %w", err)
	}

	// 6b. Outbox Dead-Letter Count (poison events that exhausted retry budget)
	const outboxDLQQuery = `
		SELECT COUNT(*)
		FROM outbox
		WHERE status = 'dead_letter';
	`
	err = s.db.QueryRow(ctx, outboxDLQQuery).Scan(&status.OutboxDeadLetterCount)
	if err != nil {
		return status, fmt.Errorf("outbox dlq check failed: %w", err)
	}

	// 6c. Outbox Lag Seconds (age of oldest pending/failed event ready for processing).
	// COALESCE to 0 when no backlog exists (nothing ready → lag is 0).
	const outboxLagQuery = `
		SELECT COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(created_at))), 0)
		FROM outbox
		WHERE status IN ('pending', 'failed')
		  AND next_attempt_at <= NOW();
	`
	err = s.db.QueryRow(ctx, outboxLagQuery).Scan(&status.OutboxLagSeconds)
	if err != nil {
		return status, fmt.Errorf("outbox lag check failed: %w", err)
	}

	// Log warning if outbox backlog is high (> 1000)
	if status.OutboxPendingCount > 1000 {
		s.logger.Warn("Outbox backlog high",
			zap.Int("pending_events", status.OutboxPendingCount),
		)
	}

	// Log warning if DLQ is growing
	if status.OutboxDeadLetterCount > 0 {
		s.logger.Warn("Outbox dead-letter events present",
			zap.Int("dead_letter_events", status.OutboxDeadLetterCount),
		)
	}

	// Log warning if outbox has stuck events
	if status.OutboxStuckCount > 0 {
		s.logger.Warn("Outbox has stuck events",
			zap.Int("stuck_events", status.OutboxStuckCount),
		)
	}

	// 7. Subscription Orphaned Payment Count
	// Payments with reference_type='subscription' and status='settlement' but no matching subscription
	const orphanedPaymentQuery = `
		SELECT COUNT(*)
		FROM payments p
		WHERE p.reference_type = 'subscription'
		  AND p.status = 'settlement'
		  AND NOT EXISTS (
		    SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = p.id
		  );
	`
	err = s.db.QueryRow(ctx, orphanedPaymentQuery).Scan(&status.OrphanedPaymentCount)
	if err != nil {
		return status, fmt.Errorf("orphaned payment check failed: %w", err)
	}

	// 8. Payment Subscription Conversion Rate
	// Ratio of successful subscription payments that have corresponding subscriptions
	const conversionRateQuery = `
		WITH payment_stats AS (
		  SELECT
		    COUNT(*) FILTER (WHERE status = 'settlement') as total_settlement,
		    COUNT(*) FILTER (
		      WHERE status = 'settlement'
		      AND EXISTS (SELECT 1 FROM seller_subscriptions s WHERE s.payment_id = payments.id)
		    ) as converted
		  FROM payments
		  WHERE reference_type = 'subscription'
		)
		SELECT
		  CASE
		    WHEN total_settlement = 0 THEN 1.0
		    ELSE CAST(converted AS FLOAT) / CAST(total_settlement AS FLOAT)
		  END as conversion_rate
		FROM payment_stats;
	`
	err = s.db.QueryRow(ctx, conversionRateQuery).Scan(&status.PaymentSubscriptionConversionRate)
	if err != nil {
		return status, fmt.Errorf("conversion rate check failed: %w", err)
	}

	// 9. Active Subscription Count
	const activeSubQuery = `
		SELECT COUNT(*)
		FROM seller_subscriptions
		WHERE status = 'active';
	`
	err = s.db.QueryRow(ctx, activeSubQuery).Scan(&status.ActiveSubscriptionCount)
	if err != nil {
		return status, fmt.Errorf("active subscription check failed: %w", err)
	}

	// 10. Expiring Subscription Count (within 7 days)
	const expiringSubQuery = `
		SELECT COUNT(*)
		FROM seller_subscriptions
		WHERE status = 'active'
		  AND expires_at <= NOW() + INTERVAL '7 days';
	`
	err = s.db.QueryRow(ctx, expiringSubQuery).Scan(&status.ExpiringSubscriptionCount)
	if err != nil {
		return status, fmt.Errorf("expiring subscription check failed: %w", err)
	}

	// 11. Expired Subscription Count
	const expiredSubQuery = `
		SELECT COUNT(*)
		FROM seller_subscriptions
		WHERE status = 'expired';
	`
	err = s.db.QueryRow(ctx, expiredSubQuery).Scan(&status.ExpiredSubscriptionCount)
	if err != nil {
		return status, fmt.Errorf("expired subscription check failed: %w", err)
	}

	// Log critical warnings for subscription health
	if status.OrphanedPaymentCount > 0 {
		s.logger.Error("CRITICAL: Orphaned subscription payments detected",
			zap.Int("orphaned_payments", status.OrphanedPaymentCount),
			zap.Float64("conversion_rate", status.PaymentSubscriptionConversionRate),
		)
	}

	if status.PaymentSubscriptionConversionRate < 0.95 {
		s.logger.Warn("Subscription payment conversion rate below 95%",
			zap.Float64("conversion_rate", status.PaymentSubscriptionConversionRate),
			zap.Int("orphaned_payments", status.OrphanedPaymentCount),
		)
	}

	return status, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


