package entity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

func TestNewOrderItem_BindsProductID(t *testing.T) {
	orderID := uuid.New()
	productID := uuid.New()
	item := NewOrderItem(orderID, productID, money.New(1200), 3, "Koi")

	if item.OrderID != orderID {
		t.Fatalf("expected order_id %s, got %s", orderID, item.OrderID)
	}
	if item.ProductID != productID {
		t.Fatalf("expected product_id %s, got %s", productID, item.ProductID)
	}
	if got := item.Subtotal().Int64(); got != 3600 {
		t.Fatalf("expected subtotal 3600, got %d", got)
	}
}


