package serverboot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/middleware"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSetRole_SellerRejectedWithStableBadRequest(t *testing.T) {
	handler := &CoreUserHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/users/"+uuid.New().String()+"/role", nil)

	adminID := uuid.New()
	adminActor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{"governance.role.assign"},
	}
	c.Request = c.Request.WithContext(middleware.WithActor(context.Background(), adminActor))
	c.Set("userID", adminID)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	c.Request.Body = io.NopCloser(strings.NewReader(`{"role":"seller"}`))

	handler.SetRole(c)

	assertBadRequestRoleMessage(t, w, "seller")
}

func TestSetRole_UnknownRoleRejectedWithStableBadRequest(t *testing.T) {
	handler := &CoreUserHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/users/"+uuid.New().String()+"/role", nil)

	adminID := uuid.New()
	adminActor := &capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{"governance.role.assign"},
	}
	c.Request = c.Request.WithContext(middleware.WithActor(context.Background(), adminActor))
	c.Set("userID", adminID)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	c.Request.Body = io.NopCloser(strings.NewReader(`{"role":"superadmin"}`))

	handler.SetRole(c)

	assertBadRequestRoleMessage(t, w, "superadmin")
}

func assertBadRequestRoleMessage(t *testing.T, w *httptest.ResponseRecorder, role string) {
	t.Helper()

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error payload missing: %v", resp)
	}
	if errObj["code"] != "BAD_REQUEST" {
		t.Fatalf("error.code=%v want BAD_REQUEST", errObj["code"])
	}
	wantMessage := "Invalid role: " + role
	if errObj["message"] != wantMessage {
		t.Fatalf("error.message=%v want %s", errObj["message"], wantMessage)
	}
}
