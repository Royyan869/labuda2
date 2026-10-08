package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scheduled is the ONLY editable lifecycle state (create = publish), so every
// other state must reject an update without touching the auction.
func TestUpdateScheduled_DeniesNonScheduledStatuses(t *testing.T) {
	statuses := []Status{
		StatusActive,
		StatusWaitingSettlement,
		StatusEnded,
		StatusCancelled,
		StatusLapsed,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			auction := createTestAuction()
			auction.Status = status
			before := *auction

			err := auction.UpdateScheduled(auction.StartAt, auction.EndAt)

			require.Error(t, err)
			assert.Equal(t, before.StartAt, auction.StartAt)
			assert.Equal(t, before.EndAt, auction.EndAt)
			assert.Equal(t, before.Status, auction.Status)
		})
	}
}
