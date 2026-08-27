package worker

// projection_admin_handler.go — dev-only HTTP endpoints for the projection worker.
//
// These routes are ONLY registered when cfg.IsDevelopment() is true.
// They are NOT compiled out of the binary, but the routes_core.go guard ensures
// they are never reachable in production environments.
//
// Endpoints (all require RequireAdminMiddleware in routes_core.go):
//
//	GET  /api/v1/admin/projection/status   — current projection lag + row counts
//	POST /api/v1/admin/projection/rebuild  — full RebuildAll() (slow, idempotent)
//	POST /api/v1/admin/projection/process  — one incremental ManualProcess() batch

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProjectionAdminHandler exposes dev-only HTTP control for the projection worker.
type ProjectionAdminHandler struct {
	worker *ProjectionWorker
	log    *zap.Logger
}

// NewProjectionAdminHandler constructs the handler. Both arguments must be non-nil.
func NewProjectionAdminHandler(w *ProjectionWorker, log *zap.Logger) *ProjectionAdminHandler {
	if w == nil {
		panic("ProjectionAdminHandler: worker must not be nil")
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &ProjectionAdminHandler{worker: w, log: log}
}

// GetStatus handles GET /api/v1/admin/projection/status
//
// Returns:
//
//	{
//	  "running":            bool,
//	  "last_processed_at": time,
//	  "pending_count":     int,   // outbox events not yet projected
//	  "processed_count":   int,   // events in projection_tracker
//	  "order_count":       int,   // rows in order_summaries
//	  "account_count":     int    // rows in account_balances
//	}
func (h *ProjectionAdminHandler) GetStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status, err := h.worker.GetProjectionStatus(ctx)
	if err != nil {
		h.log.Error("projection_status_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"running":           h.worker.IsRunning(),
		"last_processed_at": status.LastProcessedAt,
		"pending_count":     status.PendingCount,
		"processed_count":   status.ProcessedCount,
		"order_count":       status.OrderCount,
		"account_count":     status.AccountCount,
	})
}

// Rebuild handles POST /api/v1/admin/projection/rebuild
//
// Calls RebuildAll() synchronously (may take up to 60 s on large datasets).
// Returns the post-rebuild status on success.
//
// Safe to call while the worker is running: RebuildAll is race-safe.
func (h *ProjectionAdminHandler) Rebuild(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	h.log.Info("projection_rebuild_requested_via_admin_endpoint")

	if err := h.worker.RebuildAll(ctx); err != nil {
		h.log.Error("projection_rebuild_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status, err := h.worker.GetProjectionStatus(ctx)
	if err != nil {
		// Rebuild succeeded; status query is best-effort.
		c.JSON(http.StatusOK, gin.H{
			"rebuilt":      true,
			"status_error": err.Error(),
		})
		return
	}

	h.log.Info("projection_rebuild_complete",
		zap.Int("order_count", status.OrderCount),
		zap.Int("account_count", status.AccountCount),
		zap.Int("processed_count", status.ProcessedCount),
	)

	c.JSON(http.StatusOK, gin.H{
		"rebuilt":         true,
		"order_count":     status.OrderCount,
		"account_count":   status.AccountCount,
		"processed_count": status.ProcessedCount,
		"pending_count":   status.PendingCount,
	})
}

// Process handles POST /api/v1/admin/projection/process
//
// Triggers one incremental ManualProcess() batch (up to BatchSize events).
// Useful for smoke-testing event-driven updates without waiting for the poll interval.
func (h *ProjectionAdminHandler) Process(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.worker.ManualProcess(ctx); err != nil {
		h.log.Error("projection_manual_process_failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status, err := h.worker.GetProjectionStatus(ctx)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"processed": true})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"processed":       true,
		"pending_count":   status.PendingCount,
		"processed_count": status.ProcessedCount,
		"order_count":     status.OrderCount,
	})
}


