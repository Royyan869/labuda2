package entity

import (
	"time"

	"github.com/google/uuid"
)

// Tier represents the seller tier badge.
// Tier is a visual indicator only - no financial impact.
//
// Current ladder: Basic → Pro → Elite.
// Legend tier is intentionally deferred — when adding it, append TierLegend
// here and update evaluateTierFromAggregates + isTierDowngrade + GatedSellerTier.
type Tier string

const (
	TierBasic Tier = "basic"
	TierPro   Tier = "pro"
	TierElite Tier = "elite"
	// TierLegend Tier = "legend" — intentionally deferred; uncomment when ready.
)

// SellerProfile represents the business identity of a seller.
//
// Seller Domain is the Business Identity Layer:
// - 1 user = 1 seller profile
// - Tier = badge only (no financial impact)
// - No ledger coupling
// - Seller capability is derived from seller_subscriptions (not from profile)
//
// Invariants:
// - user_id is unique (enforced by DB)
// - store_name is required
// - tier defaults to 'basic'
type SellerProfile struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	StoreName string
	Tier      Tier
	CreatedAt time.Time
	UpdatedAt time.Time
}



