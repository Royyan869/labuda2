package serverboot

import (
	chatApp "github.com/hishumi/backend/internal/interaction/chat/application"
	"github.com/hishumi/backend/pkg/db"
)

func newChatResourceProjectionResolver(database *db.DB) chatApp.ResourceProjectionResolver {
	return newResourceProjectionAggregateResolver(
		newProfileProjectionBatchResolver(database),
		newContentProjectionBatchResolver(database),
		newForSaleProjectionBatchResolver(database),
		newAuctionProjectionBatchResolver(database),
	)
}
