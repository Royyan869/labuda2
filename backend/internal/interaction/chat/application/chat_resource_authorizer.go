package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
)

// ResourceAuthorizer is the canonical authorization port for Chat resource
// occurrences. It is owned by the Chat application layer; Commerce and
// Identity domains implement it. Chat must never import Commerce
// implementation packages — all resource policy flows through this port.
type ResourceAuthorizer interface {
	// SHARE authorization — checks whether viewerID may share the resource.
	// Returns (fallback_json, nil) on success, or a typed error on failure.
	AuthorizeShare(ctx context.Context, tx interface{}, viewerID uuid.UUID, resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID) (json.RawMessage, error)

	// DIRECT_COMMERCE_INSERT authorization — checks ownership, market
	// capability, and promotability for direct commerce insert.
	// Returns (fallback_json, nil) on success, or a typed error on failure.
	AuthorizeDirect(ctx context.Context, tx interface{}, actorID uuid.UUID, resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID) (json.RawMessage, error)

	// BuildFallback resolves and builds the server fallback snapshot for a
	// resource without performing authorization. Used for hydration, not
	// for access control.
	BuildFallback(ctx context.Context, tx interface{}, resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID) (json.RawMessage, error)
}

// authorizationErrors maps typed authorizer results to canonical Chat errors.
func authorizationError(err error) error {
	if err == nil {
		return nil
	}
	// Pass through typed domain errors directly
	switch {
	case err == chatRepo.ErrResourceNotFound:
		return chatRepo.ErrResourceNotFound
	case err == chatRepo.ErrResourceNotAccessible:
		return chatRepo.ErrResourceNotAccessible
	case err == chatRepo.ErrNotResourceOwner:
		return chatRepo.ErrNotResourceOwner
	case err == chatRepo.ErrMarketAuthorityRequired:
		return chatRepo.ErrMarketAuthorityRequired
	case err == chatRepo.ErrResourceNotPromotable:
		return chatRepo.ErrResourceNotPromotable
	default:
		return err
	}
}
