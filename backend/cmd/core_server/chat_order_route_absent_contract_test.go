package main

import (
	"os"
	"strings"
	"testing"
)

// TestChatOrderRouteAbsent_DeadEndpointMustNotReturn guards the N6 purge of the
// legacy chat order entry point:
//
//	POST /api/v1/chat/rooms/:room_id/order
//
// The endpoint had zero first-party callers and no capability unique versus the
// canonical checkout path (pricing preview → pricing token → POST /orders →
// OrderCreationService.CreateFromSaleSurface). It must never be re-registered,
// and its handler must never come back.
func TestChatOrderRouteAbsent_DeadEndpointMustNotReturn(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}

	code := string(src)

	if strings.Contains(code, `chatRoutes.POST("/rooms/:room_id/order"`) {
		t.Fatal("regression: dead route POST /chat/rooms/:room_id/order must not be registered")
	}
	if strings.Contains(code, "CreateOrderFromChat") {
		t.Fatal("regression: CreateOrderFromChat handler must not be referenced in route registration")
	}
}
