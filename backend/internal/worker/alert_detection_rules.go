package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// Constants for detection thresholds
const (
	// PaymentFailureSpikeThreshold is the number of payment failures in a window to trigger an alert
	PaymentFailureSpikeThreshold = 10
	// PaymentFailureSpikeWindowMinutes is the time window to check for payment failures
	PaymentFailureSpikeWindowMinutes = 15

	// PaymentStuckThresholdMinutes is how long a payment can be pending before alerting
	PaymentStuckThresholdMinutes = 30

	// DisputeSpikeThreshold is the number of disputes in a window to trigger an alert
	DisputeSpikeThreshold = 5
	// DisputeSpikeWindowMinutes is the time window to check for disputes
	DisputeSpikeWindowMinutes = 60

	// SellerRiskDisputeThreshold is the number of disputes to flag a seller as risky
	SellerRiskDisputeThreshold = 3
	// SellerRiskDisputeWindowHours is the time window to check for seller disputes
	SellerRiskDisputeWindowHours = 24

	// CoinsAnomalyThreshold is the number of coin transactions to trigger an alert
	CoinsAnomalyThreshold = 50
	// CoinsAnomalyWindowMinutes is the time window to check for coin anomalies
	CoinsAnomalyWindowMinutes = 10

	// WithdrawalAnomalyThreshold is the amount threshold for suspicious withdrawals, in Rupiah
	WithdrawalAnomalyThreshold = 1000000 // Rp1,000,000
	// WithdrawalAnomalyWindowHours is the time window to check for withdrawal anomalies
	WithdrawalAnomalyWindowHours = 1
)

// PaymentFailureSpikeRule detects sudden increases in payment failures.
type PaymentFailureSpikeRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewPaymentFailureSpikeRule creates a new payment failure spike rule.
func NewPaymentFailureSpikeRule(db db.Transactor, log *zap.Logger) *PaymentFailureSpikeRule {
	return &PaymentFailureSpikeRule{db: db, log: log}
}

func (r *PaymentFailureSpikeRule) Name() string {
	return "payment_failure_spike"
}

func (r *PaymentFailureSpikeRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-time.Duration(PaymentFailureSpikeWindowMinutes) * time.Minute)

	var failureCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM audit_events
		WHERE event_type = $1
		AND created_at >= $2
	`, "payment.failed", windowStart).Scan(&failureCount)

	if err != nil {
		return false, nil, fmt.Errorf("query payment failures: %w", err)
	}

	if failureCount >= PaymentFailureSpikeThreshold {
		groupKey := fmt.Sprintf("payment_failure_spike:%d", PaymentFailureSpikeWindowMinutes)

		metadata := alertentity.AlertMetadata{
			"failure_count":  failureCount,
			"window_minutes": PaymentFailureSpikeWindowMinutes,
			"threshold":      PaymentFailureSpikeThreshold,
			"detected_at":    time.Now(),
		}

		severity := alertentity.SeverityMedium
		if failureCount > PaymentFailureSpikeThreshold*2 {
			severity = alertentity.SeverityHigh
		}
		if failureCount > PaymentFailureSpikeThreshold*5 {
			severity = alertentity.SeverityCritical
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypePaymentFailureSpike,
			Severity:   severity,
			EntityType: "system",
			EntityID:   uuid.Nil,
			Message: fmt.Sprintf("Payment failure spike detected: %d failures in %d minutes (threshold: %d)",
				failureCount, PaymentFailureSpikeWindowMinutes, PaymentFailureSpikeThreshold),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

// PaymentStuckRule detects payments stuck in pending state.
type PaymentStuckRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewPaymentStuckRule creates a new payment stuck rule.
func NewPaymentStuckRule(db db.Transactor, log *zap.Logger) *PaymentStuckRule {
	return &PaymentStuckRule{db: db, log: log}
}

func (r *PaymentStuckRule) Name() string {
	return "payment_stuck"
}

func (r *PaymentStuckRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	stuckThreshold := time.Now().Add(-time.Duration(PaymentStuckThresholdMinutes) * time.Minute)

	// Check for stuck payments
	rows, err := tx.Query(ctx, `
		SELECT p.id, p.created_at
		FROM payments p
		WHERE p.status = 'pending'
		AND p.created_at < $1
		ORDER BY p.created_at ASC
		LIMIT 10
	`, stuckThreshold)

	if err != nil {
		return false, nil, fmt.Errorf("query stuck payments: %w", err)
	}
	defer rows.Close()

	var stuckPayments []struct {
		ID        uuid.UUID
		CreatedAt time.Time
	}

	for rows.Next() {
		var p struct {
			ID        uuid.UUID
			CreatedAt time.Time
		}
		if err := rows.Scan(&p.ID, &p.CreatedAt); err != nil {
			continue
		}
		stuckPayments = append(stuckPayments, p)
	}

	if len(stuckPayments) > 0 {
		groupKey := fmt.Sprintf("payment_stuck:%d", PaymentStuckThresholdMinutes)

		metadata := alertentity.AlertMetadata{
			"stuck_count":            len(stuckPayments),
			"threshold_minutes":      PaymentStuckThresholdMinutes,
			"sample_payment_ids":     getPaymentIDs(stuckPayments),
			"oldest_payment_minutes": int(time.Since(stuckPayments[0].CreatedAt).Minutes()),
			"detected_at":            time.Now(),
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypePaymentStuck,
			Severity:   alertentity.SeverityHigh,
			EntityType: "payment",
			EntityID:   stuckPayments[0].ID,
			Message: fmt.Sprintf("%d payments stuck in pending state for > %d minutes",
				len(stuckPayments), PaymentStuckThresholdMinutes),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

func getPaymentIds(payments []struct {
	ID        uuid.UUID
	CreatedAt time.Time
}) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(payments))
	for _, p := range payments {
		ids = append(ids, p.ID)
	}
	return ids
}

// DisputeSpikeRule detects sudden increases in disputes.
type DisputeSpikeRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewDisputeSpikeRule creates a new dispute spike rule.
func NewDisputeSpikeRule(db db.Transactor, log *zap.Logger) *DisputeSpikeRule {
	return &DisputeSpikeRule{db: db, log: log}
}

func (r *DisputeSpikeRule) Name() string {
	return "dispute_spike"
}

func (r *DisputeSpikeRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-time.Duration(DisputeSpikeWindowMinutes) * time.Minute)

	var disputeCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM disputes
		WHERE created_at >= $1
	`, windowStart).Scan(&disputeCount)

	if err != nil {
		return false, nil, fmt.Errorf("query disputes: %w", err)
	}

	if disputeCount >= DisputeSpikeThreshold {
		groupKey := fmt.Sprintf("dispute_spike:%d", DisputeSpikeWindowMinutes)

		metadata := alertentity.AlertMetadata{
			"dispute_count":  disputeCount,
			"window_minutes": DisputeSpikeWindowMinutes,
			"threshold":      DisputeSpikeThreshold,
			"detected_at":    time.Now(),
		}

		severity := alertentity.SeverityMedium
		if disputeCount > DisputeSpikeThreshold*2 {
			severity = alertentity.SeverityHigh
		}
		if disputeCount > DisputeSpikeThreshold*5 {
			severity = alertentity.SeverityCritical
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeDisputeSpike,
			Severity:   severity,
			EntityType: "system",
			EntityID:   uuid.Nil,
			Message: fmt.Sprintf("Dispute spike detected: %d disputes in %d minutes (threshold: %d)",
				disputeCount, DisputeSpikeWindowMinutes, DisputeSpikeThreshold),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

// SellerRiskRule detects sellers with high risk metrics.
type SellerRiskRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewSellerRiskRule creates a new seller risk rule.
func NewSellerRiskRule(db db.Transactor, log *zap.Logger) *SellerRiskRule {
	return &SellerRiskRule{db: db, log: log}
}

func (r *SellerRiskRule) Name() string {
	return "seller_risk"
}

func (r *SellerRiskRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-time.Duration(SellerRiskDisputeWindowHours) * time.Hour)

	// Find sellers with high dispute count
	rows, err := tx.Query(ctx, `
		SELECT o.seller_id, COUNT(d.id) as dispute_count
		FROM disputes d
		INNER JOIN orders o ON o.id = d.order_id
		WHERE d.created_at >= $1
		GROUP BY o.seller_id
		HAVING COUNT(d.id) >= $2
		ORDER BY dispute_count DESC
		LIMIT 5
	`, windowStart, SellerRiskDisputeThreshold)

	if err != nil {
		return false, nil, fmt.Errorf("query seller disputes: %w", err)
	}
	defer rows.Close()

	type sellerRisk struct {
		SellerID     uuid.UUID
		DisputeCount int
	}

	var riskySellers []sellerRisk
	for rows.Next() {
		var sr sellerRisk
		if err := rows.Scan(&sr.SellerID, &sr.DisputeCount); err != nil {
			continue
		}
		riskySellers = append(riskySellers, sr)
	}

	if len(riskySellers) > 0 {
		// Alert for the highest risk seller
		seller := riskySellers[0]
		groupKey := fmt.Sprintf("seller_risk:%s", seller.SellerID.String())

		metadata := alertentity.AlertMetadata{
			"seller_id":         seller.SellerID.String(),
			"dispute_count":     seller.DisputeCount,
			"window_hours":      SellerRiskDisputeWindowHours,
			"threshold":         SellerRiskDisputeThreshold,
			"all_risky_sellers": riskySellers,
			"detected_at":       time.Now(),
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeSellerRisk,
			Severity:   alertentity.SeverityHigh,
			EntityType: "seller",
			EntityID:   seller.SellerID,
			Message: fmt.Sprintf("Seller risk detected: %d disputes in %d hours (threshold: %d)",
				seller.DisputeCount, SellerRiskDisputeWindowHours, SellerRiskDisputeThreshold),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

// CoinsAnomalyRule detects unusual coin activity.
type CoinsAnomalyRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewCoinsAnomalyRule creates a new coins anomaly rule.
func NewCoinsAnomalyRule(db db.Transactor, log *zap.Logger) *CoinsAnomalyRule {
	return &CoinsAnomalyRule{db: db, log: log}
}

func (r *CoinsAnomalyRule) Name() string {
	return "coins_anomaly"
}

func (r *CoinsAnomalyRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-time.Duration(CoinsAnomalyWindowMinutes) * time.Minute)

	// Find users with unusual coin activity
	rows, err := tx.Query(ctx, `
		SELECT user_id, COUNT(*) as tx_count, COALESCE(SUM(amount), 0) as total_amount
		FROM coins_transactions
		WHERE created_at >= $1
		GROUP BY user_id
		HAVING COUNT(*) >= $2
		ORDER BY tx_count DESC
		LIMIT 5
	`, windowStart, CoinsAnomalyThreshold)

	if err != nil {
		return false, nil, fmt.Errorf("query coin transactions: %w", err)
	}
	defer rows.Close()

	type coinAnomaly struct {
		UserID      uuid.UUID
		TxCount     int
		TotalAmount int64
	}

	var anomalies []coinAnomaly
	for rows.Next() {
		var ca coinAnomaly
		if err := rows.Scan(&ca.UserID, &ca.TxCount, &ca.TotalAmount); err != nil {
			continue
		}
		anomalies = append(anomalies, ca)
	}

	if len(anomalies) > 0 {
		anomaly := anomalies[0]
		groupKey := fmt.Sprintf("coins_anomaly:%s", anomaly.UserID.String())

		metadata := alertentity.AlertMetadata{
			"user_id":           anomaly.UserID.String(),
			"transaction_count": anomaly.TxCount,
			"total_amount":      anomaly.TotalAmount,
			"window_minutes":    CoinsAnomalyWindowMinutes,
			"threshold":         CoinsAnomalyThreshold,
			"all_anomalies":     anomalies,
			"detected_at":       time.Now(),
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeCoinsAnomaly,
			Severity:   alertentity.SeverityHigh,
			EntityType: "user",
			EntityID:   anomaly.UserID,
			Message: fmt.Sprintf("Coins anomaly detected: %d transactions in %d minutes (threshold: %d)",
				anomaly.TxCount, CoinsAnomalyWindowMinutes, CoinsAnomalyThreshold),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

// WithdrawalAnomalyRule detects suspicious withdrawal patterns.
type WithdrawalAnomalyRule struct {
	db  db.Transactor
	log *zap.Logger
}

// NewWithdrawalAnomalyRule creates a new withdrawal anomaly rule.
func NewWithdrawalAnomalyRule(db db.Transactor, log *zap.Logger) *WithdrawalAnomalyRule {
	return &WithdrawalAnomalyRule{db: db, log: log}
}

func (r *WithdrawalAnomalyRule) Name() string {
	return "withdrawal_anomaly"
}

func (r *WithdrawalAnomalyRule) Detect(ctx context.Context, tx db.Tx) (bool, *AnomalyFinding, error) {
	windowStart := time.Now().Add(-time.Duration(WithdrawalAnomalyWindowHours) * time.Hour)

	// Find unusual withdrawal patterns by tracing the current ledger authority:
	// ledger_transactions.reference_type -> ledger_entries.account_id -> financial_accounts.user_id.
	//
	// We aggregate per user because the alert entity and consumers are user-level.
	// Only withdrawal_request transactions on the user-owned SELLER_PAYABLE account
	// are counted, which excludes system accounts and reversal/restore entries.
	rows, err := tx.Query(ctx, `
		WITH withdrawal_rows AS (
			SELECT DISTINCT
				lt.id AS transaction_id,
				fa.user_id,
				le.amount
			FROM ledger_transactions lt
			JOIN ledger_entries le ON le.transaction_id = lt.id
			JOIN financial_accounts fa ON fa.id = le.account_id
			WHERE lt.reference_type = $1
			  AND lt.created_at >= $2
			  AND fa.account_type = $3
			  AND fa.user_id IS NOT NULL
			  AND le.entry_type = 'credit'
		)
		SELECT user_id, COUNT(*) AS withdrawal_count, COALESCE(SUM(amount), 0) AS total_amount
		FROM withdrawal_rows
		GROUP BY user_id
		HAVING COALESCE(SUM(amount), 0) >= $4
		ORDER BY total_amount DESC
		LIMIT 5
	`, "withdrawal_request", windowStart.Unix(), finance.AccountSellerPayable, WithdrawalAnomalyThreshold)

	if err != nil {
		return false, nil, fmt.Errorf("query withdrawals: %w", err)
	}
	defer rows.Close()

	type withdrawalAnomaly struct {
		UserID          uuid.UUID
		WithdrawalCount int
		TotalAmount     int64
	}

	var anomalies []withdrawalAnomaly
	for rows.Next() {
		var wa withdrawalAnomaly
		if err := rows.Scan(&wa.UserID, &wa.WithdrawalCount, &wa.TotalAmount); err != nil {
			continue
		}
		anomalies = append(anomalies, wa)
	}

	if len(anomalies) > 0 {
		anomaly := anomalies[0]
		groupKey := fmt.Sprintf("withdrawal_anomaly:%s", anomaly.UserID.String())

		absAmount := anomaly.TotalAmount
		if absAmount < 0 {
			absAmount = -absAmount
		}

		metadata := alertentity.AlertMetadata{
			"user_id":          anomaly.UserID.String(),
			"account_type":     finance.AccountSellerPayable,
			"reference_type":   "withdrawal_request",
			"withdrawal_count": anomaly.WithdrawalCount,
			"total_amount":     absAmount,
			"window_hours":     WithdrawalAnomalyWindowHours,
			"threshold":        WithdrawalAnomalyThreshold,
			"all_anomalies":    anomalies,
			"detected_at":      time.Now(),
		}

		return true, &AnomalyFinding{
			AlertType:  alertentity.AlertTypeWithdrawalAnomaly,
			Severity:   alertentity.SeverityHigh,
			EntityType: "user",
			EntityID:   anomaly.UserID,
			Message: fmt.Sprintf("Withdrawal anomaly detected: %d withdrawals totaling Rp%d in %d hours",
				anomaly.WithdrawalCount, absAmount, WithdrawalAnomalyWindowHours),
			Metadata: metadata,
			GroupKey: &groupKey,
		}, nil
	}

	return false, nil, nil
}

// getPaymentIDs extracts payment IDs from stuck payments slice.
func getPaymentIDs(payments []struct {
	ID        uuid.UUID
	CreatedAt time.Time
}) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(payments))
	for _, p := range payments {
		ids = append(ids, p.ID)
	}
	return ids
}
