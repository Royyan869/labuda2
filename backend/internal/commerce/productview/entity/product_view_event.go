// Package entity defines the canonical Product View event.
//
// PRODUCT VIEW IS ITS OWN DOMAIN. It records one observational fact only:
// a product detail page was successfully opened by a viewer entitled to see
// it. It never determines price, status, sold state, auction state, bids,
// orders, payments, seller reputation, or promotion performance.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// ProductViewEvent is one immutable Product View observation.
//
// It is the single canonical authority for Product View. The identity is the
// canonical PRODUCT (products.id) — not a For Sale or Auction row — so both
// selling surfaces share one view authority and relist/reuse does not create a
// second one.
//
// ViewerUserID is the authenticated viewer, or nil for an anonymous viewer.
// Seller self-views and admin/moderator views are never represented here
// (excluded at the producer).
type ProductViewEvent struct {
	ID           uuid.UUID
	ProductID    uuid.UUID
	ViewerUserID *uuid.UUID
	ViewedAt     time.Time
}
