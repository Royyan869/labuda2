package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// SellerSubscriptionRepositoryImpl handles seller subscription persistence using pgx-based DB layer.
// Enforces one active subscription per user via partial unique index.
type SellerSubscriptionRepositoryImpl struct{}

// NewSellerSubscriptionRepository creates a new SellerSubscriptionRepositoryImpl.
func NewSellerSubscriptionRepository() *SellerSubscriptionRepositoryImpl {
	return &SellerSubscriptionRepositoryImpl{}
}

// InsertTx creates a new subscription record within a transaction.
// Returns subscriptionEntity.ErrDuplicateActiveSubscription if the user already has
// an active subscription (enforced by DB partial unique index).
func (r *SellerSubscriptionRepositoryImpl) InsertTx(
	ctx context.Context,
	tx db.Tx,
	s *subscriptionEntity.SellerSubscription,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status,
			started_at, expires_at,
			duration_days,
			amount_paid, currency,
			payment_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		s.ID,
		s.UserID,
		s.Status,
		s.StartedAt,
		s.ExpiresAt,
		s.DurationDays,
		s.AmountPaid.Int64(),
		s.Currency,
		s.PaymentID,
		s.CreatedAt,
		s.UpdatedAt,
	)

	if err != nil {
		// Check for UNIQUE violation on partial index (user_id with active status)
		if isUniqueViolationError(err) {
			return &subscriptionEntity.ErrDuplicateActiveSubscription{UserID: s.UserID}
		}
		return fmt.Errorf("insert seller subscription failed: %w", err)
	}

	return nil
}

// UpdateStatusTx transitions a subscription status with guard protection.
// Uses WHERE clause with both id AND fromStatus for atomic guard.
func (r *SellerSubscriptionRepositoryImpl) UpdateStatusTx(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	fromStatus, toStatus subscriptionEntity.Status,
) error {
	result, err := tx.Exec(ctx, `
		UPDATE seller_subscriptions
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		  AND status = $3
	`,
		toStatus,
		id,
		fromStatus,
	)

	if err != nil {
		return fmt.Errorf("update subscription status failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Fetch current status for error reporting
		sub, err := r.GetByID(ctx, tx, id)
		if err != nil {
			return &subscriptionEntity.ErrTransitionGuardFailed{
				ID:           id,
				ExpectedFrom: fromStatus,
				ActualFrom:   "unknown",
				To:           toStatus,
			}
		}
		if sub == nil {
			return fmt.Errorf("subscription not found: %s", id)
		}
		return &subscriptionEntity.ErrTransitionGuardFailed{
			ID:           id,
			ExpectedFrom: fromStatus,
			ActualFrom:   sub.Status,
			To:           toStatus,
		}
	}

	return nil
}

// GetByIDForUpdate retrieves a subscription by ID with row-level lock.
// Uses FOR UPDATE to prevent concurrent modifications.
func (r *SellerSubscriptionRepositoryImpl) GetByIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var userID, paymentID uuid.UUID
	var status subscriptionEntity.Status
	var startedAt, expiresAt, createdAt, updatedAt time.Time
	var durationDays int
	var amountPaid int64
	var currency string

	err := tx.QueryRow(ctx, `
		SELECT user_id, status,
		       started_at, expires_at,
		       duration_days,
		       amount_paid, currency,
		       payment_id,
		       created_at, updated_at
		FROM seller_subscriptions
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&userID, &status,
		&startedAt, &expiresAt,
		&durationDays,
		&amountPaid, &currency,
		&paymentID,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get subscription by id for update failed: %w", err)
	}

	return &subscriptionEntity.SellerSubscription{
		ID:           id,
		UserID:       userID,
		Status:       status,
		StartedAt:    startedAt,
		ExpiresAt:    expiresAt,
		DurationDays: durationDays,
		AmountPaid:   money.New(amountPaid),
		Currency:     currency,
		PaymentID:    paymentID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// GetByID retrieves a subscription by ID without locking.
func (r *SellerSubscriptionRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var userID, paymentID uuid.UUID
	var status subscriptionEntity.Status
	var startedAt, expiresAt, createdAt, updatedAt time.Time
	var durationDays int
	var amountPaid int64
	var currency string

	err := tx.QueryRow(ctx, `
		SELECT user_id, status,
		       started_at, expires_at,
		       duration_days,
		       amount_paid, currency,
		       payment_id,
		       created_at, updated_at
		FROM seller_subscriptions
		WHERE id = $1
	`, id).Scan(
		&userID, &status,
		&startedAt, &expiresAt,
		&durationDays,
		&amountPaid, &currency,
		&paymentID,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get subscription by id failed: %w", err)
	}

	return &subscriptionEntity.SellerSubscription{
		ID:           id,
		UserID:       userID,
		Status:       status,
		StartedAt:    startedAt,
		ExpiresAt:    expiresAt,
		DurationDays: durationDays,
		AmountPaid:   money.New(amountPaid),
		Currency:     currency,
		PaymentID:    paymentID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// GetLatestByUserID retrieves the current active subscription interval for a user.
// Returns nil if no currently active interval exists.
func (r *SellerSubscriptionRepositoryImpl) GetLatestByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var id, paymentID uuid.UUID
	var status subscriptionEntity.Status
	var startedAt, expiresAt, createdAt, updatedAt time.Time
	var durationDays int
	var amountPaid int64
	var currency string

	err := tx.QueryRow(ctx, `
		SELECT id, user_id, status,
		       started_at, expires_at,
		       duration_days,
		       amount_paid, currency,
		       payment_id,
		       created_at, updated_at
		FROM seller_subscriptions
		WHERE user_id = $1
		  AND status = 'active'
		  AND started_at <= NOW()
		  AND NOW() < expires_at
		ORDER BY started_at DESC
		LIMIT 1
	`, userID).Scan(
		&id, &userID, &status,
		&startedAt, &expiresAt,
		&durationDays,
		&amountPaid, &currency,
		&paymentID,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest subscription by user id failed: %w", err)
	}

	return &subscriptionEntity.SellerSubscription{
		ID:           id,
		UserID:       userID,
		Status:       status,
		StartedAt:    startedAt,
		ExpiresAt:    expiresAt,
		DurationDays: durationDays,
		AmountPaid:   money.New(amountPaid),
		Currency:     currency,
		PaymentID:    paymentID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// GetLatestByUserIDForUpdate returns the furthest entitlement chain end for a user.
// The caller must hold any seller-level serialization lock before invoking this.
// Returns nil if no entitlement-bearing subscription exists.
func (r *SellerSubscriptionRepositoryImpl) GetLatestByUserIDForUpdate(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var chainEnd sql.NullTime

	err := tx.QueryRow(ctx, `
		SELECT MAX(expires_at)
		FROM seller_subscriptions
		WHERE user_id = $1
		  AND status IN ('active', 'expired')
	`, userID).Scan(&chainEnd)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get entitlement chain end by user id failed: %w", err)
	}

	if !chainEnd.Valid {
		return nil, nil
	}

	return &subscriptionEntity.SellerSubscription{
		UserID:    userID,
		ExpiresAt: chainEnd.Time,
	}, nil
}

// GetActiveByUserID retrieves the active subscription for a user.
// Returns nil if no active subscription exists.
func (r *SellerSubscriptionRepositoryImpl) GetActiveByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var id, paymentID uuid.UUID
	var status subscriptionEntity.Status
	var startedAt, expiresAt, createdAt, updatedAt time.Time
	var durationDays int
	var amountPaid int64
	var currency string

	err := tx.QueryRow(ctx, `
		SELECT id, user_id, status,
		       started_at, expires_at,
		       duration_days,
		       amount_paid, currency,
		       payment_id,
		       created_at, updated_at
		FROM seller_subscriptions
		WHERE user_id = $1
		  AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&id, &userID, &status,
		&startedAt, &expiresAt,
		&durationDays,
		&amountPaid, &currency,
		&paymentID,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get active subscription by user id failed: %w", err)
	}

	return &subscriptionEntity.SellerSubscription{
		ID:           id,
		UserID:       userID,
		Status:       status,
		StartedAt:    startedAt,
		ExpiresAt:    expiresAt,
		DurationDays: durationDays,
		AmountPaid:   money.New(amountPaid),
		Currency:     currency,
		PaymentID:    paymentID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

// FetchActiveExpiredBatch retrieves subscriptions that have transitioned from
// active to expired (expires_at <= now). Uses FOR UPDATE SKIP LOCKED for
// concurrent-safe batch processing.
func (r *SellerSubscriptionRepositoryImpl) FetchActiveExpiredBatch(
	ctx context.Context,
	tx db.Tx,
	now time.Time,
	limit int,
) ([]*subscriptionEntity.SellerSubscription, error) {
	query := `
		SELECT id, user_id, status,
		       started_at, expires_at,
		       duration_days,
		       amount_paid, currency,
		       payment_id,
		       created_at, updated_at
		FROM seller_subscriptions
		WHERE status = 'active'
		  AND expires_at <= $1
		FOR UPDATE SKIP LOCKED
		LIMIT $2
	`

	return r.fetchBatch(ctx, tx, query, now, limit)
}

// fetchBatch is a helper for batch queries with FOR UPDATE SKIP LOCKED.
func (r *SellerSubscriptionRepositoryImpl) fetchBatch(
	ctx context.Context,
	tx db.Tx,
	query string,
	now time.Time,
	limit int,
) ([]*subscriptionEntity.SellerSubscription, error) {
	rows, err := tx.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch batch failed: %w", err)
	}
	defer rows.Close()

	var subscriptions []*subscriptionEntity.SellerSubscription
	for rows.Next() {
		var id, userID, paymentID uuid.UUID
		var status subscriptionEntity.Status
		var startedAt, expiresAt, createdAt, updatedAt time.Time
		var durationDays int
		var amountPaid int64
		var currency string

		if err := rows.Scan(
			&id, &userID, &status,
			&startedAt, &expiresAt,
			&durationDays,
			&amountPaid, &currency,
			&paymentID,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription failed: %w", err)
		}

		subscriptions = append(subscriptions, &subscriptionEntity.SellerSubscription{
			ID:           id,
			UserID:       userID,
			Status:       status,
			StartedAt:    startedAt,
			ExpiresAt:    expiresAt,
			DurationDays: durationDays,
			AmountPaid:   money.New(amountPaid),
			Currency:     currency,
			PaymentID:    paymentID,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate subscriptions failed: %w", rows.Err())
	}

	return subscriptions, nil
}

// GetActiveConfig retrieves the currently enabled subscription configuration.
// Returns nil if no enabled config exists.
func (r *SellerSubscriptionRepositoryImpl) GetActiveConfig(
	ctx context.Context,
	tx db.Tx,
) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	var id uuid.UUID
	var yearlyFeeRupiah int64
	var durationDays, renewalReminderDays int
	var enabled bool
	var createdAt time.Time

	err := tx.QueryRow(ctx, `
		SELECT id, yearly_fee_rupiah, duration_days, renewal_reminder_days, enabled, created_at
		FROM seller_subscription_configs
		WHERE enabled = true
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&id, &yearlyFeeRupiah, &durationDays, &renewalReminderDays, &enabled, &createdAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get active config failed: %w", err)
	}

	return &subscriptionEntity.SellerSubscriptionConfig{
		ID:                  id,
		YearlyFeeRupiah:     yearlyFeeRupiah,
		DurationDays:        durationDays,
		RenewalReminderDays: renewalReminderDays,
		Enabled:             enabled,
		CreatedAt:           createdAt,
	}, nil
}

// isUniqueViolationError checks if the error is a PostgreSQL UNIQUE constraint violation.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}
	pgErr, ok := err.(*pgconn.PgError)
	return ok && pgErr.Code == "23505" // UNIQUE_VIOLATION
}

// FetchActiveExpiredBatchIDs retrieves subscription IDs that have expired.
// Returns IDs only, without locking. Locking happens per-entity in transaction.
func (r *SellerSubscriptionRepositoryImpl) FetchActiveExpiredBatchIDs(
	ctx context.Context,
	tx db.Tx,
	now time.Time,
	limit int,
) ([]uuid.UUID, error) {
	query := `
		SELECT id
		FROM seller_subscriptions
		WHERE status = 'active'
		  AND expires_at <= $1
		LIMIT $2
	`

	return r.fetchBatchIDs(ctx, tx, query, now, limit)
}

// fetchBatchIDs is a helper for ID-only batch queries.
func (r *SellerSubscriptionRepositoryImpl) fetchBatchIDs(
	ctx context.Context,
	tx db.Tx,
	query string,
	now time.Time,
	limit int,
) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch batch ids failed: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan subscription id failed: %w", err)
		}
		ids = append(ids, id)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate subscription ids failed: %w", rows.Err())
	}

	return ids, nil
}

// ExistsActiveByUserID checks if a user has an active subscription.
// Returns true if at least one active subscription exists, false otherwise.
func (r *SellerSubscriptionRepositoryImpl) ExistsActiveByUserID(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (bool, error) {
	var exists bool

	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM seller_subscriptions
			WHERE user_id = $1
			AND status = 'active'
			LIMIT 1
		)
	`, userID).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check active subscription exists failed: %w", err)
	}

	return exists, nil
}

// UpdateConfigTx atomically updates a subscription configuration within a transaction.
//
// Atomic Safety: When enabling a config (enabled = true), this method automatically
// disables all other configs to ensure only ONE enabled config exists at any time.
// This is critical for operational consistency - pricing ambiguity leads to billing disputes.
//
// Parameters:
// - configID: The ID of the config to update
// - yearlyFee: New yearly fee in cents (can be 0 to keep current value)
// - durationDays: New duration in days (can be 0 to keep current value)
// - renewalReminderDays: New renewal reminder offset in days (can be 0 to keep current value)
// - isEnabled: Whether to enable this config (true) or disable it (false)
//
// Business Rules:
// - When isEnabled = true: ALL other configs are automatically disabled
// - When isEnabled = false: Only this config is disabled
// - Pricing updates (yearlyFee, durationDays, renewalReminderDays) only affect new subscriptions
// - Existing subscriptions retain their pricing snapshots
func (r *SellerSubscriptionRepositoryImpl) UpdateConfigTx(
	ctx context.Context,
	tx db.Tx,
	configID uuid.UUID,
	yearlyFeeRupiah int64,
	durationDays int,
	renewalReminderDays int,
	enabled bool,
) error {
	// Build the update query dynamically based on provided parameters
	// If pricing parameters are 0, keep existing values
	var setClauses []string
	var args []interface{}
	argPos := 1

	if yearlyFeeRupiah > 0 {
		setClauses = append(setClauses, fmt.Sprintf("yearly_fee_rupiah = $%d", argPos))
		args = append(args, yearlyFeeRupiah)
		argPos++
	}

	if durationDays > 0 {
		setClauses = append(setClauses, fmt.Sprintf("duration_days = $%d", argPos))
		args = append(args, durationDays)
		argPos++
	}

	if renewalReminderDays > 0 {
		setClauses = append(setClauses, fmt.Sprintf("renewal_reminder_days = $%d", argPos))
		args = append(args, renewalReminderDays)
		argPos++
	}

	// Always update enabled
	setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", argPos))
	args = append(args, enabled)
	argPos++

	// Add WHERE clause parameter
	args = append(args, configID)

	// Execute the update
	setClause := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE seller_subscription_configs SET %s WHERE id = $%d", setClause, argPos)

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update config failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("config not found: %s", configID)
	}

	// ATOMIC SAFETY: If enabling this config, disable all others
	// This ensures only ONE enabled config exists at any time
	if enabled {
		_, err := tx.Exec(ctx, `
			UPDATE seller_subscription_configs
			SET enabled = false
			WHERE id != $1
			  AND enabled = true
		`, configID)
		if err != nil {
			return fmt.Errorf("disable other configs failed: %w", err)
		}
	}

	return nil
}
