package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/midtrans"
)

type refundGatewayClientMock struct {
	called bool
}

func (m *refundGatewayClientMock) RefundWithKey(context.Context, string, string, int64, string) (*midtrans.RefundResponse, error) {
	m.called = true
	return nil, nil
}

func TestInitiateGatewayRefund_RequiresExplicitCaller(t *testing.T) {
	repo := newMockRefundRepo()
	client := &refundGatewayClientMock{}
	svc := &RefundService{
		refundRepo:    repo,
		gatewayClient: client,
	}

	baseInput := InitiateGatewayRefundInput{
		RefundID:       uuid.New(),
		Amount:         100,
		Reason:         "other",
		IdempotencyKey: "idem-1",
	}

	cases := []struct {
		name  string
		input InitiateGatewayRefundInput
	}{
		{"missing caller", baseInput},
		{"system marker mismatch", func() InitiateGatewayRefundInput {
			in := baseInput
			in.CallerID = uuid.New()
			in.CallerType = GatewayRefundCallerTypeSystem
			return in
		}()},
		{"admin with system caller", func() InitiateGatewayRefundInput {
			in := baseInput
			in.CallerID = auth.SystemCallerID
			in.CallerType = GatewayRefundCallerTypeAdmin
			return in
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.InitiateGatewayRefund(context.Background(), noopTx{}, tc.input)
			if err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
			var callerErr *ErrGatewayRefundCallerProvenanceRequired
			if !errors.As(err, &callerErr) {
				t.Fatalf("expected caller provenance error, got %v", err)
			}
			if client.called {
				t.Fatal("gateway client must not be called on provenance failure")
			}
		})
	}
}

func TestCreateAndDispatchSystemRefund_UsesExplicitSystemCaller(t *testing.T) {
	existing := entity.NewSystemRefund(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		auth.SystemCallerID,
		entity.RefundReason("other"),
		1000,
		0,
		nil,
	)
	now := time.Now()
	_ = existing.MarkGatewayDispatched("idem-system", nil, now)

	repo := newMockRefundRepo()
	repo.refundByIdempotencyKey["idem-system"] = existing
	repo.refundByID[existing.ID] = existing

	svc := &RefundService{
		refundRepo:    repo,
		gatewayClient: &refundGatewayClientMock{},
	}

	refund, err := svc.CreateAndDispatchSystemRefund(
		context.Background(),
		noopTx{},
		SystemRefundInput{
			OrderID:        existing.OrderID,
			BuyerID:        existing.BuyerID,
			SellerID:       existing.SellerID,
			AdminID:        auth.SystemCallerID,
			ProductAmount:  1000,
			ShippingAmount: 0,
			PD:             1000,
			S:              0,
			C:              0,
			K:              0,
			Reason:         entity.RefundReason("other"),
			IdempotencyKey: "idem-system",
		},
	)
	if err != nil {
		t.Fatalf("system refund flow should succeed: %v", err)
	}
	if refund == nil || refund.ID != existing.ID {
		t.Fatal("expected existing refund to be returned")
	}
}

func TestCreateAndDispatchSystemRefundFromApproval_UsesExplicitSystemCaller(t *testing.T) {
	refund := entity.NewSystemRefund(
		uuid.New(),
		uuid.New(),
		uuid.New(),
		auth.SystemCallerID,
		entity.RefundReason("other"),
		1000,
		0,
		nil,
	)
	now := time.Now()
	_ = refund.MarkGatewayDispatched("idem-approval", nil, now)

	repo := newMockRefundRepo()
	repo.refundByIdempotencyKey["idem-approval"] = refund
	repo.refundByID[refund.ID] = refund

	svc := &RefundService{
		refundRepo:    repo,
		gatewayClient: &refundGatewayClientMock{},
	}

	refreshed, err := svc.CreateAndDispatchSystemRefundFromApproval(context.Background(), noopTx{}, refund, 1000, "idem-approval")
	if err != nil {
		t.Fatalf("approval refund flow should succeed: %v", err)
	}
	if refreshed == nil || refreshed.ID != refund.ID {
		t.Fatal("expected approval refund to return existing refund")
	}
}
