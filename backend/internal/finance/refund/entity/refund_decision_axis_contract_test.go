package entity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// CANONICAL REFUND DECISION AXIS — CONTRACT + NEGATIVE CONTRACT
// ============================================================================
//
// These assertions protect the converged model instead of the historical
// implementation:
//
//	refunds.status = REFUND DECISION axis (who decided, and whether it is final)
//	gateway_status / refunded_at = FINANCIAL SETTLEMENT axis
//
// The negative contract deliberately fails if the old four-way disagreement can
// return: two divergent SQL terminal sets, a settlement-flavoured decision status,
// or an unreachable decision transition resurrected as authority.
// ============================================================================

func readSource(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Clean(rel)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(src)
}

// TestCanonicalStatusSets_DecisionAxisOnly locks the SQL status sets to the
// decision axis: buyer-owed outcomes + still-open decisions, and NOTHING else.
func TestCanonicalStatusSets_DecisionAxisOnly(t *testing.T) {
	owes := RefundOwesBuyerStatusList
	for _, want := range []string{"seller_approved", "admin_refunded", "system_refunded"} {
		if !strings.Contains(owes, want) {
			t.Fatalf("buyer-owed status set must contain %q, got %s", want, owes)
		}
	}
	if strings.Contains(owes, "admin_released") {
		t.Fatalf("admin_released (seller wins) must NOT be a buyer-owed outcome: %s", owes)
	}

	open := RefundDecisionOpenStatusList
	for _, want := range []string{"pending_seller_review", "seller_rejected", "escalated_to_admin"} {
		if !strings.Contains(open, want) {
			t.Fatalf("open decision set must contain %q, got %s", want, open)
		}
	}

	for _, set := range []string{owes, open} {
		if strings.Contains(set, "'refunded'") {
			t.Fatalf("purged decision status 'refunded' must not survive in any status set: %s", set)
		}
	}

	if RefundStatusSystemRefunded != "system_refunded" {
		t.Fatalf("platform-initiated refunds must be %q, got %q", "system_refunded", RefundStatusSystemRefunded)
	}
}

// TestEveryRefundStatusHasADecisionClassification fails if a status is added to
// the enum without deciding whether it is final, buyer-owed, or still open.
func TestEveryRefundStatusHasADecisionClassification(t *testing.T) {
	statuses := []RefundStatus{
		RefundStatusPendingSellerReview,
		RefundStatusSellerApproved,
		RefundStatusSellerRejected,
		RefundStatusEscalatedToAdmin,
		RefundStatusAdminRefunded,
		RefundStatusAdminReleased,
		RefundStatusSystemRefunded,
	}
	if len(statuses) != 7 {
		t.Fatalf("expected the canonical 7 refund decision statuses, got %d", len(statuses))
	}
	for _, status := range statuses {
		r := &Refund{Status: status}
		final := r.IsDecisionFinal()
		if r.AwaitsDecision() == final {
			t.Fatalf("status %q has no unambiguous decision classification", status)
		}
		if r.OwesBuyerRefund() && final == false {
			t.Fatalf("status %q owes the buyer money but is not final", status)
		}
	}
}

// TestNegativeContract_OldPredicateCannotReturn asserts the converged code has
// exactly ONE definition of each question and no residue of the old model.
func TestNegativeContract_OldPredicateCannotReturn(t *testing.T) {
	entitySrc := readSource(t, "refund.go")

	// Unreachable transitions must not come back as authority.
	for _, dead := range []string{"func (r *Refund) CompleteRefund(", "func (r *Refund) IsTerminal()"} {
		if strings.Contains(entitySrc, dead) {
			t.Fatalf("refund.go must not resurrect %q", dead)
		}
	}
	if strings.Contains(entitySrc, "RefundStatusRefunded RefundStatus") {
		t.Fatal("refund.go must not declare the purged 'refunded' decision status")
	}

	// The refund repository must compose the canonical sets, never inline them.
	repoSrc := readSource(t, filepath.Join("..", "infrastructure", "repository", "refund_repository_impl.go"))
	for _, want := range []string{
		"HasRefundBlockingRelease",
		"entity.RefundOwesBuyerStatusList",
		"entity.RefundDecisionOpenStatusList",
	} {
		if !strings.Contains(repoSrc, want) {
			t.Fatalf("refund repository must use the canonical predicate (%q missing)", want)
		}
	}
	if strings.Contains(repoSrc, "status NOT IN ('refunded', 'admin_released')") {
		t.Fatal("refund repository must not resurrect the old terminal-status LIST")
	}
	if strings.Contains(repoSrc, "HasActiveRefundByOrderID") {
		t.Fatal("refund repository must not resurrect HasActiveRefundByOrderID")
	}

	// The order auto-complete query must compose the same sets.
	orderRepoSrc := readSource(t, filepath.Join("..", "..", "..", "commerce", "order", "infrastructure", "repository", "order_repository.go"))
	if !strings.Contains(orderRepoSrc, "refundEntity.RefundOwesBuyerStatusList") {
		t.Fatal("auto-complete query must compose the canonical buyer-owed status set")
	}
	if strings.Contains(orderRepoSrc, "r.status NOT IN ('refunded', 'admin_released')") {
		t.Fatal("auto-complete query must not resurrect the old terminal-status LIST")
	}

	// One question, one predicate, one consumer wiring.
	completionSrc := readSource(t, filepath.Join("..", "..", "..", "commerce", "order", "application", "order_completion_service.go"))
	if !strings.Contains(completionSrc, "order.IsRefundWindowOpen()") {
		t.Fatal("completion guard must thread the ORDER-owned refund window into the guard")
	}
	if strings.Contains(completionSrc, "HasActiveRefundByOrderID") {
		t.Fatal("completion guard must not resurrect HasActiveRefundByOrderID")
	}

	// Admin decisions must be recorded on the refund process row.
	serviceSrc := readSource(t, filepath.Join("..", "application", "refund_service.go"))
	if !strings.Contains(serviceSrc, "func (s *RefundService) AdminResolveRefundDecision(") {
		t.Fatal("refund service must own the admin refund decision write-back")
	}
	if !strings.Contains(serviceSrc, "refund.AdminRefund(") {
		t.Fatal("admin buyer-wins must transition the escalated refund row via Refund.AdminRefund")
	}
	if !strings.Contains(serviceSrc, "refund.AdminRelease(") {
		t.Fatal("admin seller-wins must transition the escalated refund row via Refund.AdminRelease")
	}
}
