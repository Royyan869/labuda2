// Tests for the admin emergency auction cancel/override handler (PASS_5B).
//
// These use fakes for gatewayRefundInitiator-style dependencies
// (adminAuctionCanceller, db.Transactor, audit.AdminAuditLogger) so no real
// database or gateway is touched — mirroring the P5-01 admin refund handler
// test pattern.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/audit"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeAdminAuctionCanceller returns a canned AdminCancel result.
type fakeAdminAuctionCanceller struct {
	auction        *entity.Auction
	statusBefore   entity.Status
	err            error
	receivedReason string
}

func (f *fakeAdminAuctionCanceller) AdminCancel(_ context.Context, _ db.Tx, input auctionApp.AdminCancelInput) (*entity.Auction, entity.Status, error) {
	f.receivedReason = input.Reason
	return f.auction, f.statusBefore, f.err
}

// fakeAuctionTransactor runs fn against a nil Tx (the fake canceller above
// never touches it).
type fakeAuctionTransactor struct{}

func (f *fakeAuctionTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(nil)
}

type auctionAuditCall struct {
	actorID    uuid.UUID
	actionType string
	targetType string
	targetID   uuid.UUID
	metadata   map[string]interface{}
}

type fakeAuctionAuditLogger struct {
	calls []auctionAuditCall
}

func (f *fakeAuctionAuditLogger) Log(_ context.Context, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	f.calls = append(f.calls, auctionAuditCall{actorID, actionType, targetType, targetID, metadata})
	return nil
}

func (f *fakeAuctionAuditLogger) LogSafe(_ context.Context, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
	f.calls = append(f.calls, auctionAuditCall{actorID, actionType, targetType, targetID, metadata})
}

func (f *fakeAuctionAuditLogger) LogTx(_ context.Context, _ db.Tx, actorID uuid.UUID, actionType, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	f.calls = append(f.calls, auctionAuditCall{actorID, actionType, targetType, targetID, metadata})
	return nil
}

func withAuthenticatedAuctionAdmin(adminID uuid.UUID, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", adminID)
		next(c)
	}
}

func mustAuctionJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestAdminAuctionHandler_MissingReason_Returns400(t *testing.T) {
	h := &AdminAuctionHandler{
		auctionService:   &fakeAdminAuctionCanceller{},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: &fakeAuctionAuditLogger{},
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(uuid.New(), h.CancelAuction))

	cases := []map[string]any{
		{"reason": ""},
		{"reason": "   "},
		{},
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost,
			"/admin/auctions/"+uuid.New().String()+"/cancel",
			bytes.NewReader(mustAuctionJSON(t, body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestAdminAuctionHandler_BadAuctionID_Returns400(t *testing.T) {
	h := &AdminAuctionHandler{
		auctionService:   &fakeAdminAuctionCanceller{},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: &fakeAuctionAuditLogger{},
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(uuid.New(), h.CancelAuction))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/not-a-uuid/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "test"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid auction id")
}

func TestAdminAuctionHandler_MissingActor_Returns401(t *testing.T) {
	h := &AdminAuctionHandler{
		auctionService:   &fakeAdminAuctionCanceller{},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: &fakeAuctionAuditLogger{},
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel", h.CancelAuction) // no auth injected

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/"+uuid.New().String()+"/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "test"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminAuctionHandler_Success_WritesAuditLogAndReturns200(t *testing.T) {
	auctionID := uuid.New()
	sellerID := uuid.New()
	adminID := uuid.New()

	auditLogger := &fakeAuctionAuditLogger{}
	h := &AdminAuctionHandler{
		auctionService: &fakeAdminAuctionCanceller{
			auction: &entity.Auction{
				ID:       auctionID,
				SellerID: sellerID,
				Status:   entity.StatusCancelled,
			},
			statusBefore: entity.StatusActive,
		},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(adminID, h.CancelAuction))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/"+auctionID.String()+"/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "seller unreachable, trust & safety stop"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	require.Len(t, auditLogger.calls, 1, "successful admin auction cancel must write exactly one admin audit log entry")
	call := auditLogger.calls[0]
	assert.Equal(t, adminID, call.actorID)
	assert.Equal(t, audit.ActionAuctionAdminCancelled, call.actionType)
	assert.Equal(t, audit.TargetTypeAuction, call.targetType)
	assert.Equal(t, auctionID, call.targetID)
	assert.Equal(t, "seller unreachable, trust & safety stop", call.metadata["reason"])
	assert.Equal(t, "active", call.metadata["status_before"])
	assert.Equal(t, "cancelled", call.metadata["status_after"])
	assert.Equal(t, sellerID.String(), call.metadata["seller_id"])

	var respBody map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &respBody))
}

func TestAdminAuctionHandler_ReasonRequiredFromService_Returns400_NoAuditLog(t *testing.T) {
	auditLogger := &fakeAuctionAuditLogger{}
	h := &AdminAuctionHandler{
		auctionService: &fakeAdminAuctionCanceller{
			err: auctionApp.ErrAuctionCancelReasonRequired,
		},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(uuid.New(), h.CancelAuction))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/"+uuid.New().String()+"/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "x"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, auditLogger.calls, "no audit log entry must be written on failure")
}

func TestAdminAuctionHandler_Conflict_Returns409_NoAuditLog(t *testing.T) {
	auctionID := uuid.New()
	auditLogger := &fakeAuctionAuditLogger{}
	h := &AdminAuctionHandler{
		auctionService: &fakeAdminAuctionCanceller{
			err: &auctionApp.ErrAuctionCancelConflict{
				AuctionID:     auctionID,
				CurrentStatus: entity.StatusEnded,
				Reason:        "auction is already in a terminal state",
			},
		},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(uuid.New(), h.CancelAuction))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/"+auctionID.String()+"/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "seller unreachable"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "AUCTION_CANCEL_CONFLICT")
	assert.Empty(t, auditLogger.calls, "no audit log entry must be written on conflict — nothing was mutated")
}

func TestAdminAuctionHandler_NotFound_Returns404(t *testing.T) {
	auditLogger := &fakeAuctionAuditLogger{}
	h := &AdminAuctionHandler{
		auctionService: &fakeAdminAuctionCanceller{
			err: assertAuctionNotFoundErr{},
		},
		database:         &fakeAuctionTransactor{},
		adminAuditLogger: auditLogger,
		log:              zap.NewNop(),
	}

	router := gin.New()
	router.POST("/admin/auctions/:id/cancel",
		withAuthenticatedAuctionAdmin(uuid.New(), h.CancelAuction))

	req := httptest.NewRequest(http.MethodPost,
		"/admin/auctions/"+uuid.New().String()+"/cancel",
		bytes.NewReader(mustAuctionJSON(t, map[string]any{"reason": "seller unreachable"})))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Empty(t, auditLogger.calls)
}

// assertAuctionNotFoundErr mimics the plain-string "auction not found: <id>"
// error shape auctionRepo.GetForUpdate returns (no typed not-found error
// exists in this domain today — matching its existing convention).
type assertAuctionNotFoundErr struct{}

func (assertAuctionNotFoundErr) Error() string { return "auction not found: some-id" }
