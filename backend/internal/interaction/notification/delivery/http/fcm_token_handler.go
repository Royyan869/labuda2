package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// FCMTokenHandler handles HTTP requests for FCM token operations.
type FCMTokenHandler struct {
	db  *db.DB
	log *zap.Logger
}

// NewFCMTokenHandler creates a new FCMTokenHandler.
func NewFCMTokenHandler(db *db.DB, log *zap.Logger) *FCMTokenHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &FCMTokenHandler{
		db:  db,
		log: log,
	}
}

// RegisterTokenRequest represents the request to register/update an FCM token.
type RegisterTokenRequest struct {
	Token      string  `json:"token" binding:"required"`
	Platform   string  `json:"platform" binding:"required,oneof=android ios web"`
	DeviceID   *string `json:"device_id"`
	DeviceName *string `json:"device_name"`
	AppVersion *string `json:"app_version"`
}

// RegisterToken handles POST /api/v1/notifications/fcm-token
//
// Registers or updates an FCM token for the authenticated user.
// If a token with the same user_id and device_id exists, it is updated.
func (h *FCMTokenHandler) RegisterToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate platform
	platform := entity.FCMPlatform(req.Platform)

	// Create FCM token entity
	fcmToken := entity.NewFCMToken(
		userID,
		req.Token,
		platform,
		req.DeviceID,
		req.DeviceName,
		req.AppVersion,
	)

	// Persist to database — tx is always non-nil here so pool=nil is safe.
	repo := repository.NewFCMTokenRepository(nil)
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return repo.Insert(ctx, tx, fcmToken)
	})

	if err != nil {
		h.log.Error("Failed to register FCM token",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to register token")
		return
	}

	h.log.Debug("FCM token registered",
		zap.String("user_id", userID.String()),
		zap.String("platform", req.Platform),
		zap.String("device_id", safeDeref(req.DeviceID)),
	)

	response.Success(c, gin.H{
		"success": true,
		"token_id": fcmToken.ID.String(),
	})
}

// UnregisterToken handles DELETE /api/v1/notifications/fcm-token
//
// Unregisters an FCM token for the authenticated user.
// Used when user logs out or token becomes invalid.
func (h *FCMTokenHandler) UnregisterToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body with optional token string
	var req struct {
		Token     *string `json:"token"`
		DeviceID  *string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// If body is empty or invalid, check query parameters
		token := c.Query("token")
		deviceID := c.Query("device_id")
		if token != "" {
			req.Token = &token
		}
		if deviceID != "" {
			req.DeviceID = &deviceID
		}
	}

	// tx is always non-nil inside WithTx so pool=nil is safe.
	repo := repository.NewFCMTokenRepository(nil)
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		if req.Token != nil && *req.Token != "" {
			// Deactivate specific token
			return repo.DeactivateByToken(ctx, tx, *req.Token)
		} else if req.DeviceID != nil && *req.DeviceID != "" {
			// Deactivate all tokens for this device
			return repo.DeactivateByUserAndDevice(ctx, tx, userID, *req.DeviceID)
		} else {
			// Deactivate all tokens for this user
			return repo.DeactivateByUserAndDevice(ctx, tx, userID, "")
		}
	})

	if err != nil {
		h.log.Error("Failed to unregister FCM token",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to unregister token")
		return
	}

	h.log.Debug("FCM token unregistered",
		zap.String("user_id", userID.String()),
	)

	response.Success(c, gin.H{"success": true})
}

// safeDeref safely dereferences a string pointer.
func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}


