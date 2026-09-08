package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// ContractTargetRepository persists the rolling target queue for a promotion contract.
type ContractTargetRepository interface {
	AddTarget(ctx context.Context, tx db.Tx, target *entity.ContractTarget) error
	ListTargets(ctx context.Context, tx db.Tx, contractID uuid.UUID) ([]*entity.ContractTarget, error)
	RemoveTarget(ctx context.Context, tx db.Tx, contractID, targetID uuid.UUID) error
	CountTargets(ctx context.Context, tx db.Tx, contractID uuid.UUID) (int, error)
	ResolveEffectiveTarget(ctx context.Context, tx db.Tx, contractID uuid.UUID, checker func(targetType promoentity.TargetType, targetID *uuid.UUID) (bool, string, error)) (*entity.ContractTarget, error)
	GetTargetForUpdate(ctx context.Context, tx db.Tx, contractID, targetID uuid.UUID) (*entity.ContractTarget, error)
}
