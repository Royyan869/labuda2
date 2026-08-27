package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	shippingquoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// expiryTestShippingQuoteRepo is a minimal ShippingQuoteRepository stub
// controlling only GetByIDForUpdate (quote lookup) and UpdateStatus (mark
// used), which is all validateShippingQuoteForOrder needs.
type expiryTestShippingQuoteRepo struct {
	quote *shippingquoteEntity.ShippingQuote
}

func (r *expiryTestShippingQuoteRepo) Create(context.Context, db.Tx, *shippingquoteEntity.ShippingQuote) error {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) GetLatestByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingquoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) GetLatestRevisionByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingquoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*shippingquoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) GetByIDForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*shippingquoteEntity.ShippingQuote, error) {
	return r.quote, nil
}
func (r *expiryTestShippingQuoteRepo) UpdateStatus(context.Context, db.Tx, uuid.UUID, shippingquoteEntity.QuoteStatus, *interface{}) error {
	return nil
}
func (r *expiryTestShippingQuoteRepo) ReactivateQuote(context.Context, db.Tx, uuid.UUID) error {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) GetCurrentActiveByChatAndSource(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID) (*shippingquoteEntity.ShippingQuote, error) {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) SupersedeCurrentQuotes(context.Context, db.Tx, uuid.UUID, uuid.UUID, string, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (int64, error) {
	panic("not exercised")
}
func (r *expiryTestShippingQuoteRepo) InvalidateQuotesByProduct(context.Context, db.Tx, uuid.UUID) error {
	panic("not exercised")
}

// newValidatableShippingQuote builds a shipping quote whose fields all match
// the parameters validateShippingQuoteForOrder is called with in the tests
// below, so the ONLY thing under test is the expiry check (STEP 3).
func newValidatableShippingQuote(
	chatID, productID, sourceID, sellerID, buyerID uuid.UUID,
	provinceID, cityID string,
	expiresAt *time.Time,
) *shippingquoteEntity.ShippingQuote {
	sourceType := "for_sale"
	return &shippingquoteEntity.ShippingQuote{
		ID:                    uuid.New(),
		ChatID:                chatID,
		ProductID:             productID,
		SourceType:            &sourceType,
		SourceID:              &sourceID,
		SellerID:              sellerID,
		BuyerID:               buyerID,
		Status:                shippingquoteEntity.QuoteStatusActive,
		DestinationProvinceID: &provinceID,
		DestinationCityID:     &cityID,
		ExpiresAt:             expiresAt,
	}
}

// TestValidateShippingQuoteForOrder_RejectsExpiredQuote is the PASS_18P
// regression proof that checkout validation actually rejects an expired
// shipping quote. Before this pass, ExpiresAt was always nil in practice (no
// caller ever supplied expires_in_hours), so IsExpired() always returned
// false and this guard was structurally moot — a manually-issued quote could
// be redeemed indefinitely. This test uses an ExpiresAt in the past to prove
// the guard is now reachable and effective.
func TestValidateShippingQuoteForOrder_RejectsExpiredQuote(t *testing.T) {
	chatID := uuid.New()
	productID := uuid.New()
	sourceID := uuid.New()
	sellerID := uuid.New()
	buyerID := uuid.New()
	expiredAt := time.Now().Add(-1 * time.Hour)

	quote := newValidatableShippingQuote(chatID, productID, sourceID, sellerID, buyerID, "prov-1", "city-1", &expiredAt)
	svc := &OrderCreationService{
		shippingQuoteRepo: &expiryTestShippingQuoteRepo{quote: quote},
	}

	_, err := svc.validateShippingQuoteForOrder(
		context.Background(), nil, quote.ID, &chatID, productID, sourceID, nil,
		sellerID, buyerID, "prov-1", "city-1",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")
}

// TestValidateShippingQuoteForOrder_AcceptsNonExpiredQuote proves a quote
// whose ExpiresAt is still in the future passes checkout validation.
func TestValidateShippingQuoteForOrder_AcceptsNonExpiredQuote(t *testing.T) {
	chatID := uuid.New()
	productID := uuid.New()
	sourceID := uuid.New()
	sellerID := uuid.New()
	buyerID := uuid.New()
	notYetExpired := time.Now().Add(23 * time.Hour)

	quote := newValidatableShippingQuote(chatID, productID, sourceID, sellerID, buyerID, "prov-1", "city-1", &notYetExpired)
	svc := &OrderCreationService{
		shippingQuoteRepo: &expiryTestShippingQuoteRepo{quote: quote},
	}

	got, err := svc.validateShippingQuoteForOrder(
		context.Background(), nil, quote.ID, &chatID, productID, sourceID, nil,
		sellerID, buyerID, "prov-1", "city-1",
	)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, quote.ID, got.ID)
}
