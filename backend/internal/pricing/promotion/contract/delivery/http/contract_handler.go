// Package http exposes the canonical Promotion Contract lifecycle over HTTP.
//
// This is the PHASE 4A production surface: it makes the already-implemented
// canonical PromotionContractService reachable from the runtime for
//
//	Create / List / Get / Pause / Resume / Finalize
//
// It does NOT expose delivery (ticket issuance / qualification) — there is no
// delivery trigger, no pacing, no target authority and no funding entry here.
// Delivery remains dormant behind the contract lifecycle.
package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeApp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	contractApp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contractEntity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	deliveryEntity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	"go.uber.org/zap"
)

// ContractAnalyticsReader is the delivery measurement projection reader the
// contract handler uses for owner-scoped analytics reads. The projection is
// NOT a financial authority — the ledger owns money truth.
type ContractAnalyticsReader interface {
	GetContractAnalytics(ctx context.Context, contractID uuid.UUID) (*deliveryEntity.DeliveryAnalytics, error)
}

// ContractHandler owns the canonical Promotion Contract HTTP surface.
type ContractHandler struct {
	service         *contractApp.PromotionContractService
	analyticsReader ContractAnalyticsReader
	log             *zap.Logger
}

// NewContractHandler wires the canonical contract handler. analyticsReader is
// optional (nil disables the analytics endpoint with a truthful 503).
func NewContractHandler(service *contractApp.PromotionContractService, analyticsReader ContractAnalyticsReader, log *zap.Logger) *ContractHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &ContractHandler{service: service, analyticsReader: analyticsReader, log: log}
}

// ============================================================================
// REQUEST DTOs
// ============================================================================

// CreateContractRequest captures the canonical seller inputs. planned_start /
// planned_finish are deliberately NOT accepted — both derive server-side from
// duration_days (canonical Phase 2 authority).
type CreateContractRequest struct {
	Kind         string   `json:"kind" binding:"required,oneof=internal external"`
	BudgetRupiah int64    `json:"budget_rupiah" binding:"required,min=1"`
	DurationDays int64    `json:"duration_days" binding:"required,min=1"`
	CityIDs      []string `json:"city_ids"`
}

// ============================================================================
// RESPONSE DTO
// ============================================================================

// ContractResponse is the owner-facing canonical contract projection.
type ContractResponse struct {
	ID                  string   `json:"id"`
	SellerID            string   `json:"seller_id"`
	Kind                string   `json:"kind"`
	Status              string   `json:"status"`
	BudgetRupiah        int64    `json:"budget_rupiah"`
	CPMRupiah           int64    `json:"cpm_rupiah"`
	PlannedStart        string   `json:"planned_start"`
	PlannedFinish       string   `json:"planned_finish"`
	AllocationAccountID string   `json:"allocation_account_id"`
	PausedAt            *string  `json:"paused_at"`
	FinalizedAt         *string  `json:"finalized_at"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	CityIDs             []string `json:"city_ids"`
	// EstimatedImpressions is an informational-only projection of how many
	// Qualified Impressions the seller's budget can approximately buy at the
	// contract's immutable CPM snapshot. It is NOT a guarantee, NOT a
	// financial authority, and NOT a billing unit — purely informational
	// (Owner truth: estimated impressions are informational only).
	EstimatedImpressions int64    `json:"estimated_impressions"`
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func optionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toContractResponse(c *contractEntity.Contract) ContractResponse {
	return toContractResponseWithGeography(c, nil)
}

func toContractResponseWithGeography(c *contractEntity.Contract, cityIDs []string) ContractResponse {
	if c == nil {
		return ContractResponse{}
	}
	if cityIDs == nil {
		cityIDs = []string{}
	}
	// Informational-only estimated impression count (canonical estimator —
	// no second calculator). Zero on error (config-dependent CPM).
	est, _ := finance.PromotionEstimatedImpressions(c.BudgetRupiah, c.CPMRupiah)
	return ContractResponse{
		ID:                  c.ID.String(),
		SellerID:            c.SellerID.String(),
		Kind:                string(c.Kind),
		Status:              string(c.Status),
		BudgetRupiah:        c.BudgetRupiah,
		CPMRupiah:           c.CPMRupiah,
		PlannedStart:        formatTime(c.PlannedStart),
		PlannedFinish:       formatTime(c.PlannedFinish),
		AllocationAccountID: c.AllocationAccountID.String(),
		PausedAt:            optionalTime(c.PausedAt),
		FinalizedAt:         optionalTime(c.FinalizedAt),
		CreatedAt:           formatTime(c.CreatedAt),
		UpdatedAt:           formatTime(c.UpdatedAt),
		CityIDs:             cityIDs,
		EstimatedImpressions: est,
	}
}

// ============================================================================
// CREATE — authenticated, eligible seller
// ============================================================================

// CreateContract handles POST /api/v1/promotions/contracts.
// Ownership: the contract is bound to the authenticated caller; no seller id
// is accepted from the request body.
func (h *ContractHandler) CreateContract(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req CreateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	created, err := h.service.Create(c.Request.Context(), contractApp.CreatePromotionInput{
		SellerID:     callerID,
		Kind:         contractEntity.Kind(req.Kind),
		BudgetRupiah: req.BudgetRupiah,
		DurationDays: req.DurationDays,
		CityIDs:      req.CityIDs,
	})
	if err != nil {
		h.writeError(c, "create promotion contract", err)
		return
	}
	cityIDs, _ := h.service.GetGeographyCityIDs(c.Request.Context(), created.ID)

	response.Created(c, gin.H{
		"message":  "Promotion contract created and funded",
		"contract": toContractResponseWithGeography(created, cityIDs),
	})
}

// ============================================================================
// PREVIEW FUNDING — read-only shortage projection
// ============================================================================

// PreviewFundingRequest mirrors CreateContractRequest for the preview endpoint.
// The seller supplies the same inputs they would for creation; the system
// returns the funding sufficiency projection without creating anything.
type PreviewFundingRequest struct {
	Kind         string   `json:"kind" binding:"required,oneof=internal external"`
	BudgetRupiah int64    `json:"budget_rupiah" binding:"required,min=1"`
	DurationDays int64    `json:"duration_days" binding:"required,min=1"`
	CityIDs      []string `json:"city_ids"`
}

// PreviewFunding handles POST /api/v1/promotions/contracts/preview-funding.
//
// It returns a read-only FundingPreview with the exact shortage the seller
// would face if they attempted to create this promotion. No contract, no
// allocation, and no financial mutation occurs.
//
// AUTHORITY: single canonical funding projection endpoint. There is no
// second calculation path.
func (h *ContractHandler) PreviewFunding(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	var req PreviewFundingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	preview, err := h.service.PreviewFunding(c.Request.Context(), contractApp.CreatePromotionInput{
		SellerID:     callerID,
		Kind:         contractEntity.Kind(req.Kind),
		BudgetRupiah: req.BudgetRupiah,
		DurationDays: req.DurationDays,
		CityIDs:      req.CityIDs,
	})
	if err != nil {
		h.writeError(c, "preview promotion funding", err)
		return
	}

	response.Success(c, gin.H{
		"preview": preview,
	})
}

// ============================================================================
// LIST / GET — owner-scoped reads
// ============================================================================

// ListContracts handles GET /api/v1/promotions/contracts.
// Seller scoping is enforced by the service query on the authenticated caller.
func (h *ContractHandler) ListContracts(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}

	contracts, err := h.service.List(c.Request.Context(), callerID)
	if err != nil {
		h.writeError(c, "list promotion contracts", err)
		return
	}

	out := make([]ContractResponse, 0, len(contracts))
	for _, contract := range contracts {
		cityIDs, _ := h.service.GetGeographyCityIDs(c.Request.Context(), contract.ID)
		out = append(out, toContractResponseWithGeography(contract, cityIDs))
	}
	response.Success(c, gin.H{
		"contracts": out,
		"count":     len(out),
	})
}

// GetContract handles GET /api/v1/promotions/contracts/:id.
// Ownership is enforced by the canonical service (SellerID must match caller).
func (h *ContractHandler) GetContract(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}

	contract, err := h.service.Get(c.Request.Context(), callerID, contractID)
	if err != nil {
		h.writeError(c, "get promotion contract", err)
		return
	}
	cityIDs, _ := h.service.GetGeographyCityIDs(c.Request.Context(), contract.ID)
	response.Success(c, gin.H{"contract": toContractResponseWithGeography(contract, cityIDs)})
}

// ============================================================================
// ANALYTICS — owner-scoped delivery measurement projection (contract authority)
// ============================================================================

// DeliveryAnalyticsResponse is the seller-visible delivery measurement
// summary keyed by contract_id. included_count, impression_count and
// click_count are SEPARATE truthful counts over persisted canonical events
// (canonical_promotion_delivery_events). No derived metrics (CTR), no
// fabricated dimensions.
type DeliveryAnalyticsResponse struct {
	ContractID      string `json:"contract_id"`
	IncludedCount   int    `json:"included_count"`
	ImpressionCount int    `json:"impression_count"`
	ClickCount      int    `json:"click_count"`
}

// GetContractAnalytics handles GET /api/v1/promotions/contracts/:id/analytics.
// Ownership is enforced by the canonical contract service (SellerID must
// match the caller) before the projection is read — a caller can never read
// another seller's delivery measurement.
func (h *ContractHandler) GetContractAnalytics(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}

	// Ownership gate first: another seller's contract id is indistinguishable
	// from a missing one (CONTRACT_NOT_FOUND).
	if _, err := h.service.Get(c.Request.Context(), callerID, contractID); err != nil {
		h.writeError(c, "get promotion contract analytics", err)
		return
	}
	if h.analyticsReader == nil {
		response.Error(c, http.StatusServiceUnavailable, "ANALYTICS_UNAVAILABLE", "delivery measurement authority not configured")
		return
	}
	a, err := h.analyticsReader.GetContractAnalytics(c.Request.Context(), contractID)
	if err != nil {
		h.log.Error("get promotion contract analytics failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, "ANALYTICS_FAILED", "Failed to read promotion contract analytics")
		return
	}
	response.Success(c, DeliveryAnalyticsResponse{
		ContractID:      contractID.String(),
		IncludedCount:   a.IncludedCount,
		ImpressionCount: a.ImpressionCount,
		ClickCount:      a.ClickCount,
	})
}

// ============================================================================
// PAUSE / RESUME / FINALIZE — owner-scoped lifecycle actions
// ============================================================================

// PauseContract handles POST /api/v1/promotions/contracts/:id/pause.
func (h *ContractHandler) PauseContract(c *gin.Context) {
	h.ownerAction(c, func(callerID, contractID uuid.UUID) error {
		return h.service.Pause(c.Request.Context(), contractApp.PausePromotionInput{SellerID: callerID, ContractID: contractID})
	})
}

// ResumeContract handles POST /api/v1/promotions/contracts/:id/resume.
func (h *ContractHandler) ResumeContract(c *gin.Context) {
	h.ownerAction(c, func(callerID, contractID uuid.UUID) error {
		return h.service.Resume(c.Request.Context(), contractApp.ResumePromotionInput{SellerID: callerID, ContractID: contractID})
	})
}

// FinalizeContract handles POST /api/v1/promotions/contracts/:id/finalize.
func (h *ContractHandler) FinalizeContract(c *gin.Context) {
	h.ownerAction(c, func(callerID, contractID uuid.UUID) error {
		return h.service.Finalize(c.Request.Context(), contractApp.FinalizePromotionInput{SellerID: callerID, ContractID: contractID})
	})
}

// ownerAction runs a seller-scoped lifecycle action: it resolves the
// authenticated caller and the :id path param, then invokes the action. The
// canonical service enforces ownership (SellerID must match the contract row),
// so a caller can never pause/resume/finalize another seller's contract.
func (h *ContractHandler) ownerAction(c *gin.Context, action func(callerID, contractID uuid.UUID) error) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}
	if err := action(callerID, contractID); err != nil {
		h.writeError(c, "promotion contract action", err)
		return
	}
	response.Success(c, gin.H{
		"message":     "Promotion contract updated",
		"contract_id": contractID.String(),
	})
}

// ============================================================================
// ERROR MAPPING
// ============================================================================

type AddTargetRequest struct {
	TargetType string `json:"target_type" binding:"required"`
	TargetID   string `json:"target_id" binding:"required"`
}

func (h *ContractHandler) AddTarget(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}
	var req AddTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tid, err := uuid.Parse(req.TargetID)
	if err != nil {
		response.BadRequest(c, "Invalid target_id")
		return
	}
	ct, err := h.service.AddTarget(c.Request.Context(), contractApp.AddTargetInput{
		SellerID:   callerID,
		ContractID: contractID,
		TargetType: req.TargetType,
		TargetID:   tid,
	})
	if err != nil {
		h.writeError(c, "add promotion target", err)
		return
	}
	response.Created(c, gin.H{"target": ct})
}

func (h *ContractHandler) RemoveTarget(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}
	targetID, err := uuid.Parse(c.Param("target_id"))
	if err != nil {
		response.BadRequest(c, "Invalid target id")
		return
	}
	if err := h.service.RemoveTarget(c.Request.Context(), contractApp.RemoveTargetInput{SellerID: callerID, ContractID: contractID, TargetID: targetID}); err != nil {
		h.writeError(c, "remove promotion target", err)
		return
	}
	response.Success(c, gin.H{"message": "target removed"})
}

func (h *ContractHandler) ListTargets(c *gin.Context) {
	callerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	contractID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid contract id")
		return
	}
	list, err := h.service.ListTargets(c.Request.Context(), callerID, contractID)
	if err != nil {
		h.writeError(c, "list promotion targets", err)
		return
	}
	response.Success(c, gin.H{"targets": list, "count": len(list)})
}

func (h *ContractHandler) writeError(c *gin.Context, op string, err error) {
	// Struct-typed domain errors need errors.As with pointer targets, so they
	// are handled before the switch over the sentinel errors.
	var slotErr *contractApp.ErrPromotionSellerSlotOccupied
	if errors.As(err, &slotErr) {
		response.Error(c, http.StatusConflict, "PROMOTION_SLOT_OCCUPIED",
			"You already have an active promotion contract of this kind")
		return
	}
	var budgetErr *contractApp.ErrPromotionBudgetBelowMinimum
	if errors.As(err, &budgetErr) {
		response.Error(c, http.StatusBadRequest, "PROMOTION_BUDGET_BELOW_MINIMUM",
			"Promotion budget is below the minimum required for the chosen duration")
		return
	}

	switch {
	case errors.Is(err, auth.ErrMarketAuthorityRequired):
		response.MarketAuthorityRequired(c, "Active seller subscription required to create a promotion contract")
	case errors.Is(err, financeApp.ErrPromoteBalanceInsufficient):
		response.Error(c, http.StatusBadRequest, "PROMOTE_BALANCE_INSUFFICIENT",
			"Insufficient promote balance to fund this promotion contract")
	case errors.Is(err, contractRepo.ErrContractNotFound),
		errors.Is(err, contractApp.ErrPromotionContractNotFound):
		response.Error(c, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Promotion contract not found")
	case errors.Is(err, contractApp.ErrPromotionContractNotOwned):
		response.Error(c, http.StatusForbidden, "CONTRACT_NOT_OWNED", "You do not own this promotion contract")
	case errors.Is(err, contractApp.ErrPromotionAlreadyFinalized):
		response.Error(c, http.StatusConflict, "PROMOTION_ALREADY_FINALIZED", "Promotion contract is already finalized")
	case errors.Is(err, contractApp.ErrPromotionAlreadyPaused):
		response.Error(c, http.StatusConflict, "PROMOTION_ALREADY_PAUSED", "Promotion contract is already paused")
	case errors.Is(err, contractApp.ErrPromotionPauseNotAllowed):
		response.Error(c, http.StatusConflict, "PAUSE_NOT_ALLOWED", "Promotion contract cannot be paused in its current status")
	case errors.Is(err, contractApp.ErrPromotionResumeNotAllowed):
		response.Error(c, http.StatusConflict, "RESUME_NOT_ALLOWED", "Promotion contract cannot be resumed in its current status")
	case errors.Is(err, contractApp.ErrPromotionResumeClockInvalid):
		response.Error(c, http.StatusConflict, "RESUME_CLOCK_INVALID", "Promotion resume clock is invalid")
	case errors.Is(err, contractApp.ErrPromotionKindInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_KIND", "Promotion kind must be internal or external")
	case errors.Is(err, contractApp.ErrPromotionBudgetInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_BUDGET", "Promotion budget must be a positive Rupiah integer")
	case errors.Is(err, contractApp.ErrQueueFull):
		response.Error(c, http.StatusConflict, "QUEUE_FULL", "Promotion target queue is full (max 10)")
	case errors.Is(err, contractApp.ErrQueueDuplicate):
		response.Error(c, http.StatusConflict, "QUEUE_DUPLICATE", "Target already in queue")
	case errors.Is(err, contractApp.ErrQueueExternalMismatch):
		response.Error(c, http.StatusBadRequest, "QUEUE_KIND_MISMATCH", "Target type does not match contract kind")
	case errors.Is(err, contractApp.ErrPromotionDurationInvalid):
		response.Error(c, http.StatusBadRequest, "INVALID_PROMOTION_DURATION", "Promotion duration is invalid or out of representable range")
	default:
		h.log.Error(op+" failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to "+op)
	}
}
