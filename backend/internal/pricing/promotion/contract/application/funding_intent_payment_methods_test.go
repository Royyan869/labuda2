package application

import (
	"testing"

	"github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The disclosure surface is a pure function of (exact shortage obligation,
// enabled methods). These tests pin the fee base to the shortage — never the
// promotion budget, never a fee-inclusive amount (no fee-on-fee) — and pin the
// fee formula to the canonical payment method authority.

func TestBuildFundingPaymentMethodOptions_FeeIsComputedOnShortageNotBudget(t *testing.T) {
	shortage := money.New(35_000) // exact obligation; budget was 45_000

	methods := []entity.Method{
		{
			Code:        "gopay",
			DisplayName: "GoPay",
			Enabled:     true,
			FeeType:     entity.FeeTypePercent,
			PercentBps:  150, // 1.5%
		},
	}

	options := buildFundingPaymentMethodOptions(shortage, methods, nil)

	require.Len(t, options, 1)
	// ceil(35_000 * 150 / 10_000) = ceil(525) = 525
	assert.Equal(t, int64(525), options[0].ServiceFeeAmount)
	assert.Equal(t, int64(35_525), options[0].GrossAmount)
	assert.Equal(t, "gopay", options[0].MethodCode)
	assert.Equal(t, "GoPay", options[0].DisplayName)
}

func TestBuildFundingPaymentMethodOptions_FlatPercentPlusFlatAndClamps(t *testing.T) {
	shortage := money.New(10_000)
	minFee := money.New(2_500)
	maxFee := money.New(3_000)

	methods := []entity.Method{
		{
			Code:        "flat",
			DisplayName: "Flat",
			Enabled:     true,
			FeeType:     entity.FeeTypeFlat,
			FlatAmount:  money.New(4_000),
		},
		{
			Code:        "percent_plus_flat",
			DisplayName: "Percent + Flat",
			Enabled:     true,
			FeeType:     entity.FeeTypePercentPlusFlat,
			PercentBps:  100, // 1%
			FlatAmount:  money.New(500),
		},
		{
			Code:        "min_clamped",
			DisplayName: "Min clamped",
			Enabled:     true,
			FeeType:     entity.FeeTypePercent,
			PercentBps:  10, // 0.1% → 10 rupiah, below min
			MinFee:      &minFee,
		},
		{
			Code:        "max_clamped",
			DisplayName: "Max clamped",
			Enabled:     true,
			FeeType:     entity.FeeTypePercent,
			PercentBps:  5000, // 50% → 5_000, above max
			MaxFee:      &maxFee,
		},
	}

	options := buildFundingPaymentMethodOptions(shortage, methods, nil)
	require.Len(t, options, 4)

	byCode := map[string]FundingPaymentMethodOption{}
	for _, o := range options {
		byCode[o.MethodCode] = o
	}

	assert.Equal(t, int64(4_000), byCode["flat"].ServiceFeeAmount)
	assert.Equal(t, int64(14_000), byCode["flat"].GrossAmount)

	// ceil(10_000 * 1%) = 100, plus 500 flat
	assert.Equal(t, int64(600), byCode["percent_plus_flat"].ServiceFeeAmount)
	assert.Equal(t, int64(10_600), byCode["percent_plus_flat"].GrossAmount)

	assert.Equal(t, int64(2_500), byCode["min_clamped"].ServiceFeeAmount,
		"min_fee clamp must apply")
	assert.Equal(t, int64(12_500), byCode["min_clamped"].GrossAmount)
	assert.Equal(t, int64(3_000), byCode["max_clamped"].ServiceFeeAmount,
		"max_fee clamp must apply")
	assert.Equal(t, int64(13_000), byCode["max_clamped"].GrossAmount)
}

func TestBuildFundingPaymentMethodOptions_InvalidFeeFormulaIsSkippedNotFatal(t *testing.T) {
	shortage := money.New(20_000)
	methods := []entity.Method{
		{Code: "bogus", DisplayName: "Bogus", Enabled: true, FeeType: entity.FeeType("nope")},
		{Code: "gopay", DisplayName: "GoPay", Enabled: true, FeeType: entity.FeeTypePercent, PercentBps: 150},
	}

	invalid := map[string]error{}
	options := buildFundingPaymentMethodOptions(shortage, methods, func(code string, err error) {
		invalid[code] = err
	})

	require.Len(t, options, 1, "only the valid method is disclosed")
	assert.Equal(t, "gopay", options[0].MethodCode)
	require.Contains(t, invalid, "bogus")
	assert.ErrorIs(t, invalid["bogus"], entity.ErrUnknownFeeType)
}

func TestBuildFundingPaymentMethodOptions_NoMethodsYieldsEmptyNonNilList(t *testing.T) {
	options := buildFundingPaymentMethodOptions(money.New(1_000), nil, nil)
	require.NotNil(t, options, "must serialize as [] not null")
	assert.Empty(t, options)
}
