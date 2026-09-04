package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/platform/response"
)

// StrictBrowseLabudaAuthMiddleware is the Labuda counterpart of StrictBrowseAuthMiddleware.
//
// Behavior (mirrors StrictBrowseAuth but with Labuda JWT):
//  1. No Authorization header → anonymous, pass through (no claims)
//  2. Authorization header present but not "Bearer <token>" → 401
//  3. Bearer token invalid/expired/wrong type (not access) → 401
//  4. Bearer token valid Labuda access JWT → inject canonical user_id + UserClaims, pass through
//
// Use this for public browse routes where unauthenticated readers must be permitted
// but invalid tokens must be rejected so the client knows to clear stale credentials.
// Firebase tokens are NOT accepted here — they will be rejected as 401.
func StrictBrowseLabudaAuthMiddleware(tokenService *application.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Case 1: No token → anonymous
		if authHeader == "" {
			c.Next()
			return
		}

		// Case 2: Malformed header
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Case 3: Validate Labuda access token (type + expiry + signature)
		claims, err := tokenService.ValidateAccessToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired access token")
			c.Abort()
			return
		}

		// Case 4: Valid → inject canonical identity
		c.Set("user_id", claims.UserID)
		c.Set("userID", claims.UserID)
		c.Set(AuthContextKey, &UserClaims{
			UID: claims.UserID.String(),
		})
		c.Next()
	}
}
