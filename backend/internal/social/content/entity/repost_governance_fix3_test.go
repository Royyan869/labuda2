package entity

// FIX-3 — Malformed repost target UUID fail-closed regression lock.
//
// The content detail handler (GET /contents/:id) now fails closed when
// share_reference.targetId is not a valid UUID. Prior to this fix, a
// parse failure silently continued and the repost rendered normally
// (fail-open). The corrected logic is:
//
//	origID, parseErr := uuid.Parse(content.ShareReference.TargetID)
//	if parseErr != nil {
//	    repostBlocked = true   // fail-closed
//	    return nil
//	}
//
// These tests pin:
//  1. The uuid.Parse semantics the handler depends on (malformed → error).
//  2. That ShareReference.TargetID is a raw string (no entity-level guard),
//     so corrupted data can reach the handler path.
//  3. The gate condition now used by Content.IsRepost(): OriginalAuthorID only.

import (
	"testing"

	"github.com/google/uuid"
)

// TestRepostGovernanceFix3_MalformedUUIDParseErrors pins that the values
// the handler would treat as "malformed" actually fail uuid.Parse.
// If any of these stop returning errors, the fail-closed branch stops firing.
func TestRepostGovernanceFix3_MalformedUUIDParseErrors(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
		{"not-a-uuid", "not-a-uuid"},
		{"partial uuid", "12345678-1234-1234"},
		{"null bytes", "\x00\x00\x00\x00"},
		{"spaces", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uuid.Parse(tc.id)
			if err == nil {
				t.Errorf("uuid.Parse(%q) returned no error; handler fail-closed guard would not fire", tc.id)
			}
		})
	}
}

// TestRepostGovernanceFix3_ValidUUIDParseSucceeds pins that a canonical UUID
// string does NOT trigger the fail-closed branch, allowing normal DB lookup.
func TestRepostGovernanceFix3_ValidUUIDParseSucceeds(t *testing.T) {
	id := uuid.New()
	parsed, err := uuid.Parse(id.String())
	if err != nil {
		t.Fatalf("uuid.Parse(%q) returned error for valid UUID: %v", id.String(), err)
	}
	if parsed != id {
		t.Errorf("round-trip mismatch: got %v, want %v", parsed, id)
	}
}

// TestRepostGovernanceFix3_ShareReferenceTargetIDIsRawString confirms that
// ShareReference.TargetID has no entity-level UUID validation. Corrupted data
// can reach the handler's uuid.Parse call, making the fail-closed branch
// the only safety net.
func TestRepostGovernanceFix3_ShareReferenceTargetIDIsRawString(t *testing.T) {
	malformed := "not-a-valid-uuid"
	ref := &ShareReference{
		TargetType: ShareTargetTypeContent,
		TargetID:   malformed,
	}
	// Entity stores it as-is — no panic, no error.
	if ref.TargetID != malformed {
		t.Errorf("ShareReference.TargetID should store raw string; got %q", ref.TargetID)
	}
}

// TestRepostGovernanceFix3_OriginalAuthorIDControlsRepostState pins the
// canonical repost bit used by the content entity: OriginalAuthorID.
func TestRepostGovernanceFix3_OriginalAuthorIDControlsRepostState(t *testing.T) {
	origID := uuid.New()

	t.Run("no OriginalAuthorID => not a repost", func(t *testing.T) {
		c := &Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   StatusActive,
		}
		if c.IsRepost() {
			t.Error("expected non-repost when OriginalAuthorID is nil")
		}
	})

	t.Run("OriginalAuthorID set => repost", func(t *testing.T) {
		c := &Content{
			ID:               uuid.New(),
			AuthorID:         uuid.New(),
			Status:           StatusActive,
			OriginalAuthorID: &origID,
		}
		if !c.IsRepost() {
			t.Error("expected repost when OriginalAuthorID is set")
		}
	})
}
