package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	negotiationentity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// fakeNegotiationRepo tracks FOR UPDATE calls and returns canned session.
type fakeNegotiationRepo struct {
	negotiationRepo.Repository
	session              *negotiationentity.NegotiationSession
	getForUpdateCalls    int
	updateSessionCalls   int
	lastUpdatedSessionID uuid.UUID
	getErr               error
}

func (f *fakeNegotiationRepo) GetSession(_ context.Context, _ db.Tx, _ uuid.UUID) (*negotiationentity.NegotiationSession, error) {
	// N4: canonical path must NOT call GetSession for settlement — only GetSessionForUpdate.
	return nil, fmt.Errorf("GetSession must not be called in N4 canonical path")
}

func (f *fakeNegotiationRepo) GetSessionForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*negotiationentity.NegotiationSession, error) {
	f.getForUpdateCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	// return a copy so caller mutation does not affect canned value for next call unless we update
	if f.session == nil {
		return nil, fmt.Errorf("negotiation session not found: %s", uuid.New())
	}
	// shallow copy
	cp := *f.session
	// copy pointers
	if f.session.OrderID != nil {
		oid := *f.session.OrderID
		cp.OrderID = &oid
	}
	if f.session.AcceptedPrice != nil {
		ap := *f.session.AcceptedPrice
		cp.AcceptedPrice = &ap
	}
	if f.session.CurrentPrice != nil {
		cur := *f.session.CurrentPrice
		cp.CurrentPrice = &cur
	}
	if f.session.ExpiresAt != nil {
		exp := *f.session.ExpiresAt
		cp.ExpiresAt = &exp
	}
	if f.session.AcceptedAt != nil {
		aa := *f.session.AcceptedAt
		cp.AcceptedAt = &aa
	}
	return &cp, nil
}

func (f *fakeNegotiationRepo) UpdateSession(_ context.Context, _ db.Tx, session *negotiationentity.NegotiationSession) error {
	f.updateSessionCalls++
	f.lastUpdatedSessionID = session.ID
	// persist OrderID back to canned session to simulate DB write for second call
	if session.OrderID != nil {
		oid := *session.OrderID
		f.session.OrderID = &oid
	}
	return nil
}

func newNegotiationLockFixtures(t *testing.T, session *negotiationentity.NegotiationSession) (*OrderCreationService, CreateFromSaleSurfaceInput, *happyPathOrderRepo, *fakeNegotiationRepo) {
	t.Helper()
	svc, input, orderRepo, _ := newHappyPathFixtures(t)
	fakeNegRepo := &fakeNegotiationRepo{session: session}
	svc.negotiationRepo = fakeNegRepo
	// wire input to match session
	forSale, _ := svc.forSaleRepo.(*happyPathForSaleRepo)
	if session != nil {
		forSale.forSale.ID = session.ForSaleID
	}
	input.ProductID = forSale.forSale.ProductID
	input.SourceID = forSale.forSale.ID
	if session != nil {
		input.NegotiationID = &session.ID
		input.BuyerID = session.BuyerID
		// N8-B: snapshot must carry same negotiation identity as request
		if input.PricingSnapshot != nil {
			input.PricingSnapshot.NegotiationID = &session.ID
			input.PricingSnapshot.UnitPrice = money.New(*session.AcceptedPrice)
			input.PricingSnapshot.Subtotal = money.New(*session.AcceptedPrice)
		}
	}
	return svc, input, orderRepo, fakeNegRepo
}

func acceptedNegotiationSession(buyerID, sellerID, forSaleID uuid.UUID) *negotiationentity.NegotiationSession {
	now := time.Now()
	exp := now.Add(24 * time.Hour)
	price := int64(5_000_000)
	acceptedAt := now
	return &negotiationentity.NegotiationSession{
		ID:               uuid.New(),
		ResourceType:     negotiationentity.NegotiationResourceForSale,
		ForSaleID:        forSaleID,
		BuyerID:          buyerID,
		SellerID:         sellerID,
		Status:           negotiationentity.NegotiationStatusAccepted,
		CurrentPrice:     &price,
		AcceptedPrice:    &price,
		AcceptedAt:       &acceptedAt,
		ExpiresAt:        &exp,
		OrderID:          nil,
		ProposalSequence: 1,
		CreatedAt:        now.Add(-1 * time.Hour),
		UpdatedAt:        now,
	}
}

func TestNegotiationSettlement_AlreadySettledFailsBeforeOrderCreation(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	oid := uuid.New()
	session.OrderID = &oid // already settled

	svc, input, orderRepo, fakeRepo := newNegotiationLockFixtures(t, session)
	// fix forSale product to match
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		session.ForSaleID = forSaleID
		input.ProductID = productID
	}

	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already settled")
	require.Equal(t, 0, orderRepo.createOrderCalls, "order must not be created for already-settled negotiation")
	require.Equal(t, 1, fakeRepo.getForUpdateCalls, "must use GetSessionForUpdate (FOR UPDATE) for settlement check")
	require.Equal(t, 0, fakeRepo.updateSessionCalls)
}

func TestNegotiationSettlement_ExpiredRejectedAfterLock(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	exp := time.Now().Add(-1 * time.Hour) // expired
	session.ExpiresAt = &exp

	svc, input, orderRepo, fakeRepo := newNegotiationLockFixtures(t, session)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}

	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
	require.Equal(t, 0, orderRepo.createOrderCalls)
	require.Equal(t, 1, fakeRepo.getForUpdateCalls, "expiry must be validated after FOR UPDATE lock")
}

func TestNegotiationSettlement_ValidCreatesOrderAndMarksSettled(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)

	svc, input, orderRepo, fakeRepo := newNegotiationLockFixtures(t, session)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}

	order, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, 1, orderRepo.createOrderCalls)
	require.Equal(t, 1, fakeRepo.getForUpdateCalls, "canonical path must lock negotiation exactly once")
	require.Equal(t, 1, fakeRepo.updateSessionCalls, "must persist order_id on same locked row, no second SELECT FOR UPDATE")
	require.NotNil(t, fakeRepo.session.OrderID)
	require.Equal(t, order.ID, *fakeRepo.session.OrderID)
	// Price authority from locked canonical row
	require.Equal(t, int64(5_000_000), order.UnitPrice.Int64())
}

func TestNegotiationSettlement_SecondAttemptFailsViaAlreadySettledNotUniqueViolation(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)

	svc, input, orderRepo, fakeRepo := newNegotiationLockFixtures(t, session)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}

	// first succeeds
	order1, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.NoError(t, err)
	require.NotNil(t, order1)
	require.Equal(t, 1, orderRepo.createOrderCalls)

	// second attempt with same negotiation (simulates concurrent Tx2 after Tx1 commit: OrderID now set)
	// Reset orderRepo create count to detect second insert
	orderRepo.createOrderCalls = 0
	// Need fresh service input but same negotiation repo (now has OrderID set)
	input2 := input
	// Use same pricing token? Need different token to avoid token unique, but still same negotiation — create new snapshot token id
	input2.PricingSnapshot = &PricingSnapshot{
		UnitPrice:          money.New(5_000_000),
		Subtotal:           money.New(5_000_000),
		ShippingTotal:      money.New(15_000),
		CommissionPercent:  5,
		CommissionAmount:   money.New(250_000),
		EscrowAmount:       money.New(5_015_000),
		ServiceFeeAmount:   money.New(3_000),
		TotalPayableAmount: money.New(5_018_000),
		ShippingSetupName:  "JNE",
		NegotiationID:      input.NegotiationID,
		TokenID:            uuid.New(), // different token
		PaymentMethod:      PaymentMethodInstant,
	}
	_, err = svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already settled")
	require.Equal(t, 0, orderRepo.createOrderCalls, "second order must not be inserted; must fail via application already-settled check, not DB unique violation")
	require.Equal(t, 2, fakeRepo.getForUpdateCalls, "second attempt must have re-locked and seen OrderID != nil")
}
