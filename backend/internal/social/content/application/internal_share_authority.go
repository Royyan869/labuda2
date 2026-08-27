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
	internalShareSourceEntrypointRepostEndpoint              = "repost_endpoint"
	internalShareSourceEntrypointCreateContentShareReference = "create_content_share_reference"
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

	originalAuthorID := uuid.UUID{}
	switch {
	case input.OriginalAuthorID != nil:
		originalAuthorID = *input.OriginalAuthorID
	case input.SourceEntrypoint == internalShareSourceEntrypointRepostEndpoint:
		originalAuthorID = originalContent.AuthorID
	default:
		return nil, fmt.Errorf("original_author_id is required when sharing content")
	}

	repost := entity.NewContent(input.ActorID, input.Caption)
	repost.Visibility = entity.VisibilityPublic

	if err := repost.MarkAsRepostWithStatus(
		originalContent.ID,
		originalAuthorID,
		"",
		"",
		false,
	); err != nil {
		return nil, fmt.Errorf("mark as repost failed: %w", err)
	}

	if err := s.contentRepo.Create(ctx, tx, repost); err != nil {
		return nil, fmt.Errorf("create repost failed: %w", err)
	}

	occ := entity.NewContentResourceOccurrence(repost.ID, input.ActorID, &entity.ContentResourceOccurrenceIdentity{
		Operation:    entity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: entity.ContentResourceOccurrenceResourceTypeContent,
		ResourceID:   originalContent.ID,
	})
	if err := createContentResourceOccurrence(ctx, tx, s.contentRepo, occ); err != nil {
		return nil, fmt.Errorf("create repost occurrence failed: %w", err)
	}

	return repost, nil
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

	content := entity.NewContent(input.ActorID, input.Caption)
	content.Visibility = entity.VisibilityPublic
	content.City = input.City
	content.Province = input.Province

	if err := s.contentRepo.Create(ctx, tx, content); err != nil {
		return nil, fmt.Errorf("create content failed: %w", err)
	}

	occurrence := &entity.ContentResourceOccurrenceIdentity{
		Operation:    entity.ContentResourceOccurrenceOperationShareToFeed,
		ResourceType: shareTargetTypeToOccurrenceType(input.TargetType),
		ResourceID:   mustParseUUID(input.TargetID),
	}
	if err := createContentResourceOccurrence(ctx, tx, s.contentRepo, entity.NewContentResourceOccurrence(content.ID, input.ActorID, occurrence)); err != nil {
		return nil, fmt.Errorf("create content occurrence failed: %w", err)
	}

	return content, nil
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
