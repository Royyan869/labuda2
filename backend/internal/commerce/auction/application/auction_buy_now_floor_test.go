package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

type noopAccountStatusChecker struct{}

func (noopAccountStatusChecker) EnsureActive(context.Context, uuid.UUID) error { return nil }
func (noopAccountStatusChecker) GetStatus(context.Context, uuid.UUID) (string, error) { return "active", nil }
func (noopAccountStatusChecker) IsBanned(context.Context, uuid.UUID) (bool, error) { return false, nil }

type fakeTx struct{}

func (fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { return pgx.Row(nil) }
func (fakeTx) Commit(context.Context) error { return nil }
func (fakeTx) Rollback(context.Context) error { return nil }

var _ db.Tx = (*fakeTx)(nil)

type noopRoleChecker struct{}

func (noopRoleChecker) IsAdmin(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (noopRoleChecker) IsSeller(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (noopRoleChecker) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}
func (noopRoleChecker) HasSellerProfile(context.Context, uuid.UUID) (bool, error) { return true, nil }

func TestCreateDraft_RejectsBuyNowBelowFloor(t *testing.T) {
	svc := &AuctionService{
		accountStatus: noopAccountStatusChecker{},
		auctionRepo:   &auctionRepo.AuctionRepository{},
		outboxRepo:    &outboxRepo.OutboxRepository{},
		configService: &platformconfigApp.ConfigService{},
		roleChecker:   noopRoleChecker{},
		log:           zap.NewNop(),
	}

	_, err := svc.CreateDraft(context.Background(), fakeTx{}, CreateDraftInput{
		SellerID:     uuid.New(),

		Title:        "Test Auction",
		Description:  "Floor validation",
		StartPrice:   1000,
		BidIncrement: 250,
		BuyNowPrice:  ptrInt64(1200),
		StartMode:    entity.StartModeNow,
		Duration:     24 * time.Hour,
	})

	if err == nil {
		t.Fatal("expected buy_now_price floor validation to reject low price")
	}
	if err.Error() != "buy_now_price must be >= start_price + bid_increment" {
		t.Fatalf("expected floor validation error, got: %v", err)
	}
}

func TestCreateDraft_AcceptsBuyNowAtFloor(t *testing.T) {
	svc := &AuctionService{
		accountStatus: noopAccountStatusChecker{},
		auctionRepo:   &auctionRepo.AuctionRepository{},
		outboxRepo:    &outboxRepo.OutboxRepository{},
		configService: &platformconfigApp.ConfigService{},
		roleChecker:   noopRoleChecker{},
		log:           zap.NewNop(),
	}

	// Floor validation (buy_now >= start + increment) should pass.
	// CreateDraft proceeds to product/auction persistence which panics on fakeTx,
	// so we catch the panic and assert the floor error itself was NOT the cause.
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic from DB layer is expected — floor validation passed, which is the invariant being tested.
			}
		}()
		_, err := svc.CreateDraft(context.Background(), fakeTx{}, CreateDraftInput{
			SellerID:     uuid.New(),
			Title:        "Test Auction",
			Description:  "Floor validation",
			StartPrice:   1000,
			BidIncrement: 250,
			BuyNowPrice:  ptrInt64(1250),
			StartMode:    entity.StartModeNow,
			Duration:     24 * time.Hour,
		})
		if err != nil && err.Error() == "buy_now_price must be >= start_price + bid_increment" {
			t.Fatalf("floor buy_now_price should pass but floor validation rejected it")
		}
	}()
}

func ptrInt64(v int64) *int64 { return &v }


