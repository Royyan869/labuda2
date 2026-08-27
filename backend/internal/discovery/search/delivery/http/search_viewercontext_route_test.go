package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/middleware"
	capabilityentity "github.com/labuda/backend/internal/platform/capability/entity"
)

type searchRouteActorResolver struct {
	actor *capabilityentity.Actor
	err   error
}

func (m *searchRouteActorResolver) ResolveActor(ctx interface{}, userID uuid.UUID) (*capabilityentity.Actor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.actor == nil {
		return nil, errors.New("actor not found")
	}
	m.actor.ID = userID
	return m.actor, nil
}

func TestSearchViewerContext_RouteActorChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	anonReq := func() (*httptest.ResponseRecorder, *viewercontext.ViewerContext) {
		router := gin.New()
		router.Use(middleware.StrictBrowseAuthMiddleware(nil))

		var vc *viewercontext.ViewerContext
		router.GET("/search/content", func(c *gin.Context) {
			vc = constructSearchContentViewerContext(c, nil)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/search/content", nil)
		router.ServeHTTP(w, req)
		return w, vc
	}

	_, vc := anonReq()
	if vc == nil || !vc.IsAnonymous() {
		t.Fatal("anonymous browse request must yield AnonymousViewer")
	}
	if vc.Capability().IsSeller {
		t.Fatal("anonymous browse request must not report seller capability")
	}

	cases := []struct {
		name       string
		actor      *capabilityentity.Actor
		wantSeller bool
	}{
		{
			name: "buyer_false",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
			},
			wantSeller: false,
		},
		{
			name: "active_seller_true",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
				EmailVerified: true,
				SellerStatus:  searchStringPtr(string(capabilityentity.SellerStatusActive)),
			},
			wantSeller: true,
		},
		{
			name: "expired_seller_false",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
				EmailVerified: true,
				SellerStatus:  searchStringPtr(string(capabilityentity.SellerStatusExpired)),
			},
			wantSeller: false,
		},
		{
			name: "kyc_state_does_not_change_seller_bit",
			actor: &capabilityentity.Actor{
				Role:               "user",
				AccountStatus:      "active",
				EmailVerified:      false,
				IsIdentityComplete: false,
				SellerStatus:       searchStringPtr(string(capabilityentity.SellerStatusActive)),
			},
			wantSeller: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.StrictBrowseAuthMiddleware(nil))
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uuid.New())
				c.Next()
			})
			router.Use(middleware.ActorContextInject(&searchRouteActorResolver{actor: tc.actor}, middleware.ActorContextInjectOptions{}))

			var vc *viewercontext.ViewerContext
			router.GET("/search/content", func(c *gin.Context) {
				vc = constructSearchContentViewerContext(c, nil)
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/search/content", nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if vc == nil {
				t.Fatal("viewer context was not captured")
			}
			if got := vc.Capability().IsSeller; got != tc.wantSeller {
				t.Fatalf("Capability.IsSeller = %v, want %v", got, tc.wantSeller)
			}
			if vc.IsAnonymous() {
				t.Fatal("authenticated route case must not yield AnonymousViewer")
			}
		})
	}

	t.Run("actor_resolution_failure_is_safe", func(t *testing.T) {
		router := gin.New()
		router.Use(middleware.StrictBrowseAuthMiddleware(nil))
		router.Use(func(c *gin.Context) {
			c.Set("user_id", uuid.New())
			c.Next()
		})
		router.Use(middleware.ActorContextInject(&searchRouteActorResolver{err: errors.New("resolver failure")}, middleware.ActorContextInjectOptions{}))

		var vc *viewercontext.ViewerContext
		router.GET("/search/content", func(c *gin.Context) {
			vc = constructSearchContentViewerContext(c, nil)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/search/content", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if vc == nil {
			t.Fatal("viewer context was not captured")
		}
		if vc.Capability().IsSeller {
			t.Fatal("actor resolution failure must not fabricate seller capability")
		}
		if vc.IsAnonymous() {
			t.Fatal("actor resolution failure must still be treated as authenticated request when user_id is present")
		}
	})
}

func searchStringPtr(v string) *string {
	return &v
}
