package http

import (
	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/audit"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AdminSubscriptionConfigHandler handles admin reads and updates of the
// singleton seller_subscription_configs row.
//
// GET  /admin/seller-subscription-config  — requires config.view
// PUT  /admin/seller-subscription-config  — requires config.update.financial
//
// yearly_fee_rupiah is stored as a Rupiah integer (e.g., 70000 = Rp 70,000).
// Validation mirrors the DB CHECK constraints:
//   - yearly_fee_rupiah > 0
//   - duration_days > 0
//   - renewal_reminder_days >= 0
//   - renewal_reminder_days < duration_days
type AdminSubscriptionConfigHandler struct {
	repo             subscriptionRepo.SellerSubscriptionRepository
	db               *db.DB
	log              *zap.Logger
	adminAuditLogger audit.AdminAuditLogger
}

// NewAdminSubscriptionConfigHandler constructs the handler.
func NewAdminSubscriptionConfigHandler(
	repo subscriptionRepo.SellerSubscriptionRepository,
	db *db.DB,
	log *zap.Logger,
) *AdminSubscriptionConfigHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminSubscriptionConfigHandler{
		repo:             repo,
		db:               db,
		log:              log,
		adminAuditLogger: audit.NewAdminAuditLoggerDB(db.Pool()),
	}
}

// updateSubscriptionConfigRequest is the PUT request body.
type updateSubscriptionConfigRequest struct {
	YearlyFeeRupiah     int64 `json:"yearly_fee_rupiah"        binding:"required"`
	DurationDays        int   `json:"duration_days"           binding:"required"`
	RenewalReminderDays int   `json:"renewal_reminder_days"`
	Enabled             bool  `json:"enabled"`
}

// subscriptionConfigResponse is the wire shape returned by both endpoints.
type subscriptionConfigResponse struct {
	ID                  string `json:"id"`
	YearlyFeeRupiah     int64  `json:"yearly_fee_rupiah"`
	DurationDays        int    `json:"duration_days"`
	RenewalReminderDays int    `json:"renewal_reminder_days"`
	Enabled             bool   `json:"enabled"`
	CreatedAt           string `json:"created_at"`
}

func configToResponse(e *subscriptionEntity.SellerSubscriptionConfig) *subscriptionConfigResponse {
	return &subscriptionConfigResponse{
		ID:                  e.ID.String(),
		YearlyFeeRupiah:     e.YearlyFeeRupiah,
		DurationDays:        e.DurationDays,
		RenewalReminderDays: e.RenewalReminderDays,
		Enabled:             e.Enabled,
		CreatedAt:           e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// GetConfig handles GET /api/v1/admin/seller-subscription-config
//
// Returns the currently enabled seller subscription configuration.
// Returns 404 if no enabled config exists (canonical seed should prevent this).
func (h *AdminSubscriptionConfigHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapConfigView.String()) {
		response.Forbidden(c, "config.view capability required")
		return
	}

	var cfg *subscriptionConfigResponse
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		entity, err := h.repo.GetActiveConfig(ctx, tx)
		if err != nil {
			return err
		}
		if entity != nil {
			cfg = configToResponse(entity)
		}
		return nil
	})
	if err != nil {
		h.log.Error("failed to get subscription config", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve subscription config")
		return
	}
	if cfg == nil {
		response.NotFound(c, "No active subscription config found")
		return
	}

	response.Success(c, gin.H{"config": cfg})
}

// UpdateConfig handles PUT /api/v1/admin/seller-subscription-config
//
// Updates the currently enabled seller subscription config in place.
// When enabled = true, the repository atomically disables all other rows.
// Returns 404 if no enabled config exists to update.
func (h *AdminSubscriptionConfigHandler) UpdateConfig(c *gin.Context) {
	ctx := c.Request.Context()

	actor := capability.GetActor(ctx)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability(capability.CapConfigUpdateFinancial.String()) {
		response.Forbidden(c, "config.update.financial capability required")
		return
	}

	var req updateSubscriptionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.YearlyFeeRupiah <= 0 {
		response.BadRequest(c, "yearly_fee_rupiah must be > 0")
		return
	}
	if req.DurationDays <= 0 {
		response.BadRequest(c, "duration_days must be > 0")
		return
	}
	if req.RenewalReminderDays < 0 {
		response.BadRequest(c, "renewal_reminder_days must be >= 0")
		return
	}
	if req.RenewalReminderDays >= req.DurationDays {
		response.BadRequest(c, "renewal_reminder_days must be < duration_days")
		return
	}

	notFound := false
	var updated *subscriptionConfigResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		current, err := h.repo.GetActiveConfig(ctx, tx)
		if err != nil {
			return err
		}
		if current == nil {
			notFound = true
			return nil
		}

		if err := h.repo.UpdateConfigTx(ctx, tx,
			current.ID,
			req.YearlyFeeRupiah,
			req.DurationDays,
			req.RenewalReminderDays,
			req.Enabled,
		); err != nil {
			return err
		}

		// Re-fetch only when enabled = true so we return the saved state.
		// If disabled, there is no active row to re-fetch.
		if req.Enabled {
			saved, err := h.repo.GetActiveConfig(ctx, tx)
			if err != nil {
				return err
			}
			if saved != nil {
				updated = configToResponse(saved)
			}
		}
		return nil
	})

	if err != nil {
		h.log.Error("failed to update subscription config",
			zap.Int64("yearly_fee_rupiah", req.YearlyFeeRupiah),
			zap.Int("duration_days", req.DurationDays),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to update subscription config")
		return
	}
	if notFound {
		response.NotFound(c, "No active subscription config found")
		return
	}

	h.adminAuditLogger.LogSafe(ctx, actor.ID,
		"subscription_config_updated", "subscription_config", actor.ID,
		map[string]interface{}{
			"yearly_fee_rupiah":     req.YearlyFeeRupiah,
			"duration_days":         req.DurationDays,
			"renewal_reminder_days": req.RenewalReminderDays,
			"enabled":               req.Enabled,
		},
	)

	if updated != nil {
		response.Success(c, gin.H{"config": updated, "message": "Subscription config updated"})
	} else {
		response.Success(c, gin.H{"message": "Subscription config updated (disabled)"})
	}
}
