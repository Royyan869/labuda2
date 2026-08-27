package serverboot

import (
	"errors"
	"testing"
	"time"
)

func baseInput(now, expired time.Time) SnapBuilderInput {
	return SnapBuilderInput{
		MidtransOrderID: "LAB-abc123",
		GrossAmount:     103000, // Rp 103,000 (Rupiah integer, PASS_18H)
		ExpiredAt:       expired,
		OrderNumber:      "ORD-20260501-AB12CD34",
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
// as-is, with NO /100 (or any other) scaling in either direction. An order
// total of Rp103,000 must produce a Snap gross_amount of exactly 103000 —
// not 1030.
func TestBuildSnapRequest_GrossAmountSentAsRupiahIntegerNoConversion(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.GrossAmount = 103000 // Rp 103,000

	req, err := buildSnapRequest(in)
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

	req, err := buildSnapRequest(in)
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

	req, err := buildSnapRequest(in)
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

func TestBuildSnapRequest_ExpiryDurationFuture15Min(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))

	req, err := buildSnapRequest(in)
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
	// Must parse with the documented Midtrans format.
	if _, perr := time.Parse(snapTimeFormat, req.Expiry.StartTime); perr != nil {
		t.Errorf("StartTime %q does not match Snap format: %v", req.Expiry.StartTime, perr)
	}
}

func TestBuildSnapRequest_ExpiredReturnsError(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(-1*time.Minute))

	_, err := buildSnapRequest(in)
	if !errors.Is(err, ErrSnapPaymentExpired) {
		t.Errorf("want ErrSnapPaymentExpired, got %v", err)
	}
}

func TestBuildSnapRequest_ExactlyAtExpiryReturnsError(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now)

	_, err := buildSnapRequest(in)
	if !errors.Is(err, ErrSnapPaymentExpired) {
		t.Errorf("want ErrSnapPaymentExpired at boundary, got %v", err)
	}
}

func TestBuildSnapRequest_ShortExpiryClampsToMinOne(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(10*time.Second)) // <1 minute remaining

	req, err := buildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry.Duration != 1 {
		t.Errorf("short expiry: want clamp to 1, got %d", req.Expiry.Duration)
	}
}

func TestBuildSnapRequest_LongExpiryClampsToMax1440(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(72*time.Hour)) // 4320 min

	req, err := buildSnapRequest(in)
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
	in.Buyer = SnapBuyerInfo{} // all empty

	req, err := buildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.CustomerDetails != nil {
		t.Errorf("empty buyer must omit CustomerDetails, got %+v", req.CustomerDetails)
	}
	if req.TransactionDetails.OrderID == "" || req.TransactionDetails.GrossAmount <= 0 {
		t.Errorf("request must remain valid with empty buyer")
	}
}

func TestBuildSnapRequest_PartialBuyerKeepsOnlyProvided(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(15*time.Minute))
	in.Buyer = SnapBuyerInfo{Email: "only@example.com"}

	req, err := buildSnapRequest(in)
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

	req, err := buildSnapRequest(in)
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

	req, err := buildSnapRequest(in)
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

	req, err := buildSnapRequest(in)
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

	_, err := buildSnapRequest(in)
	if !errors.Is(err, ErrSnapMissingOrderID) {
		t.Errorf("want ErrSnapMissingOrderID, got %v", err)
	}
}

func TestBuildSnapRequest_RejectZeroOrNegativeAmount(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for _, c := range []int64{0, -1, -100} {
		in := baseInput(now, now.Add(15*time.Minute))
		in.GrossAmount = c
		if _, err := buildSnapRequest(in); !errors.Is(err, ErrSnapInvalidAmount) {
			t.Errorf("amount %d: want ErrSnapInvalidAmount, got %v", c, err)
		}
	}
}

func TestBuildSnapRequest_RejectZeroExpiredAt(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, time.Time{}) // zero
	if _, err := buildSnapRequest(in); !errors.Is(err, ErrSnapZeroExpiredAt) {
		t.Errorf("want ErrSnapZeroExpiredAt, got %v", err)
	}
}

func TestBuildSnapRequest_DurationCeilingFor90Seconds(t *testing.T) {
	// 90s remaining → ceil(1.5) = 2 minutes.
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	in := baseInput(now, now.Add(90*time.Second))

	req, err := buildSnapRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Expiry.Duration != 2 {
		t.Errorf("90s remaining: want duration 2 (ceil), got %d", req.Expiry.Duration)
	}
}


