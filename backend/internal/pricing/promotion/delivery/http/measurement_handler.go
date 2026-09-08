package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	deliveryApp "github.com/labuda/backend/internal/pricing/promotion/delivery/application"
	"go.uber.org/zap"
)

// MeasurementHandler owns the client-explicit delivery measurement
// acknowledgement surface (POST /api/v1/promotions/impressions and
// POST /api/v1/promotions/clicks). It is projection-only: the recorded
// events never move money — the ledger owns financial truth.
type MeasurementHandler struct {
	service *deliveryApp.DeliveryMeasurementService
	log     *zap.Logger
}

// NewMeasurementHandler wires the canonical measurement handler.
func NewMeasurementHandler(service *deliveryApp.DeliveryMeasurementService, log *zap.Logger) *MeasurementHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &MeasurementHandler{service: service, log: log}
}

// ImpressionRequest echoes the canonical exposure identity the card carried
// together with the canonical contract id the client rendered.
type ImpressionRequest struct {
	ExposureID uuid.UUID `json:"exposure_id" binding:"required"`
	ContractID uuid.UUID `json:"contract_id" binding:"required"`
}

// AcknowledgeImpression handles POST /api/v1/promotions/impressions: a
// client-explicit acknowledgement that a canonical promotion card was
// actually exposed/rendered.
//
// Response semantics:
//   - 204 on every non-error outcome: impression durably recorded, OR a
//     repeated acknowledgement of the same exposure (idempotent), OR an
//     unknown/stale exposure (safely ignored — never converted into an
//     impression).
//   - 400 when the acknowledgement is malformed or mismatched (exposure
//     belongs to another contract / was issued to another viewer) — client
//     or attacker error, never an impression.
func (h *MeasurementHandler) AcknowledgeImpression(c *gin.Context) {
	viewerID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req ImpressionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "exposure_id and contract_id are required UUIDs")
		return
	}

	if h.service == nil {
		response.Error(c, http.StatusInternalServerError, "IMPRESSION_UNAVAILABLE", "impression authority not configured")
		return
	}

	_, err := h.service.AcknowledgeImpression(c.Request.Context(), viewerID, req.ExposureID, req.ContractID)
	if err != nil {
		if errors.Is(err, deliveryApp.ErrInvalidExposure) ||
			errors.Is(err, deliveryApp.ErrExposurePromotionMismatch) ||
			errors.Is(err, deliveryApp.ErrExposureViewerMismatch) {
			response.Error(c, http.StatusBadRequest, "INVALID_EXPOSURE", err.Error())
			return
		}
		h.log.Error("canonical impression acknowledgement failed",
			zap.String("exposure_id", req.ExposureID.String()),
			zap.Error(err),
		)
		response.Error(c, http.StatusInternalServerError, "IMPRESSION_FAILED", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ClickRequest echoes ONLY the canonical exposure identity the card carried.
// The canonical contract identity is derived server-side from the issued
// exposure — the client never supplies a contract id, so an arbitrary claim
// can never fabricate a click.
type ClickRequest struct {
	ExposureID uuid.UUID `json:"exposure_id" binding:"required"`
}

// AcknowledgeClick handles POST /api/v1/promotions/clicks: a client-explicit
// acknowledgement that a viewer tapped a canonical promotion card that
// carried an issued exposure identity.
//
// Response semantics mirror POST /impressions exactly:
//   - 204 on every non-error outcome: click durably recorded, OR a repeated
//     acknowledgement of the same exposure (idempotent), OR an unknown/stale
//     exposure (safely ignored — never converted into a click).
//   - 400 when the acknowledgement is malformed or mismatched (exposure is
//     not an issued inclusion / was issued to another viewer) — client or
//     attacker error, never a click.
func (h *MeasurementHandler) AcknowledgeClick(c *gin.Context) {
	viewerID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	var req ClickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "exposure_id is a required UUID")
		return
	}

	if h.service == nil {
		response.Error(c, http.StatusInternalServerError, "CLICK_UNAVAILABLE", "click authority not configured")
		return
	}

	_, err := h.service.AcknowledgeClick(c.Request.Context(), viewerID, req.ExposureID)
	if err != nil {
		if errors.Is(err, deliveryApp.ErrInvalidExposure) ||
			errors.Is(err, deliveryApp.ErrExposureViewerMismatch) {
			response.Error(c, http.StatusBadRequest, "INVALID_EXPOSURE", err.Error())
			return
		}
		h.log.Error("canonical click acknowledgement failed",
			zap.String("exposure_id", req.ExposureID.String()),
			zap.Error(err),
		)
		response.Error(c, http.StatusInternalServerError, "CLICK_FAILED", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}