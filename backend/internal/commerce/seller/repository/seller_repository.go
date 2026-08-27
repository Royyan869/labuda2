package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
)

// SellerRepository defines the persistence operations for seller domain.
//
// This repository handles:
// - SellerProfile CRUD operations
// - SellerMonthlyMetric insert (ANALYTICS ONLY — never read for tier decisions)
// - SellerReputationState upsert and reads (LIVE REPUTATION AUTHORITY)
//
// No business logic - all validation belongs in service layer.
type SellerRepository interface {
	// InsertProfileTx creates a new seller profile within a transaction.
	InsertProfileTx(ctx context.Context, tx db.Tx, p *entity.SellerProfile) error

	// GetByUserID retrieves a seller profile by user ID.
	// Returns nil if not found.
	GetByUserID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.SellerProfile, error)

	// GetByUserIDForUpdate retrieves a seller profile by user ID with row-level lock (FOR UPDATE).
	// Use this for updates to prevent concurrent modifications during onboarding.
	GetByUserIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.SellerProfile, error)

	// EnsureProfileExistsTx safely creates or retrieves a seller profile within a transaction.
	// Behavior:
	// - If profile exists for userID: returns existing profile
	// - If not exists: creates new profile with basic tier
	// Atomic inside tx - uses UNIQUE(user_id) for race protection.
	// Returns the profile (existing or newly created).
	EnsureProfileExistsTx(ctx context.Context, tx db.Tx, userID uuid.UUID, storeName string) (*entity.SellerProfile, error)

	// GetByIDForUpdate retrieves a seller profile by ID with row-level lock (FOR UPDATE).
	// Use this for updates to prevent concurrent modifications.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.SellerProfile, error)

	// UpdateTierTx updates the tier of a seller profile.
	UpdateTierTx(ctx context.Context, tx db.Tx, id uuid.UUID, tier entity.Tier) error

	// InsertMonthlyMetricTx creates a new monthly metric snapshot within a transaction.
	// Analytics-only write: seller_monthly_metrics is never read for tier decisions.
	InsertMonthlyMetricTx(ctx context.Context, tx db.Tx, m *entity.SellerMonthlyMetric) error

	// UpsertReputationStateTx creates or overwrites the live reputation state for a seller.
	// Called by SellerReputationRecomputeWorker on every nightly cycle.
	// Safe to call multiple times — last-write-wins (UPSERT semantics).
	UpsertReputationStateTx(ctx context.Context, tx db.Tx, state *entity.SellerReputationState) error

	// GetReputationStateForUpdate retrieves the live reputation state with a row-level lock.
	// Returns nil if no state row exists yet (seller not yet processed by recompute worker).
	GetReputationStateForUpdate(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerReputationState, error)
}


