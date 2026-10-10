package application

import (
	"errors"
	"testing"
	"time"

	"github.com/hishumi/backend/pkg/midtrans"
)

func baseInput(now, expired time.Time) SnapSessionInput {
	return SnapSessionInput{
		MidtransOrderID: "LAB-abc123",
		GrossAmount:     103000, // Rp 103,000 (Rupiah integer, PASS_18H)
		ExpiredAt:       expired,
		OrderNumber:     "ORD-20260501-AB12CD34",
		Buyer: SnapBuyerInfo{
			FirstName: "Buyer",
			LastName:  "One",
			Email:     "buyer@example.com",
			Phone:     "+628123456789",
		},
		FrontendURL: "https://app.example.com",
		Now:         now,
	}
}

// TestBuildSnapRequest_GrossAmountSentAsRupiahIntegerNoConversion locks the
// PASS_18H money-unit fix: GrossAmount is a Rupiah integer sent to Midtrans
// as-is, with NO /100 (or any other) scaling in either direction.
func TestBuildSnapRequest_GrossAmountSentAsRupiahIntegerNoConversion(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.GrossAmount = 103000

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.TransactionDetails.GrossAmount != 103000 {
		t.Errorf("gross_amount: want 103000 (no conversion), got %v", req.TransactionDetails.GrossAmount)
	}
}

func TestBuildSnapRequest_OrderIDUsesMidtransOrderID(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.MidtransOrderID = "LAB-unique-xyz"

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.TransactionDetails.OrderID != "LAB-unique-xyz" {
		t.Errorf("OrderID: want %q, got %q", "LAB-unique-xyz", req.TransactionDetails.OrderID)
	}
	if len(req.ItemDetails) != 1 || req.ItemDetails[0].ID != "LAB-unique-xyz" {
		t.Errorf("item ID should equal MidtransOrderID, got %+v", req.ItemDetails)
	}
}

func TestBuildSnapRequest_ItemTotalEqualsGross(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.ItemDetails) != 1 {
		t.Fatalf("want exactly 1 synthetic item, got %d", len(req.ItemDetails))
	}
	got := req.ItemDetails[0].Price * float64(req.ItemDetails[0].Quantity)
	if got != req.TransactionDetails.GrossAmount {
		t.Errorf("item total %v != gross %v", got, req.TransactionDetails.GrossAmount)
	}
}

// TestBuildSnapRequest_ItemNameOverride locks the canonical item-name override
// used by non-order purchases (seller subscription) so the Snap page shows the
// purchased product name from the shared builder.
func TestBuildSnapRequest_ItemNameOverride(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.ItemName = "Langganan Penjual 1 Tahun"

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ItemDetails[0].Name != "Langganan Penjual 1 Tahun" {
		t.Errorf("item name override not honored, got %q", req.ItemDetails[0].Name)
	}
}

func TestBuildSnapRequest_ExpiryDurationFuture15Min(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry == nil || req.Expiry.Unit != "minute" {
		t.Fatalf("expiry malformed: %+v", req.Expiry)
	}
	if req.Expiry.Duration != 15 {
		t.Errorf("duration: want 15, got %d", req.Expiry.Duration)
	}
	if req.Expiry.StartTime == "" {
		t.Errorf("StartTime must be non-empty")
	}
	if _, perr := time.Parse(snapTimeFormat, req.Expiry.StartTime); perr != nil {
		t.Errorf("StartTime %q does not match Snap format: %v", req.Expiry.StartTime, perr)
	}
}

func TestBuildSnapRequest_ExpiredReturnsError(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(-1*time.Minute))

	_, err := BuildSnapRequest(in)
	if !errors.Is(err, ErrSnapPaymentExpired) {
		t.Errorf("want ErrSnapPaymentExpired, got %v", err)
	}
}

func TestBuildSnapRequest_ExactlyAtExpiryReturnsError(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now)

	_, err := BuildSnapRequest(in)
	if !errors.Is(err, ErrSnapPaymentExpired) {
		t.Errorf("want ErrSnapPaymentExpired at boundary, got %v", err)
	}
}

func TestBuildSnapRequest_ShortExpiryClampsToMinOne(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(10*time.Second))

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry.Duration != 1 {
		t.Errorf("short expiry: want clamp to 1, got %d", req.Expiry.Duration)
	}
}

func TestBuildSnapRequest_LongExpiryClampsToMax1440(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(72*time.Hour))

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry.Duration != 1440 {
		t.Errorf("long expiry: want clamp to 1440, got %d", req.Expiry.Duration)
	}
}

func TestBuildSnapRequest_EmptyBuyerOmitsCustomerDetails(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.Buyer = SnapBuyerInfo{}

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.CustomerDetails != nil {
		t.Errorf("empty buyer must omit CustomerDetails, got %+v", req.CustomerDetails)
	}
}

func TestBuildSnapRequest_PartialBuyerKeepsOnlyProvided(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.Buyer = SnapBuyerInfo{Email: "only@example.com"}

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.CustomerDetails == nil || req.CustomerDetails.Email != "only@example.com" {
		t.Errorf("partial buyer email must be carried, got %+v", req.CustomerDetails)
	}
	if req.CustomerDetails.FirstName != "" || req.CustomerDetails.Phone != "" {
		t.Errorf("unset buyer fields must remain empty, got %+v", req.CustomerDetails)
	}
}

func TestBuildSnapRequest_FinishCallbackBuiltFromFrontendURL(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.FrontendURL = "https://app.example.com"
	in.MidtransOrderID = "LAB-fin-1"

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://app.example.com/payment/finish?order_id=LAB-fin-1"
	if req.Callbacks == nil || req.Callbacks.Finish != want {
		t.Errorf("finish callback: want %q, got %+v", want, req.Callbacks)
	}
}

func TestBuildSnapRequest_EmptyFrontendURLOmitsCallbacks(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.FrontendURL = ""

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Callbacks != nil {
		t.Errorf("empty FrontendURL must omit callbacks, got %+v", req.Callbacks)
	}
}

func TestBuildSnapRequest_EmptyOrderNumberFallsBackToBareOrder(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.OrderNumber = ""

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ItemDetails[0].Name != "Order" {
		t.Errorf("empty OrderNumber: want item name %q, got %q", "Order", req.ItemDetails[0].Name)
	}
}

func TestBuildSnapRequest_RejectMissingOrderID(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.MidtransOrderID = ""

	_, err := BuildSnapRequest(in)
	if !errors.Is(err, ErrSnapMissingOrderID) {
		t.Errorf("want ErrSnapMissingOrderID, got %v", err)
	}
}

func TestBuildSnapRequest_RejectZeroOrNegativeAmount(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for _, c := range []int64{0, -1, -100} {
		in := baseInput(now, now.Add(15*time.Minute))
		in.GrossAmount = c
		if _, err := BuildSnapRequest(in); !errors.Is(err, ErrSnapInvalidAmount) {
			t.Errorf("amount %d: want ErrSnapInvalidAmount, got %v", c, err)
		}
	}
}

func TestBuildSnapRequest_RejectZeroExpiredAt(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, time.Time{})
	if _, err := BuildSnapRequest(in); !errors.Is(err, ErrSnapZeroExpiredAt) {
		t.Errorf("want ErrSnapZeroExpiredAt, got %v", err)
	}
}

func TestBuildSnapRequest_DurationCeilingFor90Seconds(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(90*time.Second))

	req, err := BuildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry.Duration != 2 {
		t.Errorf("90s remaining: want duration 2 (ceil), got %d", req.Expiry.Duration)
	}
}

// ============================================================================
// SnapService authority
// ============================================================================

type fakeSnapGateway struct {
	production bool
	calls      int
	req        *midtrans.SnapRequest
	resp       *midtrans.SnapResponse
	err        error
}

func (g *fakeSnapGateway) IsProduction() bool { return g.production }

func (g *fakeSnapGateway) CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error) {
	g.calls++
	g.req = req
	if g.err != nil {
		return nil, g.err
	}
	if g.resp != nil {
		return g.resp, nil
	}
	return &midtrans.SnapResponse{RedirectURL: "https://midtrans.example/redirect"}, nil
}

func TestSnapService_RefusesProduction(t *testing.T) {
	gw := &fakeSnapGateway{production: true}
	svc := NewSnapService(gw, "https://app.example.com")
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	_, err := svc.CreateSession(SnapSessionInput{
		MidtransOrderID: "LAB-1",
		GrossAmount:     1000,
		ExpiredAt:       now.Add(time.Hour),
		Now:             now,
	})
	if !errors.Is(err, ErrSnapProductionForbidden) {
		t.Fatalf("want ErrSnapProductionForbidden, got %v", err)
	}
	if gw.calls != 0 {
		t.Fatalf("production mode must not call the gateway, calls=%d", gw.calls)
	}
}

func TestSnapService_RefusesNilGateway(t *testing.T) {
	var svc *SnapService = NewSnapService(nil, "")
	_, err := svc.CreateSession(SnapSessionInput{})
	if !errors.Is(err, ErrSnapGatewayNotConfigured) {
		t.Fatalf("want ErrSnapGatewayNotConfigured, got %v", err)
	}
}

func TestSnapService_InjectsCanonicalFrontendURLAndChannels(t *testing.T) {
	gw := &fakeSnapGateway{}
	svc := NewSnapService(gw, "https://app.example.com")
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	url, err := svc.CreateSession(SnapSessionInput{
		MidtransOrderID: "LAB-2",
		GrossAmount:     150000,
		ExpiredAt:       now.Add(time.Hour),
		ItemName:        "Langganan Penjual 1 Tahun",
		EnabledPayments: []string{"bca_va"},
		Now:             now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://midtrans.example/redirect" {
		t.Fatalf("unexpected url: %s", url)
	}
	if gw.calls != 1 {
		t.Fatalf("want 1 gateway call, got %d", gw.calls)
	}
	if gw.req.Callbacks == nil || gw.req.Callbacks.Finish != "https://app.example.com/payment/finish?order_id=LAB-2" {
		t.Fatalf("canonical finish callback not injected: %+v", gw.req.Callbacks)
	}
	if len(gw.req.EnabledPayments) != 1 || gw.req.EnabledPayments[0] != "bca_va" {
		t.Fatalf("channel restriction not carried: %+v", gw.req.EnabledPayments)
	}
}

func TestSnapService_RefusesEmptyRedirectURL(t *testing.T) {
	gw := &fakeSnapGateway{resp: &midtrans.SnapResponse{}}
	svc := NewSnapService(gw, "")
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	_, err := svc.CreateSession(SnapSessionInput{
		MidtransOrderID: "LAB-3",
		GrossAmount:     1000,
		ExpiredAt:       now.Add(time.Hour),
		Now:             now,
	})
	if !errors.Is(err, ErrSnapEmptyRedirectURL) {
		t.Fatalf("want ErrSnapEmptyRedirectURL, got %v", err)
	}
}
