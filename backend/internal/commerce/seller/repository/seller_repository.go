package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/commerce/seller/entity"
	"github.com/hishumi/backend/pkg/db"
)

// SellerRepository defines the persistence operations for seller domain.
//
// This repository handles:
// - SellerProfile CRUD operations
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

	// EnsureProfileExistsTxWithImage is the canonical variant that persists optional store_image_url at creation.
	EnsureProfileExistsTxWithImage(ctx context.Context, tx db.Tx, userID uuid.UUID, storeName string, storeImageURL *string) (*entity.SellerProfile, error)

	// UpdateSellerProfileTx atomically updates store_name and/or store_image_url.
	UpdateSellerProfileTx(ctx context.Context, tx db.Tx, userID uuid.UUID, storeName *string, storeImageURL *string) (*entity.SellerProfile, error)

	// GetByIDForUpdate retrieves a seller profile by ID with row-level lock (FOR UPDATE).
	// Use this for updates to prevent concurrent modifications.
	GetByIDForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.SellerProfile, error)

	// UpdateTierTx updates the tier of a seller profile.
	UpdateTierTx(ctx context.Context, tx db.Tx, id uuid.UUID, tier entity.Tier) error

	// UpsertReputationStateTx creates or overwrites the live reputation state for a seller.
	// Called by SellerReputationRecomputeWorker on every nightly cycle.
	// Safe to call multiple times — last-write-wins (UPSERT semantics).
	UpsertReputationStateTx(ctx context.Context, tx db.Tx, state *entity.SellerReputationState) error

	// GetReputationStateForUpdate retrieves the live reputation state with a row-level lock.
	// Returns nil if no state row exists yet (seller not yet processed by recompute worker).
	// sellerID is the canonical commerce seller identity (users.id), NOT seller_profiles.id.
	GetReputationStateForUpdate(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerReputationState, error)

	// GetReputationState retrieves the live reputation state WITHOUT a row-level lock.
	// Read projection only (e.g. Seller Performance). Returns nil if no state row exists.
	// sellerID is the canonical commerce seller identity (users.id), NOT seller_profiles.id.
	GetReputationState(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (*entity.SellerReputationState, error)
}
