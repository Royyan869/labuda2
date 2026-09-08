package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	deliveryentity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	deliveryRepo "github.com/labuda/backend/internal/pricing/promotion/delivery/repository"
	"github.com/labuda/backend/pkg/db"
)

// Sentinels for canonical exposure acknowledgement. Unknown exposure and
// repeated acknowledgement are NOT errors: they are safe no-ops (stale client
// state and idempotent retry respectively). The mismatches below are client /
// attacker errors and must never produce an impression or a click.
var (
	ErrInvalidExposure           = errors.New("exposure is not an issued canonical delivery")
	ErrExposurePromotionMismatch = errors.New("exposure belongs to a different promotion contract")
	ErrExposureViewerMismatch    = errors.New("exposure was issued to a different viewer")
)

// DeliveryInclusion is one server-side observation fact handed to the
// measurement authority by the feed bridge: a canonical promotion card was
// placed in a feed response served to ViewerID.
//
// The facts are deliberately minimal: canonical contract id + target identity
// + viewer. NO lifecycle state is reconstructed here — the promotion reached
// this boundary through the established canonical delivery path (contract
// selection → feed bridge), so the authority never re-checks eligibility,
// funding, activation, planned finish, or operability.
type DeliveryInclusion struct {
	ContractID uuid.UUID
	TargetType string
	TargetID   uuid.UUID
	ViewerID   uuid.UUID
}

// DeliveryMeasurementService is THE canonical delivery measurement authority.
// It owns the truthful observation types:
//
//   - 'included': a canonical promotion card was placed in a feed response.
//     RecordIncluded persists the observation AND issues the canonical
//     exposure identity (a server-generated UUID) that is returned to the
//     client with that card. The exposure identity is the only proof that a
//     delivery actually presented the promotion.
//
//   - 'impression': the client EXPLICITLY acknowledged that the card was
//     exposed/rendered, by echoing the issued exposure identity.
//     AcknowledgeImpression validates the echo and records exactly one
//     impression per issued exposure.
//
//   - 'click': the client EXPLICITLY acknowledged an explicit user tap on
//     the canonical card by echoing the issued exposure identity.
//     AcknowledgeClick validates the echo and records exactly one click per
//     issued exposure.
//
// The server NEVER fabricates an impression or a click on its own.
//
// Attribution: every persisted event resolves to the canonical contract id
// (promotion_contracts.id). This is a projection authority only — the
// financial ledger owns money truth.
//
// Time authority: server_occurred_at comes from the DB clock (same authority
// as every other canonical timestamp); client clocks are never trusted.
//
// Idempotency: one impression per issued exposure identity, enforced at the
// DB boundary (partial unique index on exposure_id for impression rows) and
// reflected here — a repeated acknowledgement is an idempotent success, never
// a second impression. The same boundary model applies to clicks. Different
// exposures for the same contract (repeated legitimate delivery across
// pages/refreshes) legitimately produce multiple impressions and clicks.
type DeliveryMeasurementService struct {
	db   db.Transactor
	repo deliveryRepo.MeasurementRepository
}

// NewDeliveryMeasurementService wires the canonical delivery measurement
// authority.
func NewDeliveryMeasurementService(db db.Transactor, repo deliveryRepo.MeasurementRepository) *DeliveryMeasurementService {
	return &DeliveryMeasurementService{db: db, repo: repo}
}

// RecordIncluded persists one 'included' observation per unique contract in
// inclusions (deduplicated by contract id within this call), stamps each with
// the DB clock, and ISSUES a fresh canonical exposure identity per contract.
// It returns the issued exposure identity per contract id so the feed bridge
// can embed it in the card payload the client receives. An empty inclusions
// list is a no-op.
func (s *DeliveryMeasurementService) RecordIncluded(ctx context.Context, inclusions []DeliveryInclusion) (map[uuid.UUID]uuid.UUID, error) {
	issued := make(map[uuid.UUID]uuid.UUID, len(inclusions))
	if len(inclusions) == 0 {
		return issued, nil
	}
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read delivery measurement time authority: %w", err)
		}

		seen := make(map[uuid.UUID]struct{}, len(inclusions))
		for _, inc := range inclusions {
			if _, dup := seen[inc.ContractID]; dup {
				continue
			}
			seen[inc.ContractID] = struct{}{}
			exposureID := uuid.New()
			ev, err := deliveryentity.NewIncludedEvent(inc.ContractID, inc.TargetType, inc.TargetID, inc.ViewerID, exposureID, now)
			if err != nil {
				return err
			}
			if err := s.repo.RecordDeliveryEvent(ctx, tx, ev); err != nil {
				return fmt.Errorf("record delivery event for contract %s: %w", inc.ContractID, err)
			}
			issued[inc.ContractID] = exposureID
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("canonical delivery measurement: %w", err)
	}
	return issued, nil
}

// AcknowledgeImpression records exactly one canonical 'impression' for an
// issued exposure identity, and returns whether the impression is durably
// recorded (true: by this acknowledgement or a prior identical one).
//
// Validation (never silently converted into an impression):
//   - unknown exposure id              → (false, nil): safely ignored (stale
//     client state — no impression exists and none is created);
//   - exposure is not an issued inclusion (e.g. already an impression) →
//     ErrInvalidExposure;
//   - exposure belongs to another contract than the supplied one →
//     ErrExposurePromotionMismatch;
//   - exposure was issued to a different viewer → ErrExposureViewerMismatch.
//
// The echoed facts (target snapshot, contract) come from the issued event,
// never from the client payload.
func (s *DeliveryMeasurementService) AcknowledgeImpression(ctx context.Context, viewerID uuid.UUID, exposureID uuid.UUID, contractID uuid.UUID) (bool, error) {
	var acknowledged bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		issued, err := s.repo.GetDeliveryEventByExposure(ctx, tx, exposureID)
		if err != nil {
			if errors.Is(err, deliveryRepo.ErrDeliveryEventNotFound) {
				// Unknown exposure: stale or fabricated — safely ignored,
				// never an impression.
				return nil
			}
			return err
		}
		if issued.EventType != deliveryentity.EventTypeIncluded {
			return ErrInvalidExposure
		}
		if issued.ContractID != contractID {
			return ErrExposurePromotionMismatch
		}
		if issued.ViewerID != viewerID {
			return ErrExposureViewerMismatch
		}

		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read impression time authority: %w", err)
		}
		imp, err := deliveryentity.NewImpressionEvent(issued.ContractID, issued.TargetType, issued.TargetID, viewerID, exposureID, now)
		if err != nil {
			return err
		}
		if err := s.repo.RecordDeliveryEvent(ctx, tx, imp); err != nil {
			if errors.Is(err, deliveryRepo.ErrImpressionAlreadyRecorded) {
				// Idempotent retry: a prior identical acknowledgement already
				// produced the impression.
				acknowledged = true
				return nil
			}
			return err
		}
		acknowledged = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return acknowledged, nil
}

// AcknowledgeClick records exactly one canonical 'click' for an issued
// exposure identity, and returns whether the click is durably recorded
// (true: by this acknowledgement or a prior identical one).
//
// Click semantics: an explicit user tap on a canonical promoted card that
// carried an issued exposure identity. The contract identity is DERIVED from
// the trusted issued 'included' event — the client never supplies a contract
// id, so an arbitrary claim can never fabricate a click.
//
// Validation (never silently converted into a click):
//   - unknown exposure id  → (false, nil): safely ignored (stale client
//     state — no click exists and none is created);
//   - exposure is not an issued inclusion (e.g. already an impression or a
//     click) → ErrInvalidExposure;
//   - exposure was issued to a different viewer → ErrExposureViewerMismatch.
//
// Idempotency mirrors the impression boundary: a repeated acknowledgement of
// the same exposure is an idempotent success, never a second click. Click and
// impression are independent acknowledgement types — a viewer may tap without
// ever acknowledging an impression, and the recorded facts never derive from
// each other.
func (s *DeliveryMeasurementService) AcknowledgeClick(ctx context.Context, viewerID uuid.UUID, exposureID uuid.UUID) (bool, error) {
	var acknowledged bool
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		issued, err := s.repo.GetDeliveryEventByExposure(ctx, tx, exposureID)
		if err != nil {
			if errors.Is(err, deliveryRepo.ErrDeliveryEventNotFound) {
				// Unknown exposure: stale or fabricated — safely ignored,
				// never a click.
				return nil
			}
			return err
		}
		if issued.EventType != deliveryentity.EventTypeIncluded {
			return ErrInvalidExposure
		}
		if issued.ViewerID != viewerID {
			return ErrExposureViewerMismatch
		}

		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read click time authority: %w", err)
		}
		click, err := deliveryentity.NewClickEvent(issued.ContractID, issued.TargetType, issued.TargetID, viewerID, exposureID, now)
		if err != nil {
			return err
		}
		if err := s.repo.RecordDeliveryEvent(ctx, tx, click); err != nil {
			if errors.Is(err, deliveryRepo.ErrClickAlreadyRecorded) {
				// Idempotent retry: a prior identical acknowledgement already
				// produced the click.
				acknowledged = true
				return nil
			}
			return err
		}
		acknowledged = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return acknowledged, nil
}

// GetContractAnalytics returns the truthful delivery measurement projection
// for one contract (contract_id authority). Ownership must be enforced by the
// caller (contract handler enforces it via the contract service).
func (s *DeliveryMeasurementService) GetContractAnalytics(ctx context.Context, contractID uuid.UUID) (*deliveryentity.DeliveryAnalytics, error) {
	var out *deliveryentity.DeliveryAnalytics
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		a, err := s.repo.GetContractAnalytics(ctx, tx, contractID)
		if err != nil {
			return err
		}
		out = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}