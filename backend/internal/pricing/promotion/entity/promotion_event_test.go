package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestInstance creates a minimal PromotionInstance for event construction tests.
func buildTestInstance() *PromotionInstance {
	ownerID := uuid.New()
	targetID := uuid.New()
	return &PromotionInstance{
		ID:          uuid.New(),
		OwnershipID: uuid.New(),
		UserID:      ownerID,
		TargetType:  TargetTypeForSale,
		TargetID:    &targetID,
		Status:      InstanceStatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestNewPromotionEvent_Valid(t *testing.T) {
	inst := buildTestInstance()
	viewerID := uuid.New()

	ev, err := NewPromotionEvent(inst, viewerID, PromotionEventTypeClick, PromotionEventSurfaceFeed)

	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, inst.ID, ev.InstanceID)
	assert.Equal(t, inst.TargetType, ev.TargetType)
	assert.Equal(t, inst.TargetID, ev.TargetID)
	assert.Equal(t, inst.UserID, ev.OwnerUserID)
	assert.Equal(t, viewerID, ev.ViewerUserID)
	assert.Equal(t, PromotionEventTypeClick, ev.EventType)
	assert.Equal(t, PromotionEventSurfaceFeed, ev.Surface)
	assert.NotEqual(t, uuid.Nil, ev.ID)
	assert.False(t, ev.OccurredAt.IsZero())
}

func TestNewPromotionEvent_NilInstance(t *testing.T) {
	_, err := NewPromotionEvent(nil, uuid.New(), PromotionEventTypeClick, PromotionEventSurfaceFeed)
	assert.ErrorContains(t, err, "nil")
}

func TestNewPromotionEvent_NilViewerID(t *testing.T) {
	_, err := NewPromotionEvent(buildTestInstance(), uuid.Nil, PromotionEventTypeClick, PromotionEventSurfaceFeed)
	assert.ErrorContains(t, err, "viewer_user_id")
}

func TestNewPromotionEvent_InvalidEventType(t *testing.T) {
	_, err := NewPromotionEvent(buildTestInstance(), uuid.New(), PromotionEventType("unknown"), PromotionEventSurfaceFeed)
	assert.ErrorContains(t, err, "event_type")
}

func TestNewPromotionEvent_InvalidSurface(t *testing.T) {
	_, err := NewPromotionEvent(buildTestInstance(), uuid.New(), PromotionEventTypeClick, PromotionEventSurface("unknown"))
	assert.ErrorContains(t, err, "surface")
}

func TestPromotionEventType_IsValid(t *testing.T) {
	assert.True(t, PromotionEventTypeClick.IsValid())
	assert.True(t, PromotionEventTypeImpression.IsValid())
	assert.False(t, PromotionEventType("view").IsValid())
	assert.False(t, PromotionEventType("").IsValid())
}

func TestNewPromotionEvent_ImpressionType(t *testing.T) {
	inst := buildTestInstance()
	ev, err := NewPromotionEvent(inst, uuid.New(), PromotionEventTypeImpression, PromotionEventSurfaceFeed)
	require.NoError(t, err)
	assert.Equal(t, PromotionEventTypeImpression, ev.EventType)
}

func TestNewPromotionEvent_AllEventTypes(t *testing.T) {
	validTypes := []PromotionEventType{
		PromotionEventTypeClick,
		PromotionEventTypeImpression,
	}
	inst := buildTestInstance()
	for _, et := range validTypes {
		t.Run(string(et), func(t *testing.T) {
			ev, err := NewPromotionEvent(inst, uuid.New(), et, PromotionEventSurfaceFeed)
			require.NoError(t, err)
			assert.Equal(t, et, ev.EventType)
		})
	}
}

func TestPromotionEventSurface_IsValid(t *testing.T) {
	assert.True(t, PromotionEventSurfaceFeed.IsValid())
	assert.True(t, PromotionEventSurfaceSearch.IsValid())
	assert.True(t, PromotionEventSurfaceExplore.IsValid())
	assert.False(t, PromotionEventSurface("home").IsValid())
	assert.False(t, PromotionEventSurface("").IsValid())
}

func TestNewPromotionEvent_AllSurfaces(t *testing.T) {
	surfaces := []PromotionEventSurface{
		PromotionEventSurfaceFeed,
		PromotionEventSurfaceSearch,
		PromotionEventSurfaceExplore,
	}
	inst := buildTestInstance()
	for _, s := range surfaces {
		t.Run(string(s), func(t *testing.T) {
			ev, err := NewPromotionEvent(inst, uuid.New(), PromotionEventTypeClick, s)
			require.NoError(t, err)
			assert.Equal(t, s, ev.Surface)
		})
	}
}

// TestNewPromotionEvent_ExternalProductNoTargetID verifies that external_product
// instances with nil TargetID produce a valid event (target_id stays nil).
func TestNewPromotionEvent_ExternalProductNoTargetID(t *testing.T) {
	inst := &PromotionInstance{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		TargetType: TargetTypeExternalProduct,
		TargetID:   nil, // external products may have nil target_id
		Status:     InstanceStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	ev, err := NewPromotionEvent(inst, uuid.New(), PromotionEventTypeClick, PromotionEventSurfaceFeed)
	require.NoError(t, err)
	assert.Nil(t, ev.TargetID)
}
