package http

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	userRepo "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	identityusername "github.com/labuda/backend/internal/identity/username"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// UserHandler handles user profile endpoints
// CONTRACT V1: Public profile endpoints that return honest user data
type UserHandler struct {
	userProfileService *userApp.UserProfileService
	db                 *db.DB
	log                *zap.Logger
}

type updateProfileRequest struct {
	Username      *string `json:"username"`
	Bio           *string `json:"bio"`
	AvatarURL     *string `json:"avatar_url"`
	CoverPhotoURL *string `json:"cover_photo_url"`
	Location      *string `json:"location"`
	PhoneNumber   *string `json:"phone_number"`
}

type checkUsernameResponse struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type profileUpdateError struct {
	status  int
	code    string
	message string
}

type verificationRefreshResponse struct {
	PhoneVerified   bool       `json:"phone_verified"`
	PhoneNumber     *string    `json:"phone_number"`
	PhoneVerifiedAt *time.Time `json:"phone_verified_at,omitempty"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
}

func (e *profileUpdateError) Error() string {
	return e.message
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userProfileService *userApp.UserProfileService, database *db.DB, log *zap.Logger) *UserHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &UserHandler{
		userProfileService: userProfileService,
		db:                 database,
		log:                log,
	}
}

// GetPublicUser handles GET /api/v1/users/:id
func (h *UserHandler) GetPublicUser(c *gin.Context) {
	ctx := c.Request.Context()

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	userIDVal, exists := c.Get("userID")
	var isOwnProfile bool
	var requestingUserID uuid.UUID
	if exists {
		requestingUserID, _ = userIDVal.(uuid.UUID)
		isOwnProfile = requestingUserID == targetUserID
	}

	// Block enforcement: hide profile from blocked user's viewer
	if !isOwnProfile && requestingUserID != uuid.Nil {
		var blocked bool
		_ = h.db.WithTx(ctx, func(tx db.Tx) error {
			var err error
			blocked, err = blockcheck.IsBidirectionallyBlocked(ctx, tx, requestingUserID, targetUserID)
			return err
		})
		if blocked {
			response.NotFound(c, "User not found")
			return
		}
	}

	resp, err := h.userProfileService.GetPublicProfile(ctx, targetUserID, isOwnProfile)
	if err != nil {
		h.log.Error("Failed to get public user profile",
			zap.String("user_id", targetUserID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve user profile")
		return
	}

	response.Success(c, resp)
}

// GetMyProfile handles GET /api/v1/users/me
func (h *UserHandler) GetMyProfile(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	resp, err := h.userProfileService.GetMyProfile(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get user profile",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve user profile")
		return
	}

	response.Success(c, resp)
}

// UpdateMyProfile handles PATCH /api/v1/users/me/profile.
func (h *UserHandler) UpdateMyProfile(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	repo := userRepo.NewUserRepository(h.db)
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		currentProfile, err := repo.GetProfileByID(ctx, tx, userID)
		if err != nil {
			return err
		}

		currentUsername := currentUsernameValue(currentProfile)
		if !hasProfileUpdateFields(req) {
			if currentUsername == "" {
				return &profileUpdateError{
					status:  http.StatusBadRequest,
					code:    "USERNAME_REQUIRED",
					message: "Username is required to complete profile",
				}
			}
			return nil
		}

		input := &userEntity.UpdateProfileInput{}
		if req.Username != nil {
			normalizedUsername := identityusername.Normalize(*req.Username)
			if err := identityusername.ValidateFormat(normalizedUsername); err != nil {
				return &profileUpdateError{
					status:  http.StatusBadRequest,
					code:    "USERNAME_INVALID_FORMAT",
					message: err.Error(),
				}
			}
			if identityusername.IsReserved(normalizedUsername) {
				return &profileUpdateError{
					status:  http.StatusBadRequest,
					code:    "USERNAME_RESERVED",
					message: "This username is reserved and cannot be used",
				}
			}
			// Username is immutable after first establishment: once set, it can
			// never be changed through this endpoint (or any other path — there
			// is no rename capability anywhere in the codebase). This is a
			// deliberate safety property, not a missing feature: renaming would
			// silently break identity continuity across moderation cases,
			// disputes, and every other domain that resolves username live by
			// user_id, since none of them capture a historical snapshot. If
			// rename is ever approved as a feature, it must ship together with
			// username-history/admin-traceability infrastructure, not before.
			if currentUsername != "" && normalizedUsername != currentUsername {
				return &profileUpdateError{
					status:  http.StatusConflict,
					code:    "USERNAME_ALREADY_SET",
					message: "Username is already set and cannot be changed here",
				}
			}
			if currentUsername == "" {
				taken, err := repo.IsUsernameTaken(ctx, tx, normalizedUsername, userID)
				if err != nil {
					return err
				}
				if taken {
					return &profileUpdateError{
						status:  http.StatusConflict,
						code:    "USERNAME_UNAVAILABLE",
						message: "This username is already taken",
					}
				}
				input.Username = &normalizedUsername
			}
		}
		if req.Bio != nil {
			bio := strings.TrimSpace(*req.Bio)
			input.Bio = &bio
		}
		if req.Location != nil {
			location := strings.TrimSpace(*req.Location)
			input.Location = &location
		}
		if req.PhoneNumber != nil {
			phoneNumber := strings.TrimSpace(*req.PhoneNumber)
			currentUser, err := repo.GetByIDForUpdate(ctx, tx, userID)
			if err != nil {
				return err
			}
			if currentUser == nil {
				return fmt.Errorf("user not found")
			}
			currentUser.PhoneNumber = &phoneNumber
			if err := repo.Update(ctx, tx, currentUser); err != nil {
				return err
			}
		}
		if req.AvatarURL != nil {
			avatarURL := strings.TrimSpace(*req.AvatarURL)
			if avatarURL != "" {
				if _, err := parseAbsoluteURL(avatarURL); err != nil {
					return &profileUpdateError{
						status:  http.StatusBadRequest,
						code:    "INVALID_AVATAR_URL",
						message: "avatar_url must be a valid absolute URL",
					}
				}
			}
			input.AvatarURL = &avatarURL
		}
		if req.CoverPhotoURL != nil {
			coverURL := strings.TrimSpace(*req.CoverPhotoURL)
			if coverURL != "" {
				if err := validateCoverPhotoReference(coverURL, userID); err != nil {
					return &profileUpdateError{
						status:  http.StatusBadRequest,
						code:    "INVALID_COVER_PHOTO_URL",
						message: err.Error(),
					}
				}
			}
			input.CoverPhotoURL = &coverURL
		}

		if !hasEffectiveProfileUpdate(input) {
			return nil
		}

		_, err = repo.UpdateProfile(ctx, tx, userID, input)
		return err
	})
	if err != nil {
		var updateErr *profileUpdateError
		if errors.As(err, &updateErr) {
			response.Error(c, updateErr.status, updateErr.code, updateErr.message)
			return
		}

		h.log.Error("Failed to update profile",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to update profile")
		return
	}

	profile, err := h.userProfileService.GetMyProfile(ctx, userID)
	if err != nil {
		h.log.Error("Failed to load updated profile",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve updated profile")
		return
	}

	response.Success(c, profile)
}

// CheckUsername handles GET /api/v1/users/check-username.
func (h *UserHandler) CheckUsername(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	username := identityusername.Normalize(c.Query("username"))
	if err := identityusername.ValidateFormat(username); err != nil {
		response.Success(c, checkUsernameResponse{
			Available: false,
			Reason:    "USERNAME_INVALID_FORMAT",
		})
		return
	}
	if identityusername.IsReserved(username) {
		response.Success(c, checkUsernameResponse{
			Available: false,
			Reason:    "USERNAME_RESERVED",
		})
		return
	}

	repo := userRepo.NewUserRepository(h.db)
	var available bool
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		taken, err := repo.IsUsernameTaken(ctx, tx, username, userID)
		if err != nil {
			return err
		}
		available = !taken
		return nil
	})
	if err != nil {
		h.log.Error("Failed to check username",
			zap.String("user_id", userID.String()),
			zap.String("username", username),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check username")
		return
	}

	if !available {
		response.Success(c, checkUsernameResponse{
			Available: false,
			Reason:    "USERNAME_UNAVAILABLE",
		})
		return
	}

	response.Success(c, checkUsernameResponse{Available: true})
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return uuid.Nil, false
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return uuid.Nil, false
	}
	return userID, true
}

func hasProfileUpdateFields(req updateProfileRequest) bool {
	return req.Username != nil ||
		req.Bio != nil ||
		req.AvatarURL != nil ||
		req.CoverPhotoURL != nil ||
		req.Location != nil ||
		req.PhoneNumber != nil
}

func hasEffectiveProfileUpdate(input *userEntity.UpdateProfileInput) bool {
	return input.Username != nil ||
		input.Bio != nil ||
		input.AvatarURL != nil ||
		input.CoverPhotoURL != nil ||
		input.Location != nil
}

// validateCoverPhotoReference enforces the canonical cover-photo persistence
// contract: the persisted value is a STORAGE KEY owned by the caller
// (images/profile-covers/{userID}.jpg) or an absolute URL (legacy/backfill
// values, resolved through the existing mediaresolve read path). A caller may
// never claim another user's fixed cover key.
func validateCoverPhotoReference(ref string, userID uuid.UUID) error {
	expected := "images/profile-covers/" + userID.String() + ".jpg"
	if ref == expected {
		return nil
	}
	// Absolute URL — allowed for legacy values and external references.
	if _, err := parseAbsoluteURL(ref); err == nil {
		return nil
	}
	return fmt.Errorf("cover_photo_url must be the caller's canonical storage key %s or a valid absolute URL", expected)
}

func currentUsernameValue(profile *userEntity.UserProfile) string {
	if profile == nil || profile.Username == nil {
		return ""
	}
	return strings.TrimSpace(*profile.Username)
}

func parseAbsoluteURL(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("absolute URL required")
	}
	return parsed, nil
}

// RefreshMyVerification handles POST /api/v1/users/me/verification/refresh.
// It syncs verification snapshot truth from Firebase-backed identity authority.
func (h *UserHandler) RefreshMyVerification(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var emailVerified *bool
	if claims, exists := middleware.GetUserFromContext(c); exists {
		emailVerified = &claims.EmailVerified
	}

	snapshot, err := h.userProfileService.RefreshVerificationSnapshot(ctx, userID, emailVerified)
	if err != nil {
		switch {
		case errors.Is(err, userApp.ErrUserNotProvisioned):
			response.Error(c, http.StatusNotFound, "USER_NOT_PROVISIONED", "User is not provisioned")
			return
		case errors.Is(err, userApp.ErrVerificationRefreshUpstream):
			response.Error(c, http.StatusBadGateway, "UPSTREAM_VERIFICATION_UNAVAILABLE", "Verification refresh upstream unavailable")
			return
		default:
			h.log.Error("Failed to refresh verification snapshot",
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to refresh verification snapshot")
			return
		}
	}

	response.Success(c, verificationRefreshResponse{
		PhoneVerified:   snapshot.PhoneVerified,
		PhoneNumber:     snapshot.PhoneNumber,
		PhoneVerifiedAt: snapshot.PhoneVerifiedAt,
		EmailVerified:   snapshot.EmailVerified,
		EmailVerifiedAt: snapshot.EmailVerifiedAt,
	})
}

// DeleteMyAccount handles DELETE /api/v1/users/me.
// Soft-deletes the authenticated user and emits a user.deleted outbox event.
// The mobile client is expected to call Firebase currentUser.delete() after
// a 204 response from this endpoint.
func (h *UserHandler) DeleteMyAccount(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	if err := h.userProfileService.SelfDeleteAccount(ctx, userID); err != nil {
		h.log.Error("Failed to delete account",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to delete account")
		return
	}

	c.Status(http.StatusNoContent)
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
