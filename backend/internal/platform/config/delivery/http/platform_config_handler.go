package http

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	platformconfigEntity "github.com/labuda/backend/internal/platform/config/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PlatformConfigHandler handles HTTP requests for platform config operations.
//
// Platform Config manages system configuration such as platform settings and feature flags.
// This handler ONLY calls ConfigService - NO configuration logic is implemented here.
//
// SAFETY: This handler does NOT modify financial balances, ledger, or order state.
// Configuration updates only modify platform_config values.
//
// MANAGEMENT PRE-FIX M1: Authority hardening applied:
// - Handler-level capability checks (defense-in-depth)
// - Split capabilities for financial vs general config updates
// - Audit logging for all config mutations
// - No implicit admin fallback
type PlatformConfigHandler struct {
	configService    *platformconfigApp.ConfigService
	db               *db.DB
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// financialConfigKeys require config.update.financial capability.
// Only platform commission keys remain editable here.
var financialConfigKeys = map[string]bool{
	"for_sale_commission_percent": true,
	"auction_commission_percent": true,
	"min_withdrawal":             true,
	"max_withdrawal":             true,
	"withdrawal_threshold":       true,
}

// notEditableConfigKeys are declared in the schema but have no active runtime consumer.
// PUT requests for these keys are rejected with 400 until they are promoted to editable.
var notEditableConfigKeys = map[string]bool{
	"min_withdrawal":       true,
	"max_withdrawal":       true,
	"withdrawal_threshold": true,
}

// configValueValidators maps editable keys to their value validation functions.
// A validator returns a non-nil error when the value is out of range or wrong type.
var configValueValidators = map[string]func(decimal.Decimal) error{
	"for_sale_commission_percent": validatePercent,
	"auction_commission_percent": validatePercent,
}

// validatePercent accepts decimals in [0, 100].
func validatePercent(v decimal.Decimal) error {
	if v.IsNegative() || v.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("value must be between 0 and 100")
	}
	return nil
}

// validatePositiveAmount accepts any decimal > 0.
func validatePositiveAmount(v decimal.Decimal) error {
	if !v.IsPositive() {
		return fmt.Errorf("value must be greater than 0")
	}
	return nil
}

// validatePositiveInt accepts whole numbers > 0.
func validatePositiveInt(v decimal.Decimal) error {
	if !v.IsPositive() || !v.Equal(v.Floor()) {
		return fmt.Errorf("value must be a positive integer")
	}
	return nil
}

// NewPlatformConfigHandler creates a new PlatformConfigHandler.
func NewPlatformConfigHandler(
	configService *platformconfigApp.ConfigService,
	db *db.DB,
	log *zap.Logger,
) *PlatformConfigHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &PlatformConfigHandler{
		configService:    configService,
		db:               db,
		log:              log,
		adminAuditLogger: audit.NewAdminAuditLoggerDB(db.Pool()),
	}
}

// ============================================================================
// REQUEST/RESPONSE DTOs
// ============================================================================

// UpdateConfigRequest represents the request body for updating a config.
type UpdateConfigRequest struct {
	Value string `json:"value" binding:"required"`
}

// isFinancialConfigKey checks if a config key requires financial update capability.
func (h *PlatformConfigHandler) isFinancialConfigKey(key string) bool {
	return financialConfigKeys[key]
}

// getRequiredCapability returns the appropriate capability required for updating a config key.
func (h *PlatformConfigHandler) getRequiredCapability(key string) capability.Capability {
	if h.isFinancialConfigKey(key) {
		return capability.CapConfigUpdateFinancial
	}
	return capability.CapConfigUpdateGeneral
}

// validateConfigValue enforces per-key value constraints before any DB write.
//
// Returns an error when:
//   - the key is in notEditableConfigKeys (future-only / no runtime consumer)
//   - the key has a registered validator and the value fails it
//
// Keys without a registered validator are accepted without value inspection.
func (h *PlatformConfigHandler) validateConfigValue(key, rawValue string) error {
	if notEditableConfigKeys[key] {
		return fmt.Errorf("config key %q is not currently editable (no active runtime consumer)", key)
	}

	validator, ok := configValueValidators[key]
	if !ok {
		return nil // no constraint registered for this key
	}

	d, err := decimal.NewFromString(rawValue)
	if err != nil {
		return fmt.Errorf("value must be a valid number")
	}

	return validator(d)
}

// ============================================================================
// ADMIN ENDPOINTS - Get All Configs
// ============================================================================

// GetAllConfigs handles GET /api/v1/admin/config
//
// MANAGEMENT PRE-FIX M1: Requires config.view capability
//
// Returns all platform config values.
//
// Response includes:
//   - key: config key
//   - value_numeric: numeric value (if set)
//   - value_text: text value (if set)
//   - updated_by: user who last updated
//   - updated_at: last update timestamp
func (h *PlatformConfigHandler) GetAllConfigs(c *gin.Context) {
	ctx := c.Request.Context()

	// MANAGEMENT PRE-FIX M1: Handler-level capability check (defense-in-depth)
	// This provides secondary protection even if middleware is bypassed
	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapConfigView.String()) {
		response.Forbidden(c, "config.view capability required")
		return
	}

	// Get all configs from service
	var configs []*platformconfigEntity.Config
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		configs, err = h.configService.GetAllConfigs(ctx, tx)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get all configs",
			zap.String("actor_id", actor.ID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve configs")
		return
	}

	// Convert to response format
	items := make([]gin.H, len(configs))
	for i, cfg := range configs {
		items[i] = h.configToResponse(cfg)
	}

	response.Success(c, gin.H{
		"configs": items,
		"count":   len(items),
	})
}

// ============================================================================
// ADMIN ENDPOINTS - Get Single Config
// ============================================================================

// GetConfig handles GET /api/v1/admin/config/:key
//
// MANAGEMENT PRE-FIX M1: Requires config.view capability
//
// Returns a specific platform config by key.
func (h *PlatformConfigHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()

	// MANAGEMENT PRE-FIX M1: Handler-level capability check (defense-in-depth)
	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapConfigView.String()) {
		response.Forbidden(c, "config.view capability required")
		return
	}

	// Get key from URL
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "Config key is required")
		return
	}

	// Get config from service
	var config *platformconfigEntity.Config
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		config, err = h.configService.GetConfig(ctx, tx, key)
		return err
	})

	if err != nil {
		if _, ok := err.(*platformconfigEntity.ConfigNotFoundError); ok {
			response.NotFound(c, "Config key not found")
			return
		}
		h.log.Error("Failed to get config",
			zap.String("actor_id", actor.ID.String()),
			zap.String("key", key),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve config")
		return
	}

	response.Success(c, gin.H{
		"config": h.configToResponse(config),
	})
}

// ============================================================================
// ADMIN ENDPOINTS - Update Config
// ============================================================================

// UpdateConfig handles PUT /api/v1/admin/config/:key
//
// MANAGEMENT PRE-FIX M1: Requires config.update.general or config.update.financial capability + audit logging
//
// Updates a platform config value.
//
// Request body:
//   - value: string value (will be parsed as numeric if possible, otherwise stored as text)
//
// The value is parsed as decimal if it's a valid number, otherwise stored as text.
func (h *PlatformConfigHandler) UpdateConfig(c *gin.Context) {
	ctx := c.Request.Context()

	// Get key from URL first to determine required capability
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "Config key is required")
		return
	}

	// MANAGEMENT PRE-FIX M1: Handler-level capability check (defense-in-depth)
	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	// STEP 3: Map config keys to capabilities
	requiredCapability := h.getRequiredCapability(key)
	if !actor.HasCapability(requiredCapability.String()) {
		response.Forbidden(c, requiredCapability.String()+" capability required")
		return
	}

	// Parse request body
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate value against key-specific rules (range, type, editability)
	if err := h.validateConfigValue(key, req.Value); err != nil {
		response.BadRequest(c, "Invalid config value: "+err.Error())
		return
	}

	// Try to parse as decimal first, fall back to text
	var err error
	var previousValue interface{}
	var valueType string

	// Get previous value for audit before update
	var oldConfig *platformconfigEntity.Config
	_ = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		oldConfig, err = h.configService.GetConfig(ctx, tx, key)
		return err
	})
	if oldConfig != nil {
		if oldConfig.ValueNum != nil {
			previousValue = oldConfig.ValueNum.String()
			valueType = "numeric"
		} else if oldConfig.ValueText != nil {
			previousValue = *oldConfig.ValueText
			valueType = "text"
		}
	}

	if decimalValue, parseErr := decimal.NewFromString(req.Value); parseErr == nil {
		// It's a valid number, store as numeric
		err = h.db.WithTx(ctx, func(tx db.Tx) error {
			return h.configService.SetConfigNumeric(ctx, tx, key, decimalValue, actor.ID.String())
		})
		valueType = "numeric"
	} else {
		// Store as text
		err = h.db.WithTx(ctx, func(tx db.Tx) error {
			return h.configService.SetConfigText(ctx, tx, key, req.Value, actor.ID.String())
		})
		valueType = "text"
	}

	if err != nil {
		h.log.Error("Failed to update config",
			zap.String("actor_id", actor.ID.String()),
			zap.String("key", key),
			zap.String("value", req.Value),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to update config")
		return
	}

	// MANAGEMENT PRE-FIX M1: Audit logging for config mutation
	h.adminAuditLogger.LogSafe(ctx, actor.ID,
		"config_updated", "platform_config", uuid.Nil,
		map[string]interface{}{
			"key":            key,
			"previous_value": previousValue,
			"new_value":      req.Value,
			"value_type":     valueType,
		},
	)

	// Get updated config for response
	var config *platformconfigEntity.Config
	_ = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		config, err = h.configService.GetConfig(ctx, tx, key)
		return err
	})

	response.Success(c, gin.H{
		"config":  h.configToResponse(config),
		"message": "Config updated successfully",
	})
}

// ============================================================================
// PUBLIC ENDPOINTS - Feature Flag Check
// ============================================================================

// GetFeatureFlag handles GET /api/v1/config/feature/:key
//
// Returns a feature flag value for client-side feature availability checks.
// This endpoint can be accessed by any authenticated user.
//
// The feature flag is returned as a boolean:
//   - For numeric configs: returns true if value > 0
//   - For text configs: returns true if value is "true", "1", or "enabled"
func (h *PlatformConfigHandler) GetFeatureFlag(c *gin.Context) {
	ctx := c.Request.Context()

	// Get key from URL
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "Feature key is required")
		return
	}

	// Get config from service
	var config *platformconfigEntity.Config
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		config, err = h.configService.GetConfig(ctx, tx, key)
		return err
	})

	if err != nil {
		if _, ok := err.(*platformconfigEntity.ConfigNotFoundError); ok {
			// Feature flag not found - return false
			response.Success(c, gin.H{
				"key":     key,
				"enabled": false,
			})
			return
		}
		h.log.Error("Failed to get feature flag",
			zap.String("key", key),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to check feature flag")
		return
	}

	// Determine if feature is enabled
	enabled := h.isFeatureEnabled(config)

	response.Success(c, gin.H{
		"key":     key,
		"enabled": enabled,
	})
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// configToResponse converts a Config entity to a response map.
func (h *PlatformConfigHandler) configToResponse(cfg *platformconfigEntity.Config) gin.H {
	resp := gin.H{
		"key":        cfg.Key,
		"updated_at": cfg.UpdatedAt,
	}

	if cfg.ValueNum != nil {
		resp["value_numeric"] = cfg.ValueNum.String()
	}

	if cfg.ValueText != nil {
		resp["value_text"] = *cfg.ValueText
	}

	if cfg.UpdatedBy != nil {
		resp["updated_by"] = *cfg.UpdatedBy
	}

	return resp
}

// isFeatureEnabled determines if a feature flag config represents an enabled state.
func (h *PlatformConfigHandler) isFeatureEnabled(cfg *platformconfigEntity.Config) bool {
	// Check numeric value first
	if cfg.ValueNum != nil {
		return cfg.ValueNum.IsPositive() || cfg.ValueNum.IsZero()
	}

	// Check text value
	if cfg.ValueText != nil {
		value := *cfg.ValueText
		return value == "true" || value == "1" || value == "enabled"
	}

	// No value set - consider disabled
	return false
}
