package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestChatRoomGetRoute_RegisteredWithCanonicalHandler asserts the canonical
// single-room read is registered on the existing chat room namespace, bound to
// the canonical chat handler method.
func TestChatRoomGetRoute_RegisteredWithCanonicalHandler(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}

	code := string(src)

	if !strings.Contains(code, `chatRoutes.GET("/rooms/:room_id", deps.ChatHandler.GetRoom)`) {
		t.Fatal("regression: canonical single-room read GET /chat/rooms/:room_id must be registered")
	}
}

// TestChatRoomGetRoute_StaticAndParamSiblingsResolve proves gin accepts the
// canonical route table: GET /chat/rooms/:room_id is registered alongside the
// static GET /chat/rooms/by-order/:order_id. Gin panics on an unresolved route
// conflict (which would take the whole server down at boot), and the static
// sibling must still dispatch to its own handler rather than being captured by
// the :room_id wildcard.
func TestChatRoomGetRoute_StaticAndParamSiblingsResolve(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var roomIDSeen, orderIDSeen string

	router := gin.New()
	router.GET("/api/v1/chat/rooms/by-order/:order_id", func(c *gin.Context) {
		orderIDSeen = c.Param("order_id")
		c.Status(http.StatusOK)
	})
	router.GET("/api/v1/chat/rooms/:room_id", func(c *gin.Context) {
		roomIDSeen = c.Param("room_id")
		c.Status(http.StatusOK)
	})

	// Static sibling must win for /rooms/by-order/... and must not be captured
	// by the :room_id wildcard.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/by-order/order-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("by-order status=%d want 200", w.Code)
	}
	if orderIDSeen != "order-1" {
		t.Fatalf("by-order handler saw order_id=%q want %q (wildcard shadowing regression)", orderIDSeen, "order-1")
	}
	if roomIDSeen != "" {
		t.Fatalf("static route dispatched to the :room_id handler (room_id=%q)", roomIDSeen)
	}

	// Parameterised sibling must dispatch to the single-room handler.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/room-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("single-room status=%d want 200", w.Code)
	}
	if roomIDSeen != "room-1" {
		t.Fatalf("single-room handler saw room_id=%q want %q", roomIDSeen, "room-1")
	}
}
