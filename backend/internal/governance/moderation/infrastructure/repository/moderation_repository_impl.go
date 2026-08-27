package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// ModerationRepositoryImpl handles governance case persistence using pgx-based DB layer.
type ModerationRepositoryImpl struct{}

// NewModerationRepository creates a new ModerationRepository.
func NewModerationRepository() ModerationRepository {
	return &ModerationRepositoryImpl{}
}

// Create persists a new governance case within a transaction.
func (r *ModerationRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	caseEntity *entity.GovernanceCase,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var reviewedBy *uuid.UUID
	if caseEntity.ReviewedBy != nil {
		reviewedBy = caseEntity.ReviewedBy
	}

	var decisionNote *string
	if caseEntity.DecisionNote != nil {
		decisionNote = caseEntity.DecisionNote
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO moderation_cases (
			id, resource_type, resource_id, status,
			reported_by, reviewed_by, reason, decision_note,
			created_at, reviewed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		caseEntity.ID,
		string(caseEntity.ResourceType),
		caseEntity.ResourceID,
		string(caseEntity.Status),
		caseEntity.ReportedBy,
		reviewedBy,
		caseEntity.Reason,
		decisionNote,
		caseEntity.CreatedAt,
		caseEntity.ReviewedAt,
	)

	if err != nil {
		return fmt.Errorf("create governance case failed: %w", err)
	}

	return nil
}

// selectColumns is the canonical column list for moderation_cases queries.
// All read methods must use this to keep SELECT and Scan in sync.
const selectColumns = `id, resource_type, resource_id, status,
		       reported_by, reviewed_by, reason, decision_note,
		       created_at, reviewed_at`

// GetByID retrieves a case without locking (for read-only operations).
func (r *ModerationRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	caseID uuid.UUID,
) (*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases
		WHERE id = $1`

	row := dbTx.QueryRow(ctx, query, caseID)
	kase, err := r.scanSingleRow(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("governance case not found: %s", caseID)
		}
		return nil, fmt.Errorf("get governance case failed: %w", err)
	}

	return kase, nil
}

// GetForUpdate retrieves a case with FOR UPDATE lock.
// CRITICAL: Must be used for all review operations to prevent double-review.
func (r *ModerationRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	caseID uuid.UUID,
) (*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases
		WHERE id = $1
		FOR UPDATE`

	row := dbTx.QueryRow(ctx, query, caseID)
	kase, err := r.scanSingleRow(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("moderation case not found: %s", caseID)
		}
		return nil, fmt.Errorf("get moderation case for update failed: %w", err)
	}

	return kase, nil
}

// Update persists case changes within a transaction.
func (r *ModerationRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	caseEntity *entity.GovernanceCase,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE moderation_cases
		SET status = $2,
		    reviewed_by = $3,
		    decision_note = $4,
		    reviewed_at = $5
		WHERE id = $1
	`,
		caseEntity.ID,
		string(caseEntity.Status),
		caseEntity.ReviewedBy,
		caseEntity.DecisionNote,
		caseEntity.ReviewedAt,
	)

	if err != nil {
		return fmt.Errorf("update governance case failed: %w", err)
	}

	return nil
}

// ListPending retrieves pending cases awaiting review.
// Ordered by created_at ASC (oldest first).
func (r *ModerationRepositoryImpl) ListPending(
	ctx context.Context,
	tx interface{},
	limit, offset int,
) ([]*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := dbTx.Query(ctx, query, entity.GovernanceCaseStatusPending, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending cases failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// ListByResource retrieves all cases for a specific resource.
// Useful for checking governance history.
func (r *ModerationRepositoryImpl) ListByResource(
	ctx context.Context,
	tx interface{},
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
) ([]*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC`

	rows, err := dbTx.Query(ctx, query, string(resourceType), resourceID)
	if err != nil {
		return nil, fmt.Errorf("list by resource failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// ListByReporter retrieves all cases created by a specific reporter.
// Ordered by created_at DESC (newest first).
// Supports pagination with limit and offset.
func (r *ModerationRepositoryImpl) ListByReporter(
	ctx context.Context,
	tx interface{},
	reporterID uuid.UUID,
	limit, offset int,
) ([]*entity.GovernanceCase, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases
		WHERE reported_by = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := dbTx.Query(ctx, query, reporterID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list by reporter failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// ListWithStatus retrieves cases filtered by status and optional resource type.
// If a filter is nil, that dimension is not applied.
// Ordered by created_at ASC (oldest first).
// Supports pagination with limit and offset.
func (r *ModerationRepositoryImpl) ListWithStatus(
	ctx context.Context,
	tx interface{},
	statusFilter *entity.GovernanceCaseStatus,
	resourceTypeFilter *entity.ResourceType,
	limit, offset int,
) ([]*entity.GovernanceCase, int64, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("invalid transaction type")
	}

	where := " WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if statusFilter != nil {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*statusFilter))
		argIdx++
	}

	if resourceTypeFilter != nil {
		where += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, string(*resourceTypeFilter))
		argIdx++
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM moderation_cases" + where
	if err := dbTx.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count moderation cases failed: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	query := `SELECT ` + selectColumns + `
		FROM moderation_cases` + where + fmt.Sprintf(`
		ORDER BY created_at ASC
		LIMIT $%d OFFSET $%d
	`, argIdx, argIdx+1)
	dataArgs := append(args, limit, offset)

	rows, err := dbTx.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list with status failed: %w", err)
	}
	defer rows.Close()

	cases, err := r.scanRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}

// ResourceExists checks if a resource exists in the system.
// Supported types: content, comment, for_sale, auction, user, chat_message
// Returns true if resource exists, false otherwise.
func (r *ModerationRepositoryImpl) ResourceExists(
	ctx context.Context,
	tx interface{},
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	var tableName string
	switch resourceType {
	case entity.ResourceTypeContent:
		tableName = "contents"
	case entity.ResourceTypeComment:
		tableName = "comments"
	case entity.ResourceTypeForSale:
		tableName = "for_sales"
	case entity.ResourceTypeAuction:
		tableName = "auctions"
	case entity.ResourceTypeUser:
		tableName = "users"
	case entity.ResourceTypeChatMessage:
		tableName = "chat_messages"
	default:
		// Unsupported types return false (resource doesn't exist for moderation purposes)
		return false, nil
	}

	// content / comment / user / chat_message: soft-delete pattern — guard deleted_at IS NULL.
	// fixed-price sale / auction: status-based lifecycle (no deleted_at column).
	// Reporting a withdrawn or cancelled fixed-price sale/auction is intentionally allowed;
	// the enforcement handler handles terminal states idempotently.
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE id = $1", tableName)
	if resourceType == entity.ResourceTypeContent || resourceType == entity.ResourceTypeComment || resourceType == entity.ResourceTypeUser || resourceType == entity.ResourceTypeChatMessage {
		query = fmt.Sprintf("SELECT 1 FROM %s WHERE id = $1 AND deleted_at IS NULL LIMIT 1", tableName)
	} else {
		query += " LIMIT 1"
	}

	var exists int
	err := dbTx.QueryRow(ctx, query, resourceID).Scan(&exists)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("check resource existence failed: %w", err)
	}

	return true, nil
}

// HasUserReportedResource checks if a user has already reported a specific resource.
// Returns true if the user has reported this resource before, false otherwise.
func (r *ModerationRepositoryImpl) HasUserReportedResource(
	ctx context.Context,
	tx interface{},
	reporterID uuid.UUID,
	resourceType entity.ResourceType,
	resourceID uuid.UUID,
) (bool, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, fmt.Errorf("invalid transaction type")
	}

	// moderation_cases has no deleted_at column — query all rows.
	query := `
		SELECT 1
		FROM moderation_cases
		WHERE reported_by = $1
		  AND resource_type = $2
		  AND resource_id = $3
		LIMIT 1
	`

	var exists int
	err := dbTx.QueryRow(ctx, query, reporterID, string(resourceType), resourceID).Scan(&exists)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("check existing report failed: %w", err)
	}

	return true, nil
}

// ValidateChatMessageReporter checks that the reporter is a room participant
// and the message's room is not a support room.
//
// Returns:
//   - (true, "", nil) if reporter is authorized
//   - (false, reason, nil) if reporter is not authorized (with human-readable reason)
//   - (false, "", err) on infrastructure failure
func (r *ModerationRepositoryImpl) ValidateChatMessageReporter(
	ctx context.Context,
	tx interface{},
	messageID uuid.UUID,
	reporterID uuid.UUID,
) (bool, string, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return false, "", fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT cr.room_type,
		       (cr.participant_a = $2 OR cr.participant_b = $2) AS is_participant
		FROM chat_messages cm
		JOIN chat_rooms cr ON cr.id = cm.room_id
		WHERE cm.id = $1 AND cm.deleted_at IS NULL
		LIMIT 1
	`

	var roomType string
	var isParticipant bool
	err := dbTx.QueryRow(ctx, query, messageID, reporterID).Scan(&roomType, &isParticipant)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, "chat message not found or deleted", nil
		}
		return false, "", fmt.Errorf("validate chat message reporter failed: %w", err)
	}

	if !isParticipant {
		return false, "you are not a participant in this chat room", nil
	}

	if roomType == "support" {
		return false, "reporting messages in support chats is not allowed", nil
	}

	return true, "", nil
}

// scanSingleRow scans a single governance case from a pgx.Row.
// Column order must match selectColumns.
func (r *ModerationRepositoryImpl) scanSingleRow(row pgx.Row) (*entity.GovernanceCase, error) {
	var id, resourceID, reportedBy uuid.UUID
	var resourceType, status string
	var reviewedBy *uuid.UUID
	var reason string
	var decisionNote *string
	var createdAt time.Time
	var reviewedAt *time.Time

	err := row.Scan(
		&id, &resourceType, &resourceID, &status,
		&reportedBy, &reviewedBy, &reason, &decisionNote,
		&createdAt, &reviewedAt,
	)

	if err != nil {
		return nil, err
	}

	return &entity.GovernanceCase{
		ID:           id,
		ResourceType: entity.ResourceType(resourceType),
		ResourceID:   resourceID,
		Status:       entity.GovernanceCaseStatus(status),
		ReportedBy:   reportedBy,
		ReviewedBy:   reviewedBy,
		Reason:       reason,
		DecisionNote: decisionNote,
		CreatedAt:    createdAt,
		ReviewedAt:   reviewedAt,
	}, nil
}

// scanRows scans multiple governance cases from pgx.Rows.
// Column order must match selectColumns.
func (r *ModerationRepositoryImpl) scanRows(rows pgx.Rows) ([]*entity.GovernanceCase, error) {
	var cases []*entity.GovernanceCase
	for rows.Next() {
		var id, resourceID, reportedBy uuid.UUID
		var resourceType, status string
		var reviewedBy *uuid.UUID
		var reason string
		var decisionNote *string
		var createdAt time.Time
		var reviewedAt *time.Time

		err := rows.Scan(
			&id, &resourceType, &resourceID, &status,
			&reportedBy, &reviewedBy, &reason, &decisionNote,
			&createdAt, &reviewedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("scan row failed: %w", err)
		}

		cases = append(cases, &entity.GovernanceCase{
			ID:           id,
			ResourceType: entity.ResourceType(resourceType),
			ResourceID:   resourceID,
			Status:       entity.GovernanceCaseStatus(status),
			ReportedBy:   reportedBy,
			ReviewedBy:   reviewedBy,
			Reason:       reason,
			DecisionNote: decisionNote,
			CreatedAt:    createdAt,
			ReviewedAt:   reviewedAt,
		})
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("scan rows failed: %w", rows.Err())
	}

	return cases, nil
}


