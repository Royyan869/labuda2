package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
)

// sellerSubscriptionStatusReader is an optional capability of RoleChecker
// implementations that can report the canonical seller subscription state
// ('active' | 'expired' | 'none').
//
// It is a READ of existing canonical state for denial-message copy only —
// not an authorization grant. Authorization remains HasActiveSellerCapability.
type sellerSubscriptionStatusReader interface {
	GetSellerSubscriptionStatus(ctx context.Context, userID uuid.UUID) (string, error)
}

// marketAuthorityDenialMessage returns the canonical user-facing message for a
// market-authority denial, distinguished by the seller's subscription state.
//
// INVARIANT: a seller who never successfully paid (status 'none' / no row)
// must never be told to renew. Only an ENDED subscription ('expired') may
// say "renew". Unknown / unreadable status fails closed to activation copy.
func marketAuthorityDenialMessage(subscriptionStatus string) string {
	if subscriptionStatus == "expired" {
		return "Active seller subscription required. Please renew your subscription to continue selling."
	}
	return "Active seller subscription required. Please activate your subscription to start selling."
}

// readSellerSubscriptionStatus reports the canonical seller subscription state
// for denial-message copy. Returns "" when the checker cannot report status,
// so the caller fails closed to activation copy (never-paid never renews).
func readSellerSubscriptionStatus(ctx context.Context, roleChecker auth.RoleChecker, userID uuid.UUID) string {
	reader, ok := roleChecker.(sellerSubscriptionStatusReader)
	if !ok {
		return ""
	}
	status, err := reader.GetSellerSubscriptionStatus(ctx, userID)
	if err != nil {
		return ""
	}
	return status
}

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
			// Distinguish never-paid ('none') from ended ('expired') using the
			// canonical subscription state. Never-paid sellers are told to
			// ACTIVATE, never RENEW (RF-02 residual).
			msg := marketAuthorityDenialMessage(
				readSellerSubscriptionStatus(c.Request.Context(), roleChecker, userID),
			)
			response.MarketAuthorityRequired(c, msg)
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
