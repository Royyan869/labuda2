package realtime

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
)

// maxRealtimeConnections is the soft protective cap for WebSocket connections.
const maxRealtimeConnections = 8000

// Handler handles WebSocket connections.
//
// The handler is responsible for:
// - Upgrading HTTP to WebSocket
// - Authenticating the connection (auth middleware runs before)
// - Creating and registering connections
// - Starting read/write pumps
//
// CHAT-3: All subscribe requests are gated through SubscribeGate.
// No governance-blind subscribe path remains.
type Handler struct {
	hub         *Hub
	gate        *SubscribeGate
	rateLimiter *rate.RateLimiter
	log         *zap.Logger
	presence    PresenceLeaser
}

// NewHandler creates a new WebSocket handler.
// presence may be nil only for test harness; production wiring in
// serverboot must supply the canonical *presence.Service. The handler
// stores the lease authority and passes it to each new Connection so
// that every authenticated WS session holds exactly one Redis lease.
func NewHandler(hub *Hub, gate *SubscribeGate, rateLimiter *rate.RateLimiter, log *zap.Logger, presence PresenceLeaser) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		hub:         hub,
		gate:        gate,
		rateLimiter: rateLimiter,
		log:         log,
		presence:    presence,
	}
}

// Presence returns the wired presence lease authority (nil in test harness).
func (h *Handler) Presence() PresenceLeaser { return h.presence }

// HandleWebSocket upgrades an HTTP request to a WebSocket connection.
//
// Flow:
//  1. Extract userID from context (set by auth middleware)
//  2. Upgrade to WebSocket
//  3. Create Connection (with governance gate wired in)
//  4. Register with Hub
//  5. Start read/write pumps
//
// Room subscribe governance happens inside Connection.ReadPump via SubscribeGate.
func (h *Handler) HandleWebSocket(c *gin.Context) {
	currentConnections := h.hub.GetConnectionCount()
	if currentConnections >= maxRealtimeConnections {
		h.log.Warn("Realtime connection cap reached",
			zap.Int("active_connections", currentConnections),
			zap.Int("max_connections", maxRealtimeConnections),
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server busy. Try again later.",
		})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		h.log.Error("UserID not found in context - middleware misconfiguration")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Authentication context not found",
		})
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		h.log.Error("Invalid userID in context", zap.Any("userID", userIDVal))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid user context",
		})
		return
	}

	websocket.Handler(func(ws *websocket.Conn) {
		h.log.Info("WebSocket connection established",
			zap.String("user_id", userID.String()),
			zap.String("remote_addr", ws.Request().RemoteAddr),
		)

		conn := NewConnection(userID, ws, h.hub, h.gate, h.rateLimiter, h.log, h.presence)

		h.hub.Register(conn)

		// Presence acquire: every authenticated active WS connection holds
		// exactly one lease. Uses Background with timeout because no request
		// context should gate lease creation; the connection lives beyond the
		// HTTP upgrade request lifecycle.
		// CLOSURE GATE 1: acquire failure must NOT leave an orphan WS registered
		// without a lease. On ResumeLease error we immediately cleanup
		// (unregister + close) and abort the session; no fallback without lease.
		if h.presence != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, err := h.presence.ResumeLease(ctx, conn.UserID, conn.ID)
			cancel()
			if err != nil {
				h.log.Warn("Presence ResumeLease failed on connect — closing WS",
					zap.String("user_id", conn.UserID.String()),
					zap.String("connection_id", conn.ID),
					zap.Error(err),
				)
				// Undo registration and close the session. conn.Close() is
				// idempotent and will also attempt LeaveLease (harmless when
				// acquire never succeeded). This guarantees invariant:
				// authenticated WS => lease acquired, otherwise no WS.
				conn.Close()
				return
			}
		} else {
			// Production wiring must never reach this branch; nil only for tests.
			// Gate B: serverboot fatals in production when presence is nil,
			// so this path is isolated test harness.
			h.log.Debug("Presence lease acquire skipped: nil authority (test harness)")
		}

		go conn.WritePump()

		conn.ReadPump()

		h.log.Info("WebSocket connection closed",
			zap.String("user_id", userID.String()),
			zap.String("connection_id", conn.ID),
		)
	}).ServeHTTP(c.Writer, c.Request)
}

// GetStats returns hub statistics for monitoring.
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"active_connections": h.hub.GetConnectionCount(),
		"active_rooms":       h.hub.GetRoomCount(),
	})
}

// =============================================================================
// ERROR RESPONSES
// =============================================================================

// CloseWithCode sends a close message with a status code and reason.
func CloseWithCode(ws *websocket.Conn, code int, reason string) error {
	data := fmt.Sprintf(`{"code": %d, "reason": "%s"}`, code, reason)
	_, err := ws.Write([]byte(data))
	return err
}


