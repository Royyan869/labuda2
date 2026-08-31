package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/moderation/entity"
)

// ModerationRepository is the LEGACY GovernanceCase read path.
//
// SLICE 2 TEARDOWN: The legacy Report intake (Create/ResourceExists/
// HasUserReportedResource/ValidateChatMessageReporter) and the legacy admin
// Case review path (GetForUpdate/Update/ListPending/ListByResource/
// ListByReporter/ListWithStatus) have been REMOVED. The canonical Report
// runtime (ReportRepository) is the single Report authority.
//
// The remaining GetByID exists ONLY to keep the out-of-scope Appeal domain
// (Slice 9) compiling. It reads moderation_cases, which was dropped in
// migration 000056 — so it is runtime-dead and must be replaced when the
// Appeal domain is rebuilt against the canonical Decision schema.
type ModerationRepository interface {
	// GetByID retrieves a legacy governance case by ID.
	// RUNTIME-DEAD: moderation_cases table was dropped in migration 000056.
	GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
}
