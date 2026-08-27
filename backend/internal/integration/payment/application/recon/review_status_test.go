package recon

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHasNonFailedWebhookTreatsReviewStatusesAsLive(t *testing.T) {
	webhooks := []WebhookEventRef{
		{Status: WebhookStatusManualReview},
		{Status: WebhookStatusQuarantined},
		{Status: WebhookStatusTerminalReview},
	}

	assert.True(t, hasNonFailedWebhook(webhooks))
}

func TestDetectD5SkipsReviewStatuses(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	s := Snapshot{
		Now: now,
		Gateway: GatewaySnapshot{
			MidtransOrderID: "MT-ORDER-1",
		},
		Webhooks: []WebhookEventRef{
			{
				EventID:           "evt-manual",
				Status:            WebhookStatusManualReview,
				TransactionStatus: GatewayStatusSettlement,
				TransactionID:     "tx-manual",
				ReceivedAt:        now.Add(-10 * time.Minute),
			},
			{
				EventID:           "evt-succeeded",
				Status:            WebhookStatusSucceeded,
				TransactionStatus: GatewayStatusSettlement,
				TransactionID:     "tx-success",
				ReceivedAt:        now.Add(-9 * time.Minute),
			},
		},
	}

	findings := detectD5(s)
	assert.Len(t, findings, 0, "manual_review rows must not be counted as trusted settlement evidence")
}

func TestBuildFindingUsesGatewayOrderWhenPaymentMissing(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	s := Snapshot{
		Now: now,
		Gateway: GatewaySnapshot{
			MidtransOrderID: "MT-ORDER-2",
		},
	}

	f := buildFinding(s, DriftD1GatewaySettledLocalUnpaid, SeverityCritical, "action", "notes", 1, 2, nil)
	assert.Equal(t, "MT-ORDER-2", f.MidtransOrderID)
	assert.Equal(t, now, f.DetectedAt)
	assert.Equal(t, "recon|D1_gateway_settled_local_unpaid|mt=MT-ORDER-2|d=20260606", f.IdempotencyKey)
}

func TestBuildFindingUsesOrderIDWhenPresent(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	orderID := uuid.New()
	s := Snapshot{
		Now: now,
		Order: &OrderRow{
			ID: orderID,
		},
	}

	f := buildFinding(s, DriftD8EscrowStateMismatch, SeverityHigh, "action", "notes", 0, 0, nil)
	assert.NotNil(t, f.OrderID)
	assert.Equal(t, orderID, *f.OrderID)
	assert.Contains(t, f.IdempotencyKey, orderID.String())
}


