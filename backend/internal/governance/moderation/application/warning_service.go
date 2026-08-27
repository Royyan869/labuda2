package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	userentity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
)

// warningTargetUserLookup is the minimal user lookup contract needed by the warning service.
type warningTargetUserLookup interface {
	GetByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userentity.User, error)
}

// warningOutboxWriter is the minimal outbox contract needed by the warning service.
type warningOutboxWriter interface {
	InsertEvent(ctx context.Context, tx db.Tx, eventType string, entityID uuid.UUID, payload []byte) error
}

// WarningService handles warning business logic.
//
// Warnings are issued by admins to users for policy violations.
// Multiple active warnings can lead to account restrictions.
//
// Event emission: IssueWarning emits "moderation.warning.issued" into the
// outbox within the same transaction as the warning insert, ensuring
// exactly-once delivery to downstream notification handlers.
type WarningService struct {
	warningRepo repository.WarningRepository
	userRepo    warningTargetUserLookup
	outboxRepo  warningOutboxWriter
}

// NewWarningService creates a new WarningService.
func NewWarningService(
	warningRepo repository.WarningRepository,
	userRepo warningTargetUserLookup,
	outboxRepo warningOutboxWriter,
) *WarningService {
	return &WarningService{
		warningRepo: warningRepo,
		userRepo:    userRepo,
		outboxRepo:  outboxRepo,
	}
}

// IssueWarning creates a new warning for a user.
//
// Business rules:
// - Only admins can issue warnings
// - Warnings can have different severity levels (info, warning, severe)
// - Optional expiration date for temporary warnings
func (s *WarningService) IssueWarning(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	issuedBy uuid.UUID,
	level entity.WarningLevel,
	reason string,
	expiresAt *int64, // Unix timestamp, nil for no expiration
) (*entity.UserWarning, error) {
	if s.userRepo == nil {
		return nil, fmt.Errorf("warning service user repository is not configured")
	}

	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	user, err := s.userRepo.GetByID(ctx, dbTx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, &entity.ErrWarningTargetNotFound{UserID: userID}
	}

	// OWNER DECISION REQUIRED:
	// Warning cap/frequency policy is intentionally not enforced here until a
	// canonical governance threshold is defined.

	var expiresAtTime *time.Time
	if expiresAt != nil {
		expiresAtTime = parseUnixTime(*expiresAt)
	}

	warning := entity.NewWarning(userID, issuedBy, level, reason, expiresAtTime)

	err = s.warningRepo.Create(ctx, tx, warning)
	if err != nil {
		return nil, err
	}

	// Emit outbox event for downstream notification.
	// The tx parameter is always db.Tx at runtime (handler wraps in WithTx).
	if s.outboxRepo != nil {
		payload := buildWarningIssuedPayload(warning)
		if err := s.outboxRepo.InsertEvent(ctx, dbTx, "moderation.warning.issued", warning.ID, payload); err != nil {
			return nil, err
		}
	}

	return warning, nil
}

// GetWarning retrieves a warning by ID.
func (s *WarningService) GetWarning(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
) (*entity.UserWarning, error) {
	return s.warningRepo.GetByID(ctx, tx, warningID)
}

// ListWarningsByUser retrieves all warnings for a specific user.
func (s *WarningService) ListWarningsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	limit, offset int,
) ([]*entity.UserWarning, error) {
	return s.warningRepo.ListByUser(ctx, tx, userID, limit, offset)
}

// ListActiveWarningsByUser retrieves active warnings for a specific user.
func (s *WarningService) ListActiveWarningsByUser(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) ([]*entity.UserWarning, error) {
	return s.warningRepo.ListActiveByUser(ctx, tx, userID)
}

// ListAllWarnings retrieves all warnings with optional user filter.
// If userID is nil, returns all warnings.
func (s *WarningService) ListAllWarnings(
	ctx context.Context,
	tx interface{},
	userID *uuid.UUID,
	isActive *bool,
	limit, offset int,
) ([]*entity.UserWarning, int64, error) {
	return s.warningRepo.ListAll(ctx, tx, userID, isActive, limit, offset)
}

// RevokeWarning revokes an active warning.
//
// Business rules:
// - Only active warnings can be revoked
// - Only admins can revoke warnings
func (s *WarningService) RevokeWarning(
	ctx context.Context,
	tx interface{},
	warningID uuid.UUID,
	revokedBy uuid.UUID,
) (*entity.UserWarning, error) {
	// Get warning with FOR UPDATE lock to prevent concurrent modifications
	warning, err := s.warningRepo.GetForUpdate(ctx, tx, warningID)
	if err != nil {
		return nil, err
	}

	// Revoke the warning
	err = warning.Revoke(revokedBy)
	if err != nil {
		return nil, err
	}

	// Persist the updated warning
	err = s.warningRepo.Update(ctx, tx, warning)
	if err != nil {
		return nil, err
	}

	return warning, nil
}

// parseUnixTime converts a Unix timestamp to time.Time.
func parseUnixTime(timestamp int64) *time.Time {
	t := time.Unix(timestamp, 0)
	return &t
}

// buildWarningIssuedPayload creates the JSON payload for moderation.warning.issued.
//
// Payload format:
//
//	{
//	  "warning_id": "uuid",
//	  "user_id":    "uuid",
//	  "level":      "info|warning|severe",
//	  "reason":     "text"
//	}
func buildWarningIssuedPayload(w *entity.UserWarning) []byte {
	type payload struct {
		WarningID string `json:"warning_id"`
		UserID    string `json:"user_id"`
		Level     string `json:"level"`
		Reason    string `json:"reason"`
	}
	b, _ := json.Marshal(payload{
		WarningID: w.ID.String(),
		UserID:    w.UserID.String(),
		Level:     string(w.Level),
		Reason:    w.Reason,
	})
	return b
}


