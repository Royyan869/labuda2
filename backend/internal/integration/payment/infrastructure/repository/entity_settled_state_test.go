package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSettledPaymentStatuses_IsTheSingleCanonicalDefinition pins the ONE
// definition of "settled" in the payment domain. Every consumer — the Go
// predicate used by the webhook/finalizers, the subscription recovery selector,
// the alert rules, the monitoring metrics and the admin recovery surfaces —
// must read this set instead of restating it as SQL literals or status chains.
func TestSettledPaymentStatuses_IsTheSingleCanonicalDefinition(t *testing.T) {
	assert.Equal(t,
		[]string{PaymentStatusSettlement, PaymentStatusCapture},
		SettledPaymentStatuses(),
		"settled = settlement OR capture, and nothing else",
	)
}

// TestIsSettledStatus_Matrix covers the settled-state truth table:
// settlement → settled, capture → settled, pending/deny/cancel/expire → not.
func TestIsSettledStatus_Matrix(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"settlement is settled", PaymentStatusSettlement, true},
		{"capture is settled", PaymentStatusCapture, true},
		{"pending is not settled", PaymentStatusPending, false},
		{"deny is not settled", PaymentStatusDeny, false},
		{"cancel is not settled", PaymentStatusCancel, false},
		{"expire is not settled", PaymentStatusExpire, false},
		{"empty is not settled", "", false},
		{"unknown status is not settled", "settled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSettledStatus(tt.status))
			// The entity predicate must be the SAME authority, not a parallel one.
			p := &Payment{Status: tt.status}
			assert.Equal(t, tt.want, p.IsSettled())
		})
	}
}

// TestSettledPaymentStatuses_ReturnsCopy proves callers cannot corrupt the
// canonical set by mutating the slice they are handed.
func TestSettledPaymentStatuses_ReturnsCopy(t *testing.T) {
	got := SettledPaymentStatuses()
	got[0] = "corrupted"

	assert.Equal(t,
		[]string{PaymentStatusSettlement, PaymentStatusCapture},
		SettledPaymentStatuses(),
	)
}
