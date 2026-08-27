package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	capabilityContext "github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubInteractionAuthorityService struct {
	state interactionAuthorityState
	err   error
}

func (s *stubInteractionAuthorityService) GetInteractionAuthority(ctx context.Context, userID uuid.UUID) (interactionAuthorityState, error) {
	return s.state, s.err
}

func TestRequireInteractionAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("verified user reaches next handler", func(t *testing.T) {
		service := &stubInteractionAuthorityService{
			state: interactionAuthorityState{Found: true, EmailVerified: true},
		}

		nextCalled := false
		router := gin.New()
		router.Use(func(c *gin.Context) {
			userID := uuid.New()
			c.Set("user_id", userID)
			c.Set("userID", userID)
			ctx := capabilityContext.WithActor(c.Request.Context(), &capabilityEntity.Actor{
				ID:            userID,
				EmailVerified: true,
			})
			c.Request = c.Request.WithContext(ctx)
		})
		router.Use(requireInteractionAuthorityWithService(service))
		router.GET("/", func(c *gin.Context) {
			nextCalled = true
			c.Status(http.StatusNoContent)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.True(t, nextCalled)
	})

	t.Run("unverified user gets stable forbidden code", func(t *testing.T) {
		service := &stubInteractionAuthorityService{
			state: interactionAuthorityState{Found: true, EmailVerified: false},
		}

		router := gin.New()
		router.Use(func(c *gin.Context) {
			userID := uuid.New()
			c.Set("user_id", userID)
			ctx := capabilityContext.WithActor(c.Request.Context(), &capabilityEntity.Actor{
				ID:            userID,
				EmailVerified: false,
			})
			c.Request = c.Request.WithContext(ctx)
		})
		router.Use(requireInteractionAuthorityWithService(service))
		router.GET("/", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(w, req)

		var resp response.Response
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, http.StatusForbidden, w.Code)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "EMAIL_VERIFICATION_REQUIRED", resp.Error.Code)
	})

	t.Run("missing or soft deleted user gets user not found", func(t *testing.T) {
		service := &stubInteractionAuthorityService{
			state: interactionAuthorityState{Found: false},
		}

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", uuid.New())
		})
		router.Use(requireInteractionAuthorityWithService(service))
		router.GET("/", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(w, req)

		var resp response.Response
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		require.NotNil(t, resp.Error)
		assert.Equal(t, response.ErrCodeUserNotFound, resp.Error.Code)
	})
}


