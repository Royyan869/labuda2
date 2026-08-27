// Phase-1 unit tests for the gateway-aware refund webhook helpers.
//
// We test isWebhookRefundSuccess in isolation to lock down how Midtrans
// transaction_status + status_code combinations map to "this is a refund
// success ack" vs "this is a refund failure ack". The state-machine
// invariants themselves live in the entity-package tests; the
// orchestration / DB-touching paths are covered separately by the
// integration suite (which is already gated by the refund kill-switch).
package application

import (
	"testing"

	"github.com/labuda/backend/pkg/midtrans"
	"github.com/stretchr/testify/assert"
)

func TestIsWebhookRefundSuccess(t *testing.T) {
	cases := []struct {
		name string
		n    *midtrans.NotificationPayload
		want bool
	}{
		{
			name: "full refund with no status_code is treated as success",
			n: &midtrans.NotificationPayload{
				TransactionStatus: string(midtrans.StatusRefund),
			},
			want: true,
		},
		{
			name: "full refund with explicit 200 is success",
			n: &midtrans.NotificationPayload{
				TransactionStatus: string(midtrans.StatusRefund),
				StatusCode:        "200",
			},
			want: true,
		},
		{
			name: "partial refund 200 is success",
			n: &midtrans.NotificationPayload{
				TransactionStatus: string(midtrans.StatusPartialRefund),
				StatusCode:        "200",
			},
			want: true,
		},
		{
			name: "refund 412 is failure",
			n: &midtrans.NotificationPayload{
				TransactionStatus: string(midtrans.StatusRefund),
				StatusCode:        "412",
			},
			want: false,
		},
		{
			name: "settlement is not a refund ack at all",
			n: &midtrans.NotificationPayload{
				TransactionStatus: string(midtrans.StatusSettlement),
				StatusCode:        "200",
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isWebhookRefundSuccess(tc.n))
		})
	}
}


