package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/identity/auth/delivery/http/dto"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	identityusername "github.com/labuda/backend/internal/identity/username"
	notificationrepo "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/util"
	"github.com/labuda/backend/pkg/firebase"
	"go.uber.org/zap"
)

// AuthHandler handles Firebase authentication and token generation
type AuthHandler struct {
	pool               *pgxpool.Pool
	firebase           *firebase.Client
	tokenService       *application.TokenService
	logoutService      *application.LogoutCurrentSessionService
	refreshSessionRepo *authrepo.RefreshSessionRepository
	log                *zap.Logger
	jwtConfig          *config.JWTConfig
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(pool *pgxpool.Pool, firebaseClient *firebase.Client, jwtConfig *config.JWTConfig, log *zap.Logger) *AuthHandler {
	// Create logger wrapper for TokenService (Logger embeds *zap.Logger)
	tokenService := application.NewTokenService(jwtConfig, &logger.Logger{Logger: log})
	return &AuthHandler{
		pool:               pool,
		firebase:           firebaseClient,
		tokenService:       tokenService,
		logoutService:      application.NewLogoutCurrentSessionService(application.NewLogoutCurrentSessionTransactor(pool), tokenService, authrepo.NewRefreshSessionRepository(), notificationrepo.NewFCMTokenRepository(nil), log),
		refreshSessionRepo: authrepo.NewRefreshSessionRepository(),
		log:                log,
		jwtConfig:          jwtConfig,
	}
}

// FirebaseExchange handles POST /api/v1/auth/firebase/exchange.
func (h *AuthHandler) FirebaseExchange(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.FirebaseExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "firebase_id_token is required")
		return
	}

	idToken := strings.TrimSpace(req.FirebaseIDToken)
	if idToken == "" {
		response.BadRequest(c, "firebase_id_token cannot be empty")
		return
	}

	// Canonical registration username (optional). Normalized with the same
	// authority used by every other username entry point; reserved names and
	// uniqueness are enforced against the backend, never by client-side JSON.
	var wantUsername string
	if req.Username != nil {
		wantUsername = identityusername.Normalize(*req.Username)
		if wantUsername != "" {
			if err := identityusername.ValidateFormat(wantUsername); err != nil {
				response.Error(c, http.StatusBadRequest, "USERNAME_INVALID_FORMAT", err.Error())
				return
			}
			if identityusername.IsReserved(wantUsername) {
				response.Error(c, http.StatusBadRequest, "USERNAME_RESERVED", "This username is reserved and cannot be used")
				return
			}
		}
	}

	// STEP 1: Verify Firebase ID token
	firebaseToken, err := h.firebase.VerifyIDToken(ctx, idToken)
	if err != nil {
		h.log.Warn("Failed to verify Firebase token", zap.Error(err))
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired Firebase token")
		return
	}

	// STEP 2: Extract user identity from Firebase token
	firebaseUID := firebaseToken.UID
	var email string

	// Extract email from claims
	if emailVal, ok := firebaseToken.Claims["email"].(string); ok {
		email = emailVal
	}

	// Normalize email: trim whitespace and convert to lowercase
	normalizedEmail := util.NormalizeEmail(&email)

	var firebaseEmailVerified bool
	if emailVerifiedVal, ok := firebaseToken.Claims["email_verified"].(bool); ok {
		firebaseEmailVerified = emailVerifiedVal
	}

	h.log.Info("Firebase token verified",
		zap.String("firebase_uid", firebaseUID),
		zap.String("email", email),
		zap.Bool("email_verified", firebaseEmailVerified),
	)

	// STEP 3: Start transaction for atomic identity flow
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("Failed to begin transaction", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}
	defer tx.Rollback(ctx)

	// Serialize same-email identity flows so concurrent Firebase UIDs cannot
	// fork or overwrite the same canonical account row.
	if normalizedEmail != nil {
		if err := h.lockEmailIdentity(ctx, tx, *normalizedEmail); err != nil {
			h.log.Error("Failed to lock email identity", zap.Error(err))
			response.InternalServerError(c, "Database error")
			return
		}
	}

	var authUser *authUserRecord
	created := false

	authUser, err = h.getActiveAuthUserByFirebaseUID(ctx, tx, firebaseUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		h.log.Error("Database error checking user by firebase UID", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	if errors.Is(err, pgx.ErrNoRows) {
		deletedByUID, deletedErr := h.hasSoftDeletedUserByFirebaseUID(ctx, tx, firebaseUID)
		if deletedErr != nil {
			h.log.Error("Database error checking deleted user by firebase UID", zap.Error(deletedErr))
			response.InternalServerError(c, "Database error")
			return
		}
		if deletedByUID {
			response.Error(c, http.StatusForbidden, "ACCOUNT_DELETED", "Account has been deleted")
			return
		}

		if normalizedEmail != nil {
			authUser, err = h.getActiveAuthUserByEmail(ctx, tx, *normalizedEmail)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				h.log.Error("Database error checking user by email", zap.Error(err))
				response.InternalServerError(c, "Database error")
				return
			}

			if errors.Is(err, pgx.ErrNoRows) {
				deletedByEmail, deletedErr := h.hasSoftDeletedUserByEmail(ctx, tx, *normalizedEmail)
				if deletedErr != nil {
					h.log.Error("Database error checking deleted user by email", zap.Error(deletedErr))
					response.InternalServerError(c, "Database error")
					return
				}
				if deletedByEmail {
					response.Error(c, http.StatusForbidden, "ACCOUNT_DELETED", "Account has been deleted")
					return
				}
				authUser = nil
			}
		}

		if authUser == nil {
			if normalizedEmail == nil {
				response.Error(c, http.StatusBadRequest, "EMAIL_REQUIRED", "Firebase account must include an email")
				return
			}

			createdAuthUser, createdFlag, createErr := h.createUser(ctx, tx, firebaseUID, email, firebaseEmailVerified)
			if createErr != nil {
				h.log.Error("Failed to create user", zap.Error(createErr))
				response.InternalServerError(c, "Failed to create user")
				return
			}
			authUser = createdAuthUser
			created = createdFlag
		} else {
			if authUser.AccountStatus != "active" {
				h.log.Warn("User account is not active",
					zap.String("user_id", authUser.ID.String()),
					zap.String("account_status", authUser.AccountStatus),
				)
				response.Error(c, http.StatusForbidden, "ACCOUNT_INACTIVE", fmt.Sprintf("Account is %s", authUser.AccountStatus))
				return
			}

			if linkErr := h.linkFirebaseIdentity(ctx, tx, authUser.ID, firebaseUID); linkErr != nil {
				h.log.Error("Failed to link Firebase identity", zap.Error(linkErr))
				response.InternalServerError(c, "Failed to link account")
				return
			}
			authUser.FirebaseUID = firebaseUID
		}
	}

	if authUser.AccountStatus != "active" {
		h.log.Warn("User account is not active",
			zap.String("user_id", authUser.ID.String()),
			zap.String("account_status", authUser.AccountStatus),
		)
		response.Error(c, http.StatusForbidden, "ACCOUNT_INACTIVE", fmt.Sprintf("Account is %s", authUser.AccountStatus))
		return
	}

	if err := h.syncEmailVerifiedSnapshot(ctx, tx, authUser, firebaseEmailVerified); err != nil {
		h.log.Error("Failed to sync email verification snapshot", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	// Stamp the canonical registration username (email/password signup) inside
	// the same transaction that resolves the identity. Backend remains the
	// final authority for format/reserved/uniqueness via the UNIQUE index.
	if err := h.applyRegistrationUsername(ctx, tx, authUser, wantUsername); err != nil {
		if errors.Is(err, errSignupUsernameTaken) {
			h.log.Warn("Registration username taken", zap.String("user_id", authUser.ID.String()), zap.String("username", wantUsername))
			response.Error(c, http.StatusConflict, "USERNAME_TAKEN", "This username is already taken")
			return
		}
		if errors.Is(err, errSignupUsernameImmutable) {
			h.log.Warn("Registration username cannot change", zap.String("user_id", authUser.ID.String()))
			response.Error(c, http.StatusConflict, "USERNAME_IMMUTABLE", "Username cannot be changed after registration")
			return
		}
		h.log.Error("Failed to apply registration username", zap.Error(err), zap.String("user_id", authUser.ID.String()))
		response.InternalServerError(c, "Database error")
		return
	}

	// Commit the canonical user mutation before issuing any session token.
	if err := tx.Commit(ctx); err != nil {
		h.log.Error("Failed to commit transaction", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	profileComplete := authUser.Username != nil && *authUser.Username != ""
	if !profileComplete {
		restrictedToken, restrictedExpiresAt, err := h.tokenService.GenerateRestrictedCompletionToken(authUser.ID)
		if err != nil {
			h.log.Error("Failed to generate restricted completion token", zap.Error(err))
			response.InternalServerError(c, "Failed to generate tokens")
			return
		}

		resp := dto.FirebaseExchangeIncompleteResponse{
			UserID:                    authUser.ID.String(),
			RequiresProfileCompletion: true,
			Email:                     normalizedEmail,
			AccessToken:               restrictedToken,
			ExpiresAt:                 restrictedExpiresAt.Format(time.RFC3339),
		}

		h.log.Info("Firebase exchange requires profile completion",
			zap.String("user_id", authUser.ID.String()),
		)

		response.Success(c, resp)
		return
	}

	roles := []string{authUser.Role}
	// nil familyID → new session family for this login.
	tokenPair, err := h.tokenService.GenerateTokenPair(authUser.ID, roles, nil)
	if err != nil {
		h.log.Error("Failed to generate tokens", zap.Error(err))
		response.InternalServerError(c, "Failed to generate tokens")
		return
	}

	// Persist refresh session row (second tx — user tx already committed).
	sessionTx, sessionTxErr := h.pool.Begin(ctx)
	if sessionTxErr != nil {
		h.log.Error("Failed to begin session tx", zap.Error(sessionTxErr))
		response.InternalServerError(c, "Database error")
		return
	}
	defer sessionTx.Rollback(ctx) //nolint:errcheck

	tokenHash := authrepo.HashRefreshToken(tokenPair.RefreshToken)
	session, sessionErr := authentity.NewRefreshSession(
		authUser.ID,
		tokenPair.FamilyID,
		tokenPair.RefreshJTI,
		tokenHash,
		tokenPair.RefreshExpiresAt,
	)
	if sessionErr != nil {
		h.log.Error("Failed to build refresh session entity", zap.Error(sessionErr))
		response.InternalServerError(c, "Failed to create session")
		return
	}
	if createErr := h.refreshSessionRepo.Create(ctx, sessionTx, session); createErr != nil {
		h.log.Error("Failed to persist refresh session", zap.Error(createErr))
		response.InternalServerError(c, "Failed to create session")
		return
	}
	if commitErr := sessionTx.Commit(ctx); commitErr != nil {
		h.log.Error("Failed to commit session tx", zap.Error(commitErr))
		response.InternalServerError(c, "Failed to create session")
		return
	}

	if profileComplete {
		resp := dto.FirebaseExchangeCompleteResponse{
			UserID:                    authUser.ID.String(),
			RequiresProfileCompletion: false,
			AccessToken:               tokenPair.AccessToken,
			RefreshToken:              tokenPair.RefreshToken,
			ExpiresAt:                 tokenPair.ExpiresAt.Format(time.RFC3339),
			RefreshExpiresAt:          tokenPair.RefreshExpiresAt.Format(time.RFC3339),
			Created:                   created,
		}
		h.log.Info("Firebase exchange completed with full session",
			zap.String("user_id", authUser.ID.String()),
			zap.Bool("created", created),
		)
		response.Success(c, resp)
		return
	}
}

// createUser creates a new user in PostgreSQL from Firebase auth data
// This is idempotent - it will only create the user if they don't exist
type authUserRecord struct {
	ID              uuid.UUID
	FirebaseUID     string
	Email           *string
	AccountStatus   string
	Role            string
	CreatedAt       time.Time
	EmailVerifiedAt *time.Time
	Username        *string
}

func (h *AuthHandler) createUser(ctx context.Context, tx pgx.Tx, firebaseUID, email string, firebaseEmailVerified bool) (*authUserRecord, bool, error) {
	// Generate a new UUID for the user
	userID := uuid.New()

	// Normalize email before database write
	normalizedEmail := util.NormalizeEmail(&email)

	// Prepare email value for database operations (nil or normalized string)
	var emailValue interface{}
	if normalizedEmail != nil {
		emailValue = *normalizedEmail
	} else {
		emailValue = nil
	}

	var emailVerifiedAt interface{}
	if firebaseEmailVerified {
		emailVerifiedAt = time.Now()
	}

	// Insert user record (transaction is managed by caller).
	//
	// ON CONFLICT DO NOTHING is intentional here: the surrounding transaction
	// already serialized same-email flows with an advisory lock, so any
	// conflict that still occurs is a legacy or out-of-band data issue. We
	// resolve deterministically below instead of creating a split identity.
	tag, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, email_verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, userID, firebaseUID, emailValue, "active", emailVerifiedAt)
	if err != nil {
		return nil, false, fmt.Errorf("failed to insert user: %w", err)
	}

	inserted := tag.RowsAffected() > 0

	if inserted {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (user_id, username, bio, avatar_url, followers_count, following_count, created_at, updated_at)
			VALUES ($1, $2, NULL, NULL, 0, 0, NOW(), NOW())
			ON CONFLICT (user_id) DO NOTHING
		`, userID, nil)
		if err != nil {
			return nil, false, fmt.Errorf("failed to insert user profile: %w", err)
		}

		authUser, loadErr := h.getActiveAuthUserByID(ctx, tx, userID)
		if loadErr != nil {
			return nil, false, fmt.Errorf("failed to load created user: %w", loadErr)
		}
		if authUser == nil {
			return nil, false, fmt.Errorf("created user not found after insert")
		}

		h.log.Info("User created from Firebase auth",
			zap.String("user_id", userID.String()),
			zap.String("firebase_uid", firebaseUID),
			zap.String("email", email),
		)

		return authUser, true, nil
	}

	// If the insert was skipped, resolve the canonical row deterministically.
	authUser, loadErr := h.getActiveAuthUserByFirebaseUID(ctx, tx, firebaseUID)
	if loadErr != nil && !errors.Is(loadErr, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to resolve user after insert conflict: %w", loadErr)
	}
	if authUser != nil {
		return authUser, false, nil
	}

	if normalizedEmail != nil {
		authUser, loadErr = h.getActiveAuthUserByEmail(ctx, tx, *normalizedEmail)
		if loadErr != nil && !errors.Is(loadErr, pgx.ErrNoRows) {
			return nil, false, fmt.Errorf("failed to resolve user by email after insert conflict: %w", loadErr)
		}
		if authUser != nil {
			if authUser.FirebaseUID != firebaseUID {
				if linkErr := h.linkFirebaseIdentity(ctx, tx, authUser.ID, firebaseUID); linkErr != nil {
					return nil, false, fmt.Errorf("failed to link firebase identity after insert conflict: %w", linkErr)
				}
				authUser.FirebaseUID = firebaseUID
			}
			return authUser, false, nil
		}
	}

	return nil, false, fmt.Errorf("user insert conflicted but no canonical row could be resolved")
}

func (h *AuthHandler) getActiveAuthUserByFirebaseUID(ctx context.Context, tx pgx.Tx, firebaseUID string) (*authUserRecord, error) {
	return h.getActiveAuthUser(ctx, tx, `
		SELECT u.id, u.firebase_uid, u.email, u.account_status, u.role, u.created_at, u.email_verified_at, up.username
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.firebase_uid = $1 AND u.deleted_at IS NULL
	`, firebaseUID)
}

func (h *AuthHandler) getActiveAuthUserByEmail(ctx context.Context, tx pgx.Tx, email string) (*authUserRecord, error) {
	return h.getActiveAuthUser(ctx, tx, `
		SELECT u.id, u.firebase_uid, u.email, u.account_status, u.role, u.created_at, u.email_verified_at, up.username
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE LOWER(BTRIM(u.email)) = LOWER(BTRIM($1)) AND u.deleted_at IS NULL
		LIMIT 1
	`, email)
}

func (h *AuthHandler) getActiveAuthUserByID(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*authUserRecord, error) {
	return h.getActiveAuthUser(ctx, tx, `
		SELECT u.id, u.firebase_uid, u.email, u.account_status, u.role, u.created_at, u.email_verified_at, up.username
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`, userID)
}

func (h *AuthHandler) getActiveAuthUser(ctx context.Context, tx pgx.Tx, query string, arg interface{}) (*authUserRecord, error) {
	var user authUserRecord
	var email sql.NullString
	var username sql.NullString
	err := tx.QueryRow(ctx, query, arg).Scan(
		&user.ID,
		&user.FirebaseUID,
		&email,
		&user.AccountStatus,
		&user.Role,
		&user.CreatedAt,
		&user.EmailVerifiedAt,
		&username,
	)
	if err != nil {
		return nil, err
	}
	if email.Valid {
		user.Email = &email.String
	}
	if username.Valid {
		user.Username = &username.String
	}
	return &user, nil
}

func (h *AuthHandler) hasSoftDeletedUserByFirebaseUID(ctx context.Context, tx pgx.Tx, firebaseUID string) (bool, error) {
	return h.hasSoftDeletedUser(ctx, tx, `
		SELECT EXISTS (
			SELECT 1 FROM users WHERE firebase_uid = $1 AND deleted_at IS NOT NULL
		)
	`, firebaseUID)
}

func (h *AuthHandler) hasSoftDeletedUserByEmail(ctx context.Context, tx pgx.Tx, email string) (bool, error) {
	return h.hasSoftDeletedUser(ctx, tx, `
			SELECT EXISTS (
				SELECT 1 FROM users WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1)) AND deleted_at IS NOT NULL
			)
	`, email)
}

func (h *AuthHandler) lockEmailIdentity(ctx context.Context, tx pgx.Tx, email string) error {
	_, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
	`, email)
	return err
}

func (h *AuthHandler) lockCompletionUser(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
	`, userID.String())
	return err
}

func (h *AuthHandler) hasSoftDeletedUser(ctx context.Context, tx pgx.Tx, query string, arg interface{}) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, query, arg).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (h *AuthHandler) linkFirebaseIdentity(ctx context.Context, tx pgx.Tx, userID uuid.UUID, firebaseUID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE users
		SET firebase_uid = $1, updated_at = NOW()
		WHERE id = $2
	`, firebaseUID, userID)
	return err
}

func (h *AuthHandler) syncEmailVerifiedSnapshot(ctx context.Context, tx pgx.Tx, user *authUserRecord, firebaseEmailVerified bool) error {
	if !firebaseEmailVerified || user.EmailVerifiedAt != nil {
		return nil
	}

	now := time.Now()
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET email_verified_at = $2, updated_at = NOW()
		WHERE id = $1
	`, user.ID, now); err != nil {
		return err
	}
	user.EmailVerifiedAt = &now
	return nil
}

// Sentinel errors for the registration-username application step. The caller
// maps them to the canonical USERNAME_TAKEN / USERNAME_IMMUTABLE responses.
var (
	errSignupUsernameTaken     = errors.New("username already taken")
	errSignupUsernameImmutable = errors.New("username is immutable after registration")
)

// applyRegistrationUsername stamps the canonical username chosen at
// registration onto the user's profile, exactly once.
//
//   - Empty want is a no-op (login / Google-first-sync carry no username).
//   - If the profile already has a username, the only accepted input is that
//     same username (idempotent re-sync); anything else is immutable.
//   - The user_profiles.username UNIQUE index is the final uniqueness authority;
//     a concurrent winner surfaces as 23505 → errSignupUsernameTaken.
func (h *AuthHandler) applyRegistrationUsername(ctx context.Context, tx pgx.Tx, user *authUserRecord, want string) error {
	if want == "" {
		return nil
	}
	if user.Username != nil && *user.Username != "" {
		if *user.Username == want {
			return nil
		}
		return errSignupUsernameImmutable
	}

	tag, err := tx.Exec(ctx, `
		UPDATE user_profiles
		SET username = $2, updated_at = NOW()
		WHERE user_id = $1 AND (username IS NULL OR username = '')
	`, user.ID, want)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errSignupUsernameTaken
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		// The username was established concurrently. Resolve deterministically.
		var cur sql.NullString
		if e := tx.QueryRow(ctx, `SELECT username FROM user_profiles WHERE user_id = $1`, user.ID).Scan(&cur); e != nil {
			return e
		}
		if cur.Valid && cur.String == want {
			return nil
		}
		return errSignupUsernameTaken
	}

	user.Username = &want
	return nil
}

// validateUsername validates a username for new user signup
// Checks: length, pattern, reserved words, uniqueness
func (h *AuthHandler) validateUsername(ctx context.Context, username string, excludeUserID uuid.UUID) error {
	username = identityusername.Normalize(username)
	if err := identityusername.ValidateFormat(username); err != nil {
		return err
	}

	// Check if reserved
	if identityusername.IsReserved(username) {
		return fmt.Errorf("username '%s' is reserved and cannot be used", username)
	}

	// Check if username is already taken
	var existingUserID uuid.UUID
	err := h.pool.QueryRow(ctx, `
		SELECT user_id FROM user_profiles WHERE LOWER(username) = $1
	`, username).Scan(&existingUserID)
	if err == nil {
		// Username exists, but check if it's the same user (for updates)
		if existingUserID != excludeUserID {
			return fmt.Errorf("username '%s' is already taken", username)
		}
	}

	return nil
}

// RefreshToken handles POST /api/v1/auth/refresh
//
// Implements stateful single-use refresh token rotation:
//  1. Validate JWT signature, expiry, and type="refresh".
//  2. Hash the raw token and look up the active session row.
//  3. If session is consumed/revoked (rotation attack): revoke entire family → 401.
//  4. If session is active: atomically consume old row and insert new session.
//  5. Return new access_token + refresh_token pair.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "refresh_token is required")
		return
	}

	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken == "" {
		response.BadRequest(c, "refresh_token cannot be empty")
		return
	}

	// Step 1: validate JWT (signature, expiry, type discriminator).
	claims, err := h.tokenService.ValidateRefreshToken(rawToken)
	if err != nil {
		h.log.Warn("Refresh token JWT validation failed", zap.Error(err))
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired refresh token")
		return
	}

	tokenHash := authrepo.HashRefreshToken(rawToken)

	// Parse JTI and FamilyID from claims (needed for rotation attack handling).
	claimsJTI, jtiErr := uuid.Parse(claims.ID) // RegisteredClaims.ID = jti
	claimsFamilyID, famErr := uuid.Parse(claims.FamilyID)
	if jtiErr != nil || famErr != nil {
		h.log.Warn("Refresh token claims missing jti or family_id",
			zap.String("jti", claims.ID),
			zap.String("family_id", claims.FamilyID),
		)
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid refresh token claims")
		return
	}

	// Step 2+3+4: session lookup and atomic rotation within a single tx.
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("Failed to begin refresh tx", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	session, findErr := h.refreshSessionRepo.FindActiveByTokenHash(ctx, tx, tokenHash)
	if findErr != nil {
		if errors.Is(findErr, authrepo.ErrSessionNotActive) {
			// Session exists but is consumed/revoked — rotation attack detected.
			h.log.Warn("Refresh token reuse detected — revoking session family",
				zap.String("family_id", claimsFamilyID.String()),
				zap.String("jti", claimsJTI.String()),
			)
			if revokeErr := h.refreshSessionRepo.MarkReusedAndRevokeFamily(ctx, tx, claimsFamilyID, claimsJTI); revokeErr != nil {
				h.log.Error("Failed to revoke reused refresh token family", zap.Error(revokeErr))
				response.InternalServerError(c, "Database error")
				return
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				h.log.Error("Failed to commit refresh token reuse transaction", zap.Error(commitErr))
				response.InternalServerError(c, "Database error")
				return
			}
			response.Error(c, http.StatusUnauthorized, "TOKEN_REUSE", "Refresh token already used — all sessions revoked")
			return
		}
		// Not found at all (legacy pre-rotation token or invalid hash).
		h.log.Warn("Refresh session not found", zap.String("hash_prefix", tokenHash[:8]))
		response.Error(c, http.StatusUnauthorized, "SESSION_NOT_FOUND", "Refresh session not found")
		return
	}

	// Step 3: verify user is still active.
	var accountStatus, dbRole string
	dbErr := tx.QueryRow(ctx, `
		SELECT account_status, role
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, session.UserID).Scan(&accountStatus, &dbRole)
	if dbErr != nil {
		if errors.Is(dbErr, pgx.ErrNoRows) {
			response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		h.log.Error("Database error fetching user for refresh", zap.Error(dbErr))
		response.InternalServerError(c, "Database error")
		return
	}
	if accountStatus != "active" {
		response.Error(c, http.StatusForbidden, "ACCOUNT_INACTIVE", fmt.Sprintf("Account is %s", accountStatus))
		return
	}
	roles := []string{dbRole}
	if dbRole == "" {
		roles = []string{"user"}
	}

	// Step 4: generate new token pair, preserving existing family_id.
	newPair, pairErr := h.tokenService.GenerateTokenPair(session.UserID, roles, &claimsFamilyID)
	if pairErr != nil {
		h.log.Error("Failed to generate rotated token pair", zap.Error(pairErr))
		response.InternalServerError(c, "Failed to generate tokens")
		return
	}

	// Atomically consume old session and insert new session in the same tx.
	newHash := authrepo.HashRefreshToken(newPair.RefreshToken)
	newSession, newSessErr := authentity.NewRefreshSession(
		session.UserID,
		newPair.FamilyID,
		newPair.RefreshJTI,
		newHash,
		newPair.RefreshExpiresAt,
	)
	if newSessErr != nil {
		h.log.Error("Failed to build replacement session entity", zap.Error(newSessErr))
		response.InternalServerError(c, "Failed to rotate session")
		return
	}
	if rotateErr := h.refreshSessionRepo.ConsumeAndReplace(ctx, tx, session.JTI, newSession); rotateErr != nil {
		h.log.Error("Failed to atomically rotate refresh session", zap.Error(rotateErr))
		response.InternalServerError(c, "Failed to rotate session")
		return
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		h.log.Error("Failed to commit refresh rotation tx", zap.Error(commitErr))
		response.InternalServerError(c, "Database error")
		return
	}

	h.log.Info("Refresh token rotated",
		zap.String("user_id", session.UserID.String()),
		zap.String("family_id", claimsFamilyID.String()),
		zap.String("old_jti", session.JTI.String()),
		zap.String("new_jti", newPair.RefreshJTI.String()),
	)

	response.Success(c, dto.RefreshTokenResponse{
		AccessToken:      newPair.AccessToken,
		RefreshToken:     newPair.RefreshToken,
		ExpiresAt:        newPair.ExpiresAt.Format(time.RFC3339),
		RefreshExpiresAt: newPair.RefreshExpiresAt.Format(time.RFC3339),
	})
}

// CompleteProfile handles POST /api/v1/auth/complete-profile.
func (h *AuthHandler) CompleteProfile(c *gin.Context) {
	ctx := c.Request.Context()

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(c, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN", "Restricted token is required")
		return
	}

	rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if rawToken == "" {
		response.Error(c, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN", "Restricted token is required")
		return
	}

	claims, err := h.tokenService.ValidateRestrictedCompletionToken(rawToken)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "token use mismatch") || strings.Contains(errMsg, "scope mismatch") || strings.Contains(errMsg, "token type mismatch") {
			response.Error(c, http.StatusForbidden, "INVALID_SCOPE", "Restricted token scope is invalid")
			return
		}
		response.Error(c, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN", "Invalid or expired restricted token")
		return
	}

	var req dto.CompleteProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "username is required")
		return
	}

	username := identityusername.Normalize(req.Username)
	if err := identityusername.ValidateFormat(username); err != nil {
		response.Error(c, http.StatusBadRequest, "USERNAME_INVALID_FORMAT", err.Error())
		return
	}
	if identityusername.IsReserved(username) {
		response.Error(c, http.StatusBadRequest, "USERNAME_RESERVED", "This username is reserved and cannot be used")
		return
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("Failed to begin complete profile tx", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Serialize duplicate completion attempts for the same user so only one
	// request can progress through username establishment and session minting
	// at a time.
	if err := h.lockCompletionUser(ctx, tx, claims.UserID); err != nil {
		h.log.Error("Failed to lock completion user", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	authUser, err := h.getActiveAuthUserByID(ctx, tx, claims.UserID)
	if err != nil {
		h.log.Error("Failed to load user for profile completion", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}
	if authUser == nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN", "Invalid restricted token user")
		return
	}
	if authUser.AccountStatus != "active" {
		response.Error(c, http.StatusForbidden, "ACCOUNT_RESTRICTED", fmt.Sprintf("Account is %s", authUser.AccountStatus))
		return
	}
	if authUser.Username != nil && *authUser.Username != "" {
		response.Error(c, http.StatusConflict, "PROFILE_ALREADY_COMPLETED", "Profile is already completed")
		return
	}

	tag, err := tx.Exec(ctx, `
		UPDATE user_profiles
		SET username = $2, updated_at = NOW()
		WHERE user_id = $1 AND (username IS NULL OR username = '')
	`, authUser.ID, username)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.Error(c, http.StatusConflict, "USERNAME_TAKEN", "This username is already taken")
			return
		}
		h.log.Error("Failed to update username", zap.Error(err))
		response.InternalServerError(c, "Failed to complete profile")
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(c, http.StatusConflict, "PROFILE_ALREADY_COMPLETED", "Profile is already completed")
		return
	}

	updatedUser, err := h.getActiveAuthUserByID(ctx, tx, authUser.ID)
	if err != nil {
		h.log.Error("Failed to reload completed user", zap.Error(err))
		response.InternalServerError(c, "Failed to complete profile")
		return
	}
	if updatedUser == nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_RESTRICTED_TOKEN", "Invalid restricted token user")
		return
	}
	if err := h.syncEmailVerifiedSnapshot(ctx, tx, updatedUser, updatedUser.EmailVerifiedAt != nil); err != nil {
		h.log.Error("Failed to sync post-completion snapshot", zap.Error(err))
		response.InternalServerError(c, "Failed to complete profile")
		return
	}
	roles := []string{updatedUser.Role}
	tokenPair, err := h.tokenService.GenerateTokenPair(updatedUser.ID, roles, nil)

	if err != nil {
		h.log.Error("Failed to generate full session for completion", zap.Error(err))
		response.InternalServerError(c, "Failed to generate tokens")
		return
	}

	tokenHash := authrepo.HashRefreshToken(tokenPair.RefreshToken)
	session, sessionErr := authentity.NewRefreshSession(
		updatedUser.ID,
		tokenPair.FamilyID,
		tokenPair.RefreshJTI,
		tokenHash,
		tokenPair.RefreshExpiresAt,
	)
	if sessionErr != nil {
		h.log.Error("Failed to build refresh session entity", zap.Error(sessionErr))
		response.InternalServerError(c, "Failed to create session")
		return
	}
	if createErr := h.refreshSessionRepo.Create(ctx, tx, session); createErr != nil {
		h.log.Error("Failed to persist refresh session", zap.Error(createErr))
		response.InternalServerError(c, "Failed to create session")
		return
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		h.log.Error("Failed to commit profile completion tx", zap.Error(commitErr))
		response.InternalServerError(c, "Failed to complete profile")
		return
	}

	resp := dto.FirebaseExchangeCompleteResponse{
		UserID:                    updatedUser.ID.String(),
		RequiresProfileCompletion: false,
		AccessToken:               tokenPair.AccessToken,
		RefreshToken:              tokenPair.RefreshToken,
		ExpiresAt:                 tokenPair.ExpiresAt.Format(time.RFC3339),
		RefreshExpiresAt:          tokenPair.RefreshExpiresAt.Format(time.RFC3339),
		Created:                   false,
	}

	response.Success(c, resp)
}

// Logout handles POST /api/v1/auth/logout.
//
// The caller must be authenticated by middleware. The request body must
// include the current refresh token so we can revoke the correct session
// family without a global blacklist or token_version invalidation.
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "refresh_token is required")
		return
	}

	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken == "" {
		response.BadRequest(c, "refresh_token cannot be empty")
		return
	}

	if err := h.logoutService.LogoutCurrentSession(ctx, userID, application.LogoutCurrentSessionRequest{
		RefreshToken: rawToken,
		FCMToken:     req.FCMToken,
		DeviceID:     req.DeviceID,
	}); err != nil {
		var logoutErr *application.LogoutCurrentSessionError
		if errors.As(err, &logoutErr) {
			response.Error(c, logoutErr.Status, logoutErr.Code, logoutErr.Message)
			return
		}
		h.log.Error("Failed to complete logout", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	response.Success(c, gin.H{
		"message": "Logged out successfully",
	})
}

// LogoutAll handles POST /api/v1/auth/logout-all.
//
// The caller must be authenticated by middleware. The request does not
// require a refresh token because this is an account-wide session revocation
// authority.
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req dto.LogoutAllRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "invalid request body")
		return
	}

	result, err := h.logoutService.LogoutAllSessions(ctx, userID, application.LogoutAllSessionsRequest{
		DeactivateFCMTokens: req.DeactivateFCMTokens,
	})
	if err != nil {
		var logoutErr *application.LogoutAllSessionsError
		if errors.As(err, &logoutErr) {
			response.Error(c, logoutErr.Status, logoutErr.Code, logoutErr.Message)
			return
		}
		h.log.Error("Failed to complete logout-all", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	response.Success(c, gin.H{
		"message":                      "Logged out from all devices successfully",
		"revoked_sessions_count":       result.RevokedSessionsCount,
		"deactivated_fcm_tokens_count": result.DeactivatedFCMTokensCount,
	})
}

// ListSessions handles GET /api/v1/auth/sessions.
//
// The caller must be authenticated by middleware. The response only exposes
// safe device/session snapshot fields needed by the future Login Sessions UI.
func (h *AuthHandler) ListSessions(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	sessions, err := h.logoutService.ListActiveSessions(ctx, userID)
	if err != nil {
		var sessionErr *application.SessionManagementError
		if errors.As(err, &sessionErr) {
			response.Error(c, sessionErr.Status, sessionErr.Code, sessionErr.Message)
			return
		}
		h.log.Error("Failed to list active sessions", zap.Error(err))
		response.InternalServerError(c, "Database error")
		return
	}

	response.Success(c, gin.H{
		"sessions": mapSessionDeviceSummaries(sessions),
	})
}

// RevokeSession handles DELETE /api/v1/auth/sessions/:family_id.
//
// The caller must be authenticated by middleware. The family_id is scoped to
// the authenticated user, so revoking one family never affects other users.
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	familyID, err := uuid.Parse(strings.TrimSpace(c.Param("family_id")))
	if err != nil || familyID == uuid.Nil {
		response.BadRequest(c, "family_id must be a valid UUID")
		return
	}

	result, revokeErr := h.logoutService.RevokeSessionFamily(ctx, userID, familyID)
	if revokeErr != nil {
		var sessionErr *application.SessionManagementError
		if errors.As(revokeErr, &sessionErr) {
			response.Error(c, sessionErr.Status, sessionErr.Code, sessionErr.Message)
			return
		}
		h.log.Error("Failed to revoke session family", zap.Error(revokeErr))
		response.InternalServerError(c, "Database error")
		return
	}

	resp := gin.H{
		"message":                      "Session family revoked successfully",
		"revoked_sessions_count":       result.RevokedSessionsCount,
		"deactivated_fcm_tokens_count": result.DeactivatedFCMTokensCount,
	}
	if result.FCMWarning != nil {
		resp["fcm_warning"] = result.FCMWarning.Error()
	}

	response.Success(c, resp)
}

// Helper function to log errors
func (h *AuthHandler) logError(c *gin.Context, message string, err error) {
	h.log.Error(message,
		zap.String("path", c.FullPath()),
		zap.Error(err),
	)
}

func mapSessionDeviceSummaries(sessions []*application.SessionDeviceSummary) []gin.H {
	out := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		if session == nil {
			continue
		}
		item := gin.H{
			"family_id":  session.FamilyID.String(),
			"issued_at":  session.IssuedAt.UTC().Format(time.RFC3339),
			"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
		}
		if session.DeviceID != nil {
			item["device_id"] = *session.DeviceID
		}
		if session.DeviceName != nil {
			item["device_name"] = *session.DeviceName
		}
		if session.Platform != nil {
			item["platform"] = *session.Platform
		}
		if session.AppVersion != nil {
			item["app_version"] = *session.AppVersion
		}
		if session.LastUsedAt != nil {
			item["last_used_at"] = session.LastUsedAt.UTC().Format(time.RFC3339)
		}
		if session.FCMTokenActive != nil {
			item["fcm_token_active"] = *session.FCMTokenActive
		}
		out = append(out, item)
	}
	return out
}
