package mediaupload

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/platform/s3presign"
	"go.uber.org/zap"
)

func TestRequestUploadURL_UsesFixedSellerStoreKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.MustParse("62d7e998-f5d8-4486-be84-63d81f9c0e6f")
	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	router := gin.New()
	router.POST("/media/upload-url", func(c *gin.Context) {
		c.Set("userID", userID)
		handler.RequestUploadURL(c)
	})

	body, err := json.Marshal(UploadURLRequest{
		ContentType: "image/jpeg",
		Folder:      "images",
		StorageKey:  "images/stores/" + userID.String() + ".jpg",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T; want map", resp.Data)
	}

	if got := data["storage_key"]; got != "images/stores/"+userID.String()+".jpg" {
		t.Fatalf("storage_key = %v; want canonical seller key", got)
	}
	if got := data["read_url"]; got != "https://cdn.example.com/media/images/stores/"+userID.String()+".jpg" {
		t.Fatalf("read_url = %v; want stable CDN URL", got)
	}
}

func TestRequestUploadURL_UsesFixedProfileCoverKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.MustParse("62d7e998-f5d8-4486-be84-63d81f9c0e6f")
	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	router := gin.New()
	router.POST("/media/upload-url", func(c *gin.Context) {
		c.Set("userID", userID)
		handler.RequestUploadURL(c)
	})

	body, err := json.Marshal(UploadURLRequest{
		ContentType: "image/jpeg",
		Folder:      "images",
		StorageKey:  "images/profile-covers/" + userID.String() + ".jpg",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T; want map", resp.Data)
	}

	if got := data["storage_key"]; got != "images/profile-covers/"+userID.String()+".jpg" {
		t.Fatalf("storage_key = %v; want canonical profile cover key", got)
	}
	if got := data["read_url"]; got != "https://cdn.example.com/media/images/profile-covers/"+userID.String()+".jpg" {
		t.Fatalf("read_url = %v; want stable CDN URL", got)
	}
}

func TestRequestUploadURL_RejectsUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	router := gin.New()
	router.POST("/media/upload-url", handler.RequestUploadURL)

	body, err := json.Marshal(UploadURLRequest{
		ContentType: "image/jpeg",
		Folder:      "images",
		StorageKey:  "images/stores/test-user.jpg",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestRequestUploadURL_RejectsInvalidOwnedFixedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.MustParse("62d7e998-f5d8-4486-be84-63d81f9c0e6f")
	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	cases := []struct {
		name        string
		storageKey  string
		contentType string
		wantCode    string
		wantMessage string
	}{
		{
			name:        "store key owned by another user",
			storageKey:  "images/stores/another-user.jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "cover key owned by another user",
			storageKey:  "images/profile-covers/another-user.jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "avatar key owned by another user",
			storageKey:  "images/avatars/another-user.jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "legacy cover path",
			storageKey:  "images/covers/" + userID.String() + ".jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "traversal-like path",
			storageKey:  "images/stores/../secrets.jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "non-store prefix",
			storageKey:  "stores/" + userID.String() + ".jpg",
			contentType: "image/jpeg",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key",
		},
		{
			name:        "avatar key with unsupported mime",
			storageKey:  "images/avatars/" + userID.String() + ".jpg",
			contentType: "application/pdf",
			wantCode:    "INVALID_CONTENT_TYPE",
			wantMessage: "content_type must be image/jpeg, image/png, image/webp, image/gif, or video/mp4",
		},
		{
			name:        "store key cannot use png mime",
			storageKey:  "images/stores/" + userID.String() + ".jpg",
			contentType: "image/png",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "fixed image uploads must use image/jpeg",
		},
		{
			name:        "cover key cannot use avatar metadata mime",
			storageKey:  "images/profile-covers/" + userID.String() + ".jpg",
			contentType: "image/png",
			wantCode:    "INVALID_STORAGE_KEY",
			wantMessage: "fixed image uploads must use image/jpeg",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/media/upload-url", func(c *gin.Context) {
				c.Set("userID", userID)
				handler.RequestUploadURL(c)
			})

			body, err := json.Marshal(UploadURLRequest{
				ContentType: tc.contentType,
				Folder:      "images",
				StorageKey:  tc.storageKey,
			})
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
			}

			var resp response.Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Error == nil || resp.Error.Code != tc.wantCode {
				t.Fatalf("error code = %v; want %s", resp.Error, tc.wantCode)
			}
			if resp.Error.Message != tc.wantMessage {
				t.Fatalf("error message = %q; want %q", resp.Error.Message, tc.wantMessage)
			}
		})
	}
}

func TestRequestUploadURL_AllowsOwnedAvatarFixedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.MustParse("62d7e998-f5d8-4486-be84-63d81f9c0e6f")
	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	cases := []struct {
		name       string
		storageKey string
	}{
		{
			name:       "jpeg avatar key",
			storageKey: "images/avatars/" + userID.String() + ".jpg",
		},
		{
			name:       "png avatar key",
			storageKey: "images/avatars/" + userID.String() + ".png",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/media/upload-url", func(c *gin.Context) {
				c.Set("userID", userID)
				handler.RequestUploadURL(c)
			})

			body, err := json.Marshal(UploadURLRequest{
				ContentType: "image/png",
				Folder:      "images",
				StorageKey:  tc.storageKey,
			})
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
			}

			var resp response.Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			data, ok := resp.Data.(map[string]any)
			if !ok {
				t.Fatalf("data type = %T; want map", resp.Data)
			}
			if got := data["storage_key"]; got != tc.storageKey {
				t.Fatalf("storage_key = %v; want %s", got, tc.storageKey)
			}
		})
	}
}

func TestRequestUploadURL_UsesOwnedCommercePosterKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.MustParse("62d7e998-f5d8-4486-be84-63d81f9c0e6f")
	handler := NewHandler(
		s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		"https://cdn.example.com/media",
		zap.NewNop(),
	)

	router := gin.New()
	router.POST("/media/upload-url", func(c *gin.Context) {
		c.Set("userID", userID)
		handler.RequestUploadURL(c)
	})

	storageKey := "videos/1712345678901_" + userID.String() + ".mp4_poster.jpg"
	body, err := json.Marshal(UploadURLRequest{
		ContentType: "image/jpeg",
		Folder:      "images",
		StorageKey:  storageKey,
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/media/upload-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T; want map", resp.Data)
	}
	if got := data["storage_key"]; got != storageKey {
		t.Fatalf("storage_key = %v; want %s", got, storageKey)
	}
}

// TestNoDeleteMediaEndpoint_OwnerDecisionLocked documents the locked owner
// decision from STAGE 4F-1: cover (and general media) REMOVAL clears the
// database reference only. No /media/delete-url endpoint is implemented and
// no presigned DELETE is minted. This test replaces the earlier delete-url
// contract tests (which described a design that was explicitly rejected) and
// locks the decision so a future implementation cannot silently reintroduce
// the endpoint without revisiting it.
func TestNoDeleteMediaEndpoint_OwnerDecisionLocked(t *testing.T) {
	source, err := os.ReadFile("../../../cmd/core_server/routes_core.go")
	if err != nil {
		t.Fatalf("failed to read routes_core.go: %v", err)
	}
	text := strings.ReplaceAll(string(source), "\r\n", "\n")

	if strings.Contains(text, `"/media/delete-url"`) {
		t.Fatal("delete-url route must NOT be registered per locked owner decision (STAGE 4F-1)")
	}

	handlerSource, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("failed to read handler.go: %v", err)
	}
	handlerText := strings.ReplaceAll(string(handlerSource), "\r\n", "\n")
	if strings.Contains(handlerText, "RequestDeleteURL") || strings.Contains(handlerText, "PresignDELETE") {
		t.Fatal("handler must NOT expose delete-url logic per locked owner decision (STAGE 4F-1)")
	}
}
