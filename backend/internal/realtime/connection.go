package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/rate"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
)

// Connection represents a WebSocket client connection.
//
// DESIGN:
// - Buffered channel for outbound messages (non-blocking broadcast)
// - Room tracking for subscriptions
// - Graceful close handling with sync.Once for idempotent cleanup
type Connection struct {
	// ID is a unique identifier for this connection
	ID string

	// UserID is the authenticated user's ID
	UserID uuid.UUID

	// WS is the WebSocket connection
	WS *websocket.Conn

	// Send is a buffered channel for outbound messages
	// Hub broadcasts messages to this channel
	// Write pump reads from this channel and writes to WS
	Send chan []byte

	// Rooms is the set of rooms this connection is subscribed to
	Rooms map[uuid.UUID]struct{}

	// Hub reference for unregistering on close
	hub *Hub

	// gate enforces subscribe-time governance (membership + lifecycle check)
	gate *SubscribeGate

	// Logger
	log *zap.Logger

	// Rate limiter for subscribe actions
	rateLimiter *rate.RateLimiter

	// closeOnce ensures Close() operations are performed exactly once
	closeOnce sync.Once
}

// NewConnection creates a new Connection instance.
func NewConnection(
	userID uuid.UUID,
	ws *websocket.Conn,
	hub *Hub,
	gate *SubscribeGate,
	rateLimiter *rate.RateLimiter,
	log *zap.Logger,
) *Connection {
	if log == nil {
		log = zap.NewNop()
	}

	return &Connection{
		ID:          uuid.New().String(),
		UserID:      userID,
		WS:          ws,
		Send:        make(chan []byte, 256),
		Rooms:       make(map[uuid.UUID]struct{}),
		hub:         hub,
		gate:        gate,
		rateLimiter: rateLimiter,
		log:         log,
	}
}

// ClientMessage represents a message sent from the client.
// Clients use this to subscribe/unsubscribe from rooms.
type ClientMessage struct {
	Action   string    `json:"action"` // "subscribe" or "unsubscribe"
	RoomID   uuid.UUID `json:"room_id"`
	RoomType string    `json:"room_type"` // "chat", "order", "auction"; defaults to "chat" when absent
}

type clientEnvelope struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	RoomID   string                 `json:"room_id"` // legacy top-level fallback
	RoomType string                 `json:"room_type"`
	Data     map[string]interface{} `json:"data"`
}

// sendWSError writes a typed error frame to the WebSocket connection.
// Non-fatal: write errors are logged at debug level (connection may already be closing).
func sendWSError(ws *websocket.Conn, messageID, code, action string, log *zap.Logger) {
	data := marshalWSError(messageID, code, action)
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := ws.Write(data); err != nil {
		log.Debug("Failed to send WS error frame",
			zap.String("code", code),
			zap.Error(err),
		)
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub.
//
// The application runs ReadPump in a per-connection goroutine.
// It reads client messages and handles subscribe/unsubscribe actions.
//
// CHAT-3 INVARIANT: hub.Subscribe is called ONLY after the subscribe gate
// approves the request. No direct hub.Subscribe without governance.
func (c *Connection) ReadPump() {
	defer c.Close()

	c.WS.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		var raw json.RawMessage
		err := websocket.JSON.Receive(c.WS, &raw)
		if err != nil {
			c.log.Debug("Read error, closing connection",
				zap.String("connection_id", c.ID),
				zap.Error(err),
			)
			break
		}
		msg, clientMsgID, ok := parseClientControlMessage(raw)
		if !ok {
			c.log.Debug("Invalid control message shape",
				zap.String("connection_id", c.ID),
			)
			sendWSError(c.WS, "", "invalid_message_shape", "", c.log)
			continue
		}

		c.WS.SetReadDeadline(time.Now().Add(60 * time.Second))

		switch msg.Action {
		case "ping":
			c.WS.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := c.WS.Write(marshalWSPong(clientMsgID)); err != nil {
				c.log.Debug("Failed to send pong frame",
					zap.String("connection_id", c.ID),
					zap.Error(err),
				)
			}
		case "subscribe":
			subscribeKey := fmt.Sprintf("ws:sub:%s", c.ID)
			if !c.rateLimiter.Allow(subscribeKey, 20, 1*time.Minute) {
				c.log.Debug("Subscribe rate limit exceeded",
					zap.String("connection_id", c.ID),
					zap.String("room_id", msg.RoomID.String()),
				)
				sendWSError(c.WS, clientMsgID, "rate_limit_exceeded", "subscribe", c.log)
				break
			}

			// Determine room type; default to chat when absent (canonical
			// for legacy raw-UUID clients per ParseRoomID convention).
			roomType := RoomType(msg.RoomType)
			switch roomType {
			case RoomTypeChat, RoomTypeOrder, RoomTypeAuction:
				// valid
			default:
				roomType = RoomTypeChat
			}

			// MANDATORY governance gate — DatabaseRoomAuthorizer + lifecycle + pure evaluator.
			// hub.Subscribe is called ONLY on ALLOW.
			ctx := context.Background()
			result := c.gate.Evaluate(ctx, c.UserID, msg.RoomID, roomType)
			if !result.Allowed {
				sendWSError(c.WS, clientMsgID, string(result.DenyReason), "subscribe", c.log)
				break
			}

			c.hub.Subscribe(c, msg.RoomID)
			c.WS.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := c.WS.Write(marshalWSAck(clientMsgID, "subscribe", msg.RoomID)); err != nil {
				c.log.Debug("Failed to send subscribe ack frame",
					zap.String("connection_id", c.ID),
					zap.String("room_id", msg.RoomID.String()),
					zap.Error(err),
				)
			}

		case "unsubscribe":
			c.hub.Unsubscribe(c, msg.RoomID)
			c.WS.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := c.WS.Write(marshalWSAck(clientMsgID, "unsubscribe", msg.RoomID)); err != nil {
				c.log.Debug("Failed to send unsubscribe ack frame",
					zap.String("connection_id", c.ID),
					zap.String("room_id", msg.RoomID.String()),
					zap.Error(err),
				)
			}

		default:
			c.log.Debug("Unknown action",
				zap.String("connection_id", c.ID),
				zap.String("action", msg.Action),
			)
		}
	}
}

func parseClientControlMessage(raw []byte) (ClientMessage, string, bool) {
	var legacy ClientMessage
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.Action != "" && legacy.RoomID != uuid.Nil {
		return legacy, "", true
	}

	var env clientEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ClientMessage{}, "", false
	}

	action := env.Type

	roomIDStr := env.RoomID
	roomType := env.RoomType
	if env.Data != nil {
		if roomIDStr == "" {
			if v, ok := env.Data["room_id"].(string); ok {
				roomIDStr = v
			}
		}
		if roomType == "" {
			if v, ok := env.Data["room_type"].(string); ok {
				roomType = v
			}
		}
	}

	if action == "ping" {
		return ClientMessage{Action: "ping"}, env.ID, true
	}

	if action == "" || roomIDStr == "" {
		return ClientMessage{}, env.ID, false
	}
	roomID, err := uuid.Parse(roomIDStr)
	if err != nil {
		return ClientMessage{}, env.ID, false
	}

	return ClientMessage{
		Action:   action,
		RoomID:   roomID,
		RoomType: roomType,
	}, env.ID, true
}

// WritePump pumps messages from the hub to the WebSocket connection.
//
// A goroutine running WritePump is started for each connection.
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
		c.log.Debug("Write pump stopped", zap.String("connection_id", c.ID))
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				return
			}

			c.WS.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if _, err := c.WS.Write(message); err != nil {
				c.log.Debug("Write error, closing connection",
					zap.String("connection_id", c.ID),
					zap.Error(err),
				)
				return
			}

		case <-ticker.C:
			c.WS.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := c.WS.Write(marshalWSHeartbeat()); err != nil {
				c.log.Debug("Ping error, closing connection",
					zap.String("connection_id", c.ID),
					zap.Error(err),
				)
				return
			}

			// Periodic lifecycle check — evict non-active users (removed/banned/suspended).
			// Closes the stale-socket gap where a user's status changes after WS connect.
			// Complements the event-driven user.banned eviction with a polling safety net.
			ctx := context.Background()
			if !c.gate.IsAlive(ctx, c.UserID) {
				c.log.Info("Evicting non-active user from WS",
					zap.String("connection_id", c.ID),
					zap.String("user_id", c.UserID.String()),
				)
				return // triggers deferred c.Close()
			}
		}
	}
}

// Close gracefully closes the connection.
// Idempotent: safe to call multiple times (uses sync.Once).
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		c.hub.Unregister(c)
		close(c.Send)
		c.WS.Close()

		c.log.Debug("Connection closed",
			zap.String("connection_id", c.ID),
			zap.String("user_id", c.UserID.String()),
		)
	})
}

// ServerMessage represents a message sent from the server to the client.
// This is the minimal realtime signal envelope per ADR-005 and
// governance-constitution.md §2.2 (minimal-envelope interim relaxation).
// The client fetches full message payload via REST.


