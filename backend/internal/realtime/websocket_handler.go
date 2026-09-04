package realtime

import (
	"fmt"
	"net/http"

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
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, gate *SubscribeGate, rateLimiter *rate.RateLimiter, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		hub:         hub,
		gate:        gate,
		rateLimiter: rateLimiter,
		log:         log,
	}
}

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

		conn := NewConnection(userID, ws, h.hub, h.gate, h.rateLimiter, h.log)

		h.hub.Register(conn)

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


