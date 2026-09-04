package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

const (
	internalShareSourceEntrypointRepostEndpoint = "repost_endpoint"
)

// internalShareAuthority owns the compatibility-layer internal share writer.
type internalShareAuthority interface {
	CreateInternalShare(ctx context.Context, tx db.Tx, input *CreateInternalShareRequest) (*entity.Content, error)
}

// InternalShareMediaInput keeps the phase-1 input shape compatible with the
// target architecture. Media persistence is intentionally unchanged in phase 1.
type InternalShareMediaInput struct {
	URL      string
	Type     entity.MediaType
	Position int
}

// CreateInternalShareRequest is the unified backend share input used by the
// compatibility layer.
type CreateInternalShareRequest struct {
	ActorID          uuid.UUID
	TargetType       entity.ShareTargetType
	TargetID         string
	Caption          string
	City             *string
	Province         *string
	Media            []InternalShareMediaInput
	SourceEntrypoint string
	OriginalAuthorID *uuid.UUID
}

func (s *ContentService) internalShareAuthorityOrSelf() internalShareAuthority {
	if s != nil && s.internalShareAuthority != nil {
		return s.internalShareAuthority
	}
	return s
}

// CreateInternalShare routes all internal share creation through a single
// backend authority while preserving the legacy entrypoint behavior.
func (s *ContentService) CreateInternalShare(
	ctx context.Context,
	tx db.Tx,
	input *CreateInternalShareRequest,
) (*entity.Content, error) {
	if input == nil {
		return nil, fmt.Errorf("internal share input is required")
	}

	if err := auth.ValidateCaller(input.ActorID); err != nil {
		return nil, err
	}

	if err := s.accountStatusChecker.EnsureActive(ctx, input.ActorID); err != nil {
		return nil, err
	}

	if err := validateShareTargetType(input.TargetType); err != nil {
		return nil, err
	}

	switch input.TargetType {
	case entity.ShareTargetTypeContent:
		return s.createInternalContentShare(ctx, tx, input)
	case entity.ShareTargetTypeForSale, entity.ShareTargetTypeAuction, entity.ShareTargetTypeProfile:
		return s.createInternalReferenceShare(ctx, tx, input)
	default:
		return nil, fmt.Errorf("invalid share target type: %s", input.TargetType)
	}
}

func (s *ContentService) createInternalContentShare(
	ctx context.Context,
	tx db.Tx,
	input *CreateInternalShareRequest,
) (*entity.Content, error) {
	originalContent, err := s.loadShareableContentTarget(ctx, tx, input.TargetID)
	if err != nil {
		return nil, err
	}
	occurrence := &entity.ContentResourceOccurrenceIdentity{
		Operation:    entity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: entity.ContentResourceOccurrenceResourceTypeContent,
		ResourceID:   originalContent.ID,
	}
	// Canonical path: delegate to CreateContentWithResourceOccurrence so share_to_feed has single write authority.
	// Attribution (original_author_id) is set inside CreateContentWithResourceOccurrence for content-type shares.
	return s.CreateContentWithResourceOccurrence(ctx, tx, input.ActorID, input.Caption, entity.VisibilityPublic, input.City, input.Province, occurrence, nil, nil)
}

func (s *ContentService) createInternalReferenceShare(
	ctx context.Context,
	tx db.Tx,
	input *CreateInternalShareRequest,
) (*entity.Content, error) {
	if input.OriginalAuthorID != nil {
		return nil, fmt.Errorf("original_author_id must be null when attaching non-content entities")
	}

	if err := validateShareTargetEnvelope(input.TargetType, input.TargetID); err != nil {
		return nil, err
	}

	targetID := mustParseUUID(input.TargetID)
	occurrence := &entity.ContentResourceOccurrenceIdentity{
		Operation:    entity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: shareTargetTypeToOccurrenceType(input.TargetType),
		ResourceID:   targetID,
	}
	// Single canonical writer for share_to_feed (validation + attribution inside CreateContentWithResourceOccurrence).
	return s.CreateContentWithResourceOccurrence(ctx, tx, input.ActorID, input.Caption, entity.VisibilityPublic, input.City, input.Province, occurrence, nil, nil)
}

func (s *ContentService) loadShareableContentTarget(
	ctx context.Context,
	tx db.Tx,
	contentID string,
) (*entity.Content, error) {
	targetID, err := uuid.Parse(contentID)
	if err != nil {
		return nil, fmt.Errorf("invalid target_id for content: %w", err)
	}

	targetContent, err := s.GetContentPublic(ctx, tx, targetID)
	if err != nil {
		return nil, fmt.Errorf("content not found: %w", err)
	}

	return targetContent, nil
}

func shareTargetTypeToOccurrenceType(targetType entity.ShareTargetType) entity.ContentResourceOccurrenceResourceType {
	switch targetType {
	case entity.ShareTargetTypeContent:
		return entity.ContentResourceOccurrenceResourceTypeContent
	case entity.ShareTargetTypeForSale:
		return entity.ContentResourceOccurrenceResourceTypeForSale
	case entity.ShareTargetTypeAuction:
		return entity.ContentResourceOccurrenceResourceTypeAuction
	case entity.ShareTargetTypeProfile:
		return entity.ContentResourceOccurrenceResourceTypeProfile
	default:
		return ""
	}
}

func mustParseUUID(raw string) uuid.UUID {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return id
}
