package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	chatentity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type fakeTxRunner struct{}

func (f *fakeTxRunner) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&fakeTx{})
}

type fakeTx struct{}

func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error)         { return nil, errors.New("not implemented") }
func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row                 { return nil }
func (f *fakeTx) Commit(context.Context) error                                      { return nil }
func (f *fakeTx) Rollback(context.Context) error                                    { return nil }

type fakeChatRoomReader struct {
	room   *chatentity.ChatRoom
	err    error
	sawNil bool
}

func (f *fakeChatRoomReader) GetRoomByID(_ context.Context, tx interface{}, _ uuid.UUID) (*chatentity.ChatRoom, error) {
	if tx == nil {
		f.sawNil = true
		return nil, errors.New("nil tx")
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.room, nil
}

type fakeOrderRoomReader struct{}

func (f *fakeOrderRoomReader) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*orderentity.Order, error) {
	return nil, errors.New("unused in these tests")
}

func TestDatabaseRoomAuthorizer_SubscribeDoesNotPanic_AndAllowsParticipant(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()
	room := chatentity.NewChatRoom(chatentity.RoomTypeDirect, userA, userB)
	reader := &fakeChatRoomReader{room: room}
	authz := NewDatabaseRoomAuthorizer(&fakeTxRunner{}, reader, &fakeOrderRoomReader{}, zap.NewNop())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CanSubscribeToRoom panicked: %v", r)
		}
	}()

	allowed := authz.CanSubscribeToRoom(context.Background(), userA, room.ID, RoomTypeChat)
	if !allowed {
		t.Fatal("participant should be allowed")
	}
	if reader.sawNil {
		t.Fatal("chat room lookup received nil tx; expected tx from WithTx")
	}
}

func TestDatabaseRoomAuthorizer_DeniesNonParticipant(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()
	outsider := uuid.New()
	room := chatentity.NewChatRoom(chatentity.RoomTypeDirect, userA, userB)
	reader := &fakeChatRoomReader{room: room}
	authz := NewDatabaseRoomAuthorizer(&fakeTxRunner{}, reader, &fakeOrderRoomReader{}, zap.NewNop())

	allowed := authz.CanSubscribeToRoom(context.Background(), outsider, room.ID, RoomTypeChat)
	if allowed {
		t.Fatal("non-participant should be denied")
	}
	if reader.sawNil {
		t.Fatal("chat room lookup received nil tx; expected tx from WithTx")
	}
}

func TestDatabaseRoomAuthorizer_MissingRoomDenied_NoPanic(t *testing.T) {
	userID := uuid.New()
	reader := &fakeChatRoomReader{err: errors.New("room not found")}
	authz := NewDatabaseRoomAuthorizer(&fakeTxRunner{}, reader, &fakeOrderRoomReader{}, zap.NewNop())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CanSubscribeToRoom panicked: %v", r)
		}
	}()

	allowed := authz.CanSubscribeToRoom(context.Background(), userID, uuid.New(), RoomTypeChat)
	if allowed {
		t.Fatal("missing room should be denied")
	}
	if reader.sawNil {
		t.Fatal("chat room lookup received nil tx; expected tx from WithTx")
	}
}

func TestDatabaseRoomAuthorizer_MalformedRoomTypeDenied(t *testing.T) {
	authz := NewDatabaseRoomAuthorizer(&fakeTxRunner{}, &fakeChatRoomReader{}, &fakeOrderRoomReader{}, zap.NewNop())
	allowed := authz.CanSubscribeToRoom(context.Background(), uuid.New(), uuid.New(), RoomType("bogus"))
	if allowed {
		t.Fatal("unknown room type should be denied")
	}
}

func TestDatabaseRoomAuthorizer_DoesNotUseNilTxPathOverTime(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()
	room := chatentity.NewChatRoom(chatentity.RoomTypeDirect, userA, userB)
	reader := &fakeChatRoomReader{room: room}
	authz := NewDatabaseRoomAuthorizer(&fakeTxRunner{}, reader, &fakeOrderRoomReader{}, zap.NewNop())

	for i := 0; i < 5; i++ {
		allowed := authz.CanSubscribeToRoom(context.Background(), userA, room.ID, RoomTypeChat)
		if !allowed {
			t.Fatalf("iteration %d: participant should be allowed", i)
		}
		time.Sleep(1 * time.Millisecond)
	}
	if reader.sawNil {
		t.Fatal("nil tx path detected")
	}
}



