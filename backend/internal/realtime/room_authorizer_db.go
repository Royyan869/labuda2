package realtime

import (
	"context"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/google/uuid"
	chatentity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// DatabaseRoomAuthorizer provides room authorization using database queries.
//
// Authorization rules:
// - Chat rooms: user must be a participant in the conversation
// - Order rooms: user must be buyer or seller
// - Auction rooms: any authenticated user may join (public)
//
// This is the sole canonical RoomAuthorizer implementation.
// DefaultRoomAuthorizer and CachedRoomAuthorizer have been deleted:
// - DefaultRoomAuthorizer was always-deny stubs with no production value.
// - CachedRoomAuthorizer had no TTL and could serve stale membership after
//   participant changes, making it unsafe for governance decisions.
type DatabaseRoomAuthorizer struct {
	txRunner  roomAuthTxRunner
	chatRepo  chatRoomReader
	orderRepo orderRoomReader
	log       *zap.Logger
}

type roomAuthTxRunner interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// chatRoomReader keeps chat-room authorization dependencies minimal and testable.
type chatRoomReader interface {
	GetRoomByID(ctx context.Context, tx interface{}, roomID uuid.UUID) (*chatentity.ChatRoom, error)
}

// orderRoomReader keeps order-room authorization dependencies minimal and testable.
type orderRoomReader interface {
	GetByID(ctx context.Context, tx db.Tx, orderID uuid.UUID) (*orderentity.Order, error)
}

// NewDatabaseRoomAuthorizer creates a new database-backed room authorizer.
func NewDatabaseRoomAuthorizer(
	txRunner roomAuthTxRunner,
	chatRepo chatRoomReader,
	orderRepo orderRoomReader,
	log *zap.Logger,
) *DatabaseRoomAuthorizer {
	if log == nil {
		log = zap.NewNop()
	}
	return &DatabaseRoomAuthorizer{
		txRunner:  txRunner,
		chatRepo:  chatRepo,
		orderRepo: orderRepo,
		log:       log,
	}
}

// CanSubscribeToRoom checks if a user can subscribe to a room.
// Returns false (deny) on any error — fail-closed per governance-constitution.md §5.
func (a *DatabaseRoomAuthorizer) CanSubscribeToRoom(
	ctx context.Context,
	userID uuid.UUID,
	roomID uuid.UUID,
	roomType RoomType,
) bool {
	switch roomType {
	case RoomTypeAuction:
		return true

	case RoomTypeChat:
		return a.canSubscribeToChatRoom(ctx, userID, roomID)

	case RoomTypeOrder:
		return a.canSubscribeToOrderRoom(ctx, userID, roomID)

	default:
		a.log.Warn("Unknown room type, denying subscribe",
			zap.String("room_type", string(roomType)),
			zap.String("room_id", roomID.String()),
		)
		return false
	}
}

func (a *DatabaseRoomAuthorizer) canSubscribeToChatRoom(
	ctx context.Context,
	userID uuid.UUID,
	roomID uuid.UUID,
) bool {
	var room *chatentity.ChatRoom
	err := a.txRunner.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		room, lookupErr = a.chatRepo.GetRoomByID(ctx, tx, roomID)
		return lookupErr
	})
	if err != nil {
		a.log.Debug("Chat room lookup failed during subscribe authorization",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return false
	}

	hasAccess := room.HasParticipant(userID)
	if !hasAccess {
		a.log.Warn("Chat room subscribe denied — user is not a participant",
			zap.String("room_id", roomID.String()),
			zap.String("user_id", userID.String()),
		)
	}
	return hasAccess
}

func (a *DatabaseRoomAuthorizer) canSubscribeToOrderRoom(
	ctx context.Context,
	userID uuid.UUID,
	roomID uuid.UUID,
) bool {
	var order *orderentity.Order
	err := a.txRunner.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		order, lookupErr = a.orderRepo.GetByID(ctx, tx, roomID)
		return lookupErr
	})
	if err != nil {
		a.log.Debug("Order lookup failed during subscribe authorization",
			zap.String("order_id", roomID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return false
	}

	hasAccess := order.BuyerID == userID || order.SellerID == userID
	if !hasAccess {
		a.log.Warn("Order room subscribe denied — user is not buyer or seller",
			zap.String("order_id", roomID.String()),
			zap.String("user_id", userID.String()),
		)
	}
	return hasAccess
}


