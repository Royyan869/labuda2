package http

// content_update_authority_contract_test.go — source-level negative proof
// for the canonical content-update authority (ADMIN-AUTHORITY follow-up).
//
// Locked owner decision: the UpdateContent handler's membership-only admin
// bypass was dead code (the service rejects every non-owner with
// ErrOwnerRequired, previously surfaced as a 500) and no existing capability
// represents "edit another user's content". The branch was REMOVED — content
// update is owner-only, with no admin path and no capability gate.
//
// Required matrix (adapted to the removal decision — PATH B does not exist):
//
//	CASE 1  owner                                 → allowed (service-level tests)
//	CASE 2  non-owner normal user                 → denied
//	CASE 3  admin member, no capability           → denied
//	CASE 4  admin member + capability             → denied (no admin path exists)
//	CASE 5  normal user + capability              → denied (capability ≠ authority)
//
// This file is a DB-free guard: it reads the handler source and fails if the
// membership-only branch (or any IsAdmin/role-based or capability fallback)
// is ever reintroduced into UpdateContent.

import (
	"os"
	"strings"
	"testing"
)

func TestUpdateContent_OwnerOnly_NoAdminOrCapabilityBranch(t *testing.T) {
	src, err := os.ReadFile("content_handler.go")
	if err != nil {
		t.Fatalf("read content_handler.go: %v", err)
	}
	code := string(src)

	start := strings.Index(code, "func (h *ContentHandler) UpdateContent(c *gin.Context) {")
	if start < 0 {
		t.Fatal("UpdateContent not found in content_handler.go")
	}
	next := strings.Index(code[start+len("func (h *ContentHandler) UpdateContent"):], "func (h *ContentHandler)")
	if next < 0 {
		t.Fatalf("could not find end of UpdateContent body")
	}
	end := start + len("func (h *ContentHandler) UpdateContent") + next
	body := code[start:end]

	// The owner gate must exist.
	if !strings.Contains(body, "content.AuthorID != userID") {
		t.Fatal("MISSING: UpdateContent must gate on content.AuthorID != userID")
	}

	// No membership-only, role-based, or capability fallback may exist.
	for _, forbidden := range []string{
		"roleChecker.IsAdmin",
		".IsAdmin(",
		"HasCapability(",
		"RequireCapability",
		"role == \"admin\"",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("REGRESSION: UpdateContent contains forbidden authority fallback %q — content update must be owner-only", forbidden)
		}
	}

	// Defense in depth: the service-level ErrOwnerRequired must surface as a
	// proper 403, not a 500.
	if !strings.Contains(body, "auth.ErrOwnerRequired") || !strings.Contains(body, "response.Forbidden") {
		t.Fatal("MISSING: UpdateContent must map service-level auth.ErrOwnerRequired to 403 Forbidden")
	}
}

func TestUpdateContent_DeleteContentComments_NoStaleAdminOverride(t *testing.T) {
	src, err := os.ReadFile("content_handler.go")
	if err != nil {
		t.Fatalf("read content_handler.go: %v", err)
	}
	code := string(src)

	for _, stale := range []string{"(admin can override)", "admin can override"} {
		if strings.Contains(code, stale) {
			t.Fatalf("STALE COMMENT: %q still present in content_handler.go — content mutations are owner-only", stale)
		}
	}
}
