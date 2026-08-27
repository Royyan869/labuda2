package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MetricsCollector defines the interface for metrics collection.
// Using interface allows nil-check without type assertion.
type MetricsCollector interface {
	IncrementActiveConnections()
	DecrementActiveConnections()
	SetActiveRooms(count int)
	RecordBroadcast(durationSeconds float64)
}

// Hub maintains the set of active connections and broadcasts messages to rooms.
//
// RESPONSIBILITIES:
// - Maintain thread-safe connection registry
// - Manage room subscriptions
// - Broadcast messages to rooms
//
// CONSTRAINTS:
// - No domain imports
// - No DB calls
// - No heavy payload processing
type Hub struct {
	// connections maps connection ID to Connection
	connections map[string]*Connection

	// rooms maps room ID to map of connection IDs in that room
	// map[roomID]map[connectionID]*Connection
	rooms map[uuid.UUID]map[string]*Connection

	mu sync.RWMutex

	log     *zap.Logger
	metrics MetricsCollector // Optional metrics collector
}

// NewHub creates a new Hub instance.
func NewHub(log *zap.Logger) *Hub {
	if log == nil {
		log = zap.NewNop()
	}
	return &Hub{
		connections: make(map[string]*Connection),
		rooms:       make(map[uuid.UUID]map[string]*Connection),
		log:         log,
		metrics:     nil, // No metrics by default
	}
}

// NewHubWithMetrics creates a new Hub instance with metrics enabled.
func NewHubWithMetrics(log *zap.Logger, metrics MetricsCollector) *Hub {
	if log == nil {
		log = zap.NewNop()
	}
	return &Hub{
		connections: make(map[string]*Connection),
		rooms:       make(map[uuid.UUID]map[string]*Connection),
		log:         log,
		metrics:     metrics,
	}
}

// Register adds a new connection to the hub.
func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connections[conn.ID] = conn

	// Update metrics
	if h.metrics != nil {
		h.metrics.IncrementActiveConnections()
	}

	h.log.Debug("Connection registered",
		zap.String("connection_id", conn.ID),
		zap.String("user_id", conn.UserID.String()),
	)
}

// Unregister removes a connection from the hub and all its rooms.
// Idempotent: safe to call multiple times for the same connection.
func (h *Hub) Unregister(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Idempotent check: if connection already removed, return early
	if _, exists := h.connections[conn.ID]; !exists {
		return
	}

	// Remove from all rooms
	for roomID := range conn.Rooms {
		h.unsubscribeUnsafe(conn, roomID)
	}

	// Remove connection
	delete(h.connections, conn.ID)

	// Update metrics
	if h.metrics != nil {
		h.metrics.DecrementActiveConnections()
		h.metrics.SetActiveRooms(len(h.rooms))
	}

	h.log.Debug("Connection unregistered",
		zap.String("connection_id", conn.ID),
		zap.String("user_id", conn.UserID.String()),
	)
}

// Subscribe adds a connection to a room.
func (h *Hub) Subscribe(conn *Connection, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Track if this is a new room for metrics
	wasNewRoom := h.rooms[roomID] == nil

	// Add room to connection's room set
	if conn.Rooms == nil {
		conn.Rooms = make(map[uuid.UUID]struct{})
	}
	conn.Rooms[roomID] = struct{}{}

	// Add connection to room
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[string]*Connection)
	}
	h.rooms[roomID][conn.ID] = conn

	// Update metrics if this was a new room
	if wasNewRoom && h.metrics != nil {
		h.metrics.SetActiveRooms(len(h.rooms))
	}

	h.log.Debug("Connection subscribed to room",
		zap.String("connection_id", conn.ID),
		zap.String("user_id", conn.UserID.String()),
		zap.String("room_id", roomID.String()),
	)
}

// Unsubscribe removes a connection from a room.
func (h *Hub) Unsubscribe(conn *Connection, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.unsubscribeUnsafe(conn, roomID)
}

// unsubscribeUnsafe removes a connection from a room without locking.
// Caller must hold h.mu lock.
func (h *Hub) unsubscribeUnsafe(conn *Connection, roomID uuid.UUID) {
	// Track room size before deletion for metrics
	wasLastConnection := false
	if roomConnections, exists := h.rooms[roomID]; exists {
		wasLastConnection = len(roomConnections) == 1
	}

	// Remove room from connection's room set
	delete(conn.Rooms, roomID)

	// Remove connection from room
	if roomConnections, exists := h.rooms[roomID]; exists {
		delete(roomConnections, conn.ID)
		// Clean up empty rooms
		if len(roomConnections) == 0 {
			delete(h.rooms, roomID)
		}
	}

	// Update metrics if room became empty
	if wasLastConnection && h.metrics != nil {
		h.metrics.SetActiveRooms(len(h.rooms))
	}

	h.log.Debug("Connection unsubscribed from room",
		zap.String("connection_id", conn.ID),
		zap.String("user_id", conn.UserID.String()),
		zap.String("room_id", roomID.String()),
	)
}

// BroadcastToRoomFiltered sends a message to connections in a room for which
// filter returns true. filter receives the subscriber's userID and returns true
// to deliver, false to drop. filter is called outside the hub lock and may
// perform IO (e.g. fresh lifecycle DB read).
//
// This is the canonical broadcast path per governance-constitution.md §2.2 and
// ADR-005: every subscriber must pass a fresh per-subscriber governance check
// before delivery. The blind-fanout BroadcastToRoom is retained only for
// internal/test uses where governance does not apply.
//
// Non-blocking: slow clients are disconnected rather than blocking broadcast.
func (h *Hub) BroadcastToRoomFiltered(roomID uuid.UUID, payload []byte, filter func(userID uuid.UUID) bool) {
	h.mu.RLock()
	roomConnections, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		h.log.Debug("No connections in room",
			zap.String("room_id", roomID.String()),
		)
		return
	}

	conns := make([]*Connection, 0, len(roomConnections))
	for _, c := range roomConnections {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	start := time.Now()
	sentCount := 0
	droppedCount := 0

	for _, c := range conns {
		// Per-subscriber governance filter — called outside lock, may be IO-bound.
		if filter != nil && !filter(c.UserID) {
			droppedCount++
			continue
		}

		select {
		case c.Send <- payload:
			sentCount++
		default:
			h.log.Warn("Slow client, disconnecting",
				zap.String("connection_id", c.ID),
				zap.String("room_id", roomID.String()),
			)
			h.Unregister(c)
		}
	}

	duration := time.Since(start).Seconds()
	if h.metrics != nil {
		h.metrics.RecordBroadcast(duration)
	}

	if duration > 0.05 {
		h.log.Warn("Slow broadcast detected",
			zap.Float64("duration_ms", duration*1000),
			zap.Int("connections", len(conns)),
			zap.String("room_id", roomID.String()),
		)
	}

	h.log.Debug("Filtered broadcast to room completed",
		zap.String("room_id", roomID.String()),
		zap.Int("connections", len(conns)),
		zap.Int("sent", sentCount),
		zap.Int("governance_dropped", droppedCount),
		zap.Float64("duration_seconds", duration),
	)
}

// BroadcastToRoom sends a message to ALL connections in a room without a
// governance filter. Use BroadcastToRoomFiltered for any user-facing delivery.
// This method exists for internal/test scenarios only.
func (h *Hub) BroadcastToRoom(roomID uuid.UUID, payload []byte) {
	h.BroadcastToRoomFiltered(roomID, payload, nil)
}

// BroadcastToUserFiltered sends a message to all active connections owned by
// a single user, subject to a fresh per-user filter.
//
// This is the canonical delivery path for user-targeted realtime signals such
// as room-list updates. It intentionally ignores room subscriptions so that a
// user's other active devices still receive the signal even if they are not
// currently subscribed to the chat room.
func (h *Hub) BroadcastToUserFiltered(userID uuid.UUID, payload []byte, filter func(userID uuid.UUID) bool) {
	if filter != nil && !filter(userID) {
		h.log.Debug("User-targeted broadcast dropped by filter",
			zap.String("user_id", userID.String()),
		)
		return
	}

	h.mu.RLock()
	conns := make([]*Connection, 0)
	for _, conn := range h.connections {
		if conn.UserID == userID {
			conns = append(conns, conn)
		}
	}
	h.mu.RUnlock()

	if len(conns) == 0 {
		h.log.Debug("No connections for user",
			zap.String("user_id", userID.String()),
		)
		return
	}

	start := time.Now()
	sentCount := 0

	for _, c := range conns {
		select {
		case c.Send <- payload:
			sentCount++
		default:
			h.log.Warn("Slow client, disconnecting",
				zap.String("connection_id", c.ID),
				zap.String("user_id", userID.String()),
			)
			h.Unregister(c)
		}
	}

	duration := time.Since(start).Seconds()
	if h.metrics != nil {
		h.metrics.RecordBroadcast(duration)
	}

	if duration > 0.05 {
		h.log.Warn("Slow user broadcast detected",
			zap.Float64("duration_ms", duration*1000),
			zap.Int("connections", len(conns)),
			zap.String("user_id", userID.String()),
		)
	}

	h.log.Debug("Filtered broadcast to user completed",
		zap.String("user_id", userID.String()),
		zap.Int("connections", len(conns)),
		zap.Int("sent", sentCount),
		zap.Float64("duration_seconds", duration),
	)
}

// EvictUser closes ALL active WebSocket connections for a user immediately.
// Called when a user is banned so their session is terminated before the next
// governance check would catch it (ADR-005: event-driven eviction, not polling).
// Safe for concurrent access; idempotent per connection (conn.Close uses sync.Once).
func (h *Hub) EvictUser(userID uuid.UUID) {
	h.mu.RLock()
	var conns []*Connection
	for _, conn := range h.connections {
		if conn.UserID == userID {
			conns = append(conns, conn)
		}
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		conn.Close()
		h.log.Info("Evicted WS session for banned user",
			zap.String("user_id", userID.String()),
			zap.String("connection_id", conn.ID),
		)
	}
}

// GetConnectionCount returns the number of active connections.
func (h *Hub) GetConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// GetRoomCount returns the number of active rooms.
func (h *Hub) GetRoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// GetConnection returns a connection by ID, or nil if not found.
func (h *Hub) GetConnection(connID string) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[connID]
}

// GetUserConnections returns all connections for a given user ID.
func (h *Hub) GetUserConnections(userID uuid.UUID) []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var conns []*Connection
	for _, conn := range h.connections {
		if conn.UserID == userID {
			conns = append(conns, conn)
		}
	}
	return conns
}


