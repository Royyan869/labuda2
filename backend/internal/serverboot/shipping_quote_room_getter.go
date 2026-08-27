package serverboot

import (
	"context"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/pkg/db"
)

// shippingQuoteRoomGetterAdapter bridges the shipping-quote layer's db.Tx-based
// lock-aware lookup interface to the chat repository implementation, which
// still accepts the broader transaction abstraction used elsewhere.
type shippingQuoteRoomGetterAdapter struct {
	repo chatRepo.Repository
}

func (a *shippingQuoteRoomGetterAdapter) GetRoomByID(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByID(ctx, tx, roomID)
}

func (a *shippingQuoteRoomGetterAdapter) GetRoomByIDForUpdate(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByIDForUpdate(ctx, tx, roomID)
}
