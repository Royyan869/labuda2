package http

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/labuda/backend/internal/commerce/forsale/entity"
)

// TestRequiresMarketAuthorityForPublish is the PASS_18O regression test for
// the PASS_18M market-authority bypass: UpdateForSale used to gate the
// market-authority check on the for_sale's CURRENT visibility
// (`for_sale.Visibility == entity.ForSaleVisibilityPublic`), but
// entity.Publish() unconditionally forces visibility to public on any
// draft→active transition. A private draft published via a status-only
// update (exactly what mobile sends — see
// for_sale_repository_impl.dart:346-349, "Update ONLY status field (not
// visibility - this was a bug)") never had its current visibility flipped to
// public first, so the old condition was always false and the authority
// check was skipped entirely.
//
// The fixed predicate depends only on the status transition, never on
// current/requested visibility, so every path that reaches entity.Publish()
// is caught.
func TestRequiresMarketAuthorityForPublish(t *testing.T) {
	cases := []struct {
		name          string
		currentStatus entity.ForSaleStatus
		newStatus     entity.ForSaleStatus
		want          bool
	}{
		{
			name:          "private draft status-only publish requires authority (the exact PASS_18M bug)",
			currentStatus: entity.ForSaleStatusDraft,
			newStatus:     entity.ForSaleStatusActive,
			want:          true,
		},
		{
			name:          "draft to withdrawn does not require authority (no public exposure)",
			currentStatus: entity.ForSaleStatusDraft,
			newStatus:     entity.ForSaleStatusWithdrawn,
			want:          false,
		},
		{
			name:          "draft to draft no-op does not require authority",
			currentStatus: entity.ForSaleStatusDraft,
			newStatus:     entity.ForSaleStatusDraft,
			want:          false,
		},
		{
			name:          "active to withdrawn does not require authority (leaving the market, not entering it)",
			currentStatus: entity.ForSaleStatusActive,
			newStatus:     entity.ForSaleStatusWithdrawn,
			want:          false,
		},
		{
			name:          "active to sold does not require authority",
			currentStatus: entity.ForSaleStatusActive,
			newStatus:     entity.ForSaleStatusSold,
			want:          false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := requiresMarketAuthorityForPublish(tc.currentStatus, tc.newStatus)
			if got != tc.want {
				t.Errorf("requiresMarketAuthorityForPublish(%q, %q) = %v, want %v",
					tc.currentStatus, tc.newStatus, got, tc.want)
			}
		})
	}
}

// TestUpdateForSale_MarketAuthorityCheckIsUnconditional is a
// structural regression test guarding against the PASS_18M bug pattern
// reappearing: the market-authority check inside the draft→active publish
// branch must be called unconditionally (via requiresMarketAuthorityForPublish,
// which does not inspect visibility at all), not re-wrapped in a nested
// "if for_sale.Visibility == ... Public" condition that would silently skip it
// for status-only updates from a private draft.
func TestUpdateForSale_MarketAuthorityCheckIsUnconditional(t *testing.T) {
	f, err := os.Open("for_sale_handler.go")
	if err != nil {
		t.Fatalf("failed to open for_sale_handler.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFunc := false
	foundFunc := false
	foundAuthorityCheck := false
	braceDepth := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inFunc {
			if strings.Contains(line, "func (h *ForSaleHandler) UpdateForSale(") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// The exact regression pattern: gating the authority check on the
		// for_sale's *current* visibility inside the publish branch.
		if strings.Contains(line, "if for_sale.Visibility == entity.ForSaleVisibilityPublic {") {
			t.Fatalf("for_sale_handler.go:%d reintroduces the PASS_18M bug: "+
				"market-authority check must not be gated on current visibility inside the publish branch", lineNum)
		}

		// After P1-2 publish convergence, the market-authority check was
		// moved into ForSaleService.Publish(). The handler delegates via
		// h.for_saleService.Publish(), which internally calls
		// CheckMarketAuthorityForForSale. We verify delegation here.
		if strings.Contains(line, "h.for_saleService.Publish(") {
			foundAuthorityCheck = true
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan for_sale_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("UpdateForSale function not found — for_sale update logic may have moved")
	}
	if !foundAuthorityCheck {
		t.Fatal("h.for_saleService.Publish() delegation not found inside UpdateForSale — market authority gate may have been removed or inlined")
	}
}

// ============================================================================
// P1-2: PUBLISH CONVERGENCE — ONE CANONICAL PUBLISH AUTHORITY
// ============================================================================

// TestUpdateForSale_NoInlinePublishMutation is a structural regression test
// proving the P1-2 fix: the PUT handler must NOT call for_sale.Publish()
// (the entity method) directly. Publish state mutation must flow through
// ForSaleService.Publish() — the ONE canonical publish authority with
// proper GetForUpdate() row locking.
func TestUpdateForSale_NoInlinePublishMutation(t *testing.T) {
	f, err := os.Open("for_sale_handler.go")
	if err != nil {
		t.Fatalf("failed to open for_sale_handler.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFunc := false
	foundFunc := false
	braceDepth := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inFunc {
			if strings.Contains(line, "func (h *ForSaleHandler) UpdateForSale(") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// P1-2 regression: the handler must NOT call for_sale.Publish() directly.
		// The only allowed publish path is h.for_saleService.Publish().
		if strings.Contains(line, "for_sale.Publish()") {
			t.Fatalf("for_sale_handler.go:%d: P1-2 regression — handler contains inline "+
				"for_sale.Publish() call. Publish must go through ForSaleService.Publish()", lineNum)
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan for_sale_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("UpdateForSale function not found — for_sale update logic may have moved")
	}
}

// TestUpdateForSale_DelegatesPublishToService is a structural regression test
// proving the P1-2 fix: the PUT handler must call h.for_saleService.Publish()
// (the service method) when a publish intent is detected, gaining proper
// GetForUpdate() row locking and all canonical checks.
func TestUpdateForSale_DelegatesPublishToService(t *testing.T) {
	f, err := os.Open("for_sale_handler.go")
	if err != nil {
		t.Fatalf("failed to open for_sale_handler.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFunc := false
	foundFunc := false
	foundServicePublish := false
	braceDepth := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inFunc {
			if strings.Contains(line, "func (h *ForSaleHandler) UpdateForSale(") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// P1-2: handler must delegate publish to ForSaleService.Publish()
		if strings.Contains(line, "h.for_saleService.Publish(") {
			foundServicePublish = true
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan for_sale_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("UpdateForSale function not found — for_sale update logic may have moved")
	}
	if !foundServicePublish {
		t.Fatal("h.for_saleService.Publish() not found inside UpdateForSale — " +
			"publish may not be delegated to the canonical authority")
	}
}

// TestUpdateForSale_NoInlinePublishGateChecks is a structural regression test
// proving the P1-2 fix: the PUT handler must NOT independently call
// EnsureShippingConfigured or EnsureFarmAddressValid for the publish path.
// Those checks live inside ForSaleService.Publish() and must not be
// duplicated in the handler.
func TestUpdateForSale_NoInlinePublishGateChecks(t *testing.T) {
	f, err := os.Open("for_sale_handler.go")
	if err != nil {
		t.Fatalf("failed to open for_sale_handler.go: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFunc := false
	foundFunc := false
	braceDepth := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inFunc {
			if strings.Contains(line, "func (h *ForSaleHandler) UpdateForSale(") {
				inFunc = true
				foundFunc = true
				braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// P1-2: handler must NOT call EnsureShippingConfigured or
		// EnsureFarmAddressValid — those checks live inside Publish().
		if strings.Contains(line, "EnsureShippingConfigured") {
			t.Fatalf("for_sale_handler.go:%d: P1-2 regression — handler contains "+
				"inline EnsureShippingConfigured call. This check must live in "+
				"ForSaleService.Publish() only.", lineNum)
		}
		if strings.Contains(line, "EnsureFarmAddressValid") {
			t.Fatalf("for_sale_handler.go:%d: P1-2 regression — handler contains "+
				"inline EnsureFarmAddressValid call. This check must live in "+
				"ForSaleService.Publish() only.", lineNum)
		}

		if braceDepth <= 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan for_sale_handler.go: %v", err)
	}
	if !foundFunc {
		t.Fatal("UpdateForSale function not found — for_sale update logic may have moved")
	}
}
