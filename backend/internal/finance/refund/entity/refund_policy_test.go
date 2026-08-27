package entity

import (
	"testing"

	"github.com/google/uuid"
)

// ============================================================================
// Entity-level tests for the canonical refund policy resolver (H2-A2, S2C2).
// No DB or service wiring needed — pure policy logic.
//
// CANONICAL CONTRACT:
//   - Buyer refund = product (Rpd) + shipping (Rs). Commission C is
//     seller-side and NEVER part of the buyer refund amount.
//   - ProductGross() = PD + S (excludes commission). The rejected P+S+C
//     "Gross()" model does not exist.
//   - Policy types: product_only (Rpd=PD, Rs=0), full (Rpd=PD, Rs=S),
//     admin_review_required.
// ============================================================================

var testOrder = OrderSnapshot{
	Subtotal:         100_000,
	ShippingTotal:    25_000,
	CommissionAmount: 6_250,
}

func TestOrderSnapshot_ProductGross(t *testing.T) {
	got := testOrder.ProductGross()
	want := int64(125_000) // PD + S; commission C is NOT buyer refund
	if got != want {
		t.Fatalf("ProductGross()=%d want %d", got, want)
	}
}

// --- Product-only reasons: item_damaged, defective_item ---

func TestResolveRefundPolicy_ItemDamaged_ProductOnly(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonItemDamaged, testOrder)
	if p.PolicyType != RefundPolicyProductOnly {
		t.Fatalf("policy=%s want product_only", p.PolicyType)
	}
	if p.CashRefund != testOrder.Subtotal {
		t.Fatalf("cash_refund=%d want %d (Subtotal)", p.CashRefund, testOrder.Subtotal)
	}
	if !p.IsSellerApprovable() {
		t.Fatal("item_damaged should be seller-approvable")
	}
}

func TestResolveRefundPolicy_DefectiveItem_ProductOnly(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonDefectiveItem, testOrder)
	if p.PolicyType != RefundPolicyProductOnly {
		t.Fatalf("policy=%s want product_only", p.PolicyType)
	}
	if p.CashRefund != testOrder.Subtotal {
		t.Fatalf("cash_refund=%d want %d (Subtotal)", p.CashRefund, testOrder.Subtotal)
	}
	if !p.IsSellerApprovable() {
		t.Fatal("defective_item should be seller-approvable")
	}
}

func TestResolveRefundPolicy_ProductOnly_DoesNotIncludeShipping(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonItemDamaged, testOrder)
	if p.CashRefund == testOrder.ProductGross() {
		t.Fatal("product_only must NOT equal ProductGross (must exclude shipping)")
	}
	if p.CashRefund != testOrder.Subtotal {
		t.Fatalf("product_only cash_refund=%d want Subtotal=%d", p.CashRefund, testOrder.Subtotal)
	}
}

// --- Full refund reasons: item_not_received, wrong_item ---

func TestResolveRefundPolicy_ItemNotReceived_Full(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonItemNotReceived, testOrder)
	if p.PolicyType != RefundPolicyFull {
		t.Fatalf("policy=%s want full", p.PolicyType)
	}
	if p.CashRefund != testOrder.ProductGross() {
		t.Fatalf("cash_refund=%d want %d (ProductGross = PD+S, excludes C)", p.CashRefund, testOrder.ProductGross())
	}
	if !p.IsSellerApprovable() {
		t.Fatal("item_not_received should be seller-approvable")
	}
}

func TestResolveRefundPolicy_WrongItem_Full(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonWrongItem, testOrder)
	if p.PolicyType != RefundPolicyFull {
		t.Fatalf("policy=%s want full", p.PolicyType)
	}
	if p.CashRefund != testOrder.ProductGross() {
		t.Fatalf("cash_refund=%d want %d (ProductGross = PD+S, excludes C)", p.CashRefund, testOrder.ProductGross())
	}
	if !p.IsSellerApprovable() {
		t.Fatal("wrong_item should be seller-approvable")
	}
}

// --- Admin-review-required reasons ---

func TestResolveRefundPolicy_ItemNotAsDescribed_AdminReview(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonItemNotAsDescribed, testOrder)
	if p.PolicyType != RefundPolicyAdminReviewRequired {
		t.Fatalf("policy=%s want admin_review_required", p.PolicyType)
	}
	if p.CashRefund != 0 {
		t.Fatalf("admin_review cash_refund=%d want 0", p.CashRefund)
	}
	if p.IsSellerApprovable() {
		t.Fatal("item_not_as_described must NOT be seller-approvable")
	}
}

func TestResolveRefundPolicy_DeliveryDelay_AdminReview(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonDeliveryDelay, testOrder)
	if p.PolicyType != RefundPolicyAdminReviewRequired {
		t.Fatalf("policy=%s want admin_review_required", p.PolicyType)
	}
	if p.IsSellerApprovable() {
		t.Fatal("delivery_delay must NOT be seller-approvable")
	}
}

func TestResolveRefundPolicy_ChangeOfMind_AdminReview(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonChangeOfMind, testOrder)
	if p.PolicyType != RefundPolicyAdminReviewRequired {
		t.Fatalf("policy=%s want admin_review_required", p.PolicyType)
	}
	if p.IsSellerApprovable() {
		t.Fatal("change_of_mind must NOT be seller-approvable")
	}
}

func TestResolveRefundPolicy_Other_AdminReview(t *testing.T) {
	p := ResolveRefundPolicy(RefundReasonOther, testOrder)
	if p.PolicyType != RefundPolicyAdminReviewRequired {
		t.Fatalf("policy=%s want admin_review_required", p.PolicyType)
	}
	if p.IsSellerApprovable() {
		t.Fatal("other must NOT be seller-approvable")
	}
}

// --- Buyer requested_amount cannot inflate refund above policy ---

func TestResolveRefundPolicy_BuyerRequestedAmountIrrelevant(t *testing.T) {
	// Even if buyer requests full ProductGross, item_damaged policy only gives Subtotal
	p := ResolveRefundPolicy(RefundReasonItemDamaged, testOrder)
	buyerRequestedAmount := testOrder.ProductGross() // buyer wants everything
	if p.CashRefund >= buyerRequestedAmount {
		t.Fatalf("policy cash_refund %d should be less than buyer requested %d for product_only", p.CashRefund, buyerRequestedAmount)
	}
	if p.CashRefund != testOrder.Subtotal {
		t.Fatalf("policy cash_refund=%d want Subtotal=%d regardless of buyer request", p.CashRefund, testOrder.Subtotal)
	}
}

// --- Edge: zero shipping order ---

func TestResolveRefundPolicy_ZeroShipping_ProductOnlyEqualsSubtotal(t *testing.T) {
	order := OrderSnapshot{Subtotal: 50_000, ShippingTotal: 0, CommissionAmount: 2_500}
	p := ResolveRefundPolicy(RefundReasonItemDamaged, order)
	if p.CashRefund != 50_000 {
		t.Fatalf("cash_refund=%d want 50000", p.CashRefund)
	}
	// product_only and full differ only by shipping (never commission)
	pFull := ResolveRefundPolicy(RefundReasonItemNotReceived, order)
	if pFull.CashRefund != 50_000 {
		t.Fatalf("full cash_refund=%d want 50000 (no shipping, no commission)", pFull.CashRefund)
	}
}

// --- ErrAdminReviewRequired ---

func TestErrAdminReviewRequired_ErrorMessage(t *testing.T) {
	err := &ErrAdminReviewRequired{Reason: RefundReasonChangeOfMind}
	msg := err.Error()
	if msg == "" {
		t.Fatal("error message should not be empty")
	}
}

// --- Complete reason sweep ---

func TestResolveRefundPolicy_AllReasons_Classified(t *testing.T) {
	reasons := []RefundReason{
		RefundReasonItemNotReceived,
		RefundReasonItemNotAsDescribed,
		RefundReasonItemDamaged,
		RefundReasonDefectiveItem,
		RefundReasonWrongItem,
		RefundReasonChangeOfMind,
		RefundReasonDeliveryDelay,
		RefundReasonOther,
	}
	for _, reason := range reasons {
		p := ResolveRefundPolicy(reason, testOrder)
		if p.PolicyType == "" {
			t.Fatalf("reason %q returned empty policy type", reason)
		}
		switch p.PolicyType {
		case RefundPolicyProductOnly:
			if p.CashRefund != testOrder.Subtotal {
				t.Fatalf("reason %q: product_only cash_refund=%d want %d", reason, p.CashRefund, testOrder.Subtotal)
			}
		case RefundPolicyFull:
			if p.CashRefund != testOrder.ProductGross() {
				t.Fatalf("reason %q: full cash_refund=%d want %d", reason, p.CashRefund, testOrder.ProductGross())
			}
		case RefundPolicyAdminReviewRequired:
			if p.CashRefund != 0 {
				t.Fatalf("reason %q: admin_review cash_refund=%d want 0", reason, p.CashRefund)
			}
		default:
			t.Fatalf("reason %q: unknown policy type %q", reason, p.PolicyType)
		}
	}
}

// --- Seller cannot pass custom amount (structural proof) ---

func TestSellerApprove_AmountIsSystemComputed_NotBuyerClaim(t *testing.T) {
	// Create a refund where buyer requests full ProductGross
	r := NewRefund(uuid.New(), uuid.New(), uuid.New(), RefundReasonItemDamaged, nil, testOrder.ProductGross())

	// Policy says product_only = Subtotal
	policy := ResolveRefundPolicy(r.Reason, testOrder)
	if policy.CashRefund == r.RequestedAmount {
		t.Fatal("policy amount should differ from buyer's full-gross request for product_only")
	}

	// Seller approve with policy amount
	err := r.SellerApprove(policy.CashRefund, nil, r.CreatedAt)
	if err != nil {
		t.Fatalf("approve error: %v", err)
	}
	if *r.SellerApprovedAmount != testOrder.Subtotal {
		t.Fatalf("approved_amount=%d want %d (Subtotal, not buyer's %d)", *r.SellerApprovedAmount, testOrder.Subtotal, r.RequestedAmount)
	}
}

// --- PASS_18V: buyer payment fee is explicitly non-refundable ---
//
// OrderSnapshot deliberately has no ServiceFeeAmount/buyer-payment-fee
// field, so it is structurally impossible for any seller-approvable policy
// (product_only or full) to include the buyer's payment method fee. The
// buyer payment fee is a payment-processing charge, realized as platform
// revenue at settlement (see finance/application.RecordBuyerPaymentFeeRevenue);
// it is never part of what a refund reverses, regardless of order outcome.
func TestRefundPolicy_BuyerPaymentFeeIsStructurallyExcluded(t *testing.T) {
	orderWithFee := OrderSnapshot{
		Subtotal:         100_000,
		ShippingTotal:    25_000,
		CommissionAmount: 6_250,
		// NOTE: no field exists here for buyer payment fee (e.g. 4_987 for a
		// credit_card checkout) — it cannot leak into ProductGross() even if a
		// caller wanted it to.
	}

	full := ResolveRefundPolicy(RefundReasonItemNotReceived, orderWithFee)
	if full.CashRefund != 125_000 {
		t.Fatalf("full policy cash_refund=%d want 125000 (PD+S; commission and fee excluded)", full.CashRefund)
	}

	productOnly := ResolveRefundPolicy(RefundReasonItemDamaged, orderWithFee)
	if productOnly.CashRefund != 100_000 {
		t.Fatalf("product_only policy cash_refund=%d want 100000 (Subtotal only, fee excluded)", productOnly.CashRefund)
	}
}
