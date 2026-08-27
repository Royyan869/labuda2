package monitoring

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ConnectionCounter defines the interface for getting active connection count.
type ConnectionCounter interface {
	GetConnectionCount() int
}

// SystemHealthHandler handles system health endpoint requests
type SystemHealthHandler struct {
	monitoringService *MonitoringService
	hub              ConnectionCounter // Optional: can be nil if realtime not enabled
	logger            *zap.Logger
}

// NewSystemHealthHandler creates a new system health handler
func NewSystemHealthHandler(monitoringService *MonitoringService, hub ConnectionCounter) *SystemHealthHandler {
	return &SystemHealthHandler{
		monitoringService: monitoringService,
		hub:              hub,
		logger:            zap.NewNop(),
	}
}

// GetSystemHealth returns the current system health status
// GET /health/system
// No auth required - internal use
//
// HTTP Status:
//   - 200: All healthy OR only warnings (escrow/withdrawal stuck > 0)
//   - 503: Critical issues (ledger imbalance OR auction stuck)
func (h *SystemHealthHandler) GetSystemHealth(c *gin.Context) {
	ctx := c.Request.Context()

	status, err := h.monitoringService.GetSystemHealth(ctx)
	if err != nil {
		h.logger.Error("GetSystemHealth failed", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failed to get system health",
			"detail": err.Error(),
		})
		return
	}

	// Add runtime metrics
	if h.hub != nil {
		status.RealtimeActiveConnections = h.hub.GetConnectionCount()
	}
	status.Goroutines = runtime.NumGoroutine()

	// Determine HTTP status based on health checks
	httpStatus := http.StatusOK

	// 503 if ledger imbalance OR auction stuck
	if !status.LedgerBalanced || status.AuctionStuckCount > 0 {
		httpStatus = http.StatusServiceUnavailable

		// Emit structured log for alerting pipeline
		h.logger.Error("System health check failed - 503",
			zap.Bool("ledger_balanced", status.LedgerBalanced),
			zap.Int64("ledger_imbalance_value", status.LedgerImbalanceValue),
			zap.Int("auction_stuck_count", status.AuctionStuckCount),
			zap.Int("escrow_stuck_count", status.EscrowStuckCount),
			zap.Int("withdrawal_stuck_count", status.WithdrawalStuckCount),
			zap.Int("realtime_connections", status.RealtimeActiveConnections),
			zap.Int("goroutines", status.Goroutines),
			zap.Time("timestamp", status.LastCheckedAt),
		)
	}
	// 200 (but warning state) if escrow/withdrawal stuck > 0
	// This is still 200, monitoring systems can inspect the response body

	c.JSON(httpStatus, status)
}


