package repository

import (
	"context"

	"github.com/google/uuid"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/db"
)

// ResolvePublicOrigin resolves the public origin line ("City, Province") of a
// user. It is the ONE resolver every public surface calls — the commerce
// detail seller card (for_sale and auction) and the public profile — so two
// surfaces can never show different origins for the same seller.
//
// RESOLUTION ORDER (owner rule: primary first, sender as fallback):
//
//  1. the user's PRIMARY sender address
//  2. the product's own sender address (for_sale / auction surfaces only, and
//     only when it belongs to this user)
//  3. any other sender address the user owns
//  4. the user's PRIMARY shipping address
//  5. any other shipping address
//  6. any address at all — a user who is NOT yet a seller must still show
//     where they live, primary first
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
	productFarmAddressID *uuid.UUID,
) string {
	if repos == nil || userID == uuid.Nil {
		return ""
	}

	addresses, err := repos.GetByUserIDForDisplay(ctx, tx, userID)
	if err != nil || len(addresses) == 0 {
		return ""
	}

	senders := byTag(addresses, addressEntity.TagSender)
	shippings := byTag(addresses, addressEntity.TagShipping)

	candidates := make([]*addressEntity.Address, 0, 6)
	add := func(address *addressEntity.Address) {
		if address != nil {
			candidates = append(candidates, address)
		}
	}

	add(primaryOf(senders))                        // 1. primary sender
	add(findByID(addresses, productFarmAddressID)) // 2. product's sender address
	add(pickPrimaryFirst(senders))                 // 3. any sender
	add(primaryOf(shippings))                      // 4. primary shipping
	add(pickPrimaryFirst(shippings))               // 5. any shipping
	add(pickPrimaryFirst(addresses))               // 6. non-seller fallback

	for _, address := range candidates {
		if summary := addressEntity.BuildPublicOriginSummary(address); summary != "" {
			return summary
		}
	}

	return ""
}

// byTag narrows an address list to rows carrying the given tag. Tags are a
// set: an address that is both a destination and an origin appears in both
// lists, which is exactly why the tags exist.
func byTag(
	addresses []*addressEntity.Address,
	tag addressEntity.AddressTag,
) []*addressEntity.Address {
	matched := make([]*addressEntity.Address, 0, len(addresses))
	for _, address := range addresses {
		if address != nil && address.HasTag(tag) {
			matched = append(matched, address)
		}
	}
	return matched
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

// pickPrimaryFirst returns the flagged primary row when one exists, otherwise
// the first usable row. A single-address user has that row with
// is_primary=false — the primary flag must never hide their origin.
func pickPrimaryFirst(addresses []*addressEntity.Address) *addressEntity.Address {
	if address := primaryOf(addresses); address != nil {
		return address
	}
	for _, address := range addresses {
		if address != nil {
			return address
		}
	}
	return nil
}

// findByID matches the product's own sender address inside this user's rows.
// Matching inside the user's own result set keeps ownership enforced: an
// address that does not belong to the user can never become their origin.
func findByID(addresses []*addressEntity.Address, id *uuid.UUID) *addressEntity.Address {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	for _, address := range addresses {
		if address != nil && address.ID == *id {
			return address
		}
	}
	return nil
}
