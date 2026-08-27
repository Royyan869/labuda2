package entity

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestEncodeFeedCursor_NilReturnsEmpty documents the "no cursor"
// boundary form: handler treats "" as "no next_cursor available" and
// emits null on the wire.
func TestEncodeFeedCursor_NilReturnsEmpty(t *testing.T) {
	if got := EncodeFeedCursor(nil); got != "" {
		t.Fatalf("EncodeFeedCursor(nil) = %q, want \"\"", got)
	}
}

// TestEncodeDecodeFeedCursor_Roundtrip proves the JSON-over-base64
// codec preserves both the timestamp (to nanosecond precision) and
// the row id across an encode/decode pair.
func TestEncodeDecodeFeedCursor_Roundtrip(t *testing.T) {
	id := uuid.MustParse("11112222-3333-4444-5555-666677778888")
	ts := time.Date(2026, 5, 13, 9, 30, 15, 123456789, time.UTC)

	encoded := EncodeFeedCursor(&FeedCursor{CreatedAt: ts, ID: id})
	if encoded == "" {
		t.Fatalf("EncodeFeedCursor returned empty for valid input")
	}

	got, err := DecodeFeedCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeFeedCursor(%q) returned error: %v", encoded, err)
	}
	if got == nil {
		t.Fatalf("DecodeFeedCursor(%q) returned nil cursor", encoded)
	}
	if !got.CreatedAt.Equal(ts) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", got.CreatedAt, ts)
	}
	if got.ID != id {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, id)
	}
}

// TestEncodeFeedCursor_OpaqueOnWire asserts the encoded form is not
// trivially the timestamp or UUID — clients that try to parse it as
// RFC3339 (the prior format) will fail closed, by design.
func TestEncodeFeedCursor_OpaqueOnWire(t *testing.T) {
	id := uuid.MustParse("11112222-3333-4444-5555-666677778888")
	ts := time.Date(2026, 5, 13, 9, 30, 15, 0, time.UTC)
	encoded := EncodeFeedCursor(&FeedCursor{CreatedAt: ts, ID: id})

	if _, err := time.Parse(time.RFC3339Nano, encoded); err == nil {
		t.Errorf("encoded cursor unexpectedly parsed as RFC3339Nano: %q", encoded)
	}
	if strings.Contains(encoded, id.String()) {
		t.Errorf("encoded cursor leaks raw uuid: %q", encoded)
	}
}

// TestDecodeFeedCursor_EmptyReturnsNil documents the canonical
// "first page" form: empty input → nil cursor, no error.
func TestDecodeFeedCursor_EmptyReturnsNil(t *testing.T) {
	got, err := DecodeFeedCursor("")
	if err != nil {
		t.Fatalf("DecodeFeedCursor(\"\") returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("DecodeFeedCursor(\"\") = %+v, want nil", got)
	}
}

// TestDecodeFeedCursor_Malformed exercises the failure modes the
// handler relies on to emit 400. A round-trip of all four classes —
// not-base64, not-json, missing-ts, missing-id — must surface a
// non-nil error so the handler can map to BadRequest.
func TestDecodeFeedCursor_Malformed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"not_base64", "!!!not-base64!!!"},
		{"not_json", "bm90LWpzb24"}, // base64 of "not-json"
		{"empty_object", "e30"},      // base64 of "{}"
		{"missing_id", base64Of(t, `{"ts":"2026-05-13T09:30:15Z"}`)},
		{"missing_ts", base64Of(t, `{"id":"11112222-3333-4444-5555-666677778888"}`)},
		{"legacy_rfc3339", "2026-05-13T09:30:15Z"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeFeedCursor(tc.input)
			if err == nil {
				t.Fatalf("DecodeFeedCursor(%q) = %+v, want error", tc.input, got)
			}
			if got != nil {
				t.Errorf("DecodeFeedCursor on error returned non-nil cursor: %+v", got)
			}
		})
	}
}

// base64Of URL-safe-base64-encodes a JSON literal without padding,
// matching the cursor codec convention. Used to construct malformed
// cursor inputs whose base64 layer is valid but whose JSON payload
// fails the FeedCursor invariants (missing ts / missing id).
func base64Of(t *testing.T, raw string) string {
	t.Helper()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}


