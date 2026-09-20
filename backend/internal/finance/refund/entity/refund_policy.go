// Package entity: canonical refund policy resolver (H2-A2, S2C2 rebase).
//
// CANONICAL S2C2:
//   - Commission C is seller-side — never part of buyer refund amount.
//   - Buyer refund = product (Rpd) + shipping (Rs).
//   - Coin restoration is product-proportional: floor(K * cumProductRefund / PD).
//   - Commission reversal is product-proportional: floor(C * cumProductRefund / PD).
//   - Shipping has NO commission component.
//   - Policy types: product_only (Rpd=PD, Rs=0), full (Rpd=PD, Rs=S), admin_review_required.
package entity

import "fmt"

type RefundPolicyType string

const (
	RefundPolicyProductOnly         RefundPolicyType = "product_only"
	RefundPolicyFull                RefundPolicyType = "full"
	RefundPolicyAdminReviewRequired RefundPolicyType = "admin_review_required"
)

type RefundPolicyResult struct {
	PolicyType     RefundPolicyType
	ProductAmount  int64 // Rpd
	ShippingAmount int64 // Rs
	CashRefund     int64 // Rpd + Rs (excludes C and F)
}

// OrderSnapshot carries ONLY the canonical buyer-side money components.
type OrderSnapshot struct {
	// DiscountedProduct is PD = (P − D), the DISCOUNTED product value.
	// It is the canonical PD and must never be fed the order's undiscounted
	// Subtotal (P): the producer derives it exclusively from the persisted
	// buyer-funded base (order.DiscountedProductAmount() = base − S).
	DiscountedProduct int64
	ShippingTotal     int64 // S
	CommissionAmount  int64 // C (seller-side, NOT buyer refund)
}

// ProductGross returns the canonical buyer-funded base PD + S, excluding the
// seller-side commission C and the non-refundable buyer payment fee F.
func (o OrderSnapshot) ProductGross() int64 { return o.DiscountedProduct + o.ShippingTotal }

type ErrAdminReviewRequired struct{ Reason RefundReason }
func (e *ErrAdminReviewRequired) Error() string {
	return fmt.Sprintf("refund reason %q requires admin review; seller cannot auto-approve", string(e.Reason))
}

func ResolveRefundPolicy(reason RefundReason, order OrderSnapshot) RefundPolicyResult {
	switch reason {
	case RefundReasonItemDamaged, RefundReasonDefectiveItem:
		return RefundPolicyResult{
			PolicyType: RefundPolicyProductOnly, ProductAmount: order.DiscountedProduct,
			ShippingAmount: 0, CashRefund: order.DiscountedProduct,
		}
	case RefundReasonItemNotReceived, RefundReasonWrongItem:
		return RefundPolicyResult{
			PolicyType: RefundPolicyFull, ProductAmount: order.DiscountedProduct,
			ShippingAmount: order.ShippingTotal, CashRefund: order.DiscountedProduct + order.ShippingTotal,
		}
	default:
		return RefundPolicyResult{PolicyType: RefundPolicyAdminReviewRequired}
	}
}

func (p RefundPolicyResult) IsSellerApprovable() bool {
	return p.PolicyType != RefundPolicyAdminReviewRequired
}
