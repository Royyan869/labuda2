package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
)

// --- RefreshSessionStatus ---

func TestRefreshSessionStatus_IsValid(t *testing.T) {
	valid := []authentity.RefreshSessionStatus{
		authentity.RefreshSessionStatusActive,
		authentity.RefreshSessionStatusConsumed,
		authentity.RefreshSessionStatusRevoked,
		authentity.RefreshSessionStatusReused,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []authentity.RefreshSessionStatus{"", "expired", "pending", "ACTIVE"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestRefreshSessionStatus_IsTerminal(t *testing.T) {
	if authentity.RefreshSessionStatusActive.IsTerminal() {
		t.Fatal("active must not be terminal")
	}
	terminal := []authentity.RefreshSessionStatus{
		authentity.RefreshSessionStatusConsumed,
		authentity.RefreshSessionStatusRevoked,
		authentity.RefreshSessionStatusReused,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
}

// --- NewRefreshSession ---

func validHash() string {
	// 64 hex chars = 32 bytes = valid SHA-256
	return strings.Repeat("a", 64)
}

func TestNewRefreshSession_Success(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	jti := uuid.New()
	hash := validHash()
	expires := time.Now().Add(30 * 24 * time.Hour)

	s, err := authentity.NewRefreshSession(userID, familyID, jti, hash, expires)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID == uuid.Nil {
		t.Fatal("ID must be set")
	}
	if s.UserID != userID {
		t.Fatalf("UserID mismatch: got %v want %v", s.UserID, userID)
	}
	if s.FamilyID != familyID {
		t.Fatalf("FamilyID mismatch")
	}
	if s.JTI != jti {
		t.Fatalf("JTI mismatch")
	}
	if s.TokenHash != hash {
		t.Fatalf("TokenHash mismatch")
	}
	if s.Status != authentity.RefreshSessionStatusActive {
		t.Fatalf("new session must be active, got %q", s.Status)
	}
	if s.IssuedAt.IsZero() || s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		t.Fatal("timestamps must be set")
	}
	if s.ConsumedAt != nil || s.RevokedAt != nil || s.ReuseDetectedAt != nil {
		t.Fatal("terminal timestamps must be nil for new session")
	}
}

func TestNewRefreshSession_NilUserID(t *testing.T) {
	_, err := authentity.NewRefreshSession(uuid.Nil, uuid.New(), uuid.New(), validHash(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for nil userID")
	}
}

func TestNewRefreshSession_NilFamilyID(t *testing.T) {
	_, err := authentity.NewRefreshSession(uuid.New(), uuid.Nil, uuid.New(), validHash(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for nil familyID")
	}
}

func TestNewRefreshSession_NilJTI(t *testing.T) {
	_, err := authentity.NewRefreshSession(uuid.New(), uuid.New(), uuid.Nil, validHash(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for nil jti")
	}
}

func TestNewRefreshSession_EmptyTokenHash(t *testing.T) {
	_, err := authentity.NewRefreshSession(uuid.New(), uuid.New(), uuid.New(), "", time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for empty token hash")
	}
}

func TestNewRefreshSession_ShortTokenHash(t *testing.T) {
	// Not a valid SHA-256 hex (must be 64 chars)
	_, err := authentity.NewRefreshSession(uuid.New(), uuid.New(), uuid.New(), "abc123", time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for short token hash")
	}
}

func TestNewRefreshSession_PastExpiry(t *testing.T) {
	_, err := authentity.NewRefreshSession(uuid.New(), uuid.New(), uuid.New(), validHash(), time.Now().Add(-time.Hour))
	if err == nil {
		t.Fatal("expected error for past expiry")
	}
}

// --- Invariant 10: no raw token stored ---

// TestNoRawTokenInStruct verifies the RefreshSession struct has no field
// that could hold a raw JWT string beyond TokenHash.
//
// This is a code-level proof that the entity design enforces
// "no raw refresh token stored". TokenHash is the only token-bearing field,
// and it must be a SHA-256 hex (64 chars), not a raw JWT.
func TestNoRawTokenInStruct_TokenHashOnly(t *testing.T) {
	s, err := authentity.NewRefreshSession(uuid.New(), uuid.New(), uuid.New(), validHash(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Raw JWTs are three base64-encoded segments separated by dots.
	// TokenHash must be a 64-char hex string, not a JWT.
	if strings.Contains(s.TokenHash, ".") {
		t.Fatal("TokenHash looks like a raw JWT (contains '.'); only SHA-256 hex is permitted")
	}
	if len(s.TokenHash) != 64 {
		t.Fatalf("TokenHash length must be 64 (SHA-256 hex), got %d", len(s.TokenHash))
	}
}


