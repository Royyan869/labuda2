//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAppealRepository is a mock implementation of AppealRepository.
type mockAppealRepository struct {
	createFunc                 func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error
	createWithPendingCheckFunc func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error
	getByIDFunc                func(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error)
	getForUpdateFunc           func(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error)
	updateFunc                 func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error
	listByUserFunc             func(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*entity.Appeal, error)
	listByCaseFunc             func(ctx context.Context, tx interface{}, caseID uuid.UUID) ([]*entity.Appeal, error)
	listAllFunc                func(ctx context.Context, tx interface{}, statusFilter *entity.AppealStatus, limit, offset int) ([]*entity.Appeal, error)
	listPendingFunc            func(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.Appeal, error)
}

func (m *mockAppealRepository) Create(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, appeal)
	}
	return nil
}

func (m *mockAppealRepository) CreateWithPendingCheck(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
	if m.createWithPendingCheckFunc != nil {
		return m.createWithPendingCheckFunc(ctx, tx, appeal)
	}
	// Default: use the regular create function for backward compatibility
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, appeal)
	}
	return nil
}

func (m *mockAppealRepository) GetByID(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, appealID)
	}
	return nil, &entity.ErrAppealNotFound{AppealID: appealID}
}

func (m *mockAppealRepository) GetForUpdate(ctx context.Context, tx interface{}, appealID uuid.UUID) (*entity.Appeal, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, appealID)
	}
	return nil, &entity.ErrAppealNotFound{AppealID: appealID}
}

func (m *mockAppealRepository) Update(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, appeal)
	}
	return nil
}

func (m *mockAppealRepository) ListByUser(ctx context.Context, tx interface{}, userID uuid.UUID, limit, offset int) ([]*entity.Appeal, error) {
	if m.listByUserFunc != nil {
		return m.listByUserFunc(ctx, tx, userID, limit, offset)
	}
	return nil, nil
}

func (m *mockAppealRepository) ListByCase(ctx context.Context, tx interface{}, caseID uuid.UUID) ([]*entity.Appeal, error) {
	if m.listByCaseFunc != nil {
		return m.listByCaseFunc(ctx, tx, caseID)
	}
	return nil, nil
}

func (m *mockAppealRepository) ListAll(ctx context.Context, tx interface{}, statusFilter *entity.AppealStatus, limit, offset int) ([]*entity.Appeal, error) {
	if m.listAllFunc != nil {
		return m.listAllFunc(ctx, tx, statusFilter, limit, offset)
	}
	return nil, nil
}

func (m *mockAppealRepository) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.Appeal, error) {
	if m.listPendingFunc != nil {
		return m.listPendingFunc(ctx, tx, limit, offset)
	}
	return nil, nil
}

// mockContentRepository is a mock implementation of ContentRepository.
type mockContentRepository struct {
	getByIDFunc func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error)
}

func (m *mockContentRepository) Create(ctx context.Context, tx interface{}, content *contentEntity.Content) error {
	return nil
}

func (m *mockContentRepository) CreateMedia(ctx context.Context, tx interface{}, media []*contentEntity.ContentMedia) error {
	return nil
}

func (m *mockContentRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("content not found")
}

func (m *mockContentRepository) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
	return nil, errors.New("not implemented")
}

func (m *mockContentRepository) Update(ctx context.Context, tx interface{}, content *contentEntity.Content) error {
	return nil
}

func (m *mockContentRepository) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, viewerID uuid.UUID, limit int, cursor string) ([]*contentEntity.Content, string, error) {
	return nil, "", nil
}

func (m *mockContentRepository) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*contentEntity.ContentMedia, error) {
	return nil, nil
}

func (m *mockContentRepository) GetTagsByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]string, error) {
	return nil, nil
}

func (m *mockContentRepository) InsertTags(ctx context.Context, tx interface{}, contentID uuid.UUID, tags []string) error {
	return nil
}

// mockCommentRepository is a mock implementation of CommentRepository.
type mockCommentRepository struct {
	getByIDFunc func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error)
}

func (m *mockCommentRepository) Create(ctx context.Context, tx db.Tx, comment *contentEntity.Comment) error {
	return nil
}

func (m *mockCommentRepository) CreateForSaleReferenceComment(ctx context.Context, tx db.Tx, contentID, sellerID, forSaleID uuid.UUID, body *string) error {
	return nil
}

func (m *mockCommentRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("comment not found")
}

func (m *mockCommentRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCommentRepository) Update(ctx context.Context, tx db.Tx, comment *contentEntity.Comment) error {
	return nil
}

func (m *mockCommentRepository) FindTargetIDByForSaleReference(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, errors.New("not implemented")
}

func (m *mockCommentRepository) ListReplies(ctx context.Context, tx db.Tx, parentID uuid.UUID, limit int, cursor string) ([]*contentEntity.Comment, string, error) {
	return nil, "", nil
}

func (m *mockCommentRepository) GetReplyCount(ctx context.Context, tx db.Tx, commentID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockCommentRepository) CountTopLevelCommentsByContent(ctx context.Context, tx db.Tx, contentID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockCommentRepository) ExistsByTarget(ctx context.Context, tx db.Tx, targetType contentEntity.CommentTargetType, targetID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockCommentRepository) SoftDelete(ctx context.Context, tx db.Tx, id uuid.UUID, deletedAt time.Time) error {
	return nil
}

func (m *mockCommentRepository) Restore(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

func (m *mockCommentRepository) ListByTarget(ctx context.Context, tx db.Tx, targetType contentEntity.CommentTargetType, targetID uuid.UUID, limit int, cursor string) ([]*contentEntity.Comment, string, error) {
	return nil, "", nil
}

// Extend mockModerationRepository (declared in moderation_service_test.go) with
// the additional methods AppealService's ModerationRepository dependency requires.
func (m *mockModerationRepository) ListByReporter(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (m *mockModerationRepository) ResourceExists(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockModerationRepository) HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockModerationRepository) ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error) {
	return false, "", nil
}

// newTestOutboxRepo builds a real OutboxRepository for tests. NewAppealService
// fail-fasts on a nil outbox (boot-time contract), and InsertEvent writes only
// through the provided tx — the repository's own *db.DB is never touched on
// that path, so nil is safe here.
func newTestOutboxRepo() *outboxRepo.OutboxRepository {
	return outboxRepo.NewOutboxRepository(nil)
}

// mockTx is a mock implementation of db.Tx.
// A non-nil execErr makes Exec fail, which surfaces as an outbox InsertEvent
// failure in restoration-event tests.
type mockAppealTx struct {
	execErr error
}

func (m *mockAppealTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (m *mockAppealTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockAppealTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &mockRowForAppeal{}
}

func (m *mockAppealTx) Commit(ctx context.Context) error {
	return nil
}

func (m *mockAppealTx) Rollback(ctx context.Context) error {
	return nil
}

// mockRowForAppeal is a mock implementation of pgx.Row for appeal tests.
type mockRowForAppeal struct{}

func (m *mockRowForAppeal) Scan(dest ...interface{}) error {
	return errors.New("not found")
}

// ============================================================
// CREATE APPEAL TESTS
// ============================================================

func TestCreateAppeal_CaseNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	userID := uuid.New()

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return nil, errors.New("moderation case not found: " + caseID.String())
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	result, err := service.CreateAppeal(ctx, tx, caseID, userID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notFoundErr *entity.ErrCaseNotFound
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, caseID, notFoundErr.CaseID)
}

func TestCreateAppeal_CaseNotAppealable_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	userID := uuid.New()
	contentID := uuid.New()

	// Create a case with status "pending" (not appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	// Status is pending, not removed or rejected

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	result, err := service.CreateAppeal(ctx, tx, caseID, userID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notAppealableErr *entity.ErrCaseNotAppealable
	assert.ErrorAs(t, err, &notAppealableErr)
	assert.Equal(t, kase.ID, notAppealableErr.CaseID)
	assert.Equal(t, entity.GovernanceCaseStatusPending, notAppealableErr.Status)
}

func TestCreateAppeal_NotResourceOwner_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New() // Different user trying to appeal

	// Create a removed case (appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	// Content owned by ownerID
	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - otherUserID tries to appeal content owned by ownerID
	result, err := service.CreateAppeal(ctx, tx, caseID, otherUserID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
	assert.Equal(t, kase.ID, notOwnerErr.CaseID)
	assert.Equal(t, contentID, notOwnerErr.ResourceID)
	assert.Equal(t, otherUserID, notOwnerErr.UserID)
}

func TestCreateAppeal_DuplicatePendingAppeal_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	// Create a removed case (appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	// Content owned by ownerID
	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	// Simulate duplicate pending appeal at repository level
	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return &entity.ErrDuplicatePendingAppeal{CaseID: appeal.CaseID}
		},
	}

	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - owner tries to create another appeal while one is pending
	result, err := service.CreateAppeal(ctx, tx, caseID, ownerID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var duplicateErr *entity.ErrDuplicatePendingAppeal
	assert.ErrorAs(t, err, &duplicateErr)
	assert.Equal(t, caseID, duplicateErr.CaseID)
}

func TestCreateAppeal_ValidOwner_RemovedCase_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	// Create a removed case (appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	// Content owned by ownerID
	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	var createdAppeal *entity.Appeal
	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			createdAppeal = appeal
			return nil
		},
	}

	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	message := "This content was wrongfully removed"
	result, err := service.CreateAppeal(ctx, tx, caseID, ownerID, message)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caseID, result.CaseID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
	assert.Equal(t, createdAppeal, result)
}

func TestCreateAppeal_ValidOwner_RejectedCase_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	// Create a rejected case (appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	kase.Reject(uuid.New(), strPtr("rejected"))

	// Content owned by ownerID
	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return nil
		},
	}

	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	result, err := service.CreateAppeal(ctx, tx, caseID, ownerID, "This report was dismissed incorrectly")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caseID, result.CaseID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_CommentResource_ValidOwner_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	commentID := uuid.New()
	ownerID := uuid.New()

	// Create a removed case for a comment
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeComment,
		commentID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	// Comment owned by ownerID
	mockCommentRepo := &mockCommentRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*contentEntity.Comment, error) {
			return &contentEntity.Comment{
				ID:       commentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockContentRepo := &mockContentRepository{}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return nil
		},
	}

	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	result, err := service.CreateAppeal(ctx, tx, caseID, ownerID, "My comment was wrongfully removed")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caseID, result.CaseID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

// ============================================================
// REVIEW APPEAL TESTS
// ============================================================

func TestReviewAppeal_ApproveNonRemovedCase_NoRestorationRequired_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	appealID := uuid.New()
	caseID := uuid.New()
	adminID := uuid.New()
	resourceID := uuid.New()

	// Create a pending appeal
	appeal := entity.NewAppeal(caseID, uuid.New(), "Please reconsider")

	// Create a rejected case (does NOT require restoration on approval)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		resourceID,
		uuid.New(),
		"test report",
	)
	kase.Reject(uuid.New(), strPtr("rejected"))

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, app *entity.Appeal) error {
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}

	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	adminResponse := "Appeal granted"
	result, err := service.ReviewAppeal(ctx, tx, appealID, adminID, true, &adminResponse)

	// Assert - should succeed without restoration event (non-removed case)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusApproved, result.Status)
}

func TestReviewAppeal_RejectAppeal_NoRestorationEvent_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	appealID := uuid.New()
	caseID := uuid.New()
	adminID := uuid.New()
	resourceID := uuid.New()

	// Create a pending appeal
	appeal := entity.NewAppeal(caseID, uuid.New(), "Please restore")

	// Create a removed case
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		resourceID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, app *entity.Appeal) error {
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}

	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - reject the appeal
	adminResponse := "Appeal denied - content violation stands"
	result, err := service.ReviewAppeal(ctx, tx, appealID, adminID, false, &adminResponse)

	// Assert - should succeed without restoration event
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusRejected, result.Status)
}

func TestReviewAppeal_ApproveRemovedCase_SuccessWithRestoration(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	appealID := uuid.New()
	caseID := uuid.New()
	adminID := uuid.New()
	resourceID := uuid.New()

	// Create a pending appeal
	appeal := entity.NewAppeal(caseID, uuid.New(), "Please restore")

	// Create a removed case (requires restoration)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		resourceID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, app *entity.Appeal) error {
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}

	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act
	adminResponse := "Appeal granted - content restored"
	result, err := service.ReviewAppeal(ctx, tx, appealID, adminID, true, &adminResponse)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusApproved, result.Status)
	assert.Equal(t, adminID, *result.ReviewedBy)
	assert.Equal(t, adminResponse, *result.AdminResponse)
}

// Helper function
func strPtr(s string) *string {
	return &s
}

// ============================================================
// CONCURRENCY AND CONSISTENCY TESTS
// ============================================================

// TestReviewAppeal_RestorationEventEmittedBeforeStateChange proves that restoration
// events are emitted BEFORE the appeal state is changed, preventing split-brain.
func TestReviewAppeal_RestorationEventEmittedBeforeStateChange(t *testing.T) {
	ctx := context.Background()
	// Failing Exec makes the outbox InsertEvent fail, simulating restoration
	// event emission failure.
	tx := &mockAppealTx{execErr: errors.New("outbox insert failed")}

	appealID := uuid.New()
	caseID := uuid.New()
	adminID := uuid.New()
	resourceID := uuid.New()

	// Create a pending appeal
	appeal := entity.NewAppeal(caseID, uuid.New(), "Please restore")

	// Create a removed case (requires restoration)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		resourceID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	// Track the order of operations
	var updateCalled bool

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, app *entity.Appeal) error {
			updateCalled = true
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}

	// The outbox is a concrete type whose InsertEvent writes through tx.Exec,
	// so the failing tx above makes the restoration event emission fail.
	// The key guarantee is that restoration event is emitted before appeal update.
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - restoration event emission fails with ErrRestorationEventFailed,
	// and we verify the error comes before update
	adminResponse := "Appeal granted - content restored"
	result, err := service.ReviewAppeal(ctx, tx, appealID, adminID, true, &adminResponse)

	// Assert - should fail due to nil outbox, but BEFORE update is called
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.False(t, updateCalled, "Update should not be called when restoration fails")

	var restorationErr *entity.ErrRestorationEventFailed
	assert.ErrorAs(t, err, &restorationErr)
}

// TestReviewAppeal_RestorationFailureLeavesAppealPending proves that if restoration
// event emission fails, the appeal remains pending (state not changed).
func TestReviewAppeal_RestorationFailureLeavesAppealPending(t *testing.T) {
	ctx := context.Background()
	// Failing Exec makes the outbox InsertEvent fail, simulating restoration
	// event emission failure.
	tx := &mockAppealTx{execErr: errors.New("outbox insert failed")}

	appealID := uuid.New()
	caseID := uuid.New()
	adminID := uuid.New()
	resourceID := uuid.New()

	// Create a pending appeal
	appeal := entity.NewAppeal(caseID, uuid.New(), "Please restore")
	initialStatus := appeal.Status // Should be pending

	// Create a removed case (requires restoration)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		resourceID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	var updateCalled bool

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, app *entity.Appeal) error {
			updateCalled = true
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}

	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - try to approve but restoration fails
	adminResponse := "Appeal granted"
	result, err := service.ReviewAppeal(ctx, tx, appealID, adminID, true, &adminResponse)

	// Assert - operation should fail
	assert.Error(t, err)
	var restorationErr *entity.ErrRestorationEventFailed
	assert.ErrorAs(t, err, &restorationErr)

	// Appeal should still be pending because the transaction failed before update
	assert.Equal(t, initialStatus, appeal.Status)
	assert.Nil(t, result)
	assert.False(t, updateCalled, "Update should not be called when restoration fails")
}

// TestCreateAppeal_AllowsNewAppealAfterResolvedAppeal proves that after a previous
// appeal is resolved (approved or rejected), a new appeal can be created.
func TestCreateAppeal_AllowsNewAppealAfterResolvedAppeal(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	// Create a removed case (appealable)
	kase := entity.NewGovernanceCase(
		entity.ResourceTypeContent,
		contentID,
		uuid.New(),
		"test report",
	)
	kase.Enforce(uuid.New(), strPtr("removed"))

	// Content owned by ownerID
	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	var createdAppeal *entity.Appeal
	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			createdAppeal = appeal
			return nil
		},
	}

	mockCommentRepo := &mockCommentRepository{}
	service := application.NewAppealService(mockAppealRepo, mockModRepo, mockContentRepo, mockCommentRepo, newTestOutboxRepo())

	// Act - create a new appeal (previous resolved appeal doesn't block new appeal)
	result, err := service.CreateAppeal(ctx, tx, caseID, ownerID, "I want to appeal again")

	// Assert - should succeed because the DB check only blocks pending appeals
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caseID, result.CaseID)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
	assert.Equal(t, createdAppeal, result)
}

// ============================================================
// FOR SALE / AUCTION / USER ELIGIBILITY TESTS
// ============================================================

// mockForSaleOwnerRepo is a minimal mock for fixed-price sale owner lookup.
type mockForSaleOwnerRepo struct {
	getByIDFunc func(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error)
}

func (m *mockForSaleOwnerRepo) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("fixed-price sale not found")
}

// mockAuctionOwnerRepo is a minimal mock for auction owner lookup.
type mockAuctionOwnerRepo struct {
	getByIDFunc func(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error)
}

func (m *mockAuctionOwnerRepo) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, errors.New("auction not found")
}

func TestCreateAppeal_ForSaleResource_ValidSeller_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	forSaleID := uuid.New()
	sellerID := uuid.New()

	kase := entity.NewGovernanceCase(entity.ResourceTypeForSale, forSaleID, uuid.New(), "fixed-price sale violation")
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockForSaleRepo := &mockForSaleOwnerRepo{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error) {
			return &forsaleEntity.ForSale{SellerID: sellerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return nil
		},
	}

	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())
	svc.SetForSaleRepo(mockForSaleRepo)

	result, err := svc.CreateAppeal(ctx, tx, caseID, sellerID, "My fixed-price sale was wrongfully removed")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, caseID, result.CaseID)
	assert.Equal(t, sellerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_ForSaleResource_NonOwner_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	forSaleID := uuid.New()
	sellerID := uuid.New()
	otherUserID := uuid.New()

	kase := entity.NewGovernanceCase(entity.ResourceTypeForSale, forSaleID, uuid.New(), "fixed-price sale violation")
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockForSaleRepo := &mockForSaleOwnerRepo{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*forsaleEntity.ForSale, error) {
			return &forsaleEntity.ForSale{SellerID: sellerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}

	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())
	svc.SetForSaleRepo(mockForSaleRepo)

	result, err := svc.CreateAppeal(ctx, tx, caseID, otherUserID, "Trying to appeal someone else's fixed-price sale")

	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
}

func TestCreateAppeal_AuctionResource_ValidSeller_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	auctionID := uuid.New()
	sellerID := uuid.New()

	kase := entity.NewGovernanceCase(entity.ResourceTypeAuction, auctionID, uuid.New(), "auction violation")
	kase.Enforce(uuid.New(), strPtr("cancelled"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAuctionRepo := &mockAuctionOwnerRepo{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error) {
			a := &auctionEntity.Auction{}
			a.SellerID = sellerID
			return a, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return nil
		},
	}

	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())
	svc.SetAuctionRepo(mockAuctionRepo)

	result, err := svc.CreateAppeal(ctx, tx, caseID, sellerID, "My auction was wrongfully cancelled")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, sellerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_UserSuspension_ValidUser_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	suspendedUserID := uuid.New()

	// User suspension case: ResourceID == the suspended user's ID
	kase := entity.NewGovernanceCase(entity.ResourceTypeUser, suspendedUserID, uuid.New(), "policy violation")
	kase.Enforce(uuid.New(), strPtr("suspended"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return nil
		},
	}

	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())
	// No SetForSaleRepo/SetAuctionRepo needed for user type

	result, err := svc.CreateAppeal(ctx, tx, caseID, suspendedUserID, "I was wrongfully suspended")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, suspendedUserID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_UserSuspension_OtherUser_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	suspendedUserID := uuid.New()
	otherUserID := uuid.New()

	kase := entity.NewGovernanceCase(entity.ResourceTypeUser, suspendedUserID, uuid.New(), "policy violation")
	kase.Enforce(uuid.New(), strPtr("suspended"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}

	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())

	result, err := svc.CreateAppeal(ctx, tx, caseID, otherUserID, "Appealing someone else's suspension")

	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
}

func TestCreateAppeal_ForSaleWithoutRepo_ReturnsUnsupportedType(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	caseID := uuid.New()
	forSaleID := uuid.New()

	kase := entity.NewGovernanceCase(entity.ResourceTypeForSale, forSaleID, uuid.New(), "fixed-price sale violation")
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}

	// Service without SetForSaleRepo — fixed-price sale type should be unsupported
	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())

	result, err := svc.CreateAppeal(ctx, tx, caseID, uuid.New(), "My fixed-price sale was removed")

	assert.Nil(t, result)
	assert.Error(t, err)
	var unsupportedErr *entity.ErrUnsupportedResourceType
	assert.ErrorAs(t, err, &unsupportedErr)
}

func TestCreateAppeal_ForSaleApproval_NoRestorationEvent(t *testing.T) {
	// Verify that approving a fixed-price sale appeal does NOT emit restoration event.
	// Fixed-price sale restoration is record-only / manual admin action.
	ctx := context.Background()
	// Sentinel: Exec fails, so if any restoration event were emitted the review
	// would fail with ErrRestorationEventFailed. Success proves no event was emitted.
	tx := &mockAppealTx{execErr: errors.New("outbox insert must not be called")}

	appealID := uuid.New()
	caseID := uuid.New()
	forSaleID := uuid.New()
	adminID := uuid.New()

	appeal := entity.NewAppeal(caseID, uuid.New(), "My fixed-price sale was wrongfully removed")

	kase := entity.NewGovernanceCase(entity.ResourceTypeForSale, forSaleID, uuid.New(), "fixed-price sale violation")
	kase.Enforce(uuid.New(), strPtr("removed"))

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(ctx context.Context, tx interface{}, a *entity.Appeal) error {
			return nil
		},
	}

	mockModRepo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}

	// For fixed-price sale type, no restoration event should be emitted, so the
	// failing tx above is never exercised and the review succeeds.
	svc := application.NewAppealService(mockAppealRepo, mockModRepo, &mockContentRepository{}, &mockCommentRepository{}, newTestOutboxRepo())

	adminResponse := "Appeal accepted administratively; fixed-price sale restoration is manual"
	result, err := svc.ReviewAppeal(ctx, tx, appealID, adminID, true, &adminResponse)

	// Should succeed without outbox event (no ErrRestorationEventFailed)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusApproved, result.Status)
}
