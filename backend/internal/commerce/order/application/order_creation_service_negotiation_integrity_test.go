package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/money"
)

// Test N8-B settlement binding matrix at the canonical OrderCreationService boundary.

func TestNegotiationIntegrity_DirectTokenWithNilNegotiationAllowed(t *testing.T) {
	svc, input, orderRepo, _ := newHappyPathFixtures(t)
	// direct flow: both nil by default from fixture
	order, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Nil(t, order.NegotiationID)
	require.Equal(t, 1, orderRepo.createOrderCalls)
}

func TestNegotiationIntegrity_NegotiationTokenMatchingAllowed(t *testing.T) {
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
	require.NotNil(t, order.NegotiationID)
	require.Equal(t, session.ID, *order.NegotiationID)
	require.Equal(t, 1, orderRepo.createOrderCalls)
	require.Equal(t, 1, fakeRepo.updateSessionCalls)
	require.Equal(t, session.ID, fakeRepo.lastUpdatedSessionID)
	require.NotNil(t, fakeRepo.session.OrderID)
	require.Equal(t, order.ID, *fakeRepo.session.OrderID)
}

func TestNegotiationIntegrity_NegotiationTokenMissingRequestRejected(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	svc, input, orderRepo, _ := newNegotiationLockFixtures(t, session)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}
	// C: token has negotiation, request missing
	input.NegotiationID = nil
	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negotiation binding mismatch")
	require.Equal(t, 0, orderRepo.createOrderCalls)
}

func TestNegotiationIntegrity_DirectTokenWithRequestRejected(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	// start from direct fixture (no negotiation) but inject request negotiation
	svc, input, orderRepo, _ := newHappyPathFixtures(t)
	// override forSale to match session's forSale/product so ownership check would pass if binding didn't fail first
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
		input.SourceID = forSaleID
		_ = buyerID
		_ = session
	}
	// D: direct token (snapshot NegotiationID nil) + request negotiation ID present
	negID := session.ID
	input.NegotiationID = &negID
	// snapshot remains direct (nil) from happy fixture
	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negotiation binding mismatch")
	require.Equal(t, 0, orderRepo.createOrderCalls)
}

func TestNegotiationIntegrity_TokenARequestBRejected(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	sessionA := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	sessionB := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	svc, input, orderRepo, fakeRepo := newNegotiationLockFixtures(t, sessionA)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}
	// E: token A (from fixture), request B
	input.NegotiationID = &sessionB.ID
	// also need to make fake repo return sessionB when locked, to test token vs request before lock
	// but our canonical check token vs request fails before lock, so it will be rejected without needing sessionB
	// Keep fake repo returning sessionA so mismatch is token(A) vs request(B)
	fakeRepo.session = sessionA
	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negotiation binding mismatch")
	require.Equal(t, 0, orderRepo.createOrderCalls)
}

func TestNegotiationIntegrity_PriceMismatchRejected(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()
	productID := uuid.New()
	session := acceptedNegotiationSession(buyerID, sellerID, forSaleID)
	svc, input, orderRepo, _ := newNegotiationLockFixtures(t, session)
	if fs, ok := svc.forSaleRepo.(*happyPathForSaleRepo); ok {
		fs.forSale.ProductID = productID
		fs.forSale.SellerID = sellerID
		fs.forSale.ID = forSaleID
		input.ProductID = productID
	}
	// G: token unit price != accepted_price
	input.PricingSnapshot.UnitPrice = money.New(9_999_999)
	input.PricingSnapshot.Subtotal = money.New(9_999_999)
	_, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negotiation price mismatch")
	require.Equal(t, 0, orderRepo.createOrderCalls)
}

func TestNegotiationIntegrity_ValidSettlementWritesOrderIDDoesNotChangeStatus(t *testing.T) {
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
	originalStatus := session.Status
	order, err := svc.CreateFromSaleSurface(context.Background(), happyPathTx{}, input)
	require.NoError(t, err)
	require.NotNil(t, order)
	// H: exactly one order, same order_id on locked session, status unchanged
	require.Equal(t, 1, orderRepo.createOrderCalls)
	require.NotNil(t, fakeRepo.session.OrderID)
	require.Equal(t, order.ID, *fakeRepo.session.OrderID)
	require.Equal(t, originalStatus, fakeRepo.session.Status, "settlement must not change negotiation status")
	require.NotNil(t, order.NegotiationID)
	require.Equal(t, session.ID, *order.NegotiationID)
}
