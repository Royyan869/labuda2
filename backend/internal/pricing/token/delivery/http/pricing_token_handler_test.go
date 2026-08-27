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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/platform/response"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

type previewServiceStub struct {
	t *testing.T

	fixedReq       *pricingtokenapp.GenerateForForSaleRequest
	auctionReq     *pricingtokenapp.GenerateForAuctionRequest
	negotiationReq *pricingtokenapp.GenerateForNegotiationRequest

	fixedResp       *pricingtokenapp.GenerateForForSaleResponse
	auctionResp     *pricingtokenapp.GenerateForAuctionResponse
	negotiationResp *pricingtokenapp.GenerateForNegotiationResponse
}

func (s *previewServiceStub) GenerateForForSale(_ context.Context, _ db.Tx, req *pricingtokenapp.GenerateForForSaleRequest) (*pricingtokenapp.GenerateForForSaleResponse, error) {
	s.fixedReq = req
	if s.fixedResp == nil {
		s.t.Fatalf("unexpected GenerateForForSale call")
	}
	return s.fixedResp, nil
}

func (s *previewServiceStub) GenerateForAuction(_ context.Context, _ db.Tx, req *pricingtokenapp.GenerateForAuctionRequest) (*pricingtokenapp.GenerateForAuctionResponse, error) {
	s.auctionReq = req
	if s.auctionResp == nil {
		s.t.Fatalf("unexpected GenerateForAuction call")
	}
	return s.auctionResp, nil
}

func (s *previewServiceStub) GenerateForNegotiation(_ context.Context, _ db.Tx, req *pricingtokenapp.GenerateForNegotiationRequest) (*pricingtokenapp.GenerateForNegotiationResponse, error) {
	s.negotiationReq = req
	if s.negotiationResp == nil {
		s.t.Fatalf("unexpected GenerateForNegotiation call")
	}
	return s.negotiationResp, nil
}

func (s *previewServiceStub) ValidateForOrder(_ context.Context, _ db.Tx, _ *pricingtokenapp.ValidateForOrderRequest) (*pricingtokenentity.PricingToken, error) {
	s.t.Fatalf("unexpected ValidateForOrder call")
	return nil, nil
}

func (s *previewServiceStub) GetSnapshot(_ context.Context, _ db.Tx, _ uuid.UUID) (*pricingtokenentity.PricingToken, error) {
	s.t.Fatalf("unexpected GetSnapshot call")
	return nil, nil
}

type fakeTransactor struct{}

func (fakeTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(fakeTx{})
}

type fakeTx struct{}

func (fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (fakeTx) Commit(_ context.Context) error {
	return nil
}

func (fakeTx) Rollback(_ context.Context) error {
	return nil
}

type previewResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Token                 string          `json:"token"`
		ExpiresAt             string          `json:"expires_at"`
		PricingSnapshot       json.RawMessage `json:"pricing_snapshot"`
		AuctionSettlementType string          `json:"auction_settlement_type,omitempty"`
	} `json:"data"`
	Error *response.ErrorInfo `json:"error"`
}

func TestPricingTokenHandler_GeneratePreview_RoutesForSaleDirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	productID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	forSaleID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shippingOptionID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	addressID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	userID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	service := &previewServiceStub{
		t: t,
		fixedResp: &pricingtokenapp.GenerateForForSaleResponse{
			Token:     uuid.MustParse("77777777-7777-7777-7777-777777777777"),
			ExpiresAt: "2026-06-22T12:00:00Z",
			PricingSnapshot: pricingtokenapp.PricingSnapshot{
				UnitPrice: money.New(12345),
			},
		},
	}

	handler := &PricingTokenHandler{
		tokenService: service,
		db:           fakeTransactor{},
		log:          zap.NewNop(),
	}

	resp := performGeneratePreviewRequest(t, handler, userID, GeneratePreviewRequest{
		ProductID:        productID,
		SourceType:       "for_sale",
		SourceID:         forSaleID,
		Quantity:         1,
		ShippingOptionID: &shippingOptionID,
		AddressID:        addressID,
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if service.fixedReq == nil {
		t.Fatal("expected fixed-price-sale branch to be called")
	}
	if service.auctionReq != nil {
		t.Fatal("auction branch must not be called")
	}
	if service.negotiationReq != nil {
		t.Fatal("negotiation branch must not be called")
	}
	if service.fixedReq.ProductID != productID {
		t.Fatalf("ProductID = %s, want %s", service.fixedReq.ProductID, productID)
	}
	if service.fixedReq.SourceType != "for_sale" {
		t.Fatalf("SourceType = %s, want for_sale", service.fixedReq.SourceType)
	}
	if service.fixedReq.SourceID != forSaleID {
		t.Fatalf("SourceID = %s, want %s", service.fixedReq.SourceID, forSaleID)
	}
	if service.fixedReq.ShippingOptionID == nil || *service.fixedReq.ShippingOptionID != shippingOptionID {
		t.Fatalf("ShippingOptionID = %v, want %s", service.fixedReq.ShippingOptionID, shippingOptionID)
	}
	if got := decodePreviewResponse(t, resp.Body.Bytes()); got.Data.Token != service.fixedResp.Token.String() {
		t.Fatalf("token = %s, want %s", got.Data.Token, service.fixedResp.Token)
	}
}

func TestPricingTokenHandler_GeneratePreview_RoutesAuction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	productID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	auctionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	shippingOptionID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	addressID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	userID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	service := &previewServiceStub{
		t: t,
		auctionResp: &pricingtokenapp.GenerateForAuctionResponse{
			Token:     uuid.MustParse("88888888-8888-8888-8888-888888888888"),
			ExpiresAt: "2026-06-22T12:00:00Z",
			PricingSnapshot: pricingtokenapp.PricingSnapshot{
				UnitPrice: money.New(54321),
			},
			AuctionSettlementType: string(orderentity.AuctionSettlementBuyNow),
		},
	}

	handler := &PricingTokenHandler{
		tokenService: service,
		db:           fakeTransactor{},
		log:          zap.NewNop(),
	}

	resp := performGeneratePreviewRequest(t, handler, userID, GeneratePreviewRequest{
		ProductID:        productID,
		SourceType:       "auction",
		SourceID:         auctionID,
		Quantity:         1,
		ShippingOptionID: &shippingOptionID,
		AddressID:        addressID,
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if service.auctionReq == nil {
		t.Fatal("expected auction branch to be called")
	}
	if service.fixedReq != nil {
		t.Fatal("fixed-price-sale branch must not be called")
	}
	if service.negotiationReq != nil {
		t.Fatal("negotiation branch must not be called")
	}
	if service.auctionReq.AuctionID != auctionID {
		t.Fatalf("AuctionID = %s, want %s", service.auctionReq.AuctionID, auctionID)
	}
	if service.auctionReq.ShippingOptionID != shippingOptionID {
		t.Fatalf("ShippingOptionID = %s, want %s", service.auctionReq.ShippingOptionID, shippingOptionID)
	}
	if got := decodePreviewResponse(t, resp.Body.Bytes()); got.Data.Token != service.auctionResp.Token.String() {
		t.Fatalf("token = %s, want %s", got.Data.Token, service.auctionResp.Token)
	}
	if got := decodePreviewResponse(t, resp.Body.Bytes()); got.Data.AuctionSettlementType != service.auctionResp.AuctionSettlementType {
		t.Fatalf("auction_settlement_type = %s, want %s", got.Data.AuctionSettlementType, service.auctionResp.AuctionSettlementType)
	}
}

func TestPricingTokenHandler_GeneratePreview_RoutesNegotiation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	productID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	forSaleID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	negotiationID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	shippingOptionID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	addressID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	userID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

	service := &previewServiceStub{
		t: t,
		negotiationResp: &pricingtokenapp.GenerateForNegotiationResponse{
			Token:     uuid.MustParse("99999999-9999-9999-9999-999999999999"),
			ExpiresAt: "2026-06-22T12:00:00Z",
			PricingSnapshot: pricingtokenapp.PricingSnapshot{
				UnitPrice: money.New(33333),
			},
		},
	}

	handler := &PricingTokenHandler{
		tokenService: service,
		db:           fakeTransactor{},
		log:          zap.NewNop(),
	}

	resp := performGeneratePreviewRequest(t, handler, userID, GeneratePreviewRequest{
		ProductID:        productID,
		SourceType:       "for_sale",
		SourceID:         forSaleID,
		NegotiationID:    &negotiationID,
		Quantity:         1,
		ShippingOptionID: &shippingOptionID,
		AddressID:        addressID,
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if service.negotiationReq == nil {
		t.Fatal("expected negotiation branch to be called")
	}
	if service.fixedReq != nil {
		t.Fatal("fixed-price-sale branch must not be called")
	}
	if service.auctionReq != nil {
		t.Fatal("auction branch must not be called")
	}
	if service.negotiationReq.UserID != userID {
		t.Fatalf("UserID = %s, want %s", service.negotiationReq.UserID, userID)
	}
	if service.negotiationReq.NegotiationID != negotiationID {
		t.Fatalf("NegotiationID = %s, want %s", service.negotiationReq.NegotiationID, negotiationID)
	}
	if service.negotiationReq.ShippingOptionID != shippingOptionID {
		t.Fatalf("ShippingOptionID = %s, want %s", service.negotiationReq.ShippingOptionID, shippingOptionID)
	}
	if service.negotiationReq.AddressID != addressID {
		t.Fatalf("AddressID = %s, want %s", service.negotiationReq.AddressID, addressID)
	}
	if productID == forSaleID || productID == negotiationID || forSaleID == negotiationID {
		t.Fatal("expected product, fixed-price-sale, and negotiation IDs to be distinct")
	}
	if got := decodePreviewResponse(t, resp.Body.Bytes()); got.Data.Token != service.negotiationResp.Token.String() {
		t.Fatalf("token = %s, want %s", got.Data.Token, service.negotiationResp.Token)
	}
}

func performGeneratePreviewRequest(t *testing.T, handler *PricingTokenHandler, userID uuid.UUID, req GeneratePreviewRequest) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/pricing/preview", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)

	handler.GeneratePreview(c)
	return rec
}

func decodePreviewResponse(t *testing.T, body []byte) previewResponse {
	t.Helper()

	var resp previewResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got error: %+v", resp.Error)
	}
	return resp
}
