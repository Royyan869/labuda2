package entity

import (
	"time"

	"github.com/google/uuid"
)

// ForSalePreview represents a simplified forSale for search results.
//
// PHASE 5 STAGE 1 — SELLER/FARM CONTRACT CONVERGENCE (additive):
//   - SellerUsername  ← user_profiles.username (NEVER store_name)
//   - SellerFarmName  ← seller_profiles.store_name (NEVER username)
//   - SellerAvatarURL ← user_profiles.avatar_url
type ForSalePreview struct {
	ID          uuid.UUID
	Title       string
	Description string
	Variety     string
	Price       int64
	MediaURLs   []string
	SellerID    uuid.UUID
	CreatedAt   time.Time

	// Phase 5 Stage 1 additive seller convergence fields.
	SellerUsername  string // user_profiles.username
	SellerFarmName  string // seller_profiles.store_name
	SellerAvatarURL string // user_profiles.avatar_url

	// Expired-seller visibility — raw user/subscription truth for the
	// forSale's seller. Carried INSIDE the service boundary only; the
	// search handler coarsens via viewercontext.CoarsenLifecycle /
	// CoarsenSellerTrust before emitting the SellerCard. Empty
	// SellerSubscriptionStatus means the seller has no subscription row
	// (treated as "unavailable" by CoarsenSellerTrust).
	SellerAccountStatus      string
	SellerIsDeleted          bool
	SellerSubscriptionStatus string
}

// ContentPreview represents a simplified content for search results.
//
// Author identity fields are sourced from user_profiles (the canonical
// public identity binding). PRIVACY: full_name is KYC/private and is
// not projected here.
// users.email is NEVER used as a public author field per
// viewer-context-contract.md §4.1.
type ContentPreview struct {
	ID              uuid.UUID
	AuthorID        uuid.UUID
	Type            string
	Caption         string
	MediaURLs       []string
	CreatedAt       time.Time
	AuthorUsername  string  // user_profiles.username, '' when profile row absent
	AuthorAvatarURL *string // user_profiles.avatar_url, nil when not set
}

// UserPreview represents a simplified user for search results.
type UserPreview struct {
	ID                      uuid.UUID
	Username                string
	AvatarURL               *string
	IsFollowedByCurrentUser bool // true when the authenticated viewer follows this user
}

// AuctionPreview represents a simplified auction for search results.
// AUCTION SEARCH ELIGIBILITY (Phase 3.5):
// Only auctions with specific statuses are searchable:
// - scheduled: Upcoming auctions that are publicly visible
// - active: Currently running auctions
// - ended: Completed auctions (for historical search)
//
// NOT SEARCHABLE:
// - draft: Not yet published, seller-only
// - cancelled: Terminated before completion, not discoverable
//
// PHASE 5 STAGE 1 — SELLER/FARM CONTRACT CONVERGENCE (additive):
//   - SellerUsername  ← user_profiles.username (NEVER store_name)
//   - SellerFarmName  ← seller_profiles.store_name (NEVER username)
//   - SellerAvatarURL ← user_profiles.avatar_url
type AuctionPreview struct {
	ID           uuid.UUID
	SellerID     uuid.UUID
	ProductID    uuid.UUID
	Title        string
	Description  string
	StartPrice   int64
	CurrentBid   *int64
	BuyNowPrice  *int64
	StartAt      time.Time
	EndAt        time.Time
	Status       string  // "scheduled", "active", "ended"
	ThumbnailURL *string // Primary image from product
	BidCount     int     // Number of bids (for sorting)
	CreatedAt    time.Time

	// Phase 5 Stage 1 additive seller convergence fields.
	SellerUsername  string // user_profiles.username (canonical public identity)
	SellerFarmName  string // seller_profiles.store_name (business display)
	SellerAvatarURL string // user_profiles.avatar_url

	// Expired-seller visibility — raw user/subscription truth for the
	// auction's seller. Carried INSIDE the service boundary only; the
	// search handler coarsens before emitting the SellerCard.
	SellerAccountStatus      string
	SellerIsDeleted          bool
	SellerSubscriptionStatus string
}

// SearchFilters represents common search filters.
type SearchFilters struct {
	Query   string
	Limit   int
	Offset  int
	SortBy  string // "relevance", "created_at"
	SortDir string // "asc", "desc"
}
