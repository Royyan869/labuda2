package repository

import (
	"context"

	"github.com/google/uuid"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
	"github.com/hishumi/backend/pkg/db"
)

// ResolvePublicOrigin resolves the public origin line ("City, Province") of a
// user. It is the ONE resolver every public surface calls — the commerce
// detail seller card (for_sale and auction) and the public profile — so two
// surfaces can never show different origins for the same seller.
//
// RESOLUTION ORDER (canonical account address book):
//
//  1. the account's PRIMARY address
//  2. the account's oldest address (fallback when no primary is flagged, e.g.
//     pre-reconciler data)
//
// There is no product-level origin and no role (shipping/sender) narrowing:
// every product uses the account's primary address.
//
// Reads go through GetByUserIDForDisplay: the checkout-availability flag is
// deliberately ignored, because it decides what a buyer may check out with,
// never what may be displayed.
//
// Redaction lives in entity.BuildPublicOriginSummary: city + province only —
// never street, district, recipient, phone or coordinates. Missing truth
// resolves to "" (the line is hidden, never fabricated) and a lookup failure
// never fails the read that carries it.
func ResolvePublicOrigin(
	ctx context.Context,
	tx db.Tx,
	repos AddressRepository,
	userID uuid.UUID,
) string {
	if repos == nil || userID == uuid.Nil {
		return ""
	}

	addresses, err := repos.GetByUserIDForDisplay(ctx, tx, userID)
	if err != nil || len(addresses) == 0 {
		return ""
	}

	if primary := primaryOf(addresses); primary != nil {
		if summary := addressEntity.BuildPublicOriginSummary(primary); summary != "" {
			return summary
		}
	}

	oldest := oldestOf(addresses)
	if oldest == nil {
		return ""
	}
	return addressEntity.BuildPublicOriginSummary(oldest)
}

// primaryOf returns the flagged primary row, or nil when there is none.
func primaryOf(addresses []*addressEntity.Address) *addressEntity.Address {
	for _, address := range addresses {
		if address != nil && address.IsPrimary {
			return address
		}
	}
	return nil
}

// oldestOf returns the oldest usable address (created_at, then id), matching
// the primary-promotion tie-breaker. GetByUserIDForDisplay orders by
// is_primary DESC, created_at DESC, so the last eligible row is the oldest.
func oldestOf(addresses []*addressEntity.Address) *addressEntity.Address {
	var oldest *addressEntity.Address
	for _, address := range addresses {
		if address == nil {
			continue
		}
		if oldest == nil ||
			address.CreatedAt.Before(oldest.CreatedAt) ||
			(address.CreatedAt.Equal(oldest.CreatedAt) && address.ID.String() < oldest.ID.String()) {
			oldest = address
		}
	}
	return oldest
}
