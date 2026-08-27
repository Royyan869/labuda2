package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// DisputeFreeze is the persistence model for an active dispute freeze.
type DisputeFreeze struct {
	ID           uuid.UUID
	DisputeID    uuid.UUID
	OrderID      uuid.UUID
	SellerID     uuid.UUID
	FrozenAmount int64
	Status       string // "active" | "released"
	CreatedAt    int64  // Unix epoch ms
	UpdatedAt    int64
}

// DisputeFreezeRepository is the persistence surface for dispute_freezes.
type DisputeFreezeRepository interface {
	// Create inserts a new active freeze record. Returns error if a freeze for
	// this dispute_id already exists (UNIQUE constraint on dispute_id).
	Create(ctx context.Context, tx db.Tx, freeze *DisputeFreeze) error

	// GetByDisputeID fetches the freeze for a given dispute (nil if none).
	GetByDisputeID(ctx context.Context, tx db.Tx, disputeID uuid.UUID) (*DisputeFreeze, error)

	// Release marks an active freeze as released. Idempotent: if already
	// released, returns nil without error.
	Release(ctx context.Context, tx db.Tx, disputeID uuid.UUID) error

	// ReleaseByOrderID marks any active freeze for the given order as released.
	// Idempotent: if no active freeze exists, returns nil. H2-F2b: Used by
	// the refund ack handler to release the freeze after successful gateway ack.
	ReleaseByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) error

	// GetTotalActiveBySeller sums frozen_amount for all active freezes for the
	// given seller. Used by AssertSellerWithdrawalAllowed to reduce withdrawable.
	GetTotalActiveBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (int64, error)
}


