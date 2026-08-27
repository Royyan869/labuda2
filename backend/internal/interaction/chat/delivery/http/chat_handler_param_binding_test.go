package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func makeParamBindingContext(method, target string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Set("userID", uuid.New())
	return c
}

func TestHandler_ParamBinding_UsesRoomID_LinkOrderToChat(t *testing.T) {
	h := &Handler{}
	c := makeParamBindingContext("PUT", "/chat/rooms/not-uuid/link-order")
	c.Params = gin.Params{
		{Key: "room_id", Value: "not-uuid"},
		{Key: "id", Value: uuid.New().String()},
	}
	h.LinkOrderToChat(c)
	if c.Writer.Status() != 400 {
		t.Fatalf("expected 400 for invalid room_id, got %d", c.Writer.Status())
	}
}

func TestHandler_ParamBinding_UsesRoomID_ListMessages(t *testing.T) {
	h := &Handler{}
	c := makeParamBindingContext("GET", "/chat/rooms/not-uuid/messages")
	c.Params = gin.Params{
		{Key: "room_id", Value: "not-uuid"},
		{Key: "id", Value: uuid.New().String()},
	}
	h.ListMessages(c)
	if c.Writer.Status() != 400 {
		t.Fatalf("expected 400 for invalid room_id, got %d", c.Writer.Status())
	}
}

func TestHandler_ParamBinding_UsesRoomID_SendMessage(t *testing.T) {
	h := &Handler{}
	c := makeParamBindingContext("POST", "/chat/rooms/not-uuid/messages")
	c.Params = gin.Params{
		{Key: "room_id", Value: "not-uuid"},
		{Key: "id", Value: uuid.New().String()},
	}
	h.SendMessage(c)
	if c.Writer.Status() != 400 {
		t.Fatalf("expected 400 for invalid room_id, got %d", c.Writer.Status())
	}
}

func TestHandler_ParamBinding_UsesRoomID_MarkAsRead(t *testing.T) {
	h := &Handler{}
	c := makeParamBindingContext("POST", "/chat/rooms/not-uuid/read")
	c.Params = gin.Params{
		{Key: "room_id", Value: "not-uuid"},
		{Key: "id", Value: uuid.New().String()},
	}
	h.MarkAsRead(c)
	if c.Writer.Status() != 400 {
		t.Fatalf("expected 400 for invalid room_id, got %d", c.Writer.Status())
	}
}

func TestHandler_ParamBinding_UsesRoomID_GetUnreadCount(t *testing.T) {
	h := &Handler{}
	c := makeParamBindingContext("GET", "/chat/rooms/not-uuid/unread")
	c.Params = gin.Params{
		{Key: "room_id", Value: "not-uuid"},
		{Key: "id", Value: uuid.New().String()},
	}
	h.GetUnreadCount(c)
	if c.Writer.Status() != 400 {
		t.Fatalf("expected 400 for invalid room_id, got %d", c.Writer.Status())
	}
}



