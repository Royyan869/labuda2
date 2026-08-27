package firebase

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// Client wraps Firebase Auth and Messaging clients
// P0-3: Added FCM messaging support
type Client struct {
	AuthClient     *auth.Client
	MessagingClient *messaging.Client
	log            *logger.Logger
}

// NewFirebaseClient creates a new Firebase client
// P0-3: Initializes both Auth and Messaging clients
func NewFirebaseClient(cfg *config.FirebaseConfig, log *logger.Logger) (*Client, error) {
	ctx := context.Background()

	// Initialize Firebase app with service account
	opt := option.WithCredentialsFile(cfg.ServiceAccountKeyPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Get Auth client
	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase Auth client: %w", err)
	}

	// P0-3: Get Messaging client for FCM push notifications
	messagingClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase Messaging client: %w", err)
	}

	log.Info("Firebase initialized successfully",
		zap.String("project_id", cfg.ProjectID),
	)

	return &Client{
		AuthClient:      authClient,
		MessagingClient: messagingClient,
		log:             log,
	}, nil
}

// VerifyIDToken verifies a Firebase ID token
// Handles both real Firebase Auth and mock mode (DEV_MOCK_FIREBASE_AUTH)
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	// HANDLE MOCK MODE - when AuthClient is nil, use mock implementation
	if c.AuthClient == nil {
		c.log.Debug("Using mock mode for VerifyIDToken")
		return c.VerifyIDTokenMock(ctx, idToken)
	}

	token, err := c.AuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		c.log.Warn("Failed to verify ID token", zap.Error(err))
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return token, nil
}

// GetUser gets a user by UID
func (c *Client) GetUser(ctx context.Context, uid string) (*auth.UserRecord, error) {
	if c.AuthClient == nil {
		// Mock mode - return nil, this is not used in auth flow
		return nil, fmt.Errorf("mock mode: GetUser not supported")
	}

	user, err := c.AuthClient.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserByEmail gets a user by email
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*auth.UserRecord, error) {
	if c.AuthClient == nil {
		// Mock mode - return nil, this is not used in auth flow
		return nil, fmt.Errorf("mock mode: GetUserByEmail not supported")
	}

	user, err := c.AuthClient.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// CreateUser creates a new Firebase user
func (c *Client) CreateUser(ctx context.Context, email, password string) (*auth.UserRecord, error) {
	if c.AuthClient == nil {
		// Mock mode - return nil, this is not used in auth flow
		return nil, fmt.Errorf("mock mode: CreateUser not supported")
	}

	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password).
		EmailVerified(false)

	user, err := c.AuthClient.CreateUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	c.log.Info("Firebase user created", zap.String("uid", user.UID), zap.String("email", email))
	return user, nil
}

// DeleteUser deletes a Firebase user
func (c *Client) DeleteUser(ctx context.Context, uid string) error {
	if c.AuthClient == nil {
		// Mock mode - just log and return success
		c.log.Info("Mock mode: skip delete user", zap.String("uid", uid))
		return nil
	}

	if err := c.AuthClient.DeleteUser(ctx, uid); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	c.log.Info("Firebase user deleted", zap.String("uid", uid))
	return nil
}

// SetCustomClaims sets custom claims for a user
func (c *Client) SetCustomClaims(ctx context.Context, uid string, claims map[string]interface{}) error {
	if c.AuthClient == nil {
		// Mock mode - just log and return success
		c.log.Info("Mock mode: skip set custom claims", zap.String("uid", uid))
		return nil
	}

	if err := c.AuthClient.SetCustomUserClaims(ctx, uid, claims); err != nil {
		return fmt.Errorf("failed to set custom claims: %w", err)
	}

	c.log.Info("Custom claims set", zap.String("uid", uid))
	return nil
}

// UserExists checks if a user exists in Firebase Auth by UID
// Returns false if user is not found (including deleted/disabled users)
func (c *Client) UserExists(ctx context.Context, uid string) bool {
	if c.AuthClient == nil {
		// Mock mode - always return true
		return true
	}

	_, err := c.AuthClient.GetUser(ctx, uid)
	return err == nil
}

// IsUserDisabled checks if a user account is disabled in Firebase
func (c *Client) IsUserDisabled(ctx context.Context, uid string) (bool, error) {
	if c.AuthClient == nil {
		// Mock mode - always return false (user is not disabled)
		return false, nil
	}

	user, err := c.AuthClient.GetUser(ctx, uid)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	return user.Disabled, nil
}

// =============================================================================
// P0-3: FCM Messaging Methods
// These methods handle sending push notifications via Firebase Cloud Messaging
// =============================================================================

// SendPushNotification sends a push notification to a single FCM token
// P0-3: Implements actual FCM send with invalid token handling
func (c *Client) SendPushNotification(ctx context.Context, fcmToken string, title, body string, data map[string]string, imageURL string) error {
	if c.MessagingClient == nil {
		return fmt.Errorf("messaging client not initialized")
	}

	message := &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	if imageURL != "" {
		message.Notification.ImageURL = imageURL
	}

	// Send the message
	_, err := c.MessagingClient.Send(ctx, message)
	if err != nil {
		// P0-3: Handle specific FCM errors
		if messaging.IsInvalidArgument(err) || messaging.IsUnregistered(err) {
			c.log.Warn("Invalid FCM token - should be cleaned up",
				zap.String("token", fcmToken[:20]+"..."),
				zap.Error(err),
			)
			return fmt.Errorf("invalid token: %w", err)
		}
		c.log.Error("Failed to send FCM notification",
			zap.String("token", fcmToken[:20]+"..."),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send: %w", err)
	}

	c.log.Debug("FCM notification sent",
		zap.String("token", fcmToken[:20]+"..."),
		zap.String("title", title),
	)
	return nil
}

// SendBatchPush sends a push notification to multiple FCM tokens
// P0-3: Implements batch FCM send with partial failure handling
func (c *Client) SendBatchPush(ctx context.Context, fcmTokens []string, title, body string, data map[string]string) ([]string, error) {
	if c.MessagingClient == nil {
		return nil, fmt.Errorf("messaging client not initialized")
	}

	if len(fcmTokens) == 0 {
		return nil, nil
	}

	// Build multicast message
	message := &messaging.MulticastMessage{
		Tokens: fcmTokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	// Send multicast message (max 500 tokens per request per FCM limits)
	br, err := c.MessagingClient.SendMulticast(ctx, message)
	if err != nil {
		c.log.Error("Failed to send batch FCM",
			zap.Int("count", len(fcmTokens)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to send batch: %w", err)
	}

	// P0-3: Collect invalid tokens for cleanup
	var invalidTokens []string
	for i, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsInvalidArgument(resp.Error) || messaging.IsUnregistered(resp.Error) {
				invalidTokens = append(invalidTokens, fcmTokens[i])
			}
			c.log.Warn("FCM send failed for token",
				zap.Int("index", i),
				zap.Error(resp.Error),
			)
		}
	}

	c.log.Info("Batch FCM sent",
		zap.Int("success_count", br.SuccessCount),
		zap.Int("failure_count", br.FailureCount),
	)

	return invalidTokens, nil
}

// IsInvalidToken checks if an error indicates an invalid FCM token
// P0-3: Helper for token cleanup logic
func (c *Client) IsInvalidToken(err error) bool {
	return messaging.IsInvalidArgument(err) || messaging.IsUnregistered(err)
}

// =============================================================================
// Mock Client for Testing (P5.4)
// =============================================================================

// MockClient is a mock implementation for testing without real Firebase
type MockClient struct {
	log *logger.Logger
}

// NewMockClient creates a mock Firebase client for testing
// P5.4: Used in staging when DEV_MOCK_FIREBASE_AUTH=true
func NewMockClient(log *logger.Logger) *Client {
	if log == nil {
		log = &logger.Logger{}
	}
	return &Client{
		AuthClient:      nil, // Mock client doesn't use real Auth client
		MessagingClient: nil,
		log:             log,
	}
}

// VerifyIDToken mock implementation - accepts any token for testing
// MOCK AUTH ENHANCEMENT: Generate unique UIDs based on token for multi-user testing
// Different tokens map to different mock users, allowing proper testing of user flows
//
// DEV-ONLY EMAIL-VERIFIED CONVENTION (Batch B6.4):
// When the mock bearer token contains the substring "verified" (case-
// insensitive), the returned claim set includes `email_verified: true`.
// The downstream auth handler reads this claim and writes `email_verified_at`
// at user-create time (or via syncEmailVerifiedSnapshot on subsequent
// logins, monotonically), which lets `RequireInteractionAuthority` pass
// for dev / corpus-driver flows WITHOUT touching the middleware or
// inserting rows via SQL. Real Firebase auth never reaches this mock —
// see VerifyIDToken's `c.AuthClient == nil` gate.
//
// Tokens without the substring keep today's behavior (email_verified
// absent in claims → false → email_verified_at unset). The convention is
// opt-in only and the marker is plainly visible in the token string.
func (c *Client) VerifyIDTokenMock(ctx context.Context, idToken string) (*auth.Token, error) {
	verified := isVerifiedMockToken(idToken)

	// Handle special test tokens directly
	if idToken == "seller-1" {
		claims := map[string]interface{}{
			"user_id": "seller-1",
			"email":   "seller@test.com",
			"name":    "testseller",
		}
		if verified {
			claims["email_verified"] = true
		}
		return &auth.Token{UID: "seller-1", Claims: claims}, nil
	}

	if idToken == "buyer-1" {
		claims := map[string]interface{}{
			"user_id": "buyer-1",
			"email":   "buyer@test.com",
			"name":    "testbuyer",
		}
		if verified {
			claims["email_verified"] = true
		}
		return &auth.Token{UID: "buyer-1", Claims: claims}, nil
	}

	// Generate a consistent UID based on the token for reproducible testing
	// Same token always maps to same mock user, different tokens map to different users
	mockUID := "mock-" + hashToken(idToken)

	// Parse token for custom user data (format: "role:username" or just "token")
	var email, username string
	if len(idToken) > 0 && idToken != "any-fake-token" && idToken != "test-token" && idToken != "fake-token" {
		// Use token as email/username for personalized testing
		email = sanitizeEmail(idToken)
		username = sanitizeUsername(idToken)
	} else {
		// Default mock user
		email = "mock@test.com"
		username = "mockuser"
	}

	claims := map[string]interface{}{
		"user_id": mockUID,
		"email":   email,
		"name":    username, // Display name for username preservation
	}
	if verified {
		claims["email_verified"] = true
	}
	return &auth.Token{UID: mockUID, Claims: claims}, nil
}

// isVerifiedMockToken reports whether a mock bearer token opts into the
// dev-only email-verified convention. The marker is the substring
// "verified" (case-insensitive). Centralized so the convention has exactly
// one source of truth shared by every mock path (seller-1, buyer-1, and
// the generic hash path).
func isVerifiedMockToken(idToken string) bool {
	if idToken == "" {
		return false
	}
	// strings.Contains + ToLower keeps this allocation-free for the common
	// short-token path and avoids regex / parser overhead.
	return strings.Contains(strings.ToLower(idToken), "verified")
}

// hashToken creates a consistent hash from token for UID generation
func hashToken(token string) string {
	// Simple hash for testing - convert token to numeric string
	hash := 0
	for i, c := range token {
		hash = hash*31 + int(c) + i
	}
	if hash < 0 {
		hash = -hash
	}
	return "user" + string(rune('0'+hash%10)) + string(rune('0'+(hash/10)%10)) + string(rune('0'+(hash/100)%10))
}

// sanitizeEmail creates a safe email from token for testing
func sanitizeEmail(token string) string {
	if len(token) > 30 {
		token = token[:30]
	}
	// Remove special characters and add domain
	safe := token
	for _, c := range []string{" ", "/", ":", "\\", "'", "\""} {
		safe = replaceAll(safe, c, "")
	}
	return safe + "@test.com"
}

// sanitizeUsername creates a safe username from token for testing
func sanitizeUsername(token string) string {
	if len(token) > 20 {
		token = token[:20]
	}
	// Remove special characters
	safe := token
	for _, c := range []string{" ", "/", ":", "\\", "'", "\"", ".", "@"} {
		safe = replaceAll(safe, c, "")
	}
	return safe
}

// replaceAll is a simple string replacement helper
func replaceAll(s, old, new string) string {
	result := ""
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
