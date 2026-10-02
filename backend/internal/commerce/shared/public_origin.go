package shared

import (
	"context"

	"github.com/google/uuid"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
)

// PublicListingOrigin resolves the buyer-facing origin summary ("City,
// Province") for one listing.
//
// DELEGATION: the resolution rule itself lives ONCE, on the address domain
// (identity/address/repository.ResolvePublicOrigin) — the same function the
// public profile calls — so a listing's seller card and that seller's profile
// can never show different origins. This wrapper only carries the listing's
// product facts (seller + farm address) into that rule:
//
//	primary sender address → listing's sender address → any sender → shipping.
//
// Missing truth degrades to "" (the line is HIDDEN, never fabricated) and an
// unresolvable address never fails the read that carries it.
func PublicListingOrigin(
	ctx context.Context,
	tx db.Tx,
	repos addressRepo.AddressRepository,
	product *productEntity.Product,
) string {
	if repos == nil || product == nil {
		return ""
	}

	var listingFarmAddressID *uuid.UUID
	if product.FarmAddressID != nil && *product.FarmAddressID != uuid.Nil {
		listingFarmAddressID = product.FarmAddressID
	}

	return addressRepo.ResolvePublicOrigin(
		ctx,
		tx,
		repos,
		product.SellerID,
		listingFarmAddressID,
	)
}
