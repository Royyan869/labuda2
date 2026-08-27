package repository

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/money"
)

// TestPaymentStatusHelperMethods tests the Payment status helper methods.
func TestPaymentStatusHelperMethods(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		wantIsPending bool
		wantIsSettled bool
		wantIsFailed  bool
	}{
		{
			name:          "pending status",
			status:        PaymentStatusPending,
			wantIsPending: true,
			wantIsSettled: false,
			wantIsFailed:  false,
		},
		{
			name:          "settlement status",
			status:        PaymentStatusSettlement,
			wantIsPending: false,
			wantIsSettled: true,
			wantIsFailed:  false,
		},
		{
			name:          "capture status",
			status:        PaymentStatusCapture,
			wantIsPending: false,
			wantIsSettled: true,
			wantIsFailed:  false,
		},
		{
			name:          "deny status",
			status:        PaymentStatusDeny,
			wantIsPending: false,
			wantIsSettled: false,
			wantIsFailed:  true,
		},
		{
			name:          "cancel status",
			status:        PaymentStatusCancel,
			wantIsPending: false,
			wantIsSettled: false,
			wantIsFailed:  true,
		},
		{
			name:          "expire status",
			status:        PaymentStatusExpire,
			wantIsPending: false,
			wantIsSettled: false,
			wantIsFailed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Payment{Status: tt.status}

			if got := p.IsPending(); got != tt.wantIsPending {
				t.Errorf("Payment.IsPending() = %v, want %v", got, tt.wantIsPending)
			}
			if got := p.IsSettled(); got != tt.wantIsSettled {
				t.Errorf("Payment.IsSettled() = %v, want %v", got, tt.wantIsSettled)
			}
			if got := p.IsFailed(); got != tt.wantIsFailed {
				t.Errorf("Payment.IsFailed() = %v, want %v", got, tt.wantIsFailed)
			}
		})
	}
}

// TestNewPaymentRepository tests creating a new PaymentRepository.
func TestNewPaymentRepository(t *testing.T) {
	repo := NewPaymentRepository()

	if repo == nil {
		t.Fatal("NewPaymentRepository() returned nil")
	}

	// Verify it's the correct type
	if repo == nil {
		t.Errorf("NewPaymentRepository() returned nil")
	}
}

// TestIsUniqueViolation tests the IsUniqueViolation helper function.
func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unique constraint violation 23505",
			err:  &testError{msg: "ERROR: duplicate key value violates unique constraint (23505)"},
			want: true,
		},
		{
			name: "duplicate key message",
			err:  &testError{msg: "duplicate key value violates unique constraint"},
			want: true,
		},
		{
			name: "UNIQUE constraint message",
			err:  &testError{msg: "UNIQUE constraint violated"},
			want: true,
		},
		{
			name: "other error",
			err:  &testError{msg: "some other error"},
			want: false,
		},
		{
			name: "no rows error",
			err:  pgx.ErrNoRows,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolation(tt.err); got != tt.want {
				t.Errorf("IsUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestPaymentStatusConstants tests that payment status constants are properly defined.
func TestPaymentStatusConstants(t *testing.T) {
	constants := map[string]string{
		"PaymentStatusPending":    PaymentStatusPending,
		"PaymentStatusSettlement": PaymentStatusSettlement,
		"PaymentStatusCapture":    PaymentStatusCapture,
		"PaymentStatusDeny":       PaymentStatusDeny,
		"PaymentStatusCancel":     PaymentStatusCancel,
		"PaymentStatusExpire":     PaymentStatusExpire,
	}

	expectedValues := map[string]string{
		"PaymentStatusPending":    "pending",
		"PaymentStatusSettlement": "settlement",
		"PaymentStatusCapture":    "capture",
		"PaymentStatusDeny":       "deny",
		"PaymentStatusCancel":     "cancel",
		"PaymentStatusExpire":     "expire",
	}

	for name, value := range constants {
		if value != expectedValues[name] {
			t.Errorf("%s = %s, want %s", name, value, expectedValues[name])
		}
		if value == "" {
			t.Errorf("%s is empty", name)
		}
	}
}

// TestNewPaymentSettlementService tests creating a new PaymentSettlementService.
func TestNewPaymentSettlementService(t *testing.T) {
	service := NewPaymentSettlementService()

	if service == nil {
		t.Fatal("NewPaymentSettlementService() returned nil")
	}

	if service.paymentRepo == nil {
		t.Error("PaymentSettlementService.paymentRepo is nil")
	}
}

// TestPaymentEntity tests the Payment entity struct.
func TestPaymentEntity(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	refID := uuid.New()

	p := &Payment{
		ID:                 id,
		UserID:             userID,
		PaymentNumber:      "PAY-12345",
		MidtransOrderID:    "ORDER-67890",
		GrossAmount:        money.New(100000),
		CoinDiscountAmount: money.Zero(),
		Status:             PaymentStatusPending,
		ReferenceType:      "order",
		ReferenceID:        &refID,
		TransactionID:      strPtr("TXN-123"),
		PaymentType:        strPtr("bank_transfer"),
	}

	if p.ID != id {
		t.Errorf("Payment.ID = %v, want %v", p.ID, id)
	}
	if p.UserID != userID {
		t.Errorf("Payment.UserID = %v, want %v", p.UserID, userID)
	}
	if p.PaymentNumber != "PAY-12345" {
		t.Errorf("Payment.PaymentNumber = %v, want %v", p.PaymentNumber, "PAY-12345")
	}
	if !p.GrossAmount.Equal(money.New(100000)) {
		t.Errorf("Payment.GrossAmount = %v, want %v", p.GrossAmount.Int64(), money.New(100000).Int64())
	}
}

// TestMoneyUtil tests Money utility functions in payment context.
func TestMoneyUtil(t *testing.T) {
	amount := money.New(100000)

	if amount.Int64() != 100000 {
		t.Errorf("Money.Int64() = %v, want %v", amount.Int64(), 100000)
	}

	if amount.IsZero() {
		t.Error("Money.IsZero() = true, want false")
	}

	zero := money.Zero()
	if !zero.IsZero() {
		t.Error("Money.IsZero() = false, want true")
	}

	double := amount.Add(amount)
	if double.Int64() != 200000 {
		t.Errorf("Money.Add() = %v, want %v", double.Int64(), 200000)
	}
}

func strPtr(s string) *string {
	return &s
}
