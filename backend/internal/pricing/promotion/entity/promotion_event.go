package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PromotionEventType is the type of analytics event recorded for a promoted item.
type PromotionEventType string

const (
	// PromotionEventTypeClick is recorded when a viewer taps a promoted card.
	PromotionEventTypeClick PromotionEventType = "click"

	// PromotionEventTypeImpression is recorded when a promoted card is visible
	// in the viewport (mobile: visibleFraction >= 0.5, once per session per instance).
	PromotionEventTypeImpression PromotionEventType = "impression"
)

// IsValid returns true if the event type is a known value.
func (e PromotionEventType) IsValid() bool {
	return e == PromotionEventTypeClick || e == PromotionEventTypeImpression
}

// PromotionEventSurface identifies which discovery surface generated the event.
type PromotionEventSurface string

const (
	PromotionEventSurfaceFeed    PromotionEventSurface = "feed"
	PromotionEventSurfaceSearch  PromotionEventSurface = "search"
	PromotionEventSurfaceExplore PromotionEventSurface = "explore"
)

// IsValid returns true if the surface is a known value.
func (s PromotionEventSurface) IsValid() bool {
	switch s {
	case PromotionEventSurfaceFeed, PromotionEventSurfaceSearch, PromotionEventSurfaceExplore:
		return true
	}
	return false
}

// PromotionEvent is an analytics record for a viewer interaction with a promoted item.
//
// This is NOT a finance or authority entity. It is append-only analytics data.
// instance_id is stored without FK so history is preserved if the instance is finalized.
type PromotionEvent struct {
	ID           uuid.UUID
	InstanceID   uuid.UUID
	TargetType   TargetType
	TargetID     *uuid.UUID // nil for external_product target type without a separate target_id
	OwnerUserID  uuid.UUID
	ViewerUserID uuid.UUID
	EventType    PromotionEventType
	Surface      PromotionEventSurface
	OccurredAt   time.Time
}

// NewPromotionEvent constructs a PromotionEvent from a validated PromotionInstance.
// All metadata (TargetType, TargetID, OwnerUserID) is sourced from the instance
// so that clients cannot forge analytics attribution.
func NewPromotionEvent(
	inst *PromotionInstance,
	viewerUserID uuid.UUID,
	eventType PromotionEventType,
	surface PromotionEventSurface,
) (*PromotionEvent, error) {
	if inst == nil {
		return nil, fmt.Errorf("promotion instance must not be nil")
	}
	if viewerUserID == uuid.Nil {
		return nil, fmt.Errorf("viewer_user_id must not be nil")
	}
	if !eventType.IsValid() {
		return nil, fmt.Errorf("unknown event_type: %q", eventType)
	}
	if !surface.IsValid() {
		return nil, fmt.Errorf("unknown surface: %q", surface)
	}
	return &PromotionEvent{
		ID:           uuid.New(),
		InstanceID:   inst.ID,
		TargetType:   inst.TargetType,
		TargetID:     inst.TargetID,
		OwnerUserID:  inst.UserID,
		ViewerUserID: viewerUserID,
		EventType:    eventType,
		Surface:      surface,
		OccurredAt:   time.Now().UTC(),
	}, nil
}
