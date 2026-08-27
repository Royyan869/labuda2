package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
)

// PricingTokenRepository handles pricing token persistence.
type PricingTokenRepository interface {
	// CreateTx persists a new pricing token within a transaction.
	CreateTx(ctx context.Context, tx db.Tx, token *entity.PricingToken) error

	// GetByToken retrieves a pricing token by its token UUID.
	GetByToken(ctx context.Context, tx db.Tx, token uuid.UUID) (*entity.PricingToken, error)

	// GetByTokenForUpdate retrieves a pricing token with FOR UPDATE lock.
	// This prevents concurrent modifications and must be used within a transaction.
	GetByTokenForUpdate(ctx context.Context, tx db.Tx, token uuid.UUID) (*entity.PricingToken, error)

	// MarkAsUsedTx marks a token as used and links it to an order within a transaction.
	MarkAsUsedTx(ctx context.Context, tx db.Tx, tokenID uuid.UUID, orderID uuid.UUID) error

	// DeleteExpiredTokensTx deletes expired tokens that are older than the specified duration.
	// This is a maintenance operation to clean up old tokens.
	DeleteExpiredTokensTx(ctx context.Context, tx db.Tx, olderThan string) (int64, error)
}


