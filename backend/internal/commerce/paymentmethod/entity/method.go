// DOMAIN: PAYMENT METHOD
// Canonical buyer-facing payment methods and the buyer payment fee formula.
//
// Backend is the sole authority for payment method fees. The buyer selects a
// method code before a payment is created; the backend looks up the method,
// validates it is enabled, and calculates the fee from this table. Mobile/
// admin never calculate or submit a fee amount — admin (PASS_18W) may edit
// the table itself, subject to ValidateConfig, but a config edit only ever
// affects payments created after the edit (see CorePaymentHandler.CreatePayment,
// which reads the row fresh at payment-creation time).

package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/labuda/backend/pkg/money"
)

// FeeType identifies how a method's buyer payment fee is calculated.
type FeeType string

const (
	// FeeTypeFlat charges FlatAmount regardless of the base amount.
	FeeTypeFlat FeeType = "flat"
	// FeeTypePercent charges ceil(base * PercentBps / 10000).
	FeeTypePercent FeeType = "percent"
	// FeeTypePercentPlusFlat charges ceil(base * PercentBps / 10000) + FlatAmount.
	FeeTypePercentPlusFlat FeeType = "percent_plus_flat"
)

// RateSource records where a method's fee formula came from, so admin/buyer
// surfaces can be honest about whether a rate is a real Midtrans merchant
// contract number or just an unverified public-pricing snapshot (PASS_19A).
type RateSource string

const (
	// RateSourcePublicBaseline means the rate was copied from Midtrans's
	// public pricing page/docs, not a merchant dashboard/contract. This is
	// the default for every method until an owner merchant-verifies it.
	RateSourcePublicBaseline RateSource = "public_baseline"
	// RateSourceMerchantVerified means an owner has confirmed the rate
	// against Labuda's actual Midtrans merchant contract/dashboard/report.
	RateSourceMerchantVerified RateSource = "merchant_verified"
	// RateSourceManualOverride means an admin edited the fee formula away
	// from its public-baseline seed value without merchant verification.
	RateSourceManualOverride RateSource = "manual_override"
)

// Method is the canonical, backend-owned definition of a buyer payment
// method and its fee formula. Rows are seeded by migration and are editable
// by an admin (PASS_18W) via ValidateConfig-guarded updates.
type Method struct {
	Code             string
	DisplayName      string
	Enabled          bool
	FeeType          FeeType
	FlatAmount       money.Money
	PercentBps       int64 // basis points; 100 bps = 1%
	MinFee           *money.Money
	MaxFee           *money.Money
	MidtransChannels []string // Midtrans Snap enabled_payments channel codes
	SortOrder        int

	// RateSource, RateSourceNote, and MerchantVerifiedAt (PASS_19A) record
	// fee-rate verification status. They never affect CalculateFee — they
	// exist purely so admin/buyer UI can avoid implying a public-baseline
	// number is Labuda's real Midtrans merchant rate.
	RateSource         RateSource
	RateSourceNote     string
	MerchantVerifiedAt *time.Time
}

// ErrMethodDisabled is returned when a method exists but is not enabled.
var ErrMethodDisabled = errors.New("payment method is disabled")

// ErrUnknownFeeType is returned when a method row has an unrecognized fee_type.
var ErrUnknownFeeType = errors.New("payment method has unknown fee_type")

// CalculateFee computes the buyer payment fee for baseAmount using m's fee
// formula. baseAmount must be the buyer cash amount after coin deduction
// (cashAmount = (P−D)+S − K), never a commission-inclusive gross and never a
// fee-inclusive amount, to avoid fee-on-fee. All math is integer Rupiah;
// percentage fees round up (ceiling) so the platform never absorbs a
// fractional-Rupiah loss.
func CalculateFee(baseAmount money.Money, m Method) (money.Money, error) {
	var fee money.Money

	switch m.FeeType {
	case FeeTypeFlat:
		fee = m.FlatAmount
	case FeeTypePercent:
		fee = percentOf(baseAmount, m.PercentBps)
	case FeeTypePercentPlusFlat:
		fee = percentOf(baseAmount, m.PercentBps).Add(m.FlatAmount)
	default:
		return money.Zero(), ErrUnknownFeeType
	}

	if m.MinFee != nil && fee.LessThan(*m.MinFee) {
		fee = *m.MinFee
	}
	if m.MaxFee != nil && fee.GreaterThan(*m.MaxFee) {
		fee = *m.MaxFee
	}

	return fee, nil
}

// percentOf returns ceil(amount * bps / 10000) as a Rupiah-integer Money.
func percentOf(amount money.Money, bps int64) money.Money {
	if bps <= 0 {
		return money.Zero()
	}
	product := amount.Int64() * bps
	// Ceiling division for non-negative values.
	result := (product + 9999) / 10000
	return money.New(result)
}

// ============================================================================
// ADMIN CONFIG VALIDATION (PASS_18W)
// ============================================================================

// MaxPercentBps is the sanity ceiling for a payment-processing fee: 2000 bps
// = 20%. No real Midtrans channel fee approaches this; it exists to catch a
// fat-fingered admin entry (e.g. typing 2900 meaning "2.9%" into a bps field)
// before it can ever reach a real checkout.
const MaxPercentBps int64 = 2000

// AllowedMidtransChannels is the canonical allowlist of Midtrans Snap
// enabled_payments channel codes an admin may assign to a method. This is
// deliberately a fixed Go allowlist, not admin-editable — expanding it is a
// code change (matching the pass's "do not implement every method" scope),
// not a config edit.
//
// PASS_19A owner policy: card payment (credit_card/debit_card) is allowed,
// but PayLater/installment/deferred-financing products are not. shopeepay
// (which also fronts Midtrans's ShopeePay/SPayLater grouped channel),
// akulaku, and kredivo are therefore deliberately absent — never add them
// back without an explicit owner decision, since the allowlist is the sole
// enforcement point: ValidateConfig rejects any channel not listed here, so
// nothing paylater-shaped can ever reach a method row or Snap
// enabled_payments.
var AllowedMidtransChannels = map[string]bool{
	"bca_va": true, "bni_va": true, "bri_va": true, "permata_va": true,
	"cimb_va": true, "bsi_va": true, "danamon_va": true, "maybank_va": true,
	"btn_va": true, "other_va": true,
	"gopay": true, "dana": true, "ovo": true, "linkaja": true,
	"other_qris":  true,
	"credit_card": true, "debit_card": true,
	"alfamart": true, "indomaret": true,
}

// Validation errors returned by ValidateConfig. Each is distinct so callers
// (and tests) can assert on the specific rule that failed.
var (
	ErrEmptyDisplayName           = errors.New("display_name must not be empty")
	ErrNegativeFlatAmount         = errors.New("flat_amount_rupiah must not be negative")
	ErrNegativePercentBps         = errors.New("percent_bps must not be negative")
	ErrPercentBpsTooHigh          = fmt.Errorf("percent_bps exceeds the maximum allowed (%d bps = %d%%)", MaxPercentBps, MaxPercentBps/100)
	ErrNegativeMinFee             = errors.New("min_fee_rupiah must not be negative")
	ErrNegativeMaxFee             = errors.New("max_fee_rupiah must not be negative")
	ErrMinExceedsMax              = errors.New("min_fee_rupiah must not exceed max_fee_rupiah")
	ErrEnabledMethodNeedsChannels = errors.New("an enabled method must have at least one midtrans channel")
	ErrUnknownMidtransChannel     = errors.New("unknown or unsafe midtrans channel code")
	ErrUnknownRateSource          = errors.New("unknown rate_source")
	ErrMerchantVerifiedNeedsNote  = errors.New("rate_source_note is required when rate_source is merchant_verified")
)

// ValidateConfig enforces every admin-editable invariant on m, independent
// of any DB CHECK constraint (so the handler can reject bad input with a
// specific, friendly error before ever issuing a write). It does NOT check
// "would this disable the last enabled method" — that requires knowledge of
// sibling rows and is enforced by the repository/handler at update time.
//
// method_code is deliberately not validated here: it is never part of the
// mutable config (see UpdateMethodInput), so it cannot be "wrong."
func ValidateConfig(m Method) error {
	if strings.TrimSpace(m.DisplayName) == "" {
		return ErrEmptyDisplayName
	}

	switch m.FeeType {
	case FeeTypeFlat, FeeTypePercent, FeeTypePercentPlusFlat:
	default:
		return ErrUnknownFeeType
	}

	if m.FlatAmount.IsNegative() {
		return ErrNegativeFlatAmount
	}
	if m.PercentBps < 0 {
		return ErrNegativePercentBps
	}
	if m.PercentBps > MaxPercentBps {
		return ErrPercentBpsTooHigh
	}
	if m.MinFee != nil && m.MinFee.IsNegative() {
		return ErrNegativeMinFee
	}
	if m.MaxFee != nil && m.MaxFee.IsNegative() {
		return ErrNegativeMaxFee
	}
	if m.MinFee != nil && m.MaxFee != nil && m.MinFee.GreaterThan(*m.MaxFee) {
		return ErrMinExceedsMax
	}

	if m.Enabled {
		if len(m.MidtransChannels) == 0 {
			return ErrEnabledMethodNeedsChannels
		}
		for _, ch := range m.MidtransChannels {
			if !AllowedMidtransChannels[ch] {
				return fmt.Errorf("%w: %q", ErrUnknownMidtransChannel, ch)
			}
		}
	}

	switch m.RateSource {
	case RateSourcePublicBaseline, RateSourceMerchantVerified, RateSourceManualOverride:
	default:
		return ErrUnknownRateSource
	}
	if m.RateSource == RateSourceMerchantVerified && strings.TrimSpace(m.RateSourceNote) == "" {
		return ErrMerchantVerifiedNeedsNote
	}

	return nil
}

// ============================================================================
// RATE SOURCE RECONCILIATION (PASS_19A)
// ============================================================================

// ReconcileRateSource enforces that an edit to the fee formula can never
// silently leave rate_source as public_baseline: if candidate's fee fields
// differ from before's and candidate is still (or was defaulted to)
// public_baseline, it is forced to manual_override — a manually changed
// number is no longer the unedited public-pricing snapshot it claims to be.
// Callers must run this against the currently saved row before persisting an
// admin edit (see AdminPaymentMethodHandler.UpdateMethod).
func ReconcileRateSource(candidate, before Method) Method {
	feeChanged := candidate.FeeType != before.FeeType ||
		!candidate.FlatAmount.Equal(before.FlatAmount) ||
		candidate.PercentBps != before.PercentBps ||
		!moneyPtrEqual(candidate.MinFee, before.MinFee) ||
		!moneyPtrEqual(candidate.MaxFee, before.MaxFee)

	if feeChanged && candidate.RateSource == RateSourcePublicBaseline {
		candidate.RateSource = RateSourceManualOverride
		if strings.TrimSpace(candidate.RateSourceNote) == "" {
			candidate.RateSourceNote = "Auto-set to manual_override: fee formula edited from public baseline seed."
		}
	}
	return candidate
}

// ResolveMerchantVerifiedAt computes the merchant_verified_at timestamp to
// persist for candidate, given the currently saved before row and now (the
// caller's clock, injected for deterministic tests). Rules:
//   - Not merchant_verified -> nil (clears any stale verification timestamp).
//   - Newly merchant_verified (before wasn't) -> now, recording this edit as
//     the verification moment.
//   - Already merchant_verified and staying merchant_verified -> preserve the
//     original timestamp; editing display name/sort order shouldn't reset it.
func ResolveMerchantVerifiedAt(candidate, before Method, now time.Time) *time.Time {
	if candidate.RateSource != RateSourceMerchantVerified {
		return nil
	}
	if before.RateSource == RateSourceMerchantVerified && before.MerchantVerifiedAt != nil {
		return before.MerchantVerifiedAt
	}
	t := now
	return &t
}

func moneyPtrEqual(a, b *money.Money) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
