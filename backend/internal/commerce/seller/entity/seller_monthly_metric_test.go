package entity_test

import (
	"testing"

	"github.com/labuda/backend/internal/commerce/seller/entity"
)

func TestFulfillmentRate_AllFulfilled(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        10,
		CancelledTimeoutCount: 0,
	}
	rate := m.FulfillmentRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0, got %f", rate)
	}
}

func TestFulfillmentRate_AllTimedOut(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        0,
		CancelledTimeoutCount: 5,
	}
	rate := m.FulfillmentRate()
	if rate != 0.0 {
		t.Errorf("expected 0.0, got %f", rate)
	}
}

func TestFulfillmentRate_ZeroDivision(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        0,
		CancelledTimeoutCount: 0,
	}
	rate := m.FulfillmentRate()
	if rate != 0.0 {
		t.Errorf("expected 0.0 for zero-zero, got %f", rate)
	}
}

func TestFulfillmentRate_Mixed(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        7,
		CancelledTimeoutCount: 3,
	}
	rate := m.FulfillmentRate()
	expected := 0.7
	if rate < expected-0.001 || rate > expected+0.001 {
		t.Errorf("expected ~0.7, got %f", rate)
	}
}

func TestFulfillmentRate_SingleFulfilled(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        1,
		CancelledTimeoutCount: 0,
	}
	rate := m.FulfillmentRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0, got %f", rate)
	}
}

func TestFulfillmentRate_SingleTimeout(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        0,
		CancelledTimeoutCount: 1,
	}
	rate := m.FulfillmentRate()
	if rate != 0.0 {
		t.Errorf("expected 0.0, got %f", rate)
	}
}

func TestFulfillmentRate_EqualSplit(t *testing.T) {
	m := &entity.SellerMonthlyMetric{
		FulfilledCount:        5,
		CancelledTimeoutCount: 5,
	}
	rate := m.FulfillmentRate()
	if rate != 0.5 {
		t.Errorf("expected 0.5, got %f", rate)
	}
}


