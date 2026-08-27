package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newEvidenceCapabilityTestActor(capabilities []string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{ID: uuid.New(), Role: "admin", Capabilities: capabilities}
}

func newEvidenceCapabilityTestRouter(actor *capabilityEntity.Actor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		if actor != nil {
			ctx := capability.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
			c.Set("user_id", actor.ID)
			c.Set("user_role", actor.Role)
		}
		c.Next()
	})

	handler := NewModerationHandler(nil, nil, zap.NewNop(), evidenceTestAuditLogger{})

	admin := r.Group("/admin")
	admin.GET("/moderation/cases/:id/evidence",
		middleware.RequireCapability("moderation.evidence.read"),
		handler.GetCaseEvidence)
	return r
}

func TestGetCaseEvidence_CaseReadOnly_Forbidden(t *testing.T) {
	router := newEvidenceCapabilityTestRouter(newEvidenceCapabilityTestActor([]string{"moderation.case.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/moderation/cases/"+uuid.New().String()+"/evidence", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetCaseEvidence_NoCapability_Forbidden(t *testing.T) {
	router := newEvidenceCapabilityTestRouter(newEvidenceCapabilityTestActor(nil))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/moderation/cases/"+uuid.New().String()+"/evidence", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetCaseEvidence_EvidenceRead_AllowedPastMiddleware(t *testing.T) {
	router := newEvidenceCapabilityTestRouter(newEvidenceCapabilityTestActor([]string{"moderation.evidence.read"}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/moderation/cases/"+uuid.New().String()+"/evidence", nil)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}
