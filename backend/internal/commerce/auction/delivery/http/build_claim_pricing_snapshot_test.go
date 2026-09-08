package http

import (
	"testing"

	"github.com/google/uuid"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestBuildClaimPricingSnapshot_PreservesNegotiationID(t *testing.T) {
	negID := uuid.New()
	auctionID := uuid.New()
	token := &pricingtokenentity.PricingToken{
		UnitPrice:              money.New(100000),
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(15000),
		CommissionPercent:      5,
		CommissionAmount:       money.New(5000),
		EscrowAmount:           money.New(115000),
		ServiceFeeAmount:       money.New(3000),
		TotalPayableAmount:     money.New(118000),
		DiscountAmount:         money.New(0),
		MaxCoinsAllowed:        10000,
		CoinsUsed:              0,
		OrderValueForCoins:     115000,
		ShippingSetupName:      "JNE",
		ShippingTransportType:  "train",
		AuctionID:              &auctionID,
		NegotiationID:          &negID,
		Token:                  uuid.New(),
		AddressSnapshot:        []byte(`{}`),
	}
	snap := buildClaimPricingSnapshot(token)
	require.NotNil(t, snap)
	require.NotNil(t, snap.NegotiationID)
	require.Equal(t, negID, *snap.NegotiationID)
	require.NotNil(t, snap.AuctionID)
	require.Equal(t, auctionID, *snap.AuctionID)
}

func TestBuildClaimPricingSnapshot_NilNegotiationPreservedAsNil(t *testing.T) {
	auctionID := uuid.New()
	token := &pricingtokenentity.PricingToken{
		UnitPrice:              money.New(100000),
		Subtotal:               money.New(100000),
		ShippingTotal:          money.New(15000),
		CommissionPercent:      5,
		CommissionAmount:       money.New(5000),
		EscrowAmount:           money.New(115000),
		ServiceFeeAmount:       money.New(3000),
		TotalPayableAmount:     money.New(118000),
		DiscountAmount:         money.New(0),
		MaxCoinsAllowed:        10000,
		OrderValueForCoins:     115000,
		ShippingSetupName:      "JNE",
		ShippingTransportType:  "train",
		AuctionID:              &auctionID,
		NegotiationID:          nil,
		Token:                  uuid.New(),
		AddressSnapshot:        []byte(`{}`),
	}
	snap := buildClaimPricingSnapshot(token)
	require.NotNil(t, snap)
	require.Nil(t, snap.NegotiationID)
	require.NotNil(t, snap.AuctionID)
	require.Equal(t, auctionID, *snap.AuctionID)
}

func TestBuildClaimPricingSnapshot_AuctionTokenIdentityUnchanged(t *testing.T) {
	auctionID := uuid.New()
	token := &pricingtokenentity.PricingToken{
		UnitPrice:              money.New(200000),
		Subtotal:               money.New(200000),
		ShippingTotal:          money.New(15000),
		CommissionPercent:      5,
		CommissionAmount:       money.New(10000),
		EscrowAmount:           money.New(215000),
		ServiceFeeAmount:       money.New(3000),
		TotalPayableAmount:     money.New(218000),
		DiscountAmount:         money.New(0),
		MaxCoinsAllowed:        10000,
		OrderValueForCoins:     215000,
		ShippingSetupName:      "JNE",
		ShippingTransportType:  "train",
		AuctionID:              &auctionID,
		Token:                  uuid.New(),
		AddressSnapshot:        []byte(`{}`),
	}
	snap := buildClaimPricingSnapshot(token)
	require.Equal(t, token.UnitPrice.Int64(), snap.UnitPrice.Int64())
	require.Equal(t, token.Token, snap.TokenID)
	require.Equal(t, *token.AuctionID, *snap.AuctionID)
}
