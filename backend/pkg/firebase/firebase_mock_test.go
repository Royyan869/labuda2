package firebase

import (
	"context"
	"testing"
)

// These tests pin the dev-only mock Firebase verifier's claim shape.
// They guard the email_verified convention introduced in Batch B6.4
// (corpus_driver email-verification harness) and the pre-existing
// hash-based UID + special-token contract.
//
// They run entirely in-process — no Firebase service-account key, no
// network, no DB. The mock client is the ONLY code under test; the real
// Firebase path is reached only when c.AuthClient != nil and is therefore
// unaffected by changes in VerifyIDTokenMock.

func newMockClientForTest(t *testing.T) *Client {
	t.Helper()
	c := NewMockClient(nil)
	if c.AuthClient != nil {
		t.Fatalf("NewMockClient must return AuthClient=nil so VerifyIDToken routes to VerifyIDTokenMock; got non-nil")
	}
	return c
}

func TestVerifyIDTokenMock_PlainTokenOmitsEmailVerified(t *testing.T) {
	c := newMockClientForTest(t)
	tok, err := c.VerifyIDTokenMock(context.Background(), "governance-author-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tok.Claims["email_verified"]; ok {
		t.Fatalf("plain token must NOT set email_verified claim (preserves pre-B6.4 default); got claims=%v", tok.Claims)
	}
}

func TestVerifyIDTokenMock_VerifiedSuffixSetsEmailVerifiedTrue(t *testing.T) {
	c := newMockClientForTest(t)
	tok, err := c.VerifyIDTokenMock(context.Background(), "governance-author-verified-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := tok.Claims["email_verified"].(bool)
	if !ok || !val {
		t.Fatalf("'verified' substring must set email_verified=true; got claims=%v", tok.Claims)
	}
}

func TestVerifyIDTokenMock_CaseInsensitiveMarker(t *testing.T) {
	c := newMockClientForTest(t)
	for _, tok := range []string{"VERIFIED-author", "VerifiedViewer", "test-Verified-1"} {
		got, err := c.VerifyIDTokenMock(context.Background(), tok)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tok, err)
		}
		v, ok := got.Claims["email_verified"].(bool)
		if !ok || !v {
			t.Fatalf("token %q should be email-verified (case-insensitive marker); got claims=%v", tok, got.Claims)
		}
	}
}

func TestVerifyIDTokenMock_NonMarkerTokensRemainUnverified(t *testing.T) {
	c := newMockClientForTest(t)
	for _, tok := range []string{"plainuser", "buyer-test", "seller-test", "any-fake-token"} {
		got, err := c.VerifyIDTokenMock(context.Background(), tok)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tok, err)
		}
		if _, ok := got.Claims["email_verified"]; ok {
			t.Fatalf("token %q must NOT be email-verified (no marker); got claims=%v", tok, got.Claims)
		}
	}
}

func TestVerifyIDTokenMock_SpecialTokenSeller1Unverified(t *testing.T) {
	c := newMockClientForTest(t)
	tok, err := c.VerifyIDTokenMock(context.Background(), "seller-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.UID != "seller-1" {
		t.Fatalf("seller-1 special token must yield UID=seller-1; got %q", tok.UID)
	}
	if _, ok := tok.Claims["email_verified"]; ok {
		t.Fatalf("seller-1 (no verified marker) must NOT set email_verified; got claims=%v", tok.Claims)
	}
}

func TestVerifyIDTokenMock_SpecialTokenBuyer1Unverified(t *testing.T) {
	c := newMockClientForTest(t)
	tok, err := c.VerifyIDTokenMock(context.Background(), "buyer-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.UID != "buyer-1" {
		t.Fatalf("buyer-1 special token must yield UID=buyer-1; got %q", tok.UID)
	}
	if _, ok := tok.Claims["email_verified"]; ok {
		t.Fatalf("buyer-1 (no verified marker) must NOT set email_verified; got claims=%v", tok.Claims)
	}
}

func TestVerifyIDTokenMock_DeterministicUIDForSameToken(t *testing.T) {
	c := newMockClientForTest(t)
	a, _ := c.VerifyIDTokenMock(context.Background(), "governance-viewer-verified-runX")
	b, _ := c.VerifyIDTokenMock(context.Background(), "governance-viewer-verified-runX")
	if a.UID != b.UID {
		t.Fatalf("same token must produce deterministic UID across calls (idempotency); got %q vs %q", a.UID, b.UID)
	}
}

func TestIsVerifiedMockToken_EmptyTokenIsUnverified(t *testing.T) {
	if isVerifiedMockToken("") {
		t.Fatalf("empty token must not be considered verified")
	}
}
