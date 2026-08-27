package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateDraft_DeniesNonDraftStatuses(t *testing.T) {
	statuses := []Status{
		StatusScheduled,
		StatusActive,
		StatusWaitingSettlement,
		StatusEnded,
		StatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			auction := createTestDraftAuction()
			auction.Status = status
			before := *auction

			err := auction.UpdateDraft(
				20000,
				2000,
				nil,
				auction.StartAt,
				auction.EndAt,
			)

			require.Error(t, err)
			assert.Equal(t, before.StartPrice, auction.StartPrice)
			assert.Equal(t, before.BidIncrement, auction.BidIncrement)
			assert.Equal(t, before.BuyNowPrice, auction.BuyNowPrice)
			assert.Equal(t, before.StartAt, auction.StartAt)
			assert.Equal(t, before.EndAt, auction.EndAt)
			assert.Equal(t, before.Status, auction.Status)
		})
	}
}
