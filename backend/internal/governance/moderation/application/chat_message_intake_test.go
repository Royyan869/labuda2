package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================================
// TEST INFRASTRUCTURE
// =====================================================================

type cmTransactor struct{}

func (m *cmTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&cmTx{})
}

type cmTx struct{}

func (m *cmTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *cmTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}
func (m *cmTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &cmRow{}
}
func (m *cmTx) Commit(ctx context.Context) error   { return nil }
func (m *cmTx) Rollback(ctx context.Context) error  { return nil }

type cmRow struct{}

func (m *cmRow) Scan(dest ...interface{}) error { return errors.New("not found") }

// cmRepo implements the full ModerationRepository interface.
type cmRepo struct {
	createFunc                    func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error
	getByIDFunc                   func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
	getForUpdateFunc              func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
	updateFunc                    func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error
	listPendingFunc               func(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error)
	listByResourceFunc            func(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error)
	listByReporterFunc            func(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error)
	listWithStatusFunc            func(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error)
	resourceExistsFunc            func(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error)
	hasUserReportedResourceFunc   func(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error)
	validateChatMessageReporterFn func(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error)
}

func (m *cmRepo) Create(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, kase)
	}
	return nil
}
func (m *cmRepo) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}
func (m *cmRepo) GetForUpdate(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}
func (m *cmRepo) Update(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, kase)
	}
	return nil
}
func (m *cmRepo) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error) {
	if m.listPendingFunc != nil {
		return m.listPendingFunc(ctx, tx, limit, offset)
	}
	return nil, nil
}
func (m *cmRepo) ListByResource(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error) {
	if m.listByResourceFunc != nil {
		return m.listByResourceFunc(ctx, tx, resourceType, resourceID)
	}
	return nil, nil
}
func (m *cmRepo) ListByReporter(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error) {
	if m.listByReporterFunc != nil {
		return m.listByReporterFunc(ctx, tx, reporterID, limit, offset)
	}
	return nil, nil
}
func (m *cmRepo) ListWithStatus(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error) {
	if m.listWithStatusFunc != nil {
		return m.listWithStatusFunc(ctx, tx, statusFilter, resourceTypeFilter, limit, offset)
	}
	return nil, 0, nil
}
func (m *cmRepo) ResourceExists(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.resourceExistsFunc != nil {
		return m.resourceExistsFunc(ctx, tx, resourceType, resourceID)
	}
	return true, nil
}
func (m *cmRepo) HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.hasUserReportedResourceFunc != nil {
		return m.hasUserReportedResourceFunc(ctx, tx, reporterID, resourceType, resourceID)
	}
	return false, nil
}
func (m *cmRepo) ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error) {
	if m.validateChatMessageReporterFn != nil {
		return m.validateChatMessageReporterFn(ctx, tx, messageID, reporterID)
	}
	return true, "", nil
}

// =====================================================================
// CHAT MESSAGE INTAKE TESTS
// =====================================================================

func TestCreateCase_ChatMessage_SucceedsForRoomParticipant(t *testing.T) {
	messageID := uuid.New()
	reporterID := uuid.New()
	var createdCase *entity.GovernanceCase

	repo := &cmRepo{
		createFunc: func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
			createdCase = kase
			return nil
		},
		resourceExistsFunc: func(ctx context.Context, tx interface{}, rt entity.ResourceType, rid uuid.UUID) (bool, error) {
			return true, nil
		},
		validateChatMessageReporterFn: func(ctx context.Context, tx interface{}, mid uuid.UUID, rid uuid.UUID) (bool, string, error) {
			assert.Equal(t, messageID, mid)
			assert.Equal(t, reporterID, rid)
			return true, "", nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	kase, err := service.CreateCase(context.Background(), entity.ResourceTypeChatMessage, messageID, reporterID, "Threatening message")

	require.NoError(t, err)
	require.NotNil(t, kase)
	assert.Equal(t, entity.ResourceTypeChatMessage, kase.ResourceType)
	assert.Equal(t, messageID, kase.ResourceID)
	assert.Equal(t, reporterID, kase.ReportedBy)
	assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
	assert.NotNil(t, createdCase)
}

func TestCreateCase_ChatMessage_RejectsNonParticipant(t *testing.T) {
	repo := &cmRepo{
		resourceExistsFunc: func(ctx context.Context, tx interface{}, rt entity.ResourceType, rid uuid.UUID) (bool, error) {
			return true, nil
		},
		validateChatMessageReporterFn: func(ctx context.Context, tx interface{}, mid uuid.UUID, rid uuid.UUID) (bool, string, error) {
			return false, "you are not a participant in this chat room", nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	_, err := service.CreateCase(context.Background(), entity.ResourceTypeChatMessage, uuid.New(), uuid.New(), "spam")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a participant")
}

func TestCreateCase_ChatMessage_RejectsSupportRoom(t *testing.T) {
	repo := &cmRepo{
		resourceExistsFunc: func(ctx context.Context, tx interface{}, rt entity.ResourceType, rid uuid.UUID) (bool, error) {
			return true, nil
		},
		validateChatMessageReporterFn: func(ctx context.Context, tx interface{}, mid uuid.UUID, rid uuid.UUID) (bool, string, error) {
			return false, "reporting messages in support chats is not allowed", nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	_, err := service.CreateCase(context.Background(), entity.ResourceTypeChatMessage, uuid.New(), uuid.New(), "spam")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "support chats is not allowed")
}

func TestCreateCase_ChatMessage_RejectsDeletedMessage(t *testing.T) {
	repo := &cmRepo{
		resourceExistsFunc: func(ctx context.Context, tx interface{}, rt entity.ResourceType, rid uuid.UUID) (bool, error) {
			return false, nil // Message deleted → ResourceExists returns false
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	_, err := service.CreateCase(context.Background(), entity.ResourceTypeChatMessage, uuid.New(), uuid.New(), "spam")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource not found")
}

func TestCreateCase_ChatMessage_ContentCommentUserUnaffected(t *testing.T) {
	for _, rt := range []entity.ResourceType{entity.ResourceTypeContent, entity.ResourceTypeComment, entity.ResourceTypeUser} {
		t.Run(string(rt), func(t *testing.T) {
			validateCalled := false
			repo := &cmRepo{
				createFunc: func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
					return nil
				},
				resourceExistsFunc: func(ctx context.Context, tx interface{}, resType entity.ResourceType, rid uuid.UUID) (bool, error) {
					return true, nil
				},
				validateChatMessageReporterFn: func(ctx context.Context, tx interface{}, mid uuid.UUID, rid uuid.UUID) (bool, string, error) {
					validateCalled = true
					return true, "", nil
				},
			}

			service := application.NewModerationService(&cmTransactor{}, repo, nil)
			kase, err := service.CreateCase(context.Background(), rt, uuid.New(), uuid.New(), "test reason")

			require.NoError(t, err)
			require.NotNil(t, kase)
			assert.Equal(t, rt, kase.ResourceType)
			assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
			assert.False(t, validateCalled, "ValidateChatMessageReporter should NOT be called for %s", rt)
		})
	}
}

// =====================================================================
// ENFORCE TESTS
// =====================================================================

func TestReviewCase_ChatMessage_EnforceAllowed(t *testing.T) {
	// Phase 3: chat_message enforce is now allowed (soft-hide path).
	// With nil outboxRepo, this will fail at outbox insertion (nil pointer),
	// but the error must NOT be about "enforcement for chat_message".
	caseID := uuid.New()
	adminID := uuid.New()
	messageID := uuid.New()
	note := "remove this message"

	kase := entity.NewGovernanceCase(entity.ResourceTypeChatMessage, messageID, uuid.New(), "spam")

	repo := &cmRepo{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	_, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionEnforce, &note)

	// Error will be about outbox insertion (nil pointer), NOT about chat_message block
	if err != nil {
		assert.NotContains(t, err.Error(), "enforcement for chat_message is not yet supported")
	}
}

func TestReviewCase_ChatMessage_ApproveAllowed(t *testing.T) {
	caseID := uuid.New()
	adminID := uuid.New()
	kase := entity.NewGovernanceCase(entity.ResourceTypeChatMessage, uuid.New(), uuid.New(), "spam")
	note := "not spam"

	repo := &cmRepo{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, k *entity.GovernanceCase) error {
			return nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	result, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionApprove, &note)

	require.NoError(t, err)
	assert.Equal(t, entity.GovernanceCaseStatusApproved, result.Status)
}

func TestReviewCase_ChatMessage_RejectAllowed(t *testing.T) {
	caseID := uuid.New()
	adminID := uuid.New()
	kase := entity.NewGovernanceCase(entity.ResourceTypeChatMessage, uuid.New(), uuid.New(), "spam")
	note := "false positive"

	repo := &cmRepo{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, k *entity.GovernanceCase) error {
			return nil
		},
	}

	service := application.NewModerationService(&cmTransactor{}, repo, nil)
	result, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionReject, &note)

	require.NoError(t, err)
	assert.Equal(t, entity.GovernanceCaseStatusRejected, result.Status)
}

func TestReviewCase_Content_EnforceStillAllowed(t *testing.T) {
	// Verify existing content enforcement is NOT blocked
	caseID := uuid.New()
	adminID := uuid.New()
	kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "spam")
	note := "remove it"

	repo := &cmRepo{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, k *entity.GovernanceCase) error {
			return nil
		},
	}

	// Note: outboxRepo is nil, so this will panic if it tries to emit an event.
	// But the service should still transition the state before emitting.
	// We test that the enforce block does NOT apply to content.
	service := application.NewModerationService(&cmTransactor{}, repo, nil)

	// This will fail at outbox insertion (nil outboxRepo) but that's after the enforce check.
	// We verify the error is NOT about "enforcement for chat_message".
	_, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionEnforce, &note)

	// Error will be about outbox insertion (nil pointer), NOT about chat_message block
	if err != nil {
		assert.NotContains(t, err.Error(), "enforcement for chat_message")
	}
}

// =====================================================================
// ENTITY TESTS
// =====================================================================

func TestResourceTypeChatMessage_IsValid(t *testing.T) {
	assert.True(t, entity.ResourceTypeChatMessage.IsValid())
	assert.Equal(t, "chat_message", entity.ResourceTypeChatMessage.String())
}

func TestResourceTypeChatMessage_ExistingTypesStillValid(t *testing.T) {
	for _, rt := range []entity.ResourceType{
		entity.ResourceTypeContent,
		entity.ResourceTypeComment,
		entity.ResourceTypeForSale,
		entity.ResourceTypeAuction,
		entity.ResourceTypeUser,
	} {
		assert.True(t, rt.IsValid(), "expected %s to be valid", rt)
	}
}


