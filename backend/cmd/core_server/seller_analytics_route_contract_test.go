package main

import (
	"os"
	"strings"
	"testing"
)

// TestSellerAnalyticsRoute_UsesWorkspaceProfileGate is a source-level guard
// proving GET /api/v1/seller/analytics is registered inside the SELLER
// WORKSPACE group (RequireSellerProfileMiddleware = profile existence only,
// survives subscription expiry), and NOT inside the market-authority group
// (RequireSellerMiddleware = active subscription).
//
// Business contract: Seller Analytics covers all of the seller's products,
// including historical/terminal products, so an expired seller must still be
// able to read their own analytics. Registering under the market gate would
// silently lock expired sellers out of their historical data.
func TestSellerAnalyticsRoute_UsesWorkspaceProfileGate(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	code := string(src)

	// The market-authority group (active subscription) must NOT carry analytics.
	if strings.Contains(code, `sellerRoutes.GET("/analytics"`) {
		t.Fatal("REGRESSION: /seller/analytics must NOT be registered under sellerRoutes (market-authority gate); expired sellers would lose access to historical analytics")
	}

	// The workspace group (profile-only) must carry analytics.
	workspaceGroupStart := strings.Index(code, `sellerWorkspaceRoutes := v1.Group("/seller")`)
	if workspaceGroupStart < 0 {
		t.Fatal("seller workspace route group not found")
	}
	routeIdx := strings.Index(code, `sellerWorkspaceRoutes.GET("/analytics"`)
	if routeIdx < 0 {
		t.Fatal("MISSING: /seller/analytics route not registered under sellerWorkspaceRoutes")
	}
	if routeIdx < workspaceGroupStart {
		t.Fatal("REGRESSION: /seller/analytics must be inside the workspace group")
	}

	// The workspace group must be gated by RequireSellerProfileMiddleware
	// (profile existence), not RequireSellerMiddleware (active subscription).
	workspaceBlock := code[workspaceGroupStart : workspaceGroupStart+400]
	if !strings.Contains(workspaceBlock, "middleware.RequireSellerProfileMiddleware") {
		t.Fatal("MISSING: sellerWorkspaceRoutes must be gated by RequireSellerProfileMiddleware")
	}
	if strings.Contains(workspaceBlock, "middleware.RequireSellerMiddleware") {
		t.Fatal("REGRESSION: sellerWorkspaceRoutes must NOT be gated by RequireSellerMiddleware (that would lock expired sellers out)")
	}

	// The route must dispatch to the SellerHandler.GetAnalytics handler.
	routeBlock := code[routeIdx : routeIdx+120]
	if !strings.Contains(routeBlock, "deps.SellerHandler.GetAnalytics") {
		t.Fatal("MISSING: /seller/analytics must route to SellerHandler.GetAnalytics")
	}
}
