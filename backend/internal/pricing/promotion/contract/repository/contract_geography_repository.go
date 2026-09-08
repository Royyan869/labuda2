package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/labuda/backend/pkg/db"
)

type ContractGeographyRepository interface {
	ReplaceGeographies(ctx context.Context, tx db.Tx, contractID uuid.UUID, geos []entity.ContractGeography) error
	ListByContract(ctx context.Context, tx db.Tx, contractID uuid.UUID) ([]*entity.ContractGeography, error)
	HasGeographicRestriction(ctx context.Context, tx db.Tx, contractID uuid.UUID) (bool, error)
	IsCityAllowed(ctx context.Context, tx db.Tx, contractID uuid.UUID, cityID string) (bool, error)
}
