package application

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
)

const (
	// TokenTypeAccess identifies a short-lived access token.
	TokenTypeAccess = "access"
	// TokenTypeRefresh identifies a long-lived refresh token.
	// Refresh tokens are stateful: they are single-use and tracked in auth_refresh_sessions.
	TokenTypeRefresh = "refresh"

	// TokenUseAccess marks a normal session access token.
	TokenUseAccess = "access"
	// TokenUseIdentityCompletion marks the restricted completion-only token.
	TokenUseIdentityCompletion = "identity_completion"
	// TokenUseRefresh marks a refresh token.
	TokenUseRefresh = "refresh"

	// ScopeIdentityComplete is the only allowed scope for restricted completion tokens.
	ScopeIdentityComplete = "identity.complete"
)

// TokenService handles JWT token generation and validation.
//
// **AUTH ALIGNMENT CLARIFICATION:**
// JWT tokens contain roles for CONVENIENCE/INFORMATIONAL PURPOSES ONLY.
// The `roles` claim in the token is NOT authoritative for authorization.
//
// **AUTHORITY FLOW:**
// 1. Firebase ID token verified by AuthMiddleware
// 2. User looked up/created in PostgreSQL
// 3. Roles refreshed from PostgreSQL by RolesLookupMiddleware (on every request)
// 4. Authorization checks use RoleChecker/SellerAuthorityChecker (DB queries)
//
// This design ensures:
// - Immediate role/authority revocation without waiting for token expiry
// - Single source of truth: PostgreSQL database
// - Token roles are stale-safe (used only for UI rendering hints)
type TokenService struct {
	jwtSecret []byte
	config    *config.JWTConfig
	log       *logger.Logger
}

// NewTokenService creates a new JWT token service
func NewTokenService(cfg *config.JWTConfig, log *logger.Logger) *TokenService {
	return &TokenService{
		jwtSecret: []byte(cfg.Secret),
		config:    cfg,
		log:       log,
	}
}

// Claims represents JWT claims structure.
//
// **AUTH ALIGNMENT:** The `Roles` field is INFORMATIONAL ONLY.
// It is refreshed from PostgreSQL on every request via RolesLookupMiddleware.
// Do NOT use token roles for authorization decisions in handlers.
//
// **CANONICAL AUTHORITY:** Use RoleChecker interface methods for authoritative checks.
//
// **TOKEN TYPE:** TokenType must be checked before using any Claims.
// Use ValidateRefreshToken / ValidateToken for type-safe validation.
// RegisteredClaims.ID holds the JTI (UUID string) for refresh tokens.
type Claims struct {
	UserID               uuid.UUID `json:"user_id"`
	Roles                []string  `json:"roles"`      // Informational only - refreshed from DB on each request
	TokenType            string    `json:"token_type"` // "access" or "refresh"
	TokenUse             string    `json:"token_use,omitempty"`
	Scope                string    `json:"scope,omitempty"`
	FamilyID             string    `json:"family_id,omitempty"` // session family UUID (refresh tokens only)
	jwt.RegisteredClaims           // .ID = jti (UUID string)
}

// TokenPair contains access and refresh tokens plus metadata needed for session storage.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time // access token expiry
	RefreshExpiresAt time.Time // refresh token expiry
	// RefreshJTI and FamilyID are needed by the handler to create the session row in auth_refresh_sessions.
	// They are NOT serialized to JSON.
	RefreshJTI uuid.UUID
	FamilyID   uuid.UUID
}

// GenerateTokenPair generates both access and refresh tokens for a user.
//
// familyID: pass nil to start a new session family (new login).
//
//	pass an existing uuid.UUID to continue an existing family (token rotation).
//
// The returned TokenPair includes RefreshJTI and FamilyID for session row creation.
func (s *TokenService) GenerateTokenPair(userID uuid.UUID, roles []string, familyID *uuid.UUID) (*TokenPair, error) {
	// Resolve family: new login → generate; rotation → preserve existing.
	var resolvedFamily uuid.UUID
	if familyID != nil {
		resolvedFamily = *familyID
	} else {
		resolvedFamily = uuid.New()
	}

	refreshJTI := uuid.New()

	accessToken, expiresAt, err := s.generateAccessToken(userID, roles, refreshJTI)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token (long-lived, e.g. 30 days)
	refreshToken, refreshExpiresAt, err := s.generateRefreshToken(userID, refreshJTI, resolvedFamily)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	s.log.Info("Tokens generated",
		zap.String("user_id", userID.String()),
		zap.Time("expires_at", expiresAt),
		zap.String("family_id", resolvedFamily.String()),
	)

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
		RefreshJTI:       refreshJTI,
		FamilyID:         resolvedFamily,
	}, nil
}

// generateAccessToken generates a short-lived JWT access token.
func (s *TokenService) generateAccessToken(userID uuid.UUID, roles []string, sessionID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.Expiration)

	claims := &Claims{
		UserID:    userID,
		Roles:     roles,
		TokenType: TokenTypeAccess,
		TokenUse:  TokenUseAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "labuda-backend",
			ID:        sessionID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// generateRefreshToken generates a long-lived JWT refresh token.
// The jti is embedded in RegisteredClaims.ID and must be stored in auth_refresh_sessions.
func (s *TokenService) generateRefreshToken(userID uuid.UUID, jti uuid.UUID, familyID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour) // 30 days

	claims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		TokenUse:  TokenUseRefresh,
		FamilyID:  familyID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "labuda-backend",
			ID:        jti.String(), // jti = session identifier tracked in auth_refresh_sessions
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// GenerateRestrictedCompletionToken generates a backend-signed access token
// limited to the username-completion flow.
func (s *TokenService) GenerateRestrictedCompletionToken(userID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)

	claims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeAccess,
		TokenUse:  TokenUseIdentityCompletion,
		Scope:     ScopeIdentityComplete,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign restricted token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims.
// Does NOT enforce token type — use ValidateRefreshToken for type-safe refresh validation.
func (s *TokenService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// ValidateAccessToken validates a normal Labuda access token.
func (s *TokenService) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess || claims.TokenUse != TokenUseAccess {
		return nil, fmt.Errorf("token is not a normal access token")
	}
	if claims.UserID == uuid.Nil || claims.ID == "" {
		return nil, fmt.Errorf("access token identity is incomplete")
	}
	return claims, nil
}

// ValidateRefreshToken validates a JWT and confirms it is a refresh token.
// Returns an error if the signature/expiry is invalid OR if the token_type is not "refresh".
// The returned Claims.ID contains the JTI; Claims.FamilyID contains the family UUID string.
func (s *TokenService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, fmt.Errorf("token type mismatch: expected %q, got %q", TokenTypeRefresh, claims.TokenType)
	}
	return claims, nil
}

// ValidateRestrictedCompletionToken validates a restricted completion token.
// It must be a normal access token carrying the identity-completion token_use
// and the exact identity.complete scope.
func (s *TokenService) ValidateRestrictedCompletionToken(tokenString string) (*Claims, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, fmt.Errorf("token type mismatch: expected %q, got %q", TokenTypeAccess, claims.TokenType)
	}
	if claims.TokenUse != TokenUseIdentityCompletion {
		return nil, fmt.Errorf("token use mismatch: expected %q, got %q", TokenUseIdentityCompletion, claims.TokenUse)
	}
	if claims.Scope != ScopeIdentityComplete {
		return nil, fmt.Errorf("scope mismatch: expected %q, got %q", ScopeIdentityComplete, claims.Scope)
	}
	return claims, nil
}
