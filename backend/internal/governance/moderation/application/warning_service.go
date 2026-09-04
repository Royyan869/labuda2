package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
)

// WarningService handles warning business logic.
//
// Standalone warning creation (IssueWarning) has been removed.
// Canonical invariant: warnings may only exist with Decision provenance.
// Read and revoke paths remain for existing governance data.
type WarningService struct {
	warningRepo repository.WarningRepository
}

// NewWarningService creates a new WarningService.
func NewWarningService(
	warningRepo repository.WarningRepository,
) *WarningService {
	return &WarningService{
		warningRepo: warningRepo,
	}
}

// GetWarning retrieves a warning by ID.
func (s *WarningService) GetWarning(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
) (*entity.UserWarning, error) {
	return s.warningRepo.GetByID(ctx, tx, warningID)
}

// ListWarningsByUser retrieves all warnings for a specific user.
func (s *WarningService) ListWarningsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit, offset int,
) ([]*entity.UserWarning, error) {
	return s.warningRepo.ListByUser(ctx, tx, userID, limit, offset)
}

// ListActiveWarningsByUser retrieves active warnings for a specific user.
func (s *WarningService) ListActiveWarningsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) ([]*entity.UserWarning, error) {
	return s.warningRepo.ListActiveByUser(ctx, tx, userID)
}

// ListAllWarnings retrieves all warnings with optional user filter.
// If userID is nil, returns all warnings.
func (s *WarningService) ListAllWarnings(
	ctx context.Context,
	tx interface{},
	userID *uuid.UUID,
	isActive *bool,
	limit, offset int,
) ([]*entity.UserWarning, int64, error) {
	return s.warningRepo.ListAll(ctx, tx, userID, isActive, limit, offset)
}

// RevokeWarning revokes an active warning.
//
// Business rules:
// - Only active warnings can be revoked
// - Only admins can revoke warnings
func (s *WarningService) RevokeWarning(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
	revokedBy uuid.UUID,
) (*entity.UserWarning, error) {
	// Get warning with FOR UPDATE lock to prevent concurrent modifications
	warning, err := s.warningRepo.GetForUpdate(ctx, tx, warningID)
	if err != nil {
		return nil, err
	}

	// Revoke the warning
	err = warning.Revoke(revokedBy)
	if err != nil {
		return nil, err
	}

	// Persist the updated warning
	err = s.warningRepo.Update(ctx, tx, warning)
	if err != nil {
		return nil, err
	}

	return warning, nil
}



