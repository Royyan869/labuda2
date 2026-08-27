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

type OrderSnapshot struct {
	Subtotal         int64 // PD
	ShippingTotal    int64 // S
	CommissionAmount int64 // C (seller-side, NOT buyer refund)
}

func (o OrderSnapshot) ProductGross() int64 { return o.Subtotal + o.ShippingTotal }

type ErrAdminReviewRequired struct{ Reason RefundReason }
func (e *ErrAdminReviewRequired) Error() string {
	return fmt.Sprintf("refund reason %q requires admin review; seller cannot auto-approve", string(e.Reason))
}

func ResolveRefundPolicy(reason RefundReason, order OrderSnapshot) RefundPolicyResult {
	switch reason {
	case RefundReasonItemDamaged, RefundReasonDefectiveItem:
		return RefundPolicyResult{
			PolicyType: RefundPolicyProductOnly, ProductAmount: order.Subtotal,
			ShippingAmount: 0, CashRefund: order.Subtotal,
		}
	case RefundReasonItemNotReceived, RefundReasonWrongItem:
		return RefundPolicyResult{
			PolicyType: RefundPolicyFull, ProductAmount: order.Subtotal,
			ShippingAmount: order.ShippingTotal, CashRefund: order.Subtotal + order.ShippingTotal,
		}
	default:
		return RefundPolicyResult{PolicyType: RefundPolicyAdminReviewRequired}
	}
}

func (p RefundPolicyResult) IsSellerApprovable() bool {
	return p.PolicyType != RefundPolicyAdminReviewRequired
}
