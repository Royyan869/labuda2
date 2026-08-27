package application

import "fmt"

// ProportionalRefundBreakdown is the canonical per-refund split (S2C2 rebase).
//
// CANONICAL FORMULA:
//
//	CoinDelta = floor(K * cumProductAfter / PD) - floor(K * cumProductBefore / PD)
//	CashRefund = Rpd + Rs - CoinDelta
//	CommissionDelta = floor(C * cumProductAfter / PD) - floor(C * cumProductBefore / PD)
//	SellerComponent = Rpd + Rs - CommissionDelta
//
// KEY INVARIANT:
//
//	CashRefund + CoinDelta == Rpd + Rs
//	cumCash + cumCoins == cumProduct + cumShipping  (accounting identity)
//	cumCash <= PD + S - K                          (gateway cap)
//
// Shipping has NO commission and NO coin component.
// F is non-refundable and never included.
// C is seller-side and never in buyer CashRefund.
type ProportionalRefundBreakdown struct {
	PD, S, C, K                                             int64
	Rpd, Rs                                                 int64
	CumProductRefundBefore, CumProductRefundAfter           int64
	CumShippingRefundBefore, CumShippingRefundAfter         int64
	CumCoinsRestoredBefore, CumCoinsRestoredAfter           int64
	CumCommissionReversedBefore, CumCommissionReversedAfter int64
	CashRefund, CoinDelta, CommissionDelta                  int64
	SellerComponent                                         int64
	RoundingAdjustment                                      int64
}

func CalculateProportionalRefundBreakdown(
	pd, s, c, k int64, rpd, rs int64,
	cumProductBefore, cumShippingBefore, cumCoinsBefore, cumCommissionBefore int64,
) (*ProportionalRefundBreakdown, error) {
	if pd <= 0 {
		return nil, fmt.Errorf("PD must be positive")
	}
	if s < 0 {
		return nil, fmt.Errorf("S must be non-negative")
	}
	if c < 0 || c > pd {
		return nil, fmt.Errorf("C must be in [0, PD]")
	}
	if k < 0 || k > pd {
		return nil, fmt.Errorf("K must be in [0, PD]")
	}
	if rpd < 0 || rs < 0 {
		return nil, fmt.Errorf("Rpd and Rs must be non-negative")
	}
	if rpd == 0 && rs == 0 {
		return nil, fmt.Errorf("refund must have non-zero product or shipping")
	}
	if cumProductBefore < 0 || cumProductBefore > pd {
		return nil, fmt.Errorf("cumProductBefore out of range")
	}
	if cumShippingBefore < 0 || cumShippingBefore > s {
		return nil, fmt.Errorf("cumShippingBefore out of range")
	}
	expectedCoinsBefore := proportionalFloor(cumProductBefore, k, pd)
	if cumCoinsBefore != expectedCoinsBefore {
		return nil, fmt.Errorf("cumCoinsBefore %d does not match canonical coin total %d derived from cumProductBefore=%d",
			cumCoinsBefore, expectedCoinsBefore, cumProductBefore)
	}
	expectedCommissionBefore := proportionalFloor(cumProductBefore, c, pd)
	if cumCommissionBefore != expectedCommissionBefore {
		return nil, fmt.Errorf("cumCommissionBefore %d does not match canonical commission total %d derived from cumProductBefore=%d",
			cumCommissionBefore, expectedCommissionBefore, cumProductBefore)
	}

	cumProductAfter := cumProductBefore + rpd
	cumShippingAfter := cumShippingBefore + rs
	if cumProductAfter > pd {
		return nil, fmt.Errorf("cumProductAfter %d > PD %d", cumProductAfter, pd)
	}
	if cumShippingAfter > s {
		return nil, fmt.Errorf("cumShippingAfter %d > S %d", cumShippingAfter, s)
	}

	// Commission delta: product-proportional, floor-based.
	commissionBefore := expectedCommissionBefore
	commissionAfter := proportionalFloor(cumProductAfter, c, pd)
	commissionDelta := commissionAfter - commissionBefore

	// Coin delta: product-proportional, floor-based.
	coinsBefore := expectedCoinsBefore
	coinsAfter := proportionalFloor(cumProductAfter, k, pd)
	coinDelta := coinsAfter - coinsBefore

	cumCommissionAfter := commissionAfter
	cumCoinsAfter := coinsAfter

	// CANONICAL: gateway cash refund = Rpd + Rs - CoinDelta.
	// Coins are restored via coin balance, not via gateway payment reversal.
	cashRefund := rpd + rs - coinDelta
	if cashRefund < 0 {
		return nil, fmt.Errorf("cashRefund cannot be negative: rpd=%d rs=%d coinDelta=%d", rpd, rs, coinDelta)
	}

	// Canonical cap: cumulative gateway cash after this refund must never exceed PD + S - K.
	cumCashAfter := cumProductAfter + cumShippingAfter - cumCoinsAfter
	if cumCashAfter > pd+s-k {
		return nil, fmt.Errorf("cumulative gateway cash %d exceeds cap PD+S-K=%d",
			cumCashAfter, pd+s-k)
	}

	// Accounting identity: cash + coins == product + shipping for this event.
	if cashRefund+coinDelta != rpd+rs {
		return nil, fmt.Errorf("accounting identity broken: cash(%d)+coins(%d) != rpd(%d)+rs(%d)",
			cashRefund, coinDelta, rpd, rs)
	}

	// Seller component: what the seller gives up economically.
	// Seller receives Rpd+Rs minus the commission they owe on the product portion.
	sellerComponent := rpd + rs - commissionDelta
	if sellerComponent < 0 {
		return nil, fmt.Errorf("sellerComponent negative: rpd+rs=%d commissionDelta=%d", rpd+rs, commissionDelta)
	}

	return &ProportionalRefundBreakdown{
		PD: pd, S: s, C: c, K: k, Rpd: rpd, Rs: rs,
		CumProductRefundBefore:      cumProductBefore,
		CumProductRefundAfter:       cumProductAfter,
		CumShippingRefundBefore:     cumShippingBefore,
		CumShippingRefundAfter:      cumShippingAfter,
		CumCoinsRestoredBefore:      cumCoinsBefore,
		CumCoinsRestoredAfter:       cumCoinsAfter,
		CumCommissionReversedBefore: cumCommissionBefore,
		CumCommissionReversedAfter:  cumCommissionAfter,
		CashRefund:                  cashRefund,
		CoinDelta:                   coinDelta,
		CommissionDelta:             commissionDelta,
		SellerComponent:             sellerComponent,
		RoundingAdjustment:          0,
	}, nil
}

func proportionalFloor(amount, numerator, denominator int64) int64 {
	if amount <= 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (amount * numerator) / denominator
}

// MaxGatewayRefund returns PD + S - K, the maximum cumulative gateway cash refund.
func MaxGatewayRefund(pd, s, k int64) int64 {
	if max := pd + s - k; max > 0 {
		return max
	}
	return 0
}

func parseMidtransRefundAmount(raw string) (int64, error) {
	if raw == "" {
		return 0, fmt.Errorf("empty amount")
	}
	var amount float64
	if _, err := fmt.Sscanf(raw, "%f", &amount); err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", raw, err)
	}
	if amount < 0 {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	return int64(amount), nil
}
