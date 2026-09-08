package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	deliveryentity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	deliveryRepo "github.com/labuda/backend/internal/pricing/promotion/delivery/repository"
	"github.com/labuda/backend/pkg/db"
)

// MeasurementRepositoryImpl persists canonical delivery measurement events
// (canonical_promotion_delivery_events) keyed by contract_id — the analytics
// projection authority. Events are immutable observations only; the ledger
// remains the financial truth.
type MeasurementRepositoryImpl struct{}

// NewMeasurementRepository creates the canonical measurement repository.
func NewMeasurementRepository() *MeasurementRepositoryImpl {
	return &MeasurementRepositoryImpl{}
}

var _ deliveryRepo.MeasurementRepository = (*MeasurementRepositoryImpl)(nil)

func (r *MeasurementRepositoryImpl) GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error) {
	var t time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&t); err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (r *MeasurementRepositoryImpl) RecordDeliveryEvent(ctx context.Context, tx db.Tx, event *deliveryentity.DeliveryEvent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO canonical_promotion_delivery_events
			(id, contract_id, target_type, target_id, viewer_id, event_type, exposure_id, server_occurred_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, event.ID, event.ContractID, event.TargetType, event.TargetID, event.ViewerID, string(event.EventType), event.ExposureID, event.ServerOccurredAt, event.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "canonical_delivery_events_impression_exposure_unique") {
			return deliveryRepo.ErrImpressionAlreadyRecorded
		}
		if strings.Contains(err.Error(), "canonical_delivery_events_click_exposure_unique") {
			return deliveryRepo.ErrClickAlreadyRecorded
		}
		return fmt.Errorf("record delivery event: %w", err)
	}
	return nil
}

func (r *MeasurementRepositoryImpl) GetDeliveryEventByExposure(ctx context.Context, tx db.Tx, exposureID uuid.UUID) (*deliveryentity.DeliveryEvent, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, contract_id, target_type, target_id, viewer_id, event_type, exposure_id, server_occurred_at, created_at
		FROM canonical_promotion_delivery_events
		WHERE exposure_id = $1
		LIMIT 1
	`, exposureID)
	var ev deliveryentity.DeliveryEvent
	if err := row.Scan(&ev.ID, &ev.ContractID, &ev.TargetType, &ev.TargetID, &ev.ViewerID, &ev.EventType, &ev.ExposureID, &ev.ServerOccurredAt, &ev.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, deliveryRepo.ErrDeliveryEventNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *MeasurementRepositoryImpl) GetContractAnalytics(ctx context.Context, tx db.Tx, contractID uuid.UUID) (*deliveryentity.DeliveryAnalytics, error) {
	row := tx.QueryRow(ctx, `
		SELECT
		       COUNT(*) FILTER (WHERE event_type = 'included')  AS included_count,
		       COUNT(*) FILTER (WHERE event_type = 'impression') AS impression_count,
		       COUNT(*) FILTER (WHERE event_type = 'click')      AS click_count
		FROM canonical_promotion_delivery_events
		WHERE contract_id = $1
	`, contractID)
	var includedCount, impressionCount, clickCount int64
	if err := row.Scan(&includedCount, &impressionCount, &clickCount); err != nil {
		return nil, err
	}
	return &deliveryentity.DeliveryAnalytics{
		ContractID:      contractID,
		IncludedCount:   int(includedCount),
		ImpressionCount: int(impressionCount),
		ClickCount:      int(clickCount),
	}, nil
}