package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/stretchr/testify/require"
)

// TestOrderDetailResponse_ExposesBoundPaymentMethod proves the order-detail
// response carries the EXACT bound payment method (orders.payment_method_code)
// so the post-order retry UI can use it instead of offering invalid
// alternatives. Nil for unbound (auction bid-win) orders.
func TestOrderDetailResponse_ExposesBoundPaymentMethod(t *testing.T) {
	method := "bank_transfer"
	order := &entity.Order{
		ID:                uuid.New(),
		BuyerID:           uuid.New(),
		SellerID:          uuid.New(),
		Status:            entity.StatusPending,
		PaymentMethodCode: &method,
		CreatedAt:         time.Now().Add(-time.Hour),
		UpdatedAt:         time.Now(),
	}

	resp := OrderToDetailResponseWithIdentity(
		order,
		order.BuyerID,
		"", "", "",
		"", "", "", "",
		nil, nil, false, nil, nil, nil, nil,
	)

	require.NotNil(t, resp.PaymentMethodCode)
	require.Equal(t, "bank_transfer", *resp.PaymentMethodCode)
}

// TestOrderDetailResponse_UnboundOrderHasNilPaymentMethod proves an unbound
// order (no checkout selection) surfaces a nil method, so the UI can
// distinguish the legitimate first-selection lifecycle.
func TestOrderDetailResponse_UnboundOrderHasNilPaymentMethod(t *testing.T) {
	order := &entity.Order{
		ID:        uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		Status:    entity.StatusPending,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now(),
	}

	resp := OrderToDetailResponseWithIdentity(
		order,
		order.BuyerID,
		"", "", "",
		"", "", "", "",
		nil, nil, false, nil, nil, nil, nil,
	)

	require.Nil(t, resp.PaymentMethodCode)
}
