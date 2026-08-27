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

type feedRouteActorResolver struct {
	actor *capabilityentity.Actor
	err   error
}

func (m *feedRouteActorResolver) ResolveActor(ctx interface{}, userID uuid.UUID) (*capabilityentity.Actor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.actor == nil {
		return nil, errors.New("actor not found")
	}
	m.actor.ID = userID
	return m.actor, nil
}

func TestFeedViewerContext_RouteActorChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		actor      *capabilityentity.Actor
		wantSeller bool
		wantAnon   bool
	}{
		{
			name:       "missing_actor_is_safe",
			actor:      nil,
			wantSeller: false,
			wantAnon:   false,
		},
		{
			name: "buyer_false",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
			},
			wantSeller: false,
			wantAnon:   false,
		},
		{
			name: "active_seller_true",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
				EmailVerified: true,
				SellerStatus:  feedStringPtr(string(capabilityentity.SellerStatusActive)),
			},
			wantSeller: true,
			wantAnon:   false,
		},
		{
			name: "expired_seller_false",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
				EmailVerified: true,
				SellerStatus:  feedStringPtr(string(capabilityentity.SellerStatusExpired)),
			},
			wantSeller: false,
			wantAnon:   false,
		},
		{
			name: "actor_resolution_failure",
			actor: &capabilityentity.Actor{
				Role:          "user",
				AccountStatus: "active",
				EmailVerified: true,
				SellerStatus:  feedStringPtr(string(capabilityentity.SellerStatusActive)),
			},
			wantSeller: false,
			wantAnon:   false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uuid.New())
				c.Next()
			})
			if tc.name == "actor_resolution_failure" {
				router.Use(middleware.ActorContextInject(&feedRouteActorResolver{err: errors.New("resolver failure")}, middleware.ActorContextInjectOptions{}))
			} else if tc.actor != nil {
				router.Use(middleware.ActorContextInject(&feedRouteActorResolver{actor: tc.actor}, middleware.ActorContextInjectOptions{}))
			}

			var vc *viewercontext.ViewerContext
			router.GET("/api/v1/feed", func(c *gin.Context) {
				vc = constructFeedViewerContext(c, nil)
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/feed", nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if vc == nil {
				t.Fatal("viewer context was not captured")
			}
			if vc.Capability().IsSeller != tc.wantSeller {
				t.Fatalf("Capability.IsSeller = %v, want %v", vc.Capability().IsSeller, tc.wantSeller)
			}
			if vc.IsAnonymous() != tc.wantAnon {
				t.Fatalf("IsAnonymous = %v, want %v", vc.IsAnonymous(), tc.wantAnon)
			}
		})
	}
}

func feedStringPtr(v string) *string {
	return &v
}
