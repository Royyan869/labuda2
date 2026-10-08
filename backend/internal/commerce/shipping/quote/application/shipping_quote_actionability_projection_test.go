package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
)

// CANONICAL SHIPPING QUOTE ACTIONABILITY PROJECTION.
//
// EvaluateQuoteActionability is the single Commerce-provided answer to
// "which quote is current/actionable for this viewer". It delegates to the
// shipping entity's canonical predicates (IsCurrent / IsBuyerUsableAt) and the
// Commerce-owned buyer identity; conversation surfaces never compute this.
func TestEvaluateQuoteActionability(t *testing.T) {
	now := time.Now()
	buyer := uuid.New()
	seller := uuid.New()
	stranger := uuid.New()

	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	supersededAt := now.Add(-time.Minute)
	supersededBy := uuid.New()

	base := func() *shippingQuoteEntity.ShippingQuote {
		return &shippingQuoteEntity.ShippingQuote{
			ID:        uuid.New(),
			BuyerID:   buyer,
			SellerID:  seller,
			Status:    shippingQuoteEntity.QuoteStatusActive,
			ExpiresAt: &future,
			CreatedAt: now,
		}
	}

	cases := []struct {
		name           string
		quote          *shippingQuoteEntity.ShippingQuote
		viewer         uuid.UUID
		wantCurrent    bool
		wantActionable bool
	}{
		{
			name:           "current active quote, buyer viewer",
			quote:          base(),
			viewer:         buyer,
			wantCurrent:    true,
			wantActionable: true,
		},
		{
			name:           "seller never acts on own quote",
			quote:          base(),
			viewer:         seller,
			wantCurrent:    true,
			wantActionable: false,
		},
		{
			name: "superseded quote is not current nor actionable",
			quote: func() *shippingQuoteEntity.ShippingQuote {
				q := base()
				q.SupersededAt = &supersededAt
				q.SupersededByID = &supersededBy
				return q
			}(),
			viewer:         buyer,
			wantCurrent:    false,
			wantActionable: false,
		},
		{
			name: "expired quote is not actionable",
			quote: func() *shippingQuoteEntity.ShippingQuote {
				q := base()
				q.ExpiresAt = &past
				return q
			}(),
			viewer:         buyer,
			wantCurrent:    true,
			wantActionable: false,
		},
		{
			name: "used quote is not current nor actionable",
			quote: func() *shippingQuoteEntity.ShippingQuote {
				q := base()
				q.Status = shippingQuoteEntity.QuoteStatusUsed
				q.UsedAt = &now
				return q
			}(),
			viewer:         buyer,
			wantCurrent:    false,
			wantActionable: false,
		},
		{
			name: "invalid quote is not current nor actionable",
			quote: func() *shippingQuoteEntity.ShippingQuote {
				q := base()
				q.Status = shippingQuoteEntity.QuoteStatusInvalid
				return q
			}(),
			viewer:         buyer,
			wantCurrent:    false,
			wantActionable: false,
		},
		{
			name:           "unrelated viewer gets no actionable state",
			quote:          base(),
			viewer:         stranger,
			wantCurrent:    true,
			wantActionable: false,
		},
		{
			name:           "anonymous viewer gets no actionable state",
			quote:          base(),
			viewer:         uuid.Nil,
			wantCurrent:    true,
			wantActionable: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateQuoteActionability(tc.quote, tc.viewer, now)
			if got.IsCurrent != tc.wantCurrent {
				t.Fatalf("IsCurrent = %v, want %v", got.IsCurrent, tc.wantCurrent)
			}
			if got.ViewerActionable != tc.wantActionable {
				t.Fatalf("ViewerActionable = %v, want %v", got.ViewerActionable, tc.wantActionable)
			}
		})
	}
}

func TestEvaluateQuoteActionability_NilQuote(t *testing.T) {
	got := EvaluateQuoteActionability(nil, uuid.New(), time.Now())
	if got.IsCurrent || got.ViewerActionable {
		t.Fatalf("nil quote must project no state, got %+v", got)
	}
}
