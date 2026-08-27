package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
)

type resolveFinalityTx struct {
	row resolveFinalityRow
}

func (t *resolveFinalityTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unexpected Exec call in resolve finality test")
}

func (t *resolveFinalityTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query call in resolve finality test")
}

func (t *resolveFinalityTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return t.row
}

func (t *resolveFinalityTx) Commit(context.Context) error {
	return nil
}

func (t *resolveFinalityTx) Rollback(context.Context) error {
	return nil
}

var _ db.Tx = (*resolveFinalityTx)(nil)

type resolveFinalityRow struct{}

func (resolveFinalityRow) Scan(dest ...any) error {
	for i, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = uuid.Nil
		case *string:
			switch i {
			case 14: // status; shifted from 12 → 14 by service_fee_amount+total_payable_amount
				*v = string(orderEntity.StatusCompleted)
			case 15: // escrow_status
				*v = string(orderEntity.EscrowStatusReleased)
			default:
				*v = ""
			}
		case *sql.NullString:
			*v = sql.NullString{}
		case *sql.NullInt64:
			*v = sql.NullInt64{}
		case *sql.NullTime:
			*v = sql.NullTime{}
		case *int:
			*v = 0
		case *int64:
			*v = 0
		case *bool:
			*v = false
		case *[]byte:
			*v = nil
		case *time.Time:
			*v = time.Time{}
		default:
			return fmt.Errorf("unexpected scan destination %T at index %d", d, i)
		}
	}
	return nil
}

type resolveFinalityDisputeRepo struct {
	dispute      *disputeEntity.Dispute
	updateCalled bool
}

var _ disputeRepo.DisputeRepository = (*resolveFinalityDisputeRepo)(nil)

func (m *resolveFinalityDisputeRepo) Create(context.Context, db.Tx, *disputeEntity.Dispute) error {
	return nil
}

func (m *resolveFinalityDisputeRepo) GetByOrderID(context.Context, db.Tx, uuid.UUID) (*disputeEntity.Dispute, error) {
	return m.dispute, nil
}

func (m *resolveFinalityDisputeRepo) GetForUpdate(context.Context, db.Tx, uuid.UUID) (*disputeEntity.Dispute, error) {
	return m.dispute, nil
}

func (m *resolveFinalityDisputeRepo) Update(context.Context, db.Tx, *disputeEntity.Dispute) error {
	m.updateCalled = true
	return nil
}

func (m *resolveFinalityDisputeRepo) CreateMedia(context.Context, db.Tx, uuid.UUID, string) error {
	return nil
}

func (m *resolveFinalityDisputeRepo) ListMedia(context.Context, db.Tx, uuid.UUID) ([]string, error) {
	return nil, nil
}

func (m *resolveFinalityDisputeRepo) ListAll(context.Context, db.Tx, disputeRepo.DisputeListFilters) ([]*disputeEntity.Dispute, int64, error) {
	return nil, 0, nil
}

func (m *resolveFinalityDisputeRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*disputeEntity.Dispute, error) {
	return m.dispute, nil
}

func (m *resolveFinalityDisputeRepo) FindOverdueCandidates(context.Context, db.Tx, int) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *resolveFinalityDisputeRepo) FindTimeoutCandidates(context.Context, db.Tx, int) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *resolveFinalityDisputeRepo) GetCallerDisputeCount(context.Context, db.Tx, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}

func (m *resolveFinalityDisputeRepo) GetCallerDisputeCountAgainstParty(context.Context, db.Tx, uuid.UUID, uuid.UUID, time.Time) (int, error) {
	return 0, nil
}

func TestResolveDispute_BlocksCompletedReleasedForSystemCaller(t *testing.T) {
	orderID := uuid.New()
	disputeID := uuid.New()
	dispute := &disputeEntity.Dispute{
		ID:       disputeID,
		OrderID:  orderID,
		BuyerID:  uuid.New(),
		SellerID: uuid.New(),
		Status:   disputeEntity.DisputeStatusUnderReview,
	}

	svc := &DisputeService{
		disputeRepo: &resolveFinalityDisputeRepo{dispute: dispute},
		orderRepo:   &orderRepo.OrderRepository{},
	}

	err := svc.ResolveDispute(context.Background(), &resolveFinalityTx{}, disputeID, ResolutionRefund, auth.SystemCallerID, nil)
	if err == nil {
		t.Fatal("expected completed+released dispute resolution to be blocked, got nil")
	}
	if !errors.Is(err, ErrDisputeResolveAfterCompletion) {
		t.Fatalf("unexpected error: %s", err)
	}
	if svc.disputeRepo.(*resolveFinalityDisputeRepo).updateCalled {
		t.Fatal("dispute update should not run after finality rejection")
	}
}
