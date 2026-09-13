package main

import (
	"os"
	"strings"
	"testing"
)

// admin_authority_contract_test.go — canonical authority contract for the
// /api/v1/admin route group (ADMIN-AUTHORITY scope).
//
// Canonical Labuda model:
//
//	admin membership (coarse internal boundary)
//	AND
//	explicit required capability (granular business action)
//	=
//	ALLOW
//
// Admin identity/session discovery (GET /api/v1/admin/me) is deliberately
// NOT capability-gated: its canonical purpose is to report who the current
// admin member is and which capabilities the backend currently recognizes
// for this actor — the admin web client needs it on startup, immediately
// after role verification, to render capability-aware UI. It is
// authenticated + admin-membership only, and it grants nothing.
//
// This file is a source-level negative proof: every privileged admin route
// must be registered INSIDE the RequireAdminMiddleware group and must carry
// an explicit per-route capability gate. A route registered before the
// group, or registered without a capability gate, fails this contract.

// TestAdminRoutes_PrivilegedActions_RequireCapabilityAndAdminGroup proves
// that no privileged admin route bypasses the canonical predicate:
//
//	authenticated AND admin membership AND explicit required capability
//
// It walks every adminRoutes route registration in routes_core.go and
// asserts each carries a RequireCapability / RequireAnyCapability call in
// its route block. The two classified exceptions are asserted explicitly:
//   - GET /admin/me  → identity/session discovery (not a business action)
//   - GET /admin/test → dev/diagnostic echo behind admin membership only
func TestAdminRoutes_PrivilegedActions_RequireCapabilityAndAdminGroup(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	code := string(src)

	groupStart := strings.Index(code, `adminRoutes := v1.Group("/admin")`)
	if groupStart < 0 {
		t.Fatal("admin route group not found in routes_core.go")
	}
	groupUse := strings.Index(code[groupStart:], `adminRoutes.Use(middleware.RequireAdminMiddleware(deps.RoleChecker))`)
	if groupUse < 0 {
		t.Fatal("admin route group must apply RequireAdminMiddleware (coarse internal membership boundary)")
	}

	// Walk every adminRoutes route registration from the group start.
	rest := code[groupStart:]
	pos := 0
	checked := 0
	gates := 0
	for {
		idx := strings.Index(rest[pos:], "adminRoutes.")
		if idx < 0 {
			break
		}
		idx += pos

		// Only route registrations (GET/POST/PUT/DELETE), not .Use or .Group.
		head := rest[idx:]
		isRoute := false
		for _, m := range []string{"adminRoutes.GET(", "adminRoutes.POST(", "adminRoutes.PUT(", "adminRoutes.DELETE("} {
			if strings.HasPrefix(head, m) {
				isRoute = true
				break
			}
		}
		if !isRoute {
			pos = idx + len("adminRoutes.")
			continue
		}

		lineEnd := strings.IndexByte(rest[idx:], '\n')
		if lineEnd < 0 {
			lineEnd = len(rest) - idx
		}
		routeLine := rest[idx : idx+lineEnd]
		checked++

		switch {
		case strings.Contains(routeLine, `"/me"`):
			// Identity/session discovery — intentionally capability-free.
			// Canonical purpose: report current admin member + recognized
			// capabilities. Grants nothing; covered by
			// TestAdminMe_IdentityDiscovery_Semantics in
			// internal/platform/admin/delivery/http.
		case strings.Contains(routeLine, `"/test"`):
			// Dev/diagnostic echo — admin membership boundary only, no
			// business action, no data access. Classified, not hidden.
		default:
			// Privileged business action: the route block (registration
			// through handler call) MUST contain an explicit capability gate.
			blockEnd := idx + lineEnd + 400
			if blockEnd > len(rest) {
				blockEnd = len(rest)
			}
			block := rest[idx:blockEnd]
			if !strings.Contains(block, "RequireCapability(") &&
				!strings.Contains(block, "RequireAnyCapability(") {
				t.Fatalf("PRIVILEGED ROUTE WITHOUT CAPABILITY GATE: %s", routeLine)
			}
			gates++
		}

		pos = idx + len("adminRoutes.")
	}

	if checked == 0 {
		t.Fatal("no adminRoutes registrations found — route inventory drift")
	}
	if gates < 30 {
		t.Fatalf("implausibly few capability-gated admin routes (%d) — route inventory drift", gates)
	}
}

// TestAdminRoutes_NoRouteRegisteredBeforeAdminGroup fails if any privileged
// admin route is registered outside the RequireAdminMiddleware group (the
// historical failure mode: a /admin/... path mounted on v1 directly).
func TestAdminRoutes_NoRouteRegisteredBeforeAdminGroup(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	code := string(src)

	groupStart := strings.Index(code, `adminRoutes := v1.Group("/admin")`)
	if groupStart < 0 {
		t.Fatal("admin route group not found in routes_core.go")
	}

	// v1 route registrations that target /admin paths before the group.
	prefix := code[:groupStart]
	for _, bad := range []string{`v1.GET("/admin`, `v1.POST("/admin`, `v1.PUT("/admin`, `v1.DELETE("/admin`} {
		if strings.Contains(prefix, bad) {
			t.Fatalf("REGRESSION: privileged admin route mounted outside the admin group: %s", bad)
		}
	}
}

// TestAdminConfigUpdate_SplitCapabilities_UseCanonicalMiddleware pins the
// PUT /admin/config/:key split-capability gate to the canonical
// middleware.RequireAnyCapability implementation (admin membership +
// one-of-two capabilities), replacing the removed local duplicate helper.
func TestAdminConfigUpdate_SplitCapabilities_UseCanonicalMiddleware(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}
	code := string(src)

	if strings.Contains(code, "func requireAnyCapability(") {
		t.Fatal("REGRESSION: local requireAnyCapability helper resurrected — use middleware.RequireAnyCapability")
	}

	routeIdx := strings.Index(code, `adminRoutes.PUT("/config/:key",`)
	if routeIdx < 0 {
		t.Fatal("MISSING: PUT /admin/config/:key route not registered")
	}
	block := code[routeIdx : routeIdx+400]
	if !strings.Contains(block, `middleware.RequireAnyCapability("config.update.general", "config.update.financial")`) {
		t.Fatal("MISSING: PUT /admin/config/:key must be gated by middleware.RequireAnyCapability(config.update.general, config.update.financial)")
	}
	if !strings.Contains(block, "deps.PlatformConfigHandler.UpdateConfig") {
		t.Fatal("MISSING: PUT /admin/config/:key must route to PlatformConfigHandler.UpdateConfig")
	}
}
