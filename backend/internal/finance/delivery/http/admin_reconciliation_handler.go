// Package http provides HTTP handlers for admin reconciliation visibility.
package http

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/finance/entity"
	"github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

// ReconciliationTransactor is the minimal DB capability this handler needs to
// run read-only queries through ReconciliationRepository's tx-scoped methods.
type ReconciliationTransactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// AdminReconciliationHandler exposes read-only visibility into persisted
// ReconciliationWorkerV2 results.
//
// CONSTITUTIONAL ROLE (RUNTIME-INVARIANTS §7.1, ADR-002): reconciliation is
// verification-only. These endpoints are strictly GET — no mutation, no
// repair, no financial authority. They exist so admins can inspect full run
// history (including healthy/passed runs) instead of relying solely on the
// alert pipeline, which only surfaces non-passed runs and collapses repeat
// occurrences via dedup.
type AdminReconciliationHandler struct {
	db   ReconciliationTransactor
	repo repository.ReconciliationRepository
	log  *zap.Logger
}

// NewAdminReconciliationHandler creates a new AdminReconciliationHandler.
func NewAdminReconciliationHandler(
	db ReconciliationTransactor,
	repo repository.ReconciliationRepository,
	log *zap.Logger,
) *AdminReconciliationHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminReconciliationHandler{db: db, repo: repo, log: log}
}

// reconciliationResultResponse is the admin-safe wire shape for a persisted
// reconciliation result.
type reconciliationResultResponse struct {
	ID                 string                 `json:"id"`
	CheckedAt          string                 `json:"checked_at"`
	Severity           string                 `json:"severity"`
	ActionTaken        string                 `json:"action_taken"`
	AutoRepaired       bool                   `json:"auto_repaired"`
	TotalAccounts      int                    `json:"total_accounts"`
	MismatchedAccounts int                    `json:"mismatched_accounts"`
	Details            map[string]interface{} `json:"details"`
	CreatedAt          string                 `json:"created_at"`
}

// redactedDetailKeys are top-level detail keys that must never be echoed back
// verbatim if they were ever present. Reconciliation details only ever
// contain ledger/account-balance data (see ReconciliationWorkerV2), never
// gateway credentials — this list is defense-in-depth, not a documented
// existing risk.
var redactedDetailKeys = map[string]bool{
	"password":          true,
	"secret":            true,
	"token":             true,
	"credential":        true,
	"credentials":       true,
	"dsn":               true,
	"connection_string": true,
	"api_key":           true,
}

func redactDetails(details entity.ReconcileDetails) map[string]interface{} {
	out := make(map[string]interface{}, len(details))
	for k, v := range details {
		if redactedDetailKeys[strings.ToLower(k)] {
			out[k] = "[redacted]"
			continue
		}
		out[k] = v
	}
	return out
}

func toReconciliationResultResponse(r *entity.ReconciliationResult) reconciliationResultResponse {
	return reconciliationResultResponse{
		ID:                 r.ID.String(),
		CheckedAt:          r.CheckedAt.UTC().Format(time.RFC3339),
		Severity:           string(r.Severity),
		ActionTaken:        string(r.ActionTaken),
		AutoRepaired:       r.AutoRepaired,
		TotalAccounts:      r.TotalAccounts,
		MismatchedAccounts: r.MismatchedAccounts,
		Details:            redactDetails(r.Details),
		CreatedAt:          r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// parseReconciliationFilters builds ReconciliationFilters from query params.
// Returns an error message (empty if valid) for bad-request reporting.
func parseReconciliationFilters(c *gin.Context) (repository.ReconciliationFilters, string) {
	filters := repository.ReconciliationFilters{}

	if severityStr := c.Query("severity"); severityStr != "" {
		sev := entity.ReconcileSeverity(severityStr)
		filters.Severity = &sev
	}

	if actionStr := c.Query("action_taken"); actionStr != "" {
		action := entity.ReconcileAction(actionStr)
		filters.ActionTaken = &action
	}

	if autoStr := c.Query("auto_repaired"); autoStr != "" {
		autoRepaired, err := strconv.ParseBool(autoStr)
		if err != nil {
			return filters, "invalid auto_repaired parameter"
		}
		filters.AutoRepaired = &autoRepaired
	}

	if fromStr := c.Query("date_from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return filters, "invalid date_from parameter (expected RFC3339)"
		}
		filters.DateFrom = &t
	}

	if toStr := c.Query("date_to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return filters, "invalid date_to parameter (expected RFC3339)"
		}
		filters.DateTo = &t
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	filters.Limit = limit
	filters.Offset = offset

	return filters, ""
}

// ListReconciliationResults handles GET /api/v1/admin/reconciliation
//
// Returns paginated reconciliation results (every worker run, passed or not),
// using the pre-existing ReconciliationRepository.List/Count methods.
//
// Query parameters:
//   - severity       passed|low|medium|high|critical
//   - action_taken   none|logged|alerted|escalated
//   - auto_repaired  true|false
//   - date_from      RFC3339 (filters on checked_at)
//   - date_to        RFC3339 (filters on checked_at)
//   - limit          max rows (default 50, max 200)
//   - offset         pagination offset (default 0)
//
// Authorization: Admin only (finance.withdraw.read — shared with the
// existing read-only ledger/verifier finance-visibility endpoints).
func (h *AdminReconciliationHandler) ListReconciliationResults(c *gin.Context) {
	ctx := c.Request.Context()

	if _, ok := middleware.MustGetUserIDFromContext(c); !ok {
		return
	}

	filters, errMsg := parseReconciliationFilters(c)
	if errMsg != "" {
		response.BadRequest(c, errMsg)
		return
	}

	var results []*entity.ReconciliationResult
	var total int64

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		results, err = h.repo.List(ctx, tx, filters)
		if err != nil {
			return err
		}
		total, err = h.repo.Count(ctx, tx, filters)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list reconciliation results", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch reconciliation results")
		return
	}

	rows := make([]reconciliationResultResponse, 0, len(results))
	for _, r := range results {
		rows = append(rows, toReconciliationResultResponse(r))
	}

	c.JSON(200, gin.H{
		"results": rows,
		"total":   total,
		"limit":   filters.Limit,
		"offset":  filters.Offset,
	})
}

// GetReconciliationResult handles GET /api/v1/admin/reconciliation/:id
//
// Returns full detail for a single reconciliation result, including its
// (redacted) details payload.
//
// Authorization: Admin only (finance.withdraw.read).
func (h *AdminReconciliationHandler) GetReconciliationResult(c *gin.Context) {
	ctx := c.Request.Context()

	if _, ok := middleware.MustGetUserIDFromContext(c); !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid reconciliation result ID")
		return
	}

	var result *entity.ReconciliationResult
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.repo.GetByID(ctx, tx, id)
		return err
	})
	if err != nil {
		response.NotFound(c, "Reconciliation result not found")
		return
	}

	c.JSON(200, toReconciliationResultResponse(result))
}

// GetLatestReconciliationResult handles GET /api/v1/admin/reconciliation/latest
//
// Returns the single most recent reconciliation run, whatever its severity —
// including a PASSED result. This is the positive "the worker is alive and
// healthy" signal that the alert pipeline alone cannot provide, since no
// alert is ever created for a passed run.
//
// Authorization: Admin only (finance.withdraw.read).
func (h *AdminReconciliationHandler) GetLatestReconciliationResult(c *gin.Context) {
	ctx := c.Request.Context()

	if _, ok := middleware.MustGetUserIDFromContext(c); !ok {
		return
	}

	var result *entity.ReconciliationResult
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.repo.GetLatest(ctx, tx)
		return err
	})
	if err != nil {
		h.log.Error("Failed to get latest reconciliation result", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch latest reconciliation result")
		return
	}
	if result == nil {
		response.NotFound(c, "No reconciliation results yet")
		return
	}

	c.JSON(200, toReconciliationResultResponse(result))
}
