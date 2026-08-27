package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	db "github.com/labuda/backend/pkg/db"
)

// SellerSubscriptionRepository defines the persistence operations for seller subscriptions.
// All operations must be executed within a transaction for consistency.
//
// The repository uses SQL only - no business logic.
// Business validation is handled at the entity/service layer.
type SellerSubscriptionRepository interface {
	// InsertTx creates a new subscription record within a transaction.
	// Returns subscriptionEntity.ErrDuplicateActiveSubscription if the user
	// already has an active subscription (enforced by DB unique constraint).
	InsertTx(ctx context.Context, tx db.Tx, s *subscriptionEntity.SellerSubscription) error

	// UpdateStatusTx transitions a subscription status with guard protection.
	// Uses WHERE clause with both id AND fromStatus for atomic guard.
	// Returns subscriptionEntity.ErrTransitionGuardFailed if current status doesn't match.
	UpdateStatusTx(ctx context.Context, tx db.Tx, id uuid.UUID, fromStatus, toStatus subscriptionEntity.Status) error

	// GetByIDForUpdate retrieves a subscription by ID with row-level lock.
	// Uses FOR UPDATE to prevent concurrent modifications.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*subscriptionEntity.SellerSubscription, error)

	// GetByID retrieves a subscription by ID without locking.
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*subscriptionEntity.SellerSubscription, error)

	// GetLatestByUserID retrieves the most recent subscription for a user,
	// regardless of status. Returns nil if no subscription exists.
	GetLatestByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*subscriptionEntity.SellerSubscription, error)

	// GetLatestByUserIDForUpdate returns the furthest entitlement chain end for a user.
	// The caller must hold any necessary seller-level lock before invoking this.
	// Returns nil if no entitlement-bearing subscription exists.
	GetLatestByUserIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*subscriptionEntity.SellerSubscription, error)

	// GetActiveByUserID retrieves the current active subscription for a user.
	// Returns nil if no active subscription exists.
	GetActiveByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*subscriptionEntity.SellerSubscription, error)

	// FetchActiveExpiredBatch retrieves subscriptions that have transitioned from
	// active to expired (expires_at <= now). Uses FOR UPDATE SKIP LOCKED for
	// concurrent-safe batch processing.
	FetchActiveExpiredBatch(ctx context.Context, tx db.Tx, now time.Time, limit int) ([]*subscriptionEntity.SellerSubscription, error)

	// FetchActiveExpiredBatchIDs retrieves subscription IDs that have expired.
	// Returns IDs only, without locking. Locking happens per-entity in transaction.
	FetchActiveExpiredBatchIDs(ctx context.Context, tx db.Tx, now time.Time, limit int) ([]uuid.UUID, error)

	// ExistsActiveByUserID checks if a user has an active subscription.
	// Returns true if at least one active subscription exists, false otherwise.
	// Used by deactivation logic to prevent seller deactivation if another active subscription exists.
	ExistsActiveByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (bool, error)

	// GetActiveConfig retrieves the currently enabled subscription configuration.
	// Returns nil if no enabled config exists.
	GetActiveConfig(ctx context.Context, tx db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error)

	// UpdateConfigTx atomically updates a subscription configuration within a transaction.
	// Atomic Safety: When enabling a config (enabled = true), this method automatically
	// disables all other configs to ensure only ONE enabled config exists at any time.
	// This is critical for operational consistency - pricing ambiguity leads to billing disputes.
	//
	// Parameters:
	// - configID: The ID of the config to update
	// - yearlyFeeRupiah: New yearly fee in rupiah (can be 0 to keep current value)
	// - durationDays: New duration in days (can be 0 to keep current value)
	// - renewalReminderDays: New renewal reminder offset in days (can be 0 to keep current value)
	// - enabled: Whether to enable this config (true) or disable it (false)
	//
	// Business Rules:
	// - When isEnabled = true: ALL other configs are automatically disabled
	// - When isEnabled = false: Only this config is disabled
	// - Pricing updates (yearlyFee, durationDays, renewalReminderDays) only affect new subscriptions
	// - Existing subscriptions retain their pricing snapshots
	UpdateConfigTx(ctx context.Context, tx db.Tx, configID uuid.UUID, yearlyFeeRupiah int64, durationDays int, renewalReminderDays int, enabled bool) error
}
