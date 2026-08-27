package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// WithdrawalStatus represents the status of a withdrawal request.
// Values MUST match database withdrawal_status_enum exactly.
type WithdrawalStatus string

const (
	// WithdrawalStatusRequested is the initial status when a withdrawal is requested
	WithdrawalStatusRequested WithdrawalStatus = "REQUESTED"
	// WithdrawalStatusProcessing indicates the withdrawal is being processed (admin approved, ready for gateway submission)
	WithdrawalStatusProcessing WithdrawalStatus = "PROCESSING"
	// WithdrawalStatusSubmitted indicates the withdrawal has been submitted to payment gateway
	WithdrawalStatusSubmitted WithdrawalStatus = "SUBMITTED"
	// WithdrawalStatusSettling indicates the withdrawal is pending final confirmation from gateway
	WithdrawalStatusSettling WithdrawalStatus = "SETTLING"
	// WithdrawalStatusSettled indicates the withdrawal was completed successfully (confirmed by gateway)
	WithdrawalStatusSettled WithdrawalStatus = "SETTLED"
	// WithdrawalStatusCompleted indicates the withdrawal was completed manually by an admin
	// (bank transfer performed outside the payment gateway). Semantically distinct from
	// WithdrawalStatusSettled, which is set exclusively by the gateway webhook callback.
	WithdrawalStatusCompleted WithdrawalStatus = "COMPLETED"
	// WithdrawalStatusFailed indicates the withdrawal failed (was rejected before processing)
	WithdrawalStatusFailed WithdrawalStatus = "FAILED"
	// WithdrawalStatusFailedRetryable indicates a temporary failure that can be retried
	WithdrawalStatusFailedRetryable WithdrawalStatus = "FAILED_RETRYABLE"
	// WithdrawalStatusFailedFinal indicates a permanent failure that cannot be retried
	WithdrawalStatusFailedFinal WithdrawalStatus = "FAILED_FINAL"
	// WithdrawalStatusPilotBlocked indicates withdrawal is blocked due to pilot mode restrictions
	// Seller is not in the pilot whitelist, so payout cannot proceed until:
	// 1. Pilot mode is disabled, OR
	// 2. Seller is added to the whitelist
	// This status provides operational honesty - withdrawals are not "stuck" but explicitly blocked.
	WithdrawalStatusPilotBlocked WithdrawalStatus = "PILOT_BLOCKED"
)

// IsFinal returns true if the status is a terminal state (no further transitions possible)
func (s WithdrawalStatus) IsFinal() bool {
	return s == WithdrawalStatusSettled ||
		s == WithdrawalStatusCompleted ||
		s == WithdrawalStatusFailed ||
		s == WithdrawalStatusFailedFinal
}

// IsRetryable returns true if the status indicates a retryable failure
func (s WithdrawalStatus) IsRetryable() bool {
	return s == WithdrawalStatusFailedRetryable
}

// CanTransitionTo returns true if transition from current status to new status is valid
func (s WithdrawalStatus) CanTransitionTo(newStatus WithdrawalStatus) bool {
	// Define valid state transitions
	validTransitions := map[WithdrawalStatus][]WithdrawalStatus{
		WithdrawalStatusRequested: {
			WithdrawalStatusProcessing, // Admin approved
			WithdrawalStatusFailed,     // Admin rejected
		},
		WithdrawalStatusProcessing: {
			WithdrawalStatusSubmitted,    // Submitted to gateway
			WithdrawalStatusCompleted,    // Manual admin completion (no gateway involvement)
			WithdrawalStatusFailed,       // Cancelled before submission
			WithdrawalStatusPilotBlocked, // Blocked by pilot mode
		},
		WithdrawalStatusPilotBlocked: {
			WithdrawalStatusProcessing, // Unblocked - pilot mode disabled or seller whitelisted
			WithdrawalStatusFailed,     // Cancelled while blocked
		},
		WithdrawalStatusSubmitted: {
			WithdrawalStatusSettling,        // Pending confirmation
			WithdrawalStatusSettled,         // Confirmed immediately
			WithdrawalStatusFailedRetryable, // Temporary failure
			WithdrawalStatusFailedFinal,     // Permanent failure
		},
		WithdrawalStatusSettling: {
			WithdrawalStatusSettled,         // Confirmed
			WithdrawalStatusFailedRetryable, // Timeout/temporary failure
			WithdrawalStatusFailedFinal,     // Rejected by gateway
		},
		WithdrawalStatusFailedRetryable: {
			WithdrawalStatusSubmitted, // Retry
			WithdrawalStatusFailed,    // Cancelled (funds returned)
		},
		// Terminal states - no transitions allowed
		WithdrawalStatusSettled:     {},
		WithdrawalStatusCompleted:   {},
		WithdrawalStatusFailed:      {},
		WithdrawalStatusFailedFinal: {},
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, valid := range allowed {
		if valid == newStatus {
			return true
		}
	}
	return false
}

// Withdrawal represents a withdrawal request.
type Withdrawal struct {
	ID             uuid.UUID
	SellerID       uuid.UUID
	SellerUsername string
	SellerFarmName string
	Amount         int64
	FeeAmount      int64
	Status         WithdrawalStatus
	IdempotencyKey string

	// Bank account snapshots (immutable, captured at request time)
	BankNameSnapshot      string // Bank name snapshot at time of request
	BankCodeSnapshot      string // Bank code snapshot at time of request (for payout rail integration)
	AccountNumberSnapshot string // Account number snapshot at time of request
	AccountHolderSnapshot string // Account holder name snapshot at time of request

	// Execution metadata (for payout worker / gateway integration)
	ExternalReferenceID string // External reference from payment gateway (for idempotency)
	GatewayResponse     string // Raw gateway response (JSON, for audit/debug)
	FailureReason       string // Human-readable failure reason
	SubmittedAt         int64  // Unix timestamp when submitted to gateway
	SettledAt           int64  // Unix timestamp when confirmed completed by gateway
	RetryCount          int    // Number of retry attempts (for payout worker)

	CreatedAt int64
	UpdatedAt int64
}

// WithdrawRepository handles withdrawal persistence operations.
type WithdrawRepository struct {
	// No DB field needed - repository uses db.Tx passed as parameter
}

// NewWithdrawRepository creates a new WithdrawRepository.
func NewWithdrawRepository() *WithdrawRepository {
	return &WithdrawRepository{}
}

// Create creates a new withdrawal record.
func (r *WithdrawRepository) Create(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	status WithdrawalStatus,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		INSERT INTO withdrawals (id, seller_id, amount, fee_amount, status, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, sellerID, amount, feeAmount, status, "", now, now)

	if err != nil {
		return fmt.Errorf("withdraw: create failed: %w", err)
	}

	return nil
}

// CreateWithIdempotency creates a new withdrawal record with an idempotency key.
// If a withdrawal with the same idempotency key exists, returns a unique violation error.
// Bank account snapshots are stored at withdrawal creation time and never updated.
func (r *WithdrawRepository) CreateWithIdempotency(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	status WithdrawalStatus,
	idempotencyKey string,
	bankNameSnapshot string,
	bankCodeSnapshot string,
	accountNumberSnapshot string,
	accountHolderSnapshot string,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		INSERT INTO withdrawals (id, seller_id, amount, fee_amount, status, idempotency_key,
		                        bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
		                        created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, id, sellerID, amount, feeAmount, status, idempotencyKey,
		bankNameSnapshot, bankCodeSnapshot, accountNumberSnapshot, accountHolderSnapshot,
		now, now)

	if err != nil {
		return fmt.Errorf("withdraw: create with idempotency failed: %w", err)
	}

	return nil
}

// GetByID retrieves a withdrawal by ID.
func (r *WithdrawRepository) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*Withdrawal, error) {
	var w Withdrawal
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		WHERE w.id = $1
	`, id).Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
		&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
		&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
		&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
		&createdAt, &updatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("withdraw: not found: id=%s", id)
		}
		return nil, fmt.Errorf("withdraw: get failed: %w", err)
	}
	w.CreatedAt = createdAt.Unix()
	w.UpdatedAt = updatedAt.Unix()

	return &w, nil
}

// LockForUpdate locks a withdrawal row FOR UPDATE.
// This must be used within a transaction before any status transition.
func (r *WithdrawRepository) LockForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*Withdrawal, error) {
	var w Withdrawal
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, amount, fee_amount, status, idempotency_key,
		       bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
		       external_reference_id, gateway_response, failure_reason,
		       submitted_at, settled_at, retry_count,
		       created_at, updated_at
		FROM withdrawals WHERE id = $1
		FOR UPDATE
	`, id).Scan(&w.ID, &w.SellerID, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
		&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
		&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
		&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
		&createdAt, &updatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("withdraw: not found: id=%s", id)
		}
		return nil, fmt.Errorf("withdraw: lock failed: %w", err)
	}
	w.CreatedAt = createdAt.Unix()
	w.UpdatedAt = updatedAt.Unix()

	return &w, nil
}

// UpdateStatus updates the status of a withdrawal.
// Returns the number of rows affected.
func (r *WithdrawRepository) UpdateStatus(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	newStatus WithdrawalStatus,
) (int64, error) {
	now := time.Now()
	result, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = $1, updated_at = $2
		WHERE id = $3
	`, newStatus, now, id)

	if err != nil {
		return 0, fmt.Errorf("withdraw: update status failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// UpdateStatusWithCheck updates status only if current status matches expectedStatus.
// Returns the number of rows affected.
func (r *WithdrawRepository) UpdateStatusWithCheck(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	expectedStatus WithdrawalStatus,
	newStatus WithdrawalStatus,
) (int64, error) {
	now := time.Now()
	result, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4
	`, newStatus, now, id, expectedStatus)

	if err != nil {
		return 0, fmt.Errorf("withdraw: update status with check failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// GetBySellerID retrieves all withdrawals for a seller, ordered by creation time (newest first).
func (r *WithdrawRepository) GetBySellerID(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) ([]*Withdrawal, error) {
	rows, err := tx.Query(ctx, `
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		WHERE w.seller_id = $1
		ORDER BY w.created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query by seller failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate by seller failed: %w", err)
	}

	return withdrawals, nil
}

// GetActiveBySellerID returns the seller's latest in-flight withdrawal, if any.
//
// "In-flight" means a row that has been created but has not reached a terminal
// status — i.e. it is still consuming withdrawable balance or pending settlement.
// Terminal statuses (SETTLED, COMPLETED, FAILED, FAILED_FINAL) are excluded so
// a new request after a settled or final-failed withdrawal is allowed.
//
// FAILED_RETRYABLE and PILOT_BLOCKED are treated as in-flight: the seller's
// reservation is still on the ledger and the row is still recoverable.
//
// Returns nil, nil when no in-flight row exists. Caller uses this for the
// "single in-flight withdrawal per seller" idempotency guard at request time.
func (r *WithdrawRepository) GetActiveBySellerID(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*Withdrawal, error) {
	var w Withdrawal
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		WHERE w.seller_id = $1
		  AND w.status IN ('REQUESTED','PROCESSING','SUBMITTED','SETTLING','FAILED_RETRYABLE','PILOT_BLOCKED')
		ORDER BY w.created_at DESC
		LIMIT 1
	`, sellerID).Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
		&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
		&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
		&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
		&createdAt, &updatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("withdraw: get active by seller failed: %w", err)
	}
	w.CreatedAt = createdAt.Unix()
	w.UpdatedAt = updatedAt.Unix()
	return &w, nil
}

// GetWithdrawnTotal returns the total amount of completed withdrawals for a seller.
// Counts both SETTLED (new) and COMPLETED (legacy) statuses.
func (r *WithdrawRepository) GetWithdrawnTotal(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (int64, error) {
	var total int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM withdrawals
		WHERE seller_id = $1 AND status IN ('SETTLED', 'COMPLETED')
	`, sellerID).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("withdraw: get total withdrawn failed: %w", err)
	}

	return total, nil
}

// GetDailyWithdrawnTotal returns the total amount of withdrawals (all statuses) for a seller today.
// This is used for daily limit enforcement.
func (r *WithdrawRepository) GetDailyWithdrawnTotal(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (int64, error) {
	var total int64
	todayStart := time.Now().Truncate(24 * time.Hour)

	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM withdrawals
		WHERE seller_id = $1 AND created_at >= $2
	`, sellerID, todayStart).Scan(&total)

	if err != nil {
		return 0, fmt.Errorf("withdraw: get daily total failed: %w", err)
	}

	return total, nil
}

// ============================================================================
// PAYOUT WORKER EXECUTION METHODS
// These methods are designed for use by the payout worker / gateway integration.
// ============================================================================

// GetEligibleForSubmission retrieves withdrawals in PROCESSING status that are
// ready to be submitted to the payment gateway.
// Uses FOR UPDATE locking to prevent concurrent workers from processing the same withdrawal.
// Generates and persists external_reference_id atomically.
// Limit specifies the maximum number of withdrawals to return.
func (r *WithdrawRepository) GetEligibleForSubmission(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]*Withdrawal, error) {
	rows, err := tx.Query(ctx, `
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		WHERE w.status = 'PROCESSING'
		ORDER BY w.created_at ASC
		LIMIT $1
		FOR UPDATE OF w
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query eligible for submission failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate eligible for submission failed: %w", err)
	}

	return withdrawals, nil
}

// GetPendingSettlement retrieves withdrawals in SUBMITTED or SETTLING status
// that are waiting for confirmation from the payment gateway.
func (r *WithdrawRepository) GetPendingSettlement(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]*Withdrawal, error) {
	rows, err := tx.Query(ctx, `
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		WHERE w.status IN ('SUBMITTED', 'SETTLING')
		ORDER BY w.submitted_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query pending settlement failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate pending settlement failed: %w", err)
	}

	return withdrawals, nil
}

// UpdateForSubmission updates a withdrawal when it's submitted to the gateway.
// Sets status to SUBMITTED, records external reference ID, gateway response, and submission time.
func (r *WithdrawRepository) UpdateForSubmission(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	externalReferenceID string,
	gatewayResponse string,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'SUBMITTED',
		    external_reference_id = $1,
		    gateway_response = $2,
		    submitted_at = $3,
		    updated_at = $4
		WHERE id = $5 AND status = 'PROCESSING'
	`, externalReferenceID, gatewayResponse, now.Unix(), now, id)

	if err != nil {
		return fmt.Errorf("withdraw: update for submission failed: %w", err)
	}

	return nil
}

// EnsureExternalReference generates and persists an external reference ID if not already set.
// This must be called within a transaction that has the row locked.
// Returns the external reference ID (existing or newly generated).
func (r *WithdrawRepository) EnsureExternalReference(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (string, error) {
	// First, try to get existing external reference
	var existingRef string
	err := tx.QueryRow(ctx, `
		SELECT external_reference_id FROM withdrawals WHERE id = $1
	`, id).Scan(&existingRef)

	if err == nil && existingRef != "" {
		return existingRef, nil
	}

	// Generate new external reference
	nowTime := time.Now()
	newRef := fmt.Sprintf("WD_%s_%d", id.String(), nowTime.Unix())

	// Persist it
	_, err = tx.Exec(ctx, `
		UPDATE withdrawals
		SET external_reference_id = $1,
		    updated_at = $2
		WHERE id = $3 AND (external_reference_id = '' OR external_reference_id IS NULL)
	`, newRef, nowTime, id)

	if err != nil {
		return "", fmt.Errorf("ensure external reference: %w", err)
	}

	return newRef, nil
}

// ErrMarkFailedInvalidState is returned by MarkFailed when the withdrawal is
// not in a state that permits a failure transition (e.g. already terminal).
var ErrMarkFailedInvalidState = fmt.Errorf("withdraw: mark failed rejected — invalid state or already terminal")

// MarkFailed marks a withdrawal as failed with a reason.
// Supports both FAILED_RETRYABLE and FAILED_FINAL statuses.
//
// SQL STATUS GUARD: only transitions from non-terminal, gateway-reachable states:
//   PROCESSING       — sync gateway rejection before SUBMITTED
//   SUBMITTED        — gateway rejected after submission
//   SETTLING         — gateway rejected during settlement confirmation
//   FAILED_RETRYABLE — gateway rejected again on retry
//
// Returns ErrMarkFailedInvalidState if the row is already terminal or in any
// other state not listed above. Callers that hold a LockForUpdate and check
// IsFinal() first will never reach this error under normal flow; this is a
// defence-in-depth guard at the SQL layer.
func (r *WithdrawRepository) MarkFailed(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	newStatus WithdrawalStatus,
	failureReason string,
	gatewayResponse string,
) error {
	now := time.Now()
	result, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = $1,
		    failure_reason = $2,
		    gateway_response = $3,
		    updated_at = $4
		WHERE id = $5
		  AND status IN ('PROCESSING', 'SUBMITTED', 'SETTLING', 'FAILED_RETRYABLE')
	`, newStatus, failureReason, gatewayResponse, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: mark failed failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: withdrawal %s", ErrMarkFailedInvalidState, id)
	}

	return nil
}

// MarkSettling marks a withdrawal as settling (pending final confirmation).
// Used when gateway reports PENDING status.
func (r *WithdrawRepository) MarkSettling(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	gatewayResponse string,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'SETTLING',
		    gateway_response = $1,
		    updated_at = $2
		WHERE id = $3 AND status = 'SUBMITTED'
	`, gatewayResponse, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: mark settling failed: %w", err)
	}

	return nil
}

// MarkSettled marks a withdrawal as settled (completed).
// Records the gateway response and settlement time.
func (r *WithdrawRepository) MarkSettled(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	gatewayResponse string,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'SETTLED',
		    gateway_response = $1,
		    settled_at = $2,
		    updated_at = $3
		WHERE id = $4 AND status IN ('SUBMITTED', 'SETTLING')
	`, gatewayResponse, now.Unix(), now, id)

	if err != nil {
		return fmt.Errorf("withdraw: mark settled failed: %w", err)
	}

	return nil
}

// MarkPilotBlocked marks a withdrawal as blocked due to pilot mode restrictions.
// Used when a withdrawal is approved but the seller is not in the pilot whitelist.
func (r *WithdrawRepository) MarkPilotBlocked(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'PILOT_BLOCKED',
		    failure_reason = 'Seller not in pilot whitelist',
		    updated_at = $1
		WHERE id = $2 AND status = 'PROCESSING'
	`, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: mark pilot blocked failed: %w", err)
	}

	return nil
}

// UnblockPilotBlocked marks a pilot-blocked withdrawal as ready for processing.
// Used when pilot mode is disabled or seller is added to the whitelist.
func (r *WithdrawRepository) UnblockPilotBlocked(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET status = 'PROCESSING',
		    failure_reason = '',
		    updated_at = $1
		WHERE id = $2 AND status = 'PILOT_BLOCKED'
	`, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: unblock pilot blocked failed: %w", err)
	}

	return nil
}

// GetByExternalReference retrieves a withdrawal by its external reference ID.
// Used for webhook handling when gateway callbacks don't have our internal ID.
func (r *WithdrawRepository) GetByExternalReference(
	ctx context.Context,
	tx db.Tx,
	externalReferenceID string,
) (*Withdrawal, error) {
	var w Withdrawal
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, seller_id, amount, fee_amount, status, idempotency_key,
		       bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
		       external_reference_id, gateway_response, failure_reason,
		       submitted_at, settled_at, retry_count,
		       created_at, updated_at
		FROM withdrawals WHERE external_reference_id = $1
	`, externalReferenceID).Scan(&w.ID, &w.SellerID, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
		&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
		&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
		&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
		&createdAt, &updatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("withdraw: not found: external_ref=%s", externalReferenceID)
		}
		return nil, fmt.Errorf("withdraw: get by external reference failed: %w", err)
	}
	w.CreatedAt = createdAt.Unix()
	w.UpdatedAt = updatedAt.Unix()

	return &w, nil
}

// ============================================================================
// PAYOUT WORKER RETRY METHODS
// ============================================================================

// MaxRetries is the maximum number of retry attempts allowed.
const MaxRetries = 5

// GetRetryableWithdrawals retrieves FAILED_RETRYABLE withdrawals that are
// ready for retry based on exponential backoff.
// Uses the formula: backoff_seconds = 2^retry_count (max 32 seconds for first 5 retries)
func (r *WithdrawRepository) GetRetryableWithdrawals(
	ctx context.Context,
	tx db.Tx,
	limit int,
) ([]*Withdrawal, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, amount, fee_amount, status, idempotency_key,
		       bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
		       external_reference_id, gateway_response, failure_reason,
		       submitted_at, settled_at, retry_count,
		       created_at, updated_at
		FROM withdrawals
		WHERE status = 'FAILED_RETRYABLE'
		  AND retry_count < $1
		  AND EXTRACT(EPOCH FROM (NOW() - updated_at)) > CASE
			  WHEN retry_count = 0 THEN 0
			  WHEN retry_count = 1 THEN 2
			  WHEN retry_count = 2 THEN 4
			  WHEN retry_count = 3 THEN 8
			  WHEN retry_count = 4 THEN 16
			  ELSE 32
		  END
		ORDER BY retry_count ASC, created_at ASC
		LIMIT $2
	`, MaxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query retryable failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan retryable failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate retryable failed: %w", err)
	}

	return withdrawals, nil
}

// IncrementRetryCount increments the retry count for a withdrawal.
// Used when marking a withdrawal as FAILED_RETRYABLE.
func (r *WithdrawRepository) IncrementRetryCount(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET retry_count = retry_count + 1,
		    updated_at = $1
		WHERE id = $2
	`, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: increment retry count failed: %w", err)
	}

	return nil
}

// ResetRetryCount resets the retry count to 0.
// Used when retrying a FAILED_RETRYABLE withdrawal (submitting again).
func (r *WithdrawRepository) ResetRetryCount(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) error {
	now := time.Now()
	_, err := tx.Exec(ctx, `
		UPDATE withdrawals
		SET retry_count = 0,
		    updated_at = $1
		WHERE id = $2
	`, now, id)

	if err != nil {
		return fmt.Errorf("withdraw: reset retry count failed: %w", err)
	}

	return nil
}

// ============================================================================
// RECONCILIATION METHODS
// ============================================================================

// GetStuckPayouts retrieves withdrawals that have been in SUBMITTED or SETTLING
// status for longer than the specified threshold.
// Used by the reconciliation service to detect potentially stuck payouts.
func (r *WithdrawRepository) GetStuckPayouts(
	ctx context.Context,
	tx db.Tx,
	cutoff time.Time,
	limit int,
) ([]*Withdrawal, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, amount, fee_amount, status, idempotency_key,
		       bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
		       external_reference_id, gateway_response, failure_reason,
		       submitted_at, settled_at, retry_count,
		       created_at, updated_at
		FROM withdrawals
		WHERE status IN ('SUBMITTED', 'SETTLING')
		  AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query stuck payouts failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan stuck payouts failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate stuck payouts failed: %w", err)
	}

	return withdrawals, nil
}

// StatusCount represents the count of withdrawals in a specific status.
type StatusCount struct {
	Status WithdrawalStatus `json:"status"`
	Count  int              `json:"count"`
}

// GetStatusCounts returns the count of withdrawals grouped by status.
// Used by the reconciliation service for metrics.
func (r *WithdrawRepository) GetStatusCounts(
	ctx context.Context,
	tx db.Tx,
) ([]StatusCount, error) {
	rows, err := tx.Query(ctx, `
		SELECT status, COUNT(*) as count
		FROM withdrawals
		GROUP BY status
		ORDER BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("withdraw: query status counts failed: %w", err)
	}
	defer rows.Close()

	var counts []StatusCount
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, fmt.Errorf("withdraw: scan status counts failed: %w", err)
		}
		counts = append(counts, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate status counts failed: %w", err)
	}

	return counts, nil
}

// ============================================================================
// ADMIN LISTING METHODS
// ============================================================================

// WithdrawalListFilters holds filter parameters for listing withdrawals.
type WithdrawalListFilters struct {
	Status   *string
	SellerID *uuid.UUID
	SortBy   string
	SortDesc bool
	Page     int
	PageSize int
}

// ListWithFilters retrieves withdrawals with filtering and pagination.
// Returns withdrawals ordered by the specified sort field.
func (r *WithdrawRepository) ListWithFilters(
	ctx context.Context,
	tx db.Tx,
	filters WithdrawalListFilters,
) ([]*Withdrawal, error) {
	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	if filters.Status != nil && *filters.Status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, *filters.Status)
		argPos++
	}

	if filters.SellerID != nil {
		whereClause += fmt.Sprintf(" AND seller_id = $%d", argPos)
		args = append(args, *filters.SellerID)
		argPos++
	}

	// Build ORDER BY clause. Columns are qualified with the "w." alias
	// because this query LEFT JOINs user_profiles and seller_profiles,
	// which both also define created_at/updated_at — an unqualified
	// ORDER BY created_at/updated_at is rejected by Postgres as an
	// ambiguous column reference on every call (see
	// withdraw_repository_integration_test.go for the sibling regression
	// guard on the other JOIN queries in this file).
	orderClause := "ORDER BY w.created_at DESC"
	if filters.SortBy != "" {
		// Whitelist allowed sort columns to prevent SQL injection
		allowedColumns := map[string]bool{
			"created_at":   true,
			"updated_at":   true,
			"amount":       true,
			"status":       true,
			"submitted_at": true,
			"settled_at":   true,
		}
		if allowedColumns[filters.SortBy] {
			direction := "DESC"
			if filters.SortDesc {
				direction = "DESC"
			} else {
				direction = "ASC"
			}
			orderClause = fmt.Sprintf("ORDER BY w.%s %s", filters.SortBy, direction)
		}
	}

	// Build LIMIT/OFFSET clause
	limitClause := fmt.Sprintf("LIMIT %d", filters.PageSize)
	offsetClause := fmt.Sprintf("OFFSET %d", (filters.Page-1)*filters.PageSize)

	query := fmt.Sprintf(`
		SELECT w.id, w.seller_id, COALESCE(up.username, '') AS seller_username, COALESCE(sp.store_name, '') AS seller_farm_name,
		       w.amount, w.fee_amount, w.status, w.idempotency_key,
		       w.bank_name_snapshot, w.bank_code_snapshot, w.account_number_snapshot, w.account_holder_snapshot,
		       w.external_reference_id, w.gateway_response, w.failure_reason,
		       w.submitted_at, w.settled_at, w.retry_count,
		       w.created_at, w.updated_at
		FROM withdrawals w
		LEFT JOIN user_profiles up ON up.user_id = w.seller_id
		LEFT JOIN seller_profiles sp ON sp.user_id = w.seller_id
		%s
		%s
		%s
		%s
	`, whereClause, orderClause, limitClause, offsetClause)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("withdraw: list with filters failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		var w Withdrawal
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&w.ID, &w.SellerID, &w.SellerUsername, &w.SellerFarmName, &w.Amount, &w.FeeAmount, &w.Status, &w.IdempotencyKey,
			&w.BankNameSnapshot, &w.BankCodeSnapshot, &w.AccountNumberSnapshot, &w.AccountHolderSnapshot,
			&w.ExternalReferenceID, &w.GatewayResponse, &w.FailureReason,
			&w.SubmittedAt, &w.SettledAt, &w.RetryCount,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("withdraw: scan failed: %w", err)
		}
		w.CreatedAt = createdAt.Unix()
		w.UpdatedAt = updatedAt.Unix()
		withdrawals = append(withdrawals, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("withdraw: iterate list with filters failed: %w", err)
	}

	return withdrawals, nil
}

// CountWithFilters returns the total count of withdrawals matching the filters.
func (r *WithdrawRepository) CountWithFilters(
	ctx context.Context,
	tx db.Tx,
	filters WithdrawalListFilters,
) (int64, error) {
	// Build WHERE clause (same logic as ListWithFilters)
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	if filters.Status != nil && *filters.Status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, *filters.Status)
		argPos++
	}

	if filters.SellerID != nil {
		whereClause += fmt.Sprintf(" AND seller_id = $%d", argPos)
		args = append(args, *filters.SellerID)
		argPos++
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM withdrawals
		%s
	`, whereClause)

	var count int64
	err := tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("withdraw: count with filters failed: %w", err)
	}

	return count, nil
}


