package entity

import (
	"errors"
	"testing"
	"time"

	"github.com/labuda/backend/pkg/money"
)

func TestCalculateFee_Flat(t *testing.T) {
	m := Method{Code: "bank_transfer", FeeType: FeeTypeFlat, FlatAmount: money.New(4000)}

	fee, err := CalculateFee(money.New(103000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee.Int64() != 4000 {
		t.Fatalf("flat fee = %d, want 4000", fee.Int64())
	}
}

func TestCalculateFee_Percent_RoundsUp(t *testing.T) {
	m := Method{Code: "qris", FeeType: FeeTypePercent, PercentBps: 70} // 0.7%

	// 103000 * 70 / 10000 = 721.0 exactly -> no rounding needed
	fee, err := CalculateFee(money.New(103000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee.Int64() != 721 {
		t.Fatalf("percent fee = %d, want 721", fee.Int64())
	}

	// 100001 * 70 / 10000 = 700.007 -> must round UP to 701, never truncate
	fee2, err := CalculateFee(money.New(100001), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee2.Int64() != 701 {
		t.Fatalf("percent fee (rounding) = %d, want 701 (ceiling, not truncation)", fee2.Int64())
	}
}

func TestCalculateFee_PercentPlusFlat(t *testing.T) {
	m := Method{
		Code:       "credit_card",
		FeeType:    FeeTypePercentPlusFlat,
		PercentBps: 290, // 2.9%
		FlatAmount: money.New(2000),
	}

	// ceil(103000 * 290 / 10000) = ceil(2987.0) = 2987, + 2000 = 4987
	fee, err := CalculateFee(money.New(103000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee.Int64() != 4987 {
		t.Fatalf("percent+flat fee = %d, want 4987", fee.Int64())
	}
}

func TestCalculateFee_MinFeeClamp(t *testing.T) {
	min := money.New(2000)
	m := Method{Code: "qris", FeeType: FeeTypePercent, PercentBps: 70, MinFee: &min}

	// ceil(1000 * 70/10000) = 7, below min of 2000 -> clamps to 2000
	fee, err := CalculateFee(money.New(1000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee.Int64() != 2000 {
		t.Fatalf("fee = %d, want min-clamped 2000", fee.Int64())
	}
}

func TestCalculateFee_MaxFeeClamp(t *testing.T) {
	max := money.New(10000)
	m := Method{Code: "credit_card", FeeType: FeeTypePercentPlusFlat, PercentBps: 290, FlatAmount: money.New(2000), MaxFee: &max}

	// ceil(10_000_000 * 290/10000) = 290000, + 2000 = 292000, clamps to 10000
	fee, err := CalculateFee(money.New(10_000_000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fee.Int64() != 10000 {
		t.Fatalf("fee = %d, want max-clamped 10000", fee.Int64())
	}
}

func TestCalculateFee_UnknownFeeType(t *testing.T) {
	m := Method{Code: "mystery", FeeType: FeeType("bogus")}

	_, err := CalculateFee(money.New(1000), m)
	if err != ErrUnknownFeeType {
		t.Fatalf("err = %v, want ErrUnknownFeeType", err)
	}
}

func TestCalculateFee_ZeroBpsIsZero(t *testing.T) {
	m := Method{Code: "free", FeeType: FeeTypePercent, PercentBps: 0}

	fee, err := CalculateFee(money.New(500000), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fee.IsZero() {
		t.Fatalf("fee = %d, want 0", fee.Int64())
	}
}

// ============================================================================
// ValidateConfig (PASS_18W admin config)
// ============================================================================

func validMethod() Method {
	return Method{
		Code:             "bank_transfer",
		DisplayName:      "Transfer Bank",
		Enabled:          true,
		FeeType:          FeeTypeFlat,
		FlatAmount:       money.New(4000),
		MidtransChannels: []string{"bca_va"},
		RateSource:       RateSourcePublicBaseline,
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	if err := ValidateConfig(validMethod()); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_EmptyDisplayName(t *testing.T) {
	m := validMethod()
	m.DisplayName = "   "
	if err := ValidateConfig(m); err != ErrEmptyDisplayName {
		t.Fatalf("err = %v, want ErrEmptyDisplayName", err)
	}
}

func TestValidateConfig_UnknownFeeType(t *testing.T) {
	m := validMethod()
	m.FeeType = FeeType("bogus")
	if err := ValidateConfig(m); err != ErrUnknownFeeType {
		t.Fatalf("err = %v, want ErrUnknownFeeType", err)
	}
}

func TestValidateConfig_NegativeFlatAmount(t *testing.T) {
	m := validMethod()
	m.FlatAmount = money.New(-1)
	if err := ValidateConfig(m); err != ErrNegativeFlatAmount {
		t.Fatalf("err = %v, want ErrNegativeFlatAmount", err)
	}
}

func TestValidateConfig_NegativePercentBps(t *testing.T) {
	m := validMethod()
	m.FeeType = FeeTypePercent
	m.PercentBps = -1
	if err := ValidateConfig(m); err != ErrNegativePercentBps {
		t.Fatalf("err = %v, want ErrNegativePercentBps", err)
	}
}

func TestValidateConfig_PercentBpsTooHigh(t *testing.T) {
	m := validMethod()
	m.FeeType = FeeTypePercent
	m.PercentBps = MaxPercentBps + 1
	if err := ValidateConfig(m); err != ErrPercentBpsTooHigh {
		t.Fatalf("err = %v, want ErrPercentBpsTooHigh", err)
	}

	// Boundary: exactly the ceiling must be accepted.
	m.PercentBps = MaxPercentBps
	if err := ValidateConfig(m); err != nil {
		t.Fatalf("expected MaxPercentBps to be valid, got: %v", err)
	}
}

func TestValidateConfig_MinExceedsMax(t *testing.T) {
	m := validMethod()
	min := money.New(5000)
	max := money.New(4000)
	m.MinFee = &min
	m.MaxFee = &max
	if err := ValidateConfig(m); err != ErrMinExceedsMax {
		t.Fatalf("err = %v, want ErrMinExceedsMax", err)
	}
}

func TestValidateConfig_MinEqualsMax_Allowed(t *testing.T) {
	m := validMethod()
	v := money.New(4000)
	m.MinFee = &v
	m.MaxFee = &v
	if err := ValidateConfig(m); err != nil {
		t.Fatalf("expected min == max to be valid, got: %v", err)
	}
}

func TestValidateConfig_NegativeMinFee(t *testing.T) {
	m := validMethod()
	v := money.New(-1)
	m.MinFee = &v
	if err := ValidateConfig(m); err != ErrNegativeMinFee {
		t.Fatalf("err = %v, want ErrNegativeMinFee", err)
	}
}

func TestValidateConfig_NegativeMaxFee(t *testing.T) {
	m := validMethod()
	v := money.New(-1)
	m.MaxFee = &v
	if err := ValidateConfig(m); err != ErrNegativeMaxFee {
		t.Fatalf("err = %v, want ErrNegativeMaxFee", err)
	}
}

func TestValidateConfig_EnabledWithNoChannels(t *testing.T) {
	m := validMethod()
	m.MidtransChannels = nil
	if err := ValidateConfig(m); err != ErrEnabledMethodNeedsChannels {
		t.Fatalf("err = %v, want ErrEnabledMethodNeedsChannels", err)
	}
}

func TestValidateConfig_DisabledWithNoChannels_Allowed(t *testing.T) {
	m := validMethod()
	m.Enabled = false
	m.MidtransChannels = nil
	if err := ValidateConfig(m); err != nil {
		t.Fatalf("disabled method with no channels should be valid, got: %v", err)
	}
}

// TestValidateConfig_NilAndEmptyChannelsAreEquivalent (PASS_19C) locks that
// ValidateConfig treats a nil MidtransChannels identically to an empty
// (non-nil) slice — both have len 0, so this was already implicitly true,
// but the repository-layer nil-safety fix (PaymentMethodRepository.Update
// coalescing nil to []string{} before writing) depends on validation never
// distinguishing the two. If that ever changed, this test would catch it.
func TestValidateConfig_NilAndEmptyChannelsAreEquivalent(t *testing.T) {
	nilErr := func() error {
		m := validMethod()
		m.Enabled = true
		m.MidtransChannels = nil
		return ValidateConfig(m)
	}()
	emptyErr := func() error {
		m := validMethod()
		m.Enabled = true
		m.MidtransChannels = []string{}
		return ValidateConfig(m)
	}()
	if nilErr != ErrEnabledMethodNeedsChannels || emptyErr != ErrEnabledMethodNeedsChannels {
		t.Fatalf("enabled+nil = %v, enabled+empty = %v; want both ErrEnabledMethodNeedsChannels", nilErr, emptyErr)
	}

	nilOK := func() error {
		m := validMethod()
		m.Enabled = false
		m.MidtransChannels = nil
		return ValidateConfig(m)
	}()
	emptyOK := func() error {
		m := validMethod()
		m.Enabled = false
		m.MidtransChannels = []string{}
		return ValidateConfig(m)
	}()
	if nilOK != nil || emptyOK != nil {
		t.Fatalf("disabled+nil = %v, disabled+empty = %v; want both valid (nil)", nilOK, emptyOK)
	}
}

func TestValidateConfig_UnknownMidtransChannel(t *testing.T) {
	m := validMethod()
	m.MidtransChannels = []string{"bca_va", "totally_fake_channel"}
	err := ValidateConfig(m)
	if !errors.Is(err, ErrUnknownMidtransChannel) {
		t.Fatalf("err = %v, want wrapping ErrUnknownMidtransChannel", err)
	}
}

// TestAllowedMidtransChannels_ForbiddenPaylaterChannelsAbsent (PASS_19A
// addendum) locks the owner's card-vs-paylater policy at its single
// enforcement point: Snap enabled_payments is always built directly from a
// method's MidtransChannels, and MidtransChannels can only ever contain
// values that passed ValidateConfig's AllowedMidtransChannels check. So
// keeping these codes out of the allowlist is sufficient to guarantee they
// can never reach Snap, regardless of what an admin tries to save.
func TestAllowedMidtransChannels_ForbiddenPaylaterChannelsAbsent(t *testing.T) {
	forbidden := []string{
		"shopeepay", "spaylater", "shopeepay_installment", "shopeepay_paylater",
		"kredivo", "akulaku",
	}
	for _, ch := range forbidden {
		if AllowedMidtransChannels[ch] {
			t.Fatalf("forbidden PayLater/ShopeePay channel %q must never be in AllowedMidtransChannels", ch)
		}
	}
}

// TestValidateConfig_CardPaymentChannelsAccepted (PASS_19A addendum) proves
// card payment stays allowed — only PayLater/installment products are
// forbidden, not card payment itself, even though it is also gated through
// this same allowlist.
func TestValidateConfig_CardPaymentChannelsAccepted(t *testing.T) {
	for _, ch := range []string{"credit_card", "debit_card"} {
		m := validMethod()
		m.MidtransChannels = []string{ch}
		if err := ValidateConfig(m); err != nil {
			t.Fatalf("card channel %q should be allowed, got error: %v", ch, err)
		}
	}
}

func TestValidateConfig_AllAllowedChannelsAccepted(t *testing.T) {
	for ch := range AllowedMidtransChannels {
		m := validMethod()
		m.MidtransChannels = []string{ch}
		if err := ValidateConfig(m); err != nil {
			t.Fatalf("channel %q should be allowed, got error: %v", ch, err)
		}
	}
}

// ============================================================================
// RateSource (PASS_19A)
// ============================================================================

func TestValidateConfig_UnknownRateSource(t *testing.T) {
	m := validMethod()
	m.RateSource = RateSource("bogus")
	if err := ValidateConfig(m); err != ErrUnknownRateSource {
		t.Fatalf("err = %v, want ErrUnknownRateSource", err)
	}
}

func TestValidateConfig_AllThreeRateSourcesAccepted(t *testing.T) {
	for _, rs := range []RateSource{RateSourcePublicBaseline, RateSourceManualOverride} {
		m := validMethod()
		m.RateSource = rs
		if err := ValidateConfig(m); err != nil {
			t.Fatalf("rate_source %q should be valid, got error: %v", rs, err)
		}
	}

	// merchant_verified additionally requires a non-empty note.
	m := validMethod()
	m.RateSource = RateSourceMerchantVerified
	m.RateSourceNote = "Confirmed against Midtrans merchant dashboard 2026-08-01."
	if err := ValidateConfig(m); err != nil {
		t.Fatalf("merchant_verified with note should be valid, got error: %v", err)
	}
}

func TestValidateConfig_MerchantVerifiedWithoutNote_Rejected(t *testing.T) {
	m := validMethod()
	m.RateSource = RateSourceMerchantVerified
	m.RateSourceNote = "   "
	if err := ValidateConfig(m); err != ErrMerchantVerifiedNeedsNote {
		t.Fatalf("err = %v, want ErrMerchantVerifiedNeedsNote", err)
	}
}

func TestReconcileRateSource_FeeUnchanged_StaysPublicBaseline(t *testing.T) {
	before := validMethod()
	candidate := before // identical copy: only display_name/sort_order would change in a real edit

	got := ReconcileRateSource(candidate, before)
	if got.RateSource != RateSourcePublicBaseline {
		t.Fatalf("RateSource = %v, want unchanged public_baseline", got.RateSource)
	}
}

func TestReconcileRateSource_FeeChanged_ForcesManualOverride(t *testing.T) {
	before := validMethod() // flat/4000, public_baseline
	candidate := before
	candidate.FlatAmount = money.New(5000)

	got := ReconcileRateSource(candidate, before)
	if got.RateSource != RateSourceManualOverride {
		t.Fatalf("RateSource = %v, want manual_override after a fee edit", got.RateSource)
	}
	if got.RateSourceNote == "" {
		t.Fatal("expected an auto-generated rate_source_note explaining the forced override")
	}
}

func TestReconcileRateSource_FeeChanged_MerchantVerifiedNotOverridden(t *testing.T) {
	before := validMethod()
	before.RateSource = RateSourceMerchantVerified
	before.RateSourceNote = "Confirmed 2026-08-01."
	candidate := before
	candidate.PercentBps = 999 // pretend the merchant renegotiated the rate

	got := ReconcileRateSource(candidate, before)
	if got.RateSource != RateSourceMerchantVerified {
		t.Fatalf("RateSource = %v, want merchant_verified preserved (only public_baseline is auto-forced)", got.RateSource)
	}
}

func TestReconcileRateSource_FeeChanged_KeepsExplicitNote(t *testing.T) {
	before := validMethod()
	candidate := before
	candidate.FlatAmount = money.New(5000)
	candidate.RateSourceNote = "Admin note: matched Midtrans dashboard screenshot."

	got := ReconcileRateSource(candidate, before)
	if got.RateSourceNote != "Admin note: matched Midtrans dashboard screenshot." {
		t.Fatalf("expected explicit admin note to survive reconciliation, got %q", got.RateSourceNote)
	}
}

func TestResolveMerchantVerifiedAt_NotMerchantVerified_Nil(t *testing.T) {
	before := validMethod()
	candidate := before
	candidate.RateSource = RateSourceManualOverride

	if got := ResolveMerchantVerifiedAt(candidate, before, time.Now()); got != nil {
		t.Fatalf("expected nil merchant_verified_at, got %v", got)
	}
}

func TestResolveMerchantVerifiedAt_NewlyVerified_SetsNow(t *testing.T) {
	before := validMethod() // public_baseline
	candidate := before
	candidate.RateSource = RateSourceMerchantVerified
	candidate.RateSourceNote = "Confirmed 2026-08-01."

	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	got := ResolveMerchantVerifiedAt(candidate, before, now)
	if got == nil || !got.Equal(now) {
		t.Fatalf("expected merchant_verified_at = %v, got %v", now, got)
	}
}

func TestResolveMerchantVerifiedAt_AlreadyVerified_PreservesOriginal(t *testing.T) {
	original := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	before := validMethod()
	before.RateSource = RateSourceMerchantVerified
	before.RateSourceNote = "Confirmed 2026-08-01."
	before.MerchantVerifiedAt = &original

	candidate := before
	candidate.DisplayName = "Transfer Bank (renamed)" // unrelated edit, same verification

	later := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	got := ResolveMerchantVerifiedAt(candidate, before, later)
	if got == nil || !got.Equal(original) {
		t.Fatalf("expected preserved merchant_verified_at = %v, got %v", original, got)
	}
}
