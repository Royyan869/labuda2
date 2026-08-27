package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// Helpers
// ============================================================================

func noopLogger() *zap.Logger { return zap.NewNop() }

// buildEventRouter wraps a PromotionHandler in a Gin router so ServeHTTP
// flushes headers correctly (direct handler calls don't flush).
// userID == uuid.Nil → no user injected (unauthenticated simulation).
func buildEventRouter(h *PromotionHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if userID != uuid.Nil {
		r.Use(func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Next()
		})
	}
	r.POST("/events", h.RecordEvent)
	return r
}

// serveEvent fires POST /events through the router and returns the recorder.
func serveEvent(t *testing.T, r *gin.Engine, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ============================================================================
// Stubs
// ============================================================================

type eventRecorderStub struct {
	recorded []*promoentity.PromotionEvent
	failWith error
}

var _ promotionRepo.PromotionEventRepository = (*eventRecorderStub)(nil)

func (s *eventRecorderStub) RecordEvent(_ context.Context, _ db.Tx, ev *promoentity.PromotionEvent) error {
	if s.failWith != nil {
		return s.failWith
	}
	s.recorded = append(s.recorded, ev)
	return nil
}

func (s *eventRecorderStub) GetCampaignAnalytics(
	_ context.Context,
	_ db.Tx,
	instanceID uuid.UUID,
	from *time.Time,
	to *time.Time,
) (*promotionRepo.PromotionEventAnalyticsSummary, error) {
	return &promotionRepo.PromotionEventAnalyticsSummary{
		InstanceID:         instanceID,
		WindowFrom:         from,
		WindowTo:           to,
		ImpressionsTotal:   0,
		ClicksTotal:        0,
		CTR:                0,
		FeedImpressions:    0,
		FeedClicks:         0,
		SearchImpressions:  0,
		SearchClicks:       0,
		ExploreImpressions: 0,
		ExploreClicks:      0,
	}, nil
}

// promotionEventTestDB satisfies db.Transactor; withTxErr short-circuits fn.
type promotionEventTestDB struct {
	withTxErr error
}

func (d *promotionEventTestDB) WithTx(_ context.Context, fn func(db.Tx) error) error {
	if d.withTxErr != nil {
		return d.withTxErr
	}
	return fn(&promotionEventTestTx{})
}

// promotionEventTestTx satisfies db.Tx; QueryRow returns errRow so any
// GetInstanceByID scan fails → triggers the "instance not found → 204" path.
type promotionEventTestTx struct{}

var _ db.Tx = (*promotionEventTestTx)(nil)

func (t *promotionEventTestTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), nil
}
func (t *promotionEventTestTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (t *promotionEventTestTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &errRow{err: errors.New("no rows in result set")}
}
func (t *promotionEventTestTx) Commit(_ context.Context) error   { return nil }
func (t *promotionEventTestTx) Rollback(_ context.Context) error { return nil }

type errRow struct{ err error }

func (r *errRow) Scan(_ ...any) error { return r.err }

// ============================================================================
// Tests
// ============================================================================

// TestRecordEvent_EventRepoNil — nil eventRepo returns 204 (tracking disabled).
func TestRecordEvent_EventRepoNil(t *testing.T) {
	h := &PromotionHandler{} // eventRepo nil
	r := buildEventRouter(h, uuid.New())
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": uuid.New().String(),
		"event_type":            "click",
		"surface":               "feed",
	})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestRecordEvent_MissingAuth — 401 when user_id is absent from context.
func TestRecordEvent_MissingAuth(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{
		eventRepo: recorder,
		db:        &promotionEventTestDB{withTxErr: errors.New("should not be called")},
	}
	r := buildEventRouter(h, uuid.Nil) // no user
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": uuid.New().String(),
		"event_type":            "click",
		"surface":               "feed",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, recorder.recorded)
}

// TestRecordEvent_InvalidJSON — 400 for malformed body.
func TestRecordEvent_InvalidJSON(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{
		eventRepo: recorder,
		db:        &promotionEventTestDB{withTxErr: errors.New("should not be called")},
	}
	r := buildEventRouter(h, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRecordEvent_InvalidInstanceID — 400 for non-UUID string.
func TestRecordEvent_InvalidInstanceID(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{eventRepo: recorder, db: &promotionEventTestDB{}}
	r := buildEventRouter(h, uuid.New())
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": "not-a-uuid",
		"event_type":            "click",
		"surface":               "feed",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, recorder.recorded)
}

// TestRecordEvent_InvalidEventType — 400 for unknown event_type.
func TestRecordEvent_InvalidEventType(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{eventRepo: recorder, db: &promotionEventTestDB{}}
	r := buildEventRouter(h, uuid.New())
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": uuid.New().String(),
		"event_type":            "unknown_event",
		"surface":               "feed",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, recorder.recorded)
}

// TestRecordEvent_InvalidSurface — 400 for unknown surface.
func TestRecordEvent_InvalidSurface(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{eventRepo: recorder, db: &promotionEventTestDB{}}
	r := buildEventRouter(h, uuid.New())
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": uuid.New().String(),
		"event_type":            "click",
		"surface":               "unknown_surface",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, recorder.recorded)
}

// TestRecordEvent_InstanceNotFound — stale promotion_instance_id yields silent 204.
func TestRecordEvent_InstanceNotFound(t *testing.T) {
	recorder := &eventRecorderStub{}
	h := &PromotionHandler{
		eventRepo: recorder,
		log:       noopLogger(),
		db:        &promotionEventTestDB{withTxErr: errors.New("not found")},
	}
	r := buildEventRouter(h, uuid.New())
	w := serveEvent(t, r, map[string]string{
		"promotion_instance_id": uuid.New().String(),
		"event_type":            "click",
		"surface":               "feed",
	})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, recorder.recorded)
}

// TestRecordEvent_EventTypeValues — "click" and "impression" valid; others are 400.
func TestRecordEvent_EventTypeValues(t *testing.T) {
	cases := []struct {
		eventType string
		wantCode  int
	}{
		{"click", http.StatusNoContent},      // passes validation; instance-not-found → silent 204
		{"impression", http.StatusNoContent}, // passes validation; instance-not-found → silent 204
		{"unknown", http.StatusBadRequest},
		{"view", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			recorder := &eventRecorderStub{}
			h := &PromotionHandler{
				eventRepo: recorder,
				log:       noopLogger(),
				db:        &promotionEventTestDB{withTxErr: errors.New("instance not found")},
			}
			r := buildEventRouter(h, uuid.New())
			w := serveEvent(t, r, map[string]string{
				"promotion_instance_id": uuid.New().String(),
				"event_type":            tc.eventType,
				"surface":               "feed",
			})
			assert.Equal(t, tc.wantCode, w.Code, "event_type=%s", tc.eventType)
		})
	}
}

// TestRecordEvent_SurfaceValues — all valid surfaces pass validation.
func TestRecordEvent_SurfaceValues(t *testing.T) {
	for _, s := range []string{"feed", "search", "explore"} {
		t.Run(s, func(t *testing.T) {
			recorder := &eventRecorderStub{}
			h := &PromotionHandler{
				eventRepo: recorder,
				log:       noopLogger(),
				db:        &promotionEventTestDB{withTxErr: errors.New("instance not found")},
			}
			r := buildEventRouter(h, uuid.New())
			w := serveEvent(t, r, map[string]string{
				"promotion_instance_id": uuid.New().String(),
				"event_type":            "click",
				"surface":               s,
			})
			assert.Equal(t, http.StatusNoContent, w.Code, "surface=%s", s)
		})
	}
}

// TestRecordEvent_MissingRequiredFields — 400 when any required field is absent.
func TestRecordEvent_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		body map[string]string
	}{
		{"missing_instance_id", map[string]string{"event_type": "click", "surface": "feed"}},
		{"missing_event_type", map[string]string{"promotion_instance_id": uuid.New().String(), "surface": "feed"}},
		{"missing_surface", map[string]string{"promotion_instance_id": uuid.New().String(), "event_type": "click"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &eventRecorderStub{}
			h := &PromotionHandler{eventRepo: recorder, db: &promotionEventTestDB{}}
			r := buildEventRouter(h, uuid.New())
			w := serveEvent(t, r, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Empty(t, recorder.recorded)
		})
	}
}
