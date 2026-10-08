package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestGenerateForForSale_QuoteRequiresChatID proves the pricing preview refuses
// to mint a conversation-less token for a manual shipping quote. The chat id is
// the conversation that produced the quote and is required so order creation can
// enforce the quote is used only in that conversation.
func TestGenerateForForSale_QuoteRequiresChatID(t *testing.T) {
	quoteID := uuid.New()
	svc := &PricingTokenService{} // validation happens before any dependency use

	_, err := svc.GenerateForForSale(context.Background(), nil, &GenerateForForSaleRequest{
		UserID:          uuid.New(),
		ProductID:       uuid.New(),
		SourceType:      "for_sale",
		SourceID:        uuid.New(),
		Quantity:        1,
		ShippingQuoteID: &quoteID,
		AddressID:       uuid.New(),
		// ChatID deliberately omitted.
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "chat_id is required")
}
