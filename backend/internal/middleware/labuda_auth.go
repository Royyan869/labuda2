package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth/application"
	"github.com/labuda/backend/internal/platform/response"
)

// LabudaAuthMiddleware validates canonical Labuda Access JWTs.
// It directly sets the canonical user_id in the context.
func LabudaAuthMiddleware(tokenService *application.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := tokenService.ValidateAccessToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired access token")
			c.Abort()
			return
		}

		// Directly set canonical identities
		c.Set("user_id", claims.UserID)
		c.Set("userID", claims.UserID) // Support legacy alias

		// Create a dummy UserClaims for downstream compatibility that checks AuthContextKey.
		// However, it does not carry Firebase UID.
		userClaims := &UserClaims{
			UID: claims.UserID.String(), // Required by some older compatibility paths
		}
		c.Set(AuthContextKey, userClaims)

		c.Next()
	}
}
