package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/pkg/db"
)

// ProductRepository defines persistence operations for canonical product rows.
type ProductRepository interface {
	Create(ctx context.Context, tx db.Tx, product *entity.Product) error
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Product, error)
	Update(ctx context.Context, tx db.Tx, product *entity.Product) error
}
