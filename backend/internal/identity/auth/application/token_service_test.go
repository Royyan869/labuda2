package application_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

func newTestTokenService(t *testing.T) *application.TokenService {
	t.Helper()
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	log := &logger.Logger{Logger: zap.NewNop()}
	return application.NewTokenService(cfg, log)
}

// --- TokenType discriminator ---

func TestGenerateTokenPair_AccessTokenHasCorrectType(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken access error: %v", err)
	}
	if claims.TokenType != application.TokenTypeAccess {
		t.Errorf("access token type = %q, want %q", claims.TokenType, application.TokenTypeAccess)
	}
}

func TestGenerateTokenPair_RefreshTokenHasCorrectType(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateToken refresh error: %v", err)
	}
	if claims.TokenType != application.TokenTypeRefresh {
		t.Errorf("refresh token type = %q, want %q", claims.TokenType, application.TokenTypeRefresh)
	}
}

// --- ValidateRefreshToken type gate ---

func TestValidateAccessToken_AcceptsCanonicalAccessToken(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken error: %v", err)
	}
	if claims.UserID != userID || claims.ID == "" {
		t.Fatalf("access identity = %s/%q, want %s/non-empty", claims.UserID, claims.ID, userID)
	}
	if claims.TokenUse != application.TokenUseAccess {
		t.Fatalf("token_use = %q, want %q", claims.TokenUse, application.TokenUseAccess)
	}
}

func TestValidateAccessToken_RejectsRefreshToken(t *testing.T) {
	svc := newTestTokenService(t)
	pair, err := svc.GenerateTokenPair(uuid.New(), []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}
	if _, err := svc.ValidateAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("expected refresh token rejection")
	}
}

func TestValidateRefreshToken_RejectsAccessToken(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	_, err = svc.ValidateRefreshToken(pair.AccessToken)
	if err == nil {
		t.Fatal("expected error when passing access token to ValidateRefreshToken, got nil")
	}
	if !strings.Contains(err.Error(), "token type mismatch") {
		t.Errorf("error %q does not mention 'token type mismatch'", err.Error())
	}
}

func TestValidateRefreshToken_AcceptsRefreshToken(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken error: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("user_id = %s, want %s", claims.UserID, userID)
	}
}

// --- JTI / FamilyID embedded in refresh token ---

func TestGenerateTokenPair_RefreshJTIEmbeddedInClaims(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken error: %v", err)
	}

	// RegisteredClaims.ID must equal the returned RefreshJTI.
	parsedJTI, parseErr := uuid.Parse(claims.ID)
	if parseErr != nil {
		t.Fatalf("claims.ID %q is not a valid UUID: %v", claims.ID, parseErr)
	}
	if parsedJTI != pair.RefreshJTI {
		t.Errorf("claims.ID JTI = %s, want %s", parsedJTI, pair.RefreshJTI)
	}
}

func TestGenerateTokenPair_FamilyIDEmbeddedInClaims(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken error: %v", err)
	}

	parsedFamily, parseErr := uuid.Parse(claims.FamilyID)
	if parseErr != nil {
		t.Fatalf("claims.FamilyID %q is not a valid UUID: %v", claims.FamilyID, parseErr)
	}
	if parsedFamily != pair.FamilyID {
		t.Errorf("claims.FamilyID = %s, want %s", parsedFamily, pair.FamilyID)
	}
}

// --- Family preservation on rotation ---

func TestGenerateTokenPair_PreservesExistingFamilyID(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	existingFamily := uuid.New()

	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, &existingFamily)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}
	if pair.FamilyID != existingFamily {
		t.Errorf("FamilyID = %s, want %s (existing family not preserved)", pair.FamilyID, existingFamily)
	}
}

func TestGenerateTokenPair_GeneratesNewFamilyIDWhenNil(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()

	pair1, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	pair2, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)

	if pair1.FamilyID == pair2.FamilyID {
		t.Error("two independent logins got the same family_id; want distinct families")
	}
}

// --- RefreshExpiresAt is future ---

func TestGenerateTokenPair_RefreshExpiresAtIsFuture(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}
	if !pair.RefreshExpiresAt.After(time.Now()) {
		t.Errorf("RefreshExpiresAt %v is not in the future", pair.RefreshExpiresAt)
	}
}

// --- JTI uniqueness per issuance ---

func TestGenerateTokenPair_EachIssuanceHasUniqueJTI(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()

	pair1, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	pair2, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)

	if pair1.RefreshJTI == pair2.RefreshJTI {
		t.Error("two token pair issuances produced the same RefreshJTI; must be unique")
	}
}

// --- ValidateToken still works without type check ---

func TestValidateToken_WorksForBothTypes(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, _ := svc.GenerateTokenPair(userID, []string{"user"}, nil)

	for _, tok := range []string{pair.AccessToken, pair.RefreshToken} {
		claims, err := svc.ValidateToken(tok)
		if err != nil {
			t.Errorf("ValidateToken(%q...) error: %v", tok[:10], err)
		}
		if claims.UserID != userID {
			t.Errorf("user_id mismatch")
		}
	}
}

// --- Expired token is rejected ---

func TestValidateRefreshToken_RejectsExpiredToken(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret:     "test-secret-32-bytes-long-enough!",
		Expiration: 15 * time.Minute,
	}
	log := &logger.Logger{Logger: zap.NewNop()}
	svc := application.NewTokenService(cfg, log)

	// Craft a refresh token with an expiry in the past.
	claims := &application.Claims{
		UserID:    uuid.New(),
		TokenType: application.TokenTypeRefresh,
		FamilyID:  uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "labuda-backend",
			Subject:   uuid.New().String(),
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-48 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // already expired
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	_, err = svc.ValidateRefreshToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired refresh token, got nil")
	}
}

func TestGenerateRestrictedCompletionToken_HasScopedClaimsOnly(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()

	token, expiresAt, err := svc.GenerateRestrictedCompletionToken(userID)
	if err != nil {
		t.Fatalf("GenerateRestrictedCompletionToken error: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("restricted token expiry %v is not in the future", expiresAt)
	}

	claims, err := svc.ValidateRestrictedCompletionToken(token)
	if err != nil {
		t.Fatalf("ValidateRestrictedCompletionToken error: %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("user_id = %s, want %s", claims.UserID, userID)
	}
	if claims.TokenType != application.TokenTypeAccess {
		t.Fatalf("token_type = %q, want %q", claims.TokenType, application.TokenTypeAccess)
	}
	if claims.TokenUse != application.TokenUseIdentityCompletion {
		t.Fatalf("token_use = %q, want %q", claims.TokenUse, application.TokenUseIdentityCompletion)
	}
	if claims.Scope != application.ScopeIdentityComplete {
		t.Fatalf("scope = %q, want %q", claims.Scope, application.ScopeIdentityComplete)
	}
	if claims.Subject != "" {
		t.Fatalf("restricted token subject = %q, want empty", claims.Subject)
	}
	if claims.ID != "" {
		t.Fatalf("restricted token jti = %q, want empty", claims.ID)
	}
	if claims.FamilyID != "" {
		t.Fatalf("restricted token family_id = %q, want empty", claims.FamilyID)
	}
	if len(claims.Roles) != 0 {
		t.Fatalf("restricted token roles = %v, want none", claims.Roles)
	}
}

func TestValidateRestrictedCompletionToken_RejectsNormalAccessToken(t *testing.T) {
	svc := newTestTokenService(t)
	userID := uuid.New()
	pair, err := svc.GenerateTokenPair(userID, []string{"user"}, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair error: %v", err)
	}

	_, err = svc.ValidateRestrictedCompletionToken(pair.AccessToken)
	if err == nil {
		t.Fatal("expected restricted-token validator to reject normal access token")
	}
	if !strings.Contains(err.Error(), "token use mismatch") {
		t.Fatalf("unexpected error for normal access token: %v", err)
	}
}
