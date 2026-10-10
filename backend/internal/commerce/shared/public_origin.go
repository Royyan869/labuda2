package shared

import (
	"context"

	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
	addressRepo "github.com/hishumi/backend/internal/identity/address/repository"
	"github.com/hishumi/backend/pkg/db"
)

// PublicListingOrigin resolves the buyer-facing origin summary ("City,
// Province") for one listing.
//
// DELEGATION: the resolution rule itself lives ONCE, on the address domain
// (identity/address/repository.ResolvePublicOrigin) — the same function the
// public profile calls — so a listing's seller card and that seller's profile
// can never show different origins. The listing carries no origin address of
// its own: every product uses the seller account's primary address.
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

	return addressRepo.ResolvePublicOrigin(ctx, tx, repos, product.SellerID)
}
