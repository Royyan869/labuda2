package repository

// C7B — Comment INSERT / commerce-reference alignment tests.
//
// These tests lock the canonical storage contract after migration 000031:
//   - comments.Type = (id, author_id, body, target_id, target_type,
//     parent_id, created_at, updated_at); the legacy `type`,
//     `share_reference`, `for_sale_id` columns and `comment_type_enum`
//     are dropped.
//   - Commerce-reference comments are written via the comment_commerce_references
//     join table (exactly-one-source CHECK), not via a JSONB share blob.
//
// Uses source-file inspection (no DB required).

import (
	"strings"
	"testing"
)

func readCommentImplSource(t *testing.T) string {
	t.Helper()
	return readCommentImplFile(t) // shared helper, same directory
}

// TestC7B_InsertUsesCanonicalColumnsOnly locks the INSERT column list of
// Create(): it must use only canonical columns and never resurrect the legacy
// `type` / `share_reference` columns. `for_sale_id` is NOT flagged
// here because it legitimately lives on the comment_commerce_references join
// table (commercial canonical reference, not the removed comment column).
func TestC7B_InsertUsesCanonicalColumnsOnly(t *testing.T) {
	src := readCommentImplSource(t)

	// The INSERT templates must carry the canonical column set (reply variant
	// includes parent_id; top-level variant does not).
	for _, fragment := range []string{
		"id, author_id, body, target_id, target_type,\n\t\t\t\tparent_id, created_at, updated_at",
		"id, author_id, body, target_id, target_type,\n\t\t\t\tcreated_at, updated_at",
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("Create() INSERT missing canonical column fragment: %q", fragment)
		}
	}

	// Legacy comment-row schema tokens must never return to the comment writer.
	for _, legacy := range []string{"share_reference", `"type"`, "listing_id"} {
		if strings.Contains(src, legacy) {
			t.Errorf("Create() references removed/legacy column token %q", legacy)
		}
	}
}

// TestC7B_CommerceReferenceUsesJoinTable locks that commerce-reference comment
// persistence goes through comment_commerce_references (000031) rather than a
// JSONB share blob on the comment row.
func TestC7B_CommerceReferenceUsesJoinTable(t *testing.T) {
	// insertCommentCommerceReference lives in comment_repository_impl.go and
	// writes the canonical join table.
	src := readCommentImplFile(t)

	if !strings.Contains(src, "insertCommentCommerceReference") {
		t.Error("Create() must delegate to insertCommentCommerceReference for refs")
	}
	if !strings.Contains(src, "INSERT INTO comment_commerce_references") {
		t.Error("commerce-reference persistence must write comment_commerce_references")
	}
}