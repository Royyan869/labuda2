package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
)

// RequireAdminMiddleware creates middleware that requires admin role.
// This middleware must be used after AuthMiddleware and UserLookupMiddleware.
func RequireAdminMiddleware(roleChecker auth.RoleChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		isAdmin, err := roleChecker.IsAdmin(c.Request.Context(), userID)
		if err != nil {
			_ = c.Error(err)
			response.InternalError(c, "Failed to verify user permissions")
			c.Abort()
			return
		}

		if !isAdmin {
			response.Forbidden(c, "Admin role required")
			c.Abort()
			return
		}

		c.Set("is_admin", true)
		c.Next()
	}
}

// RequireSellerMiddleware creates middleware that requires seller authority.
// It checks market capability only.
func RequireSellerMiddleware(roleChecker auth.RoleChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		hasAuthority, err := roleChecker.HasActiveSellerCapability(c.Request.Context(), userID)
		if err != nil {
			_ = c.Error(err)
			response.InternalError(c, "Failed to verify seller permissions")
			c.Abort()
			return
		}

		if !hasAuthority {
			response.Forbidden(c, "Active seller subscription required. Please renew your subscription to continue selling.")
			c.Abort()
			return
		}

		c.Set("has_market_authority", true)
		c.Next()
	}
}

// RequireSellerProfileMiddleware creates middleware that requires seller
// profile existence only.
func RequireSellerProfileMiddleware(roleChecker auth.RoleChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		hasProfile, err := roleChecker.HasSellerProfile(c.Request.Context(), userID)
		if err != nil {
			_ = c.Error(err)
			response.InternalError(c, "Failed to verify seller profile")
			c.Abort()
			return
		}

		if !hasProfile {
			response.Forbidden(c, "Seller profile required")
			c.Abort()
			return
		}

		c.Set("has_seller_profile", true)
		c.Next()
	}
}

