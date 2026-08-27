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
	// No user_id set → MustGetUserIDFromContext returns false

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
	// No SetDeliveryQuerier called → deliveryQuerier is nil

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

// ============================================================================
// BNR ADMIN RESET ENDPOINT TESTS
// ============================================================================

// mockBNRResetter implements BNRResetter for testing.
type mockBNRResetter struct {
	resetAllCount int64
	resetAllErr   error
	resetOneOK    bool
	resetOneErr   error

	// Capture args
	lastBuyerID  uuid.UUID
	lastStrikeID uuid.UUID
}

func (m *mockBNRResetter) ResetAllForBuyer(_ context.Context, buyerID, _ uuid.UUID) (int64, error) {
	m.lastBuyerID = buyerID
	return m.resetAllCount, m.resetAllErr
}

func (m *mockBNRResetter) ResetStrike(_ context.Context, strikeID, _ uuid.UUID) (bool, error) {
	m.lastStrikeID = strikeID
	return m.resetOneOK, m.resetOneErr
}

// TestBNRReset_AllForBuyer_Success verifies admin can reset all active
// strikes for a buyer and receives the count.
func TestBNRReset_AllForBuyer_Success(t *testing.T) {
	resetter := &mockBNRResetter{resetAllCount: 3}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	buyerID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+buyerID.String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: buyerID.String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["strikes_reset"] != float64(3) {
		t.Errorf("strikes_reset=%v want 3", data["strikes_reset"])
	}
	if data["buyer_id"] != buyerID.String() {
		t.Errorf("buyer_id=%v want %s", data["buyer_id"], buyerID)
	}

	// Verify correct buyer was passed
	if resetter.lastBuyerID != buyerID {
		t.Errorf("resetter.lastBuyerID=%v want %v", resetter.lastBuyerID, buyerID)
	}
}

// TestBNRReset_AllForBuyer_DecayedNotAffected verifies that when only
// decayed strikes exist (0 active), the response reflects 0 reset.
func TestBNRReset_AllForBuyer_DecayedNotAffected(t *testing.T) {
	resetter := &mockBNRResetter{resetAllCount: 0}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	buyerID := uuid.New()
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+buyerID.String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: buyerID.String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["strikes_reset"] != float64(0) {
		t.Errorf("strikes_reset=%v want 0 (decayed strikes excluded)", data["strikes_reset"])
	}
}

// TestBNRReset_AllForBuyer_AlreadyResetNotCounted verifies idempotency:
// resetting when all strikes are already reset returns 0.
func TestBNRReset_AllForBuyer_AlreadyResetNotCounted(t *testing.T) {
	resetter := &mockBNRResetter{resetAllCount: 0}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	buyerID := uuid.New()
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+buyerID.String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: buyerID.String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["data"].(map[string]interface{})["strikes_reset"] != float64(0) {
		t.Error("already-reset strikes should not be counted")
	}
}

// TestBNRReset_AllForBuyer_NonAdminBlocked verifies that without user_id
// in context, the handler returns 401.
func TestBNRReset_AllForBuyer_NonAdminBlocked(t *testing.T) {
	resetter := &mockBNRResetter{resetAllCount: 1}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+uuid.New().String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	// No setAdminContext → MustGetUserIDFromContext returns false

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

// TestBNRReset_SingleStrike_Success verifies single strike reset returns 200.
func TestBNRReset_SingleStrike_Success(t *testing.T) {
	resetter := &mockBNRResetter{resetOneOK: true}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	strikeID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/bnr-strikes/"+strikeID.String()+"/reset", nil)
	c.Params = gin.Params{{Key: "strike_id", Value: strikeID.String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrike(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["reset"] != true {
		t.Error("expected reset=true")
	}
	if resetter.lastStrikeID != strikeID {
		t.Errorf("resetter.lastStrikeID=%v want %v", resetter.lastStrikeID, strikeID)
	}
}

// TestBNRReset_SingleStrike_NotFound verifies 404 when strike doesn't exist
// or is already reset/decayed.
func TestBNRReset_SingleStrike_NotFound(t *testing.T) {
	resetter := &mockBNRResetter{resetOneOK: false}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/bnr-strikes/"+uuid.New().String()+"/reset", nil)
	c.Params = gin.Params{{Key: "strike_id", Value: uuid.New().String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrike(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

// TestBNRReset_NilResetter verifies 500 when resetter is not wired.
func TestBNRReset_NilResetter(t *testing.T) {
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	// No SetBNRResetter → nil

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+uuid.New().String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}

// TestBNRReset_DBError verifies 500 on DB failure.
func TestBNRReset_DBError(t *testing.T) {
	resetter := &mockBNRResetter{resetAllErr: errors.New("db down")}
	handler := NewAdminHandler(nil, &mockAuditLogger{}, nil, nil)
	handler.SetBNRResetter(resetter)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	buyerID := uuid.New()
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users/"+buyerID.String()+"/bnr-strikes/reset", nil)
	c.Params = gin.Params{{Key: "id", Value: buyerID.String()}}
	setAdminContext(c, uuid.New())

	handler.ResetBNRStrikesForUser(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}


