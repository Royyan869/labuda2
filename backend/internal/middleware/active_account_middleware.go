package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// RequireActiveAccount is the canonical middleware for all chat and negotiation surfaces.
//
// Enforces two conditions in order:
//  1. Account status is "active" in the database (queries account_status — not Firebase claims,
//     which do not reflect suspension).
//  2. Email address is verified.
//
// This replaces RequireInteractionAuthority and RequireTransactionAuthority on all chat routes.
// Middleware is a supplementary gate; the service layer independently re-checks account status
// before any mutation — neither gate alone is sufficient.
func RequireActiveAccount(database *db.DB) gin.HandlerFunc {
	checker := auth.NewAccountStatusCheckerDB(database)
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		if err := checker.EnsureActive(c.Request.Context(), userID); err != nil {
			respondAccountStatusError(c, err)
			c.Abort()
			return
		}

		emailVerified := false
		if actor := GetActorFromContext(c); actor != nil {
			emailVerified = actor.EmailVerified
		} else if claims, ok := GetUserFromContext(c); ok {
			emailVerified = claims.EmailVerified
		}
		if !emailVerified {
			response.Error(c, http.StatusForbidden, "EMAIL_VERIFICATION_REQUIRED", "Email verification required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// respondAccountStatusError writes the canonical HTTP classification for an
// EnsureActive failure. Every non-active account state fails closed with a
// 4xx auth-class response; only a genuine lookup/internal failure remains 500.
func respondAccountStatusError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrAccountSuspended):
		response.Error(c, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended.")
	case errors.Is(err, auth.ErrAccountBanned):
		response.Error(c, http.StatusForbidden, "ACCOUNT_BANNED", "Your account has been banned.")
	case errors.Is(err, auth.ErrAccountRemoved):
		response.Error(c, http.StatusForbidden, "ACCOUNT_REMOVED", "Your account has been removed.")
	case errors.Is(err, auth.ErrAccountInactive):
		response.Error(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "Your account is not active.")
	case errors.Is(err, auth.ErrInvalidCaller):
		response.Error(c, http.StatusUnauthorized, "INVALID_CALLER", "Invalid caller identification.")
	default:
		response.InternalServerError(c, "Failed to verify account status")
	}
}


