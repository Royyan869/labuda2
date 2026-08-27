package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
	refundrepo "github.com/labuda/backend/internal/finance/refund/repository"
	"github.com/labuda/backend/pkg/db"
)

type refundHistoryRepoStub struct {
	calls []refundHistoryCall
	page  map[string][]*entity.Refund
}

type refundHistoryCall struct {
	orderID uuid.UUID
	limit   int
	cursor  *refundrepo.OrderRefundCursor
}

func (r *refundHistoryRepoStub) Create(context.Context, db.Tx, *entity.Refund) error {
	return nil
}

func (r *refundHistoryRepoStub) GetByID(context.Context, db.Tx, uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) GetByOrderID(context.Context, db.Tx, uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) GetForUpdate(context.Context, db.Tx, uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) Update(context.Context, db.Tx, *entity.Refund) error {
	return nil
}

func (r *refundHistoryRepoStub) ListByBuyer(context.Context, db.Tx, uuid.UUID, int, int64) ([]*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) ListBySeller(context.Context, db.Tx, uuid.UUID, int, int64) ([]*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) ListByOrderID(
	_ context.Context,
	_ db.Tx,
	orderID uuid.UUID,
	limit int,
	cursor *refundrepo.OrderRefundCursor,
) ([]*entity.Refund, error) {
	r.calls = append(r.calls, refundHistoryCall{
		orderID: orderID,
		limit:   limit,
		cursor:  cursor,
	})

	key := "initial"
	if cursor != nil {
		key = cursor.Encode()
	}
	return r.page[key], nil
}

func (r *refundHistoryRepoStub) GetByGatewayIdempotencyKey(context.Context, db.Tx, string) (*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) GetByGatewayRefundID(context.Context, db.Tx, string) (*entity.Refund, error) {
	return nil, nil
}

func (r *refundHistoryRepoStub) GetSuccessfulRefundTotalByOrder(context.Context, db.Tx, uuid.UUID, *uuid.UUID) (int64, error) {
	return 0, nil
}

func (r *refundHistoryRepoStub) HasActiveRefundByOrderID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *refundHistoryRepoStub) CreateEvidence(context.Context, db.Tx, uuid.UUID, string) error {
	return nil
}

func (r *refundHistoryRepoStub) ListEvidence(context.Context, db.Tx, uuid.UUID) ([]string, error) {
	return nil, nil
}
func (r *refundHistoryRepoStub) GetCumulativeProductRefundByOrder(context.Context, db.Tx, uuid.UUID, *uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *refundHistoryRepoStub) GetCumulativeShippingRefundByOrder(context.Context, db.Tx, uuid.UUID, *uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *refundHistoryRepoStub) GetCumulativeCoinsRefundedByOrder(context.Context, db.Tx, uuid.UUID, *uuid.UUID) (int64, error) {
	return 0, nil
}

var _ refundrepo.RefundRepository = (*refundHistoryRepoStub)(nil)

func refundHistoryFixture(
	id string,
	createdAt time.Time,
	status entity.RefundStatus,
) *entity.Refund {
	parsed := uuid.MustParse(id)
	orderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	buyerID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sellerID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	return &entity.Refund{
		ID:              parsed,
		OrderID:         orderID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		Reason:          entity.RefundReasonItemNotReceived,
		Status:          status,
		RequestedAmount: 100000,
		OpenedAt:        createdAt,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
}

func TestListRefundHistoryByOrderID_PaginatesNewestFirstAndUsesCursor(t *testing.T) {
	ctx := context.Background()
	orderID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	newest := refundHistoryFixture(
		"44444444-4444-4444-4444-444444444444",
		time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		entity.RefundStatusRefunded,
	)
	middle := refundHistoryFixture(
		"55555555-5555-5555-5555-555555555555",
		time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC),
		entity.RefundStatusSellerRejected,
	)
	oldest := refundHistoryFixture(
		"66666666-6666-6666-6666-666666666666",
		time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		entity.RefundStatusPendingSellerReview,
	)

	repo := &refundHistoryRepoStub{
		page: map[string][]*entity.Refund{
			"initial": {newest, middle, oldest},
		},
	}

	service := &RefundService{refundRepo: repo}

	page, err := service.ListRefundHistoryByOrderID(ctx, nil, orderID, 2, nil)
	if err != nil {
		t.Fatalf("ListRefundHistoryByOrderID() error = %v", err)
	}
	if got, want := len(page.Items), 2; got != want {
		t.Fatalf("initial page len = %d want %d", got, want)
	}
	if page.Items[0].ID != newest.ID || page.Items[1].ID != middle.ID {
		t.Fatalf("unexpected order: got %s then %s", page.Items[0].ID, page.Items[1].ID)
	}
	if !page.HasMore {
		t.Fatalf("expected HasMore=true on initial page")
	}
	if page.NextCursor == nil {
		t.Fatalf("expected next cursor on truncated page")
	}
	if got, want := repo.calls[0].orderID, orderID; got != want {
		t.Fatalf("orderID = %s want %s", got, want)
	}
	if got, want := repo.calls[0].limit, 3; got != want {
		t.Fatalf("limit = %d want %d", got, want)
	}
	if repo.calls[0].cursor != nil {
		t.Fatalf("expected nil cursor on first page")
	}

	repo.page[page.NextCursor.Encode()] = []*entity.Refund{oldest}

	nextPage, err := service.ListRefundHistoryByOrderID(ctx, nil, orderID, 2, page.NextCursor)
	if err != nil {
		t.Fatalf("second page error = %v", err)
	}
	if got, want := len(nextPage.Items), 1; got != want {
		t.Fatalf("second page len = %d want %d", got, want)
	}
	if nextPage.Items[0].ID != oldest.ID {
		t.Fatalf("unexpected second page id = %s want %s", nextPage.Items[0].ID, oldest.ID)
	}
	if nextPage.HasMore {
		t.Fatalf("expected HasMore=false on final page")
	}
	if nextPage.NextCursor != nil {
		t.Fatalf("expected no next cursor on final page")
	}
	if got, want := repo.calls[1].limit, 3; got != want {
		t.Fatalf("second call limit = %d want %d", got, want)
	}
	if repo.calls[1].cursor == nil || repo.calls[1].cursor.Encode() != page.NextCursor.Encode() {
		t.Fatalf("second call cursor mismatch")
	}
}

func TestListRefundHistoryByOrderID_ReflectsRefundMutations(t *testing.T) {
	ctx := context.Background()
	orderID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	refund := refundHistoryFixture(
		"77777777-7777-7777-7777-777777777777",
		time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		entity.RefundStatusPendingSellerReview,
	)
	repo := &refundHistoryRepoStub{
		page: map[string][]*entity.Refund{
			"initial": {refund},
		},
	}

	service := &RefundService{refundRepo: repo}

	page, err := service.ListRefundHistoryByOrderID(ctx, nil, orderID, 20, nil)
	if err != nil {
		t.Fatalf("initial list error = %v", err)
	}
	if page.Items[0].Status != entity.RefundStatusPendingSellerReview {
		t.Fatalf("initial status = %s", page.Items[0].Status)
	}

	now := time.Date(2026, 7, 1, 9, 30, 0, 0, time.UTC)
	if err := refund.SellerApprove(75000, nil, now); err != nil {
		t.Fatalf("SellerApprove() error = %v", err)
	}
	page, err = service.ListRefundHistoryByOrderID(ctx, nil, orderID, 20, nil)
	if err != nil {
		t.Fatalf("post-approve list error = %v", err)
	}
	if page.Items[0].Status != entity.RefundStatusSellerApproved {
		t.Fatalf("approve mutation not reflected, got %s", page.Items[0].Status)
	}

	if err := refund.SellerReject(nil, now.Add(time.Minute)); err == nil {
		t.Fatalf("expected approve -> reject transition to fail")
	}

	rejectRefund := refundHistoryFixture(
		"88888888-8888-8888-8888-888888888888",
		time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		entity.RefundStatusPendingSellerReview,
	)
	repo.page["initial"] = []*entity.Refund{rejectRefund}
	if err := rejectRefund.SellerReject(nil, now); err != nil {
		t.Fatalf("SellerReject() error = %v", err)
	}
	page, err = service.ListRefundHistoryByOrderID(ctx, nil, orderID, 20, nil)
	if err != nil {
		t.Fatalf("post-reject list error = %v", err)
	}
	if page.Items[0].Status != entity.RefundStatusSellerRejected {
		t.Fatalf("reject mutation not reflected, got %s", page.Items[0].Status)
	}

	if err := rejectRefund.EscalateToAdmin(now.Add(time.Minute)); err != nil {
		t.Fatalf("EscalateToAdmin() error = %v", err)
	}
	page, err = service.ListRefundHistoryByOrderID(ctx, nil, orderID, 20, nil)
	if err != nil {
		t.Fatalf("post-escalation list error = %v", err)
	}
	if page.Items[0].Status != entity.RefundStatusEscalatedToAdmin {
		t.Fatalf("escalation mutation not reflected, got %s", page.Items[0].Status)
	}
}
