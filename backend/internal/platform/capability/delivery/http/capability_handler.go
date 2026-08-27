// Package http provides HTTP handlers for capability management.
package http

import (
	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability/application"
	"github.com/labuda/backend/internal/platform/response"
)

// CapabilityHandler handles HTTP requests for capability management.
//
// This handler provides endpoints for:
// - Listing all available capabilities
// - Viewing user capabilities
// - Assigning capabilities to users
// - Revoking capabilities from users
//
// All endpoints require the governance.capability.assign capability.
type CapabilityHandler struct {
	service *application.CapabilityService
}

// NewCapabilityHandler creates a new CapabilityHandler.
func NewCapabilityHandler(service *application.CapabilityService) *CapabilityHandler {
	return &CapabilityHandler{
		service: service,
	}
}

// ============================================================
// REQUEST/RESPONSE DTOs
// ============================================================

// AssignCapabilityRequest represents the request body for assigning a capability.
type AssignCapabilityRequest struct {
	Capability string `json:"capability" binding:"required"`
}

// ============================================================
// CAPABILITY DEFINITION ENDPOINTS
// ============================================================

// ListCapabilities handles GET /api/v1/admin/capabilities
//
// Returns all available capability definitions.
// This is a static list of valid capabilities that can be assigned.
func (h *CapabilityHandler) ListCapabilities(c *gin.Context) {
	ctx := c.Request.Context()

	// Get all capability definitions
	capabilities := h.service.ListAllCapabilities(ctx)

	response.Success(c, gin.H{
		"capabilities": capabilities,
	})
}

// ============================================================
// USER CAPABILITY ENDPOINTS
// ============================================================

// GetUserCapabilities handles GET /api/v1/admin/users/:id/capabilities
//
// Returns all active capabilities for a specific user.
func (h *CapabilityHandler) GetUserCapabilities(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get user capabilities
	userCaps, err := h.service.GetUserCapabilities(ctx, targetUserID)
	if err != nil {
		response.InternalServerError(c, "Failed to fetch user capabilities")
		return
	}

	response.Success(c, gin.H{
		"user_id":      targetUserID,
		"capabilities": userCaps,
		"total":        len(userCaps),
	})
}

// AssignCapability handles POST /api/v1/admin/users/:id/capabilities
//
// Assigns a capability to a user.
//
// Request body:
// {
//   "capability": "finance.withdraw.read"
// }
func (h *CapabilityHandler) AssignCapability(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context (the user performing the action)
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse target user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Parse request body
	var req AssignCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Assign capability
	err = h.service.AssignCapability(ctx, targetUserID, req.Capability, actorID)
	if err != nil {
		switch err.(type) {
		case *application.ErrCapabilityAuthorityRequired:
			response.Forbidden(c, "Missing governance.capability.assign capability")
		case *application.ErrSelfCapabilityGrantForbidden:
			response.Forbidden(c, "Cannot grant a capability to yourself; ask another authorized operator")
		case *application.ErrInvalidCapability:
			response.BadRequest(c, "Invalid capability: "+req.Capability)
		case *application.ErrDuplicateCapability:
			response.Conflict(c, "User already has this capability")
		default:
			response.InternalServerError(c, "Failed to assign capability")
		}
		return
	}

	response.Success(c, gin.H{
		"message":    "Capability assigned successfully",
		"user_id":    targetUserID,
		"capability": req.Capability,
	})
}

// RevokeCapability handles DELETE /api/v1/admin/users/:id/capabilities/:cap
//
// Revokes a capability from a user.
func (h *CapabilityHandler) RevokeCapability(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context (the user performing the action)
	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse target user ID
	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	// Get capability string from URL parameter
	capabilityStr := c.Param("cap")
	if capabilityStr == "" {
		response.BadRequest(c, "Capability is required")
		return
	}

	// Revoke capability
	err = h.service.RevokeCapability(ctx, targetUserID, capabilityStr, actorID)
	if err != nil {
		switch err.(type) {
		case *application.ErrCapabilityAuthorityRequired:
			response.Forbidden(c, "Missing governance.capability.assign capability")
		case *application.ErrInvalidCapability:
			response.BadRequest(c, "Invalid capability: "+capabilityStr)
		case *application.ErrCapabilityNotFound:
			response.NotFound(c, "User does not have this capability")
		case *application.ErrCannotRevokeOwnCriticalCapability:
			response.BadRequest(c, "Cannot revoke your own critical capability")
		default:
			response.InternalServerError(c, "Failed to revoke capability")
		}
		return
	}

	response.Success(c, gin.H{
		"message":    "Capability revoked successfully",
		"user_id":    targetUserID,
		"capability": capabilityStr,
	})
}


