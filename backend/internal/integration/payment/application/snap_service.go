// Package application: canonical Midtrans Snap creation authority.
//
// ONE SNAP CREATION AUTHORITY: every incoming buyer payment flow (product
// checkout (incl. auction bid-win), seller registration, seller renewal, promotion
// funding / Promote Balance top-up) creates its gateway session through
// SnapService. The pure request builder, the production-mode refusal, the
// expiry floor/ceiling, the Finish callback shape, and the channel restriction
// live HERE and nowhere else.
//
// Business domains own pricing and finalization; they never build a
// midtrans.SnapRequest themselves.
package application

import (
	"errors"
	"math"
	"time"

	"github.com/hishumi/backend/pkg/midtrans"
)

// SnapBuyerInfo carries optional buyer identity fields for Snap CustomerDetails.
// All fields are optional; an entirely empty struct is valid.
type SnapBuyerInfo struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// SnapSessionInput is the canonical input contract for a Snap session.
// The caller resolves payment/order/buyer/config into this struct; the builder
// and service are free of repository, DB, and HTTP dependencies.
type SnapSessionInput struct {
	// MidtransOrderID is the unique identifier sent to Midtrans (payment.MidtransOrderID).
	MidtransOrderID string

	// GrossAmount is the amount the buyer is charged, in Rupiah integer —
	// Labuda's canonical money unit (PASS_18H). There is no cents/sen subunit;
	// this value is sent to Midtrans as-is. Must be positive.
	GrossAmount int64

	// ExpiredAt is the absolute payment-window deadline.
	ExpiredAt time.Time

	// OrderNumber is the human-readable order number; used for the default item
	// name. Optional.
	OrderNumber string

	// ItemName, when set, overrides the synthetic item name. Business flows with
	// a non-order purchase (e.g. seller subscription) set it so the Snap page
	// shows the purchased product name. When empty, the builder derives
	// "Order <OrderNumber>".
	ItemName string

	// Buyer is optional; an empty struct is valid.
	Buyer SnapBuyerInfo

	// FrontendURL is the base URL used to build the Snap "finish" callback.
	// Optional. SnapService injects its canonical configured value.
	FrontendURL string

	// EnabledPayments optionally restricts the Snap payment page to the given
	// Midtrans channel codes (the selected canonical method's MidtransChannels).
	// Empty/nil means Midtrans's default (all merchant-enabled channels).
	EnabledPayments []string

	// Now is injected for deterministic expiry math in tests. Zero falls back to time.Now().
	Now time.Time
}

const (
	minExpiryMinutes = 1
	maxExpiryMinutes = 1440 // 24 hours, Midtrans Snap upper bound for "minute" unit
	// snapTimeFormat is the Midtrans Snap StartTime format: "yyyy-MM-dd HH:mm:ss Z".
	snapTimeFormat = "2006-01-02 15:04:05 -0700"
)

// Sentinel errors so callers and tests can match precisely.
var (
	ErrSnapMissingOrderID       = errors.New("midtrans snap: midtrans_order_id is required")
	ErrSnapInvalidAmount        = errors.New("midtrans snap: gross amount must be positive")
	ErrSnapZeroExpiredAt        = errors.New("midtrans snap: expired_at is zero")
	ErrSnapPaymentExpired       = errors.New("midtrans snap: payment already expired")
	ErrSnapGatewayNotConfigured = errors.New("midtrans snap: gateway not configured")
	ErrSnapProductionForbidden  = errors.New("midtrans snap: production mode is forbidden in this build")
	ErrSnapEmptyRedirectURL     = errors.New("midtrans snap: returned empty redirect_url")
)

// BuildSnapRequest produces a midtrans.SnapRequest from a fully resolved input.
// Pure function: no I/O, no clock side-effects (Now is injected).
//
// This builder does NOT call Midtrans. The provider call, and the
// Notification-Url HTTP header injection that carries the canonical callback
// target (MIDTRANS_NOTIFICATION_URL), happen at the client layer.
func BuildSnapRequest(in SnapSessionInput) (*midtrans.SnapRequest, error) {
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
	switch {
	case in.ItemName != "":
		itemName = in.ItemName
	case in.OrderNumber != "":
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

// SnapGateway abstracts the canonical Midtrans Snap capability.
// Implemented by *midtrans.Client.
type SnapGateway interface {
	CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error)
	IsProduction() bool
}

// SnapService is the ONE canonical Snap creation authority. It refuses to run
// without a gateway, refuses production mode, builds the request through the
// pure builder, calls the gateway exactly once, and refuses an empty redirect.
type SnapService struct {
	gateway     SnapGateway
	frontendURL string
}

// NewSnapService creates the canonical Snap service.
func NewSnapService(gateway SnapGateway, frontendURL string) *SnapService {
	return &SnapService{gateway: gateway, frontendURL: frontendURL}
}

// CreateSession builds and creates one Snap session, returning the redirect URL.
//
// SAFETY CONTRACT:
//   - Refuses to run with a nil gateway.
//   - Refuses to run if the gateway is configured for production.
//   - Always injects the canonical FrontendURL (Finish callback).
//   - Refuses an empty RedirectURL response.
func (s *SnapService) CreateSession(in SnapSessionInput) (string, error) {
	if s == nil || s.gateway == nil {
		return "", ErrSnapGatewayNotConfigured
	}
	if s.gateway.IsProduction() {
		return "", ErrSnapProductionForbidden
	}
	in.FrontendURL = s.frontendURL

	req, err := BuildSnapRequest(in)
	if err != nil {
		return "", err
	}

	resp, err := s.gateway.CreateSnapTransaction(req)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.RedirectURL == "" {
		return "", ErrSnapEmptyRedirectURL
	}
	return resp.RedirectURL, nil
}
