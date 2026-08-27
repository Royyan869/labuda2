package dto

// FirebaseExchangeRequest is the request body for POST /api/v1/auth/firebase/exchange.
//
// Username is the canonical registration username for email/password signup.
// It is applied exactly once: when the user's profile has no username yet.
// Omit (or empty) for login / Google-first-sync, where the backend decides
// profile completion on its own.
type FirebaseExchangeRequest struct {
	FirebaseIDToken string  `json:"firebase_id_token" binding:"required"`
	Username        *string `json:"username,omitempty"`
}

// FirebaseExchangeIncompleteResponse is returned when the canonical username
// has not yet been established.
type FirebaseExchangeIncompleteResponse struct {
	UserID                    string  `json:"user_id"`
	RequiresProfileCompletion bool    `json:"requires_profile_completion"`
	Email                     *string `json:"email,omitempty"`
	AccessToken               string  `json:"access_token"`
	ExpiresAt                 string  `json:"expires_at"` // restricted token expiry (RFC3339)
}

// FirebaseExchangeCompleteResponse is returned when the canonical username
// already exists and the backend can mint a full session.
type FirebaseExchangeCompleteResponse struct {
	UserID                    string `json:"user_id"`
	RequiresProfileCompletion bool   `json:"requires_profile_completion"`
	AccessToken               string `json:"access_token"`
	RefreshToken              string `json:"refresh_token"`
	ExpiresAt                 string `json:"expires_at"`         // access token expiry (RFC3339)
	RefreshExpiresAt          string `json:"refresh_expires_at"` // refresh token expiry (RFC3339)
	Created                   bool   `json:"created"`
}

// CompleteProfileRequest is the request body for POST /api/v1/auth/complete-profile.
type CompleteProfileRequest struct {
	Username string `json:"username" binding:"required"`
}

// RefreshTokenRequest is the request body for POST /api/v1/auth/refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse is the response from POST /api/v1/auth/refresh
type RefreshTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresAt        string `json:"expires_at"`         // access token expiry (RFC3339)
	RefreshExpiresAt string `json:"refresh_expires_at"` // refresh token expiry (RFC3339)
}

// LogoutRequest is the request body for POST /api/v1/auth/logout.
//
// refresh_token is required so the backend can revoke the correct refresh
// session family without relying on token_version or a global blacklist.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	FCMToken     string `json:"fcm_token"`
	DeviceID     string `json:"device_id"`
}

// LogoutAllRequest is the request body for POST /api/v1/auth/logout-all.
//
// The request is authenticated and does not require a refresh token.
// deactivate_fcm_tokens defaults to true when omitted.
type LogoutAllRequest struct {
	DeactivateFCMTokens *bool `json:"deactivate_fcm_tokens"`
}

// UserResponse represents user data in the auth response.
// All JSON keys are snake_case — canonical contract shared with user domain DTOs.
// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
