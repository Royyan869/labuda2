package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExternalProductReviewHistory records lifecycle transitions for auditing.
type ExternalProductReviewHistory struct {
	ID                uuid.UUID
	ExternalProductID uuid.UUID
	ActorAdminID      *uuid.UUID
	ActorUserID       *uuid.UUID
	FromStatus        *ExternalProductReviewStatus
	ToStatus          ExternalProductReviewStatus
	Reason            *string
	CreatedAt         time.Time
}

// NewExternalProductReviewHistory creates a validated history record.
func NewExternalProductReviewHistory(
	externalProductID uuid.UUID,
	fromStatus *ExternalProductReviewStatus,
	toStatus ExternalProductReviewStatus,
	reason *string,
	actorAdminID *uuid.UUID,
	actorUserID *uuid.UUID,
	dbTime time.Time,
) (*ExternalProductReviewHistory, error) {
	if externalProductID == uuid.Nil {
		return nil, fmt.Errorf("external_product_id is required")
	}
	if !toStatus.IsValid() {
		return nil, fmt.Errorf("invalid to_status: %s", toStatus)
	}
	if actorAdminID == nil && actorUserID == nil {
		return nil, fmt.Errorf("at least one actor is required")
	}
	if fromStatus != nil && !fromStatus.IsValid() {
		return nil, fmt.Errorf("invalid from_status: %s", *fromStatus)
	}

	return &ExternalProductReviewHistory{
		ID:                uuid.New(),
		ExternalProductID: externalProductID,
		ActorAdminID:      actorAdminID,
		ActorUserID:       actorUserID,
		FromStatus:        fromStatus,
		ToStatus:          toStatus,
		Reason:            reason,
		CreatedAt:         dbTime,
	}, nil
}
