package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateContentBinderRejectsLegacyTopLevelTypeAndAllowsTypedMedia(t *testing.T) {
	t.Parallel()

	makeContext := func(body string) *gin.Context {
		t.Helper()

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		return ctx
	}

	t.Run("rejects legacy post type", func(t *testing.T) {
		t.Parallel()

		ctx := makeContext(`{
			"caption":"hello",
			"visibility":"public",
			"type":"post",
			"media":[{"url":"https://cdn.example.test/1.jpg","type":"image"}]
		}`)

		var req CreateContentRequest
		err := bindStrictContentJSON(ctx, &req, "share_reference", "original_author_id", "type")
		if err == nil {
			t.Fatalf("expected legacy top-level type to be rejected")
		}
		if !strings.Contains(err.Error(), "legacy field type is not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects legacy request type", func(t *testing.T) {
		t.Parallel()

		ctx := makeContext(`{
			"caption":"hello",
			"visibility":"public",
			"type":"request",
			"media":[{"url":"https://cdn.example.test/1.jpg","type":"image"}]
		}`)

		var req CreateContentRequest
		err := bindStrictContentJSON(ctx, &req, "share_reference", "original_author_id", "type")
		if err == nil {
			t.Fatalf("expected legacy top-level type to be rejected")
		}
		if !strings.Contains(err.Error(), "legacy field type is not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("accepts canonical media type payload", func(t *testing.T) {
		t.Parallel()

		ctx := makeContext(`{
			"caption":"hello",
			"visibility":"public",
			"media":[{"url":"https://cdn.example.test/1.jpg","type":"image"}]
		}`)

		var req CreateContentRequest
		err := bindStrictContentJSON(ctx, &req, "share_reference", "original_author_id", "type")
		if err != nil {
			t.Fatalf("expected canonical media payload to bind: %v", err)
		}
		if req.Caption != "hello" {
			t.Fatalf("unexpected caption: %q", req.Caption)
		}
		if len(req.Media) != 1 {
			t.Fatalf("expected one media item, got %d", len(req.Media))
		}
		if req.Media[0].Type != "image" {
			t.Fatalf("expected media type to remain canonical, got %q", req.Media[0].Type)
		}
	})
}

func TestContentFulfillAuthorityIsAbsent(t *testing.T) {
	t.Parallel()

	handlerSrc, err := os.ReadFile("content_handler.go")
	if err != nil {
		t.Fatalf("read content_handler.go: %v", err)
	}
	statusSrc, err := os.ReadFile(filepath.Join("..", "..", "entity", "content_status.go"))
	if err != nil {
		t.Fatalf("read content_status.go: %v", err)
	}

	if strings.Contains(strings.ToLower(string(handlerSrc)), "/fulfill") {
		t.Fatalf("content handler still exposes a fulfill route")
	}
	if strings.Contains(strings.ToLower(string(handlerSrc)), "fulfilled") {
		t.Fatalf("content handler still mentions fulfilled authority")
	}
	if strings.Contains(string(statusSrc), "StatusFulfilled") {
		t.Fatalf("content status enum still exposes fulfilled")
	}
	if !strings.Contains(string(statusSrc), "StatusActive") || !strings.Contains(string(statusSrc), "StatusDeleted") {
		t.Fatalf("content status enum lost the canonical active/deleted states")
	}
}
