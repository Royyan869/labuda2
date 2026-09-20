//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newChatRoomGetRouter wires the canonical single-room read alongside the room
// list, so the two contracts can be compared against each other.
func newChatRoomGetRouter(userID uuid.UUID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	chatRoutes := router.Group("/api/v1/chat")
	chatRoutes.GET("/rooms", handler.ListRooms)
	chatRoutes.GET("/rooms/:room_id", handler.GetRoom)

	return router
}

// getRoomHTTP calls the canonical single-room read.
func getRoomHTTP(t *testing.T, router *gin.Engine, roomID string) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+roomID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return w.Code, nil
	}

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data envelope, got %s", w.Body.String())
	return w.Code, data
}

// listRoomsHTTP returns the canonical room list items keyed by room id.
func listRoomsHTTP(t *testing.T, router *gin.Engine) map[string]map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// The list route nests the collection one level deeper than the
	// single-room read: {"success":true,"data":{"data":[...]}}.
	envelope, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected list data envelope, got %s", w.Body.String())

	raw, ok := envelope["data"].([]any)
	require.True(t, ok, "expected list collection, got %s", w.Body.String())

	out := make(map[string]map[string]any, len(raw))
	for _, item := range raw {
		room, ok := item.(map[string]any)
		require.True(t, ok)
		id, ok := room["id"].(string)
		require.True(t, ok)
		out[id] = room
	}
	return out
}

func insertUserBlock(t *testing.T, ctx context.Context, fixture *unreadIntegrationFixture, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := fixture.appDB.Pool().Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func lastReadAt(t *testing.T, ctx context.Context, fixture *unreadIntegrationFixture, roomID, userID uuid.UUID) time.Time {
	t.Helper()

	var got time.Time
	err := fixture.appDB.Pool().QueryRow(ctx, `
		SELECT last_read_at FROM chat_read_states WHERE room_id = $1 AND user_id = $2
	`, roomID, userID).Scan(&got)
	require.NoError(t, err)
	return got
}

// TestGetRoom_ParticipantReadsCanonicalRoom proves the canonical conversation-open
// read: a participant resolves the room by id, and the payload is the SAME
// room-summary contract as the corresponding room-list item (no parallel
// single-room representation).
func TestGetRoom_ParticipantReadsCanonicalRoom(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	userB := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

	insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "hello A", time.Now().UTC())

	router := newChatRoomGetRouter(userA, fixture.handler)

	status, data := getRoomHTTP(t, router, roomID.String())
	require.Equal(t, http.StatusOK, status)

	require.Equal(t, roomID.String(), data["id"])
	require.Equal(t, "direct", data["room_type"])
	require.Equal(t, userB.String(), data["other_user_id"])
	require.Equal(t, float64(1), data["unread_count"], "unread must come from the canonical unread authority")
	require.NotNil(t, data["last_message"], "room summary must carry the latest message preview")

	listItems := listRoomsHTTP(t, router)
	listItem, ok := listItems[roomID.String()]
	require.True(t, ok, "room must also appear in the canonical room list")

	require.Equal(t, listItem, data,
		"single-room read must reuse the canonical room-list item contract, not a parallel one")
}

// TestGetRoom_NonParticipantDenied proves the existing participant authority is
// reused: a user who is not a participant cannot read the room.
func TestGetRoom_NonParticipantDenied(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	userB := insertUnreadTestUser(t, ctx, fixture.appDB)
	outsider := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

	router := newChatRoomGetRouter(outsider, fixture.handler)

	status, _ := getRoomHTTP(t, router, roomID.String())
	require.Equal(t, http.StatusForbidden, status)
}

// TestGetRoom_NotFound proves an unknown room id yields the canonical not-found
// contract rather than a leak of existence.
func TestGetRoom_NotFound(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	router := newChatRoomGetRouter(userA, fixture.handler)

	status, _ := getRoomHTTP(t, router, uuid.NewString())
	require.Equal(t, http.StatusNotFound, status)
}

// TestGetRoom_InvalidID proves the parameter contract.
func TestGetRoom_InvalidID(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	router := newChatRoomGetRouter(userA, fixture.handler)

	status, _ := getRoomHTTP(t, router, "not-a-uuid")
	require.Equal(t, http.StatusBadRequest, status)
}

// TestGetRoom_BlockedSocialRoomHidden proves block parity with the room list and
// ListMessages: a blocked direct room does not resolve through the single-room
// read (so the conversation-open path cannot leak content the list hides).
func TestGetRoom_BlockedSocialRoomHidden(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	userB := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)
	insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "hidden", time.Now().UTC())
	insertUserBlock(t, ctx, fixture, userA, userB)

	router := newChatRoomGetRouter(userA, fixture.handler)

	status, _ := getRoomHTTP(t, router, roomID.String())
	require.Equal(t, http.StatusNotFound, status, "blocked direct room must be hidden, same as the room list")

	listItems := listRoomsHTTP(t, router)
	_, listed := listItems[roomID.String()]
	require.False(t, listed, "blocked direct room must not appear in the room list either")
}

// TestGetRoom_UnreadClearsThroughCanonicalMarkRead proves the conversation-open
// read lifecycle uses the existing read authority end to end: mark-read advances
// chat_read_states.last_read_at in persistence and the single-room read then
// reports zero unread. No second unread mechanism is involved.
func TestGetRoom_UnreadClearsThroughCanonicalMarkRead(t *testing.T) {
	ctx := context.Background()
	fixture := newUnreadIntegrationFixture(t)

	userA := insertUnreadTestUser(t, ctx, fixture.appDB)
	userB := insertUnreadTestUser(t, ctx, fixture.appDB)
	roomID := insertUnreadTestRoom(t, ctx, fixture.appDB, userA, userB)

	messageAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	insertUnreadTestMessage(t, ctx, fixture.appDB, roomID, userB, "unread for A", messageAt)

	readRouter := newUnreadAuthRouter(userA, fixture.handler)
	readStatus, data := getRoomHTTP(t, newChatRoomGetRouter(userA, fixture.handler), roomID.String())
	require.Equal(t, http.StatusOK, readStatus)
	require.Equal(t, float64(1), data["unread_count"])

	// Canonical mark-read (the same call the conversation-open path makes).
	require.Equal(t, http.StatusOK, markReadHTTP(t, readRouter, roomID, time.Now().UTC()))

	stored := lastReadAt(t, ctx, fixture, roomID, userA)
	require.True(t, stored.After(messageAt),
		"canonical mark-read must advance last_read_at past the unread message")

	_, data = getRoomHTTP(t, newChatRoomGetRouter(userA, fixture.handler), roomID.String())
	require.Equal(t, float64(0), data["unread_count"], "unread must clear via the canonical authority only")
}
