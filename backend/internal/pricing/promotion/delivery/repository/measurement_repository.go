package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	deliveryentity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	"github.com/labuda/backend/pkg/db"
)

// Measurement persistence sentinels. Unknown exposure and repeated
// acknowledgement are NOT errors: they are safe no-ops (stale client state
// and idempotent retry respectively).
var (
	// ErrDeliveryEventNotFound is returned when no event matches the exposure id.
	ErrDeliveryEventNotFound = errors.New("canonical delivery event not found")
	// ErrImpressionAlreadyRecorded is returned when the exposure already has an impression row.
	ErrImpressionAlreadyRecorded = errors.New("canonical impression already recorded")
	// ErrClickAlreadyRecorded is returned when the exposure already has a click row.
	ErrClickAlreadyRecorded = errors.New("canonical click already recorded")
)

// MeasurementRepository persists canonical delivery measurement observations
// (included / impression / click) in canonical_promotion_delivery_events.
// All methods run inside the caller-provided transaction; event rows are
// immutable and timestamped with the DB clock.
type MeasurementRepository interface {
	// GetDBTime returns the database clock (server time authority).
	GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error)

	// RecordDeliveryEvent inserts one immutable delivery measurement event.
	// Duplicate impression/click for the same exposure is rejected with
	// ErrImpressionAlreadyRecorded / ErrClickAlreadyRecorded.
	RecordDeliveryEvent(ctx context.Context, tx db.Tx, event *deliveryentity.DeliveryEvent) error

	// GetDeliveryEventByExposure reads the issued event carrying the exposure
	// identity. Returns ErrDeliveryEventNotFound when unknown.
	GetDeliveryEventByExposure(ctx context.Context, tx db.Tx, exposureID uuid.UUID) (*deliveryentity.DeliveryEvent, error)

	// GetContractAnalytics returns the truthful included/impression/click
	// counts for one contract (contract_id authority).
	GetContractAnalytics(ctx context.Context, tx db.Tx, contractID uuid.UUID) (*deliveryentity.DeliveryAnalytics, error)
}