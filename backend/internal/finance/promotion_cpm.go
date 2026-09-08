package finance

import (
	"fmt"
	"math"
)

// ============================================================================
// PROMOTION CPM ARITHMETIC — CANONICAL CUMULATIVE MODEL
// ============================================================================
//
// Pricing rule (LOCKED by LABUDA_PROMOTION_DOMAIN_CANONICAL_CONTRACT.md):
//
//	Qualified Impression = billing unit.
//	CPM = whole Rupiah per 1000 Qualified Impressions.
//
// Per-impression cost is NOT a stored money amount. A CPM that does not
// divide evenly into 1000 (e.g. CPM Rp7.500 -> per impression Rp7,5) would
// silently drift if each impression were billed by truncation or rounding.
//
// Canonical cumulative arithmetic (all integer Rupiah, no floats):
//
//	S(N)   = floor(N * CPM / 1000)     // cumulative spend for the first N QIs
//	charge(N) = S(N) - S(N-1)          // ledger charge for the N-th QI
//
// Because S is cumulative and monotone, the sum of per-QI charges is EXACTLY
// S(N) for every N — no rounding drift, no remainder ambiguity:
//
//	CPM 7500: S(1)=7,  charge(1)=7
//	          S(2)=15, charge(2)=8   -> total after 2 QIs = 15 = floor(2*7500/1000)
//	          S(3)=22, charge(3)=7   -> total after 3 QIs = 22 = floor(3*7500/1000)
//
// Final remainder handling at promotion finalization:
//
//	remaining = allocated_budget - S(final_N)   (>= 0 by construction when
//	             S(final_N) <= budget) -> released back to PROMOTE_BALANCE.
//
// Estimated impression count for a budget:
//
//	est(N) such that S(N) <= budget < S(N+1)  =>  N = floor(budget*1000/CPM)
// ============================================================================

// errPromotionCpmInvalid reports an invalid CPM argument.
type errPromotionCpmInvalid struct {
	msg string
}

func (e *errPromotionCpmInvalid) Error() string {
	return "promotion CPM arithmetic: " + e.msg
}

func promotionProduct(n, cpm int64) (int64, error) {
	if n < 0 {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("impression count must not be negative (got %d)", n)}
	}
	if cpm < 0 {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("cpm must not be negative (got %d)", cpm)}
	}
	if n == 0 || cpm == 0 {
		return 0, nil
	}
	if n > math.MaxInt64/cpm {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("integer overflow computing %d * %d", n, cpm)}
	}
	return n * cpm, nil
}

// PromotionCumulativeSpend returns S(N) = floor(N*CPM/1000), the exact
// cumulative spend in Rupiah for the first N Qualified Impressions at the
// given CPM. S(0) = 0. Pure integer arithmetic; never floating point.
func PromotionCumulativeSpend(impressions, cpm int64) (int64, error) {
	product, err := promotionProduct(impressions, cpm)
	if err != nil {
		return 0, err
	}
	return product / 1000, nil
}

// PromotionCharge returns charge(N) = S(N) - S(N-1): the exact integer
// Rupiah ledger charge for the N-th Qualified Impression. For N = 0 the
// charge is 0. Charges for successive impressions may differ by at most 1
// Rupiah so their SUM always equals S(N) exactly.
func PromotionCharge(impressions, cpm int64) (int64, error) {
	if impressions <= 0 {
		return 0, nil
	}
	sn, err := PromotionCumulativeSpend(impressions, cpm)
	if err != nil {
		return 0, err
	}
	snMinus1, err := PromotionCumulativeSpend(impressions-1, cpm)
	if err != nil {
		return 0, err
	}
	return sn - snMinus1, nil
}

// PromotionEstimatedImpressions returns the largest N such that
// S(N) <= budget, i.e. floor(budget*1000/CPM). Returns 0 when budget is 0
// or CPM is 0 (a zero CPM means unlimited impressions at zero cost).
func PromotionEstimatedImpressions(budget, cpm int64) (int64, error) {
	if budget < 0 {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("budget must not be negative (got %d)", budget)}
	}
	if cpm < 0 {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("cpm must not be negative (got %d)", cpm)}
	}
	if cpm == 0 {
		return 0, nil
	}
	if budget > math.MaxInt64/1000 {
		return 0, &errPromotionCpmInvalid{msg: fmt.Sprintf("integer overflow estimating impressions for budget %d", budget)}
	}
	return budget * 1000 / cpm, nil
}
