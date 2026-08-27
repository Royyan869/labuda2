package main

import (
	"os"
	"strings"
	"testing"
)

// TestFinanceSummaryRoute_RequiresCapability is a source-level guard
// (PASS_18Z) proving GET /api/v1/admin/finance/summary is registered inside
// the admin route group (RequireAdminMiddleware) AND gated by an explicit
// RequireCapability call — never open to any authenticated user, and never
// added outside the admin group by accident.
func TestFinanceSummaryRoute_RequiresCapability(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	code := string(src)

	adminGroupStart := strings.Index(code, `adminRoutes := v1.Group("/admin")`)
	if adminGroupStart < 0 {
		t.Fatal("admin route group not found in routes_core.go")
	}

	routeIdx := strings.Index(code, `adminRoutes.GET("/finance/summary",`)
	if routeIdx < 0 {
		t.Fatal("MISSING: GET /admin/finance/summary route not registered")
	}
	if routeIdx < adminGroupStart {
		t.Fatal("REGRESSION: /finance/summary must be registered inside the admin route group, not before it")
	}

	// The capability requirement must appear between the route registration
	// and the handler call on the following lines (same route block).
	block := code[routeIdx : routeIdx+300]
	if !strings.Contains(block, "middleware.RequireCapability(") {
		t.Fatal("MISSING: /finance/summary must be gated by middleware.RequireCapability")
	}
	if !strings.Contains(block, "deps.AdminFinanceHandler.GetSummary") {
		t.Fatal("MISSING: /finance/summary must route to AdminFinanceHandler.GetSummary")
	}
}
