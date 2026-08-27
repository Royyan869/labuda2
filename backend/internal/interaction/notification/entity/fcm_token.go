package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FCMPlatform represents the platform type for FCM tokens.
type FCMPlatform string

const (
	FCMPlatformAndroid FCMPlatform = "android"
	FCMPlatformIOS     FCMPlatform = "ios"
	FCMPlatformWeb     FCMPlatform = "web"
)

// FCMToken represents a Firebase Cloud Messaging token for push notifications.
type FCMToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	Platform  FCMPlatform
	DeviceID  *string // Optional: Unique device identifier for multi-device support
	DeviceName *string // Optional: Human-readable device name
	AppVersion *string // Optional: App version that registered the token
	IsActive  bool
	LastUsedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrFCMTokenNotFound is returned when an FCM token is not found.
type ErrFCMTokenNotFound struct {
	TokenID uuid.UUID
}

func (e *ErrFCMTokenNotFound) Error() string {
	return fmt.Sprintf("fcm token not found: %s", e.TokenID)
}

// NewFCMToken creates a new FCM token.
func NewFCMToken(userID uuid.UUID, token string, platform FCMPlatform, deviceID, deviceName, appVersion *string) *FCMToken {
	now := time.Now()
	return &FCMToken{
		ID:         uuid.New(),
		UserID:     userID,
		Token:      token,
		Platform:   platform,
		DeviceID:   deviceID,
		DeviceName: deviceName,
		AppVersion: appVersion,
		IsActive:   true,
		LastUsedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// MarkUsed updates the LastUsedAt timestamp.
func (t *FCMToken) MarkUsed() {
	now := time.Now()
	t.LastUsedAt = &now
	t.UpdatedAt = now
}

// Deactivate marks the token as inactive.
func (t *FCMToken) Deactivate() {
	t.IsActive = false
	t.UpdatedAt = time.Now()
}


