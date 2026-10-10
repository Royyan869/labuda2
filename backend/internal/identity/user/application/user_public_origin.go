package application

import (
	"context"

	"github.com/google/uuid"
	addressInfraRepo "github.com/hishumi/backend/internal/identity/address/infrastructure/repository"
	addressRepo "github.com/hishumi/backend/internal/identity/address/repository"
	"github.com/hishumi/backend/pkg/db"
)

// publicOriginLineFor resolves the buyer-facing public origin summary
// ("City, Province") for a profile.
//
// DELEGATION: it calls the SAME resolver as the commerce detail cards
// (identity/address/repository.ResolvePublicOrigin) — the account's primary
// address (oldest as fallback). One rule, one redaction, one origin per
// account. Missing truth resolves to "" — the line is hidden, never
// fabricated, and an unresolvable address never fails the read.
func publicOriginLineFor(ctx context.Context, tx db.Tx, userID uuid.UUID) string {
	if tx == nil || userID == uuid.Nil {
		return ""
	}

	return addressRepo.ResolvePublicOrigin(
		ctx,
		tx,
		addressInfraRepo.NewAddressRepository(),
		userID,
	)
}
