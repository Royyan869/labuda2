package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	notifservice "github.com/labuda/backend/internal/interaction/notification/service"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// O4: FAILED DELIVERY ENDPOINT TESTS
// ============================================================================

// mockDeliveryQuerier implements FailedDeliveryQuerier for testing.
type mockDeliveryQuerier struct {
	result *notifservice.FailedDeliveryResult
	err    error
}

func (m *mockDeliveryQuerier) GetFailedDeliveriesPaginated(_ context.Context, _, _ int, _ time.Time) (*notifservice.FailedDeliveryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockAuditLogger is a no-op audit logger for testing.
type mockAuditLogger struct{}

func (m *mockAuditLogger) Log(_ context.Context, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}
func (m *mockAuditLogger) LogSafe(_ context.Context, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) {
}
func (m *mockAuditLogger) LogTx(_ context.Context, _ dbpkg.Tx, _ uuid.UUID, _ string, _ string, _ uuid.UUID, _ map[string]interface{}) error {
	return nil
}

// setAdminContext sets the user_id in gin context to simulate authenticated admin.
func setAdminContext(c *gin.Context, userID uuid.UUID) {
	c.Set("user_id", userID)
}

func TestO4_GetFailedDeliveries_ReturnsEntries(t *testing.T) {
	notifID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	querier := &mockDeliveryQuerier{
		result: &notifservice.FailedDeliveryResult{
			Entries: []notifservice.FailedDeliveryEntry{
				{
					ID:             uuid.New(),
					NotificationID: notifID,
					UserID:         userID,
					Channel:        "push",
					Status:         "failed",
					Reason:         "FCM token expired",
					Metadata:       map[string]interface{}{"type": "order.created"},
					CreatedAt:      now,
				},
			},
			Total: 1,
		},
	}

	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetDeliveryQuerier(querier)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries", nil)
	setAdminContext(c, uuid.New())

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data not found or wrong type")
	}

	deliveries, ok := data["deliveries"].([]interface{})
	if !ok {
		t.Fatalf("deliveries not found or wrong type")
	}
	if len(deliveries) != 1 {
		t.Fatalf("len(deliveries)=%d want 1", len(deliveries))
	}

	entry := deliveries[0].(map[string]interface{})
	if entry["channel"] != "push" {
		t.Errorf("channel=%v want push", entry["channel"])
	}
	if entry["status"] != "failed" {
		t.Errorf("status=%v want failed", entry["status"])
	}
	if entry["reason"] != "FCM token expired" {
		t.Errorf("reason=%v want 'FCM token expired'", entry["reason"])
	}
	if entry["notification_id"] != notifID.String() {
		t.Errorf("notification_id=%v want %s", entry["notification_id"], notifID)
	}

	// Verify pagination meta
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("meta not found")
	}
	if meta["total"] != float64(1) {
		t.Errorf("meta.total=%v want 1", meta["total"])
	}
}

func TestO4_GetFailedDeliveries_NonAdminRejected(t *testing.T) {
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetDeliveryQuerier(&mockDeliveryQuerier{
		result: &notifservice.FailedDeliveryResult{Total: 0},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries", nil)
	// No user_id set â†’ MustGetUserIDFromContext returns false

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestO4_GetFailedDeliveries_EmptyReturnsEmptyArray(t *testing.T) {
	querier := &mockDeliveryQuerier{
		result: &notifservice.FailedDeliveryResult{
			Entries: nil,
			Total:   0,
		},
	}

	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetDeliveryQuerier(querier)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries", nil)
	setAdminContext(c, uuid.New())

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	deliveries := data["deliveries"].([]interface{})
	if len(deliveries) != 0 {
		t.Fatalf("len(deliveries)=%d want 0", len(deliveries))
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["total"] != float64(0) {
		t.Errorf("meta.total=%v want 0", meta["total"])
	}
}

func TestO4_GetFailedDeliveries_PaginationParams(t *testing.T) {
	var capturedPage, capturedPageSize int
	querier := &mockDeliveryQuerier{
		result: &notifservice.FailedDeliveryResult{Total: 0},
	}
	// Wrap with a capturing querier
	capturingQuerier := &capturingDeliveryQuerier{
		inner:  querier,
		onCall: func(page, pageSize int) { capturedPage = page; capturedPageSize = pageSize },
	}

	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetDeliveryQuerier(capturingQuerier)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries?page=3&page_size=50", nil)
	setAdminContext(c, uuid.New())

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}
	if capturedPage != 3 {
		t.Errorf("page=%d want 3", capturedPage)
	}
	if capturedPageSize != 50 {
		t.Errorf("pageSize=%d want 50", capturedPageSize)
	}
}

func TestO4_GetFailedDeliveries_QuerierError(t *testing.T) {
	querier := &mockDeliveryQuerier{
		err: errors.New("database connection lost"),
	}

	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetDeliveryQuerier(querier)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries", nil)
	setAdminContext(c, uuid.New())

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}

func TestO4_GetFailedDeliveries_NilQuerier(t *testing.T) {
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	// No SetDeliveryQuerier called â†’ deliveryQuerier is nil

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/notifications/failed-deliveries", nil)
	setAdminContext(c, uuid.New())

	handler.GetFailedDeliveries(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}

// capturingDeliveryQuerier wraps a querier and captures pagination params.
type capturingDeliveryQuerier struct {
	inner  FailedDeliveryQuerier
	onCall func(page, pageSize int)
}

func (c *capturingDeliveryQuerier) GetFailedDeliveriesPaginated(ctx context.Context, page, pageSize int, since time.Time) (*notifservice.FailedDeliveryResult, error) {
	if c.onCall != nil {
		c.onCall(page, pageSize)
	}
	return c.inner.GetFailedDeliveriesPaginated(ctx, page, pageSize, since)
}



