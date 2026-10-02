package application

import (
	"context"

	"github.com/google/uuid"
	addressInfraRepo "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
)

// publicOriginLineFor resolves the buyer-facing public origin summary
// ("City, Province") for a profile.
//
// DELEGATION: it calls the SAME resolver as the commerce detail cards
// (identity/address/repository.ResolvePublicOrigin) — the profile passes no
// listing address, so the rule reads primary sender → any sender → shipping.
// One rule, one redaction, one origin per seller. Missing truth resolves to
// "" — the line is hidden, never fabricated, and an unresolvable address
// never fails the read.
func publicOriginLineFor(ctx context.Context, tx db.Tx, userID uuid.UUID) string {
	if tx == nil || userID == uuid.Nil {
		return ""
	}

	return addressRepo.ResolvePublicOrigin(
		ctx,
		tx,
		addressInfraRepo.NewAddressRepository(),
		userID,
		nil,
	)
}
