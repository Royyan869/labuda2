package serverboot

import (
	"errors"
	"math"
	"time"

	"github.com/labuda/backend/pkg/midtrans"
)

// SnapBuyerInfo carries optional buyer identity fields for Snap CustomerDetails.
// All fields are optional; an entirely empty struct is valid and yields no CustomerDetails.
type SnapBuyerInfo struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// SnapBuilderInput is the pure input contract for buildSnapRequest.
// The caller is responsible for resolving payment/order/buyer/config into this struct.
// This keeps the builder free of repository, DB, and HTTP dependencies.
type SnapBuilderInput struct {
	// MidtransOrderID is the unique identifier sent to Midtrans (payment.MidtransOrderID).
	MidtransOrderID string

	// GrossAmount is the amount the buyer is charged, in Rupiah integer —
	// Labuda's canonical money unit (PASS_18H). There is no cents/sen
	// subunit anywhere in the system; this value is sent to Midtrans as-is,
	// with no scaling in either direction. Must be positive.
	GrossAmount int64

	// ExpiredAt is the absolute payment-window deadline (order.PaymentExpiresAt).
	ExpiredAt time.Time

	// OrderNumber is the human-readable order number; used for item naming. Optional.
	OrderNumber string

	// Buyer is optional; an empty struct is valid.
	Buyer SnapBuyerInfo

	// FrontendURL is the base URL used to build the Snap "finish" callback. Optional.
	FrontendURL string

	// Now is injected for deterministic expiry math in tests. Zero value falls back to time.Now().
	Now time.Time

	// EnabledPayments optionally restricts the Snap payment page to the given
	// Midtrans channel codes (e.g. ["bca_va", "bni_va"] for a bank-transfer
	// method, ["other_qris"] for QRIS). Empty/nil means all channels remain
	// enabled — Midtrans's default behavior.
	EnabledPayments []string
}

const (
	minExpiryMinutes = 1
	maxExpiryMinutes = 1440 // 24 hours, Midtrans Snap upper bound for "minute" unit
	// snapTimeFormat is the Midtrans Snap StartTime format: "yyyy-MM-dd HH:mm:ss Z".
	snapTimeFormat = "2006-01-02 15:04:05 -0700"
)

// Sentinel errors so callers and tests can match precisely.
var (
	ErrSnapMissingOrderID = errors.New("midtrans snap: midtrans_order_id is required")
	ErrSnapInvalidAmount  = errors.New("midtrans snap: gross amount must be positive")
	ErrSnapZeroExpiredAt  = errors.New("midtrans snap: expired_at is zero")
	ErrSnapPaymentExpired = errors.New("midtrans snap: payment already expired")
)

// buildSnapRequest produces a midtrans.SnapRequest from a fully resolved input.
// Pure function: no I/O, no clock side-effects (Now is injected).
//
// STEP A only — this builder does NOT call Midtrans. The real provider call lands in STEP B.
// Notification-Url HTTP header injection (per-request override) is also deferred to STEP B and
// belongs at the client.CreateSnapTransaction layer, not here.
func buildSnapRequest(in SnapBuilderInput) (*midtrans.SnapRequest, error) {
	if in.MidtransOrderID == "" {
		return nil, ErrSnapMissingOrderID
	}
	if in.GrossAmount <= 0 {
		return nil, ErrSnapInvalidAmount
	}
	if in.ExpiredAt.IsZero() {
		return nil, ErrSnapZeroExpiredAt
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if !in.ExpiredAt.After(now) {
		return nil, ErrSnapPaymentExpired
	}

	// Rupiah integer sent to Midtrans directly — no unit conversion.
	grossFloat := float64(in.GrossAmount)

	remaining := in.ExpiredAt.Sub(now)
	minutes := int(math.Ceil(remaining.Minutes()))
	if minutes < minExpiryMinutes {
		minutes = minExpiryMinutes
	}
	if minutes > maxExpiryMinutes {
		minutes = maxExpiryMinutes
	}

	itemName := "Order"
	if in.OrderNumber != "" {
		itemName = "Order " + in.OrderNumber
	}
	items := []midtrans.ItemDetail{{
		ID:       in.MidtransOrderID,
		Name:     itemName,
		Price:    grossFloat,
		Quantity: 1,
	}}

	var customer *midtrans.CustomerDetails
	if in.Buyer.FirstName != "" || in.Buyer.LastName != "" || in.Buyer.Email != "" || in.Buyer.Phone != "" {
		customer = &midtrans.CustomerDetails{
			FirstName: in.Buyer.FirstName,
			LastName:  in.Buyer.LastName,
			Email:     in.Buyer.Email,
			Phone:     in.Buyer.Phone,
		}
	}

	var callbacks *midtrans.Callbacks
	if in.FrontendURL != "" {
		callbacks = &midtrans.Callbacks{
			Finish: in.FrontendURL + "/payment/finish?order_id=" + in.MidtransOrderID,
		}
	}

	return &midtrans.SnapRequest{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:     in.MidtransOrderID,
			GrossAmount: grossFloat,
		},
		CustomerDetails: customer,
		ItemDetails:     items,
		Callbacks:       callbacks,
		Expiry: &midtrans.Expiry{
			StartTime: now.Format(snapTimeFormat),
			Unit:      "minute",
			Duration:  minutes,
		},
		EnabledPayments: in.EnabledPayments,
	}, nil
}
