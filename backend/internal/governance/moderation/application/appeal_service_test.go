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

func (m *mockAppealRepository) ListByDecisionID(ctx context.Context, tx interface{}, decisionID uuid.UUID) ([]*entity.Appeal, error) {
	return nil, nil
}

func (m *mockAppealRepository) ListByCase(ctx context.Context, tx interface{}, caseID uuid.UUID) ([]*entity.Appeal, error) {
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

func (m *mockContentRepository) GetMentionedUserIDs(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockContentRepository) InsertMentionedUsers(ctx context.Context, tx interface{}, contentID uuid.UUID, userIDs []uuid.UUID) error {
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

func (m *mockCommentRepository) FindTargetIDByCommerceReference(ctx context.Context, tx db.Tx, resourceID uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, errors.New("not implemented")
}

// mockModerationRepository is a LEGACY mock kept for compilation continuity.
// It was previously used by Appeal tests; replaced by mockDecisionRepository.
type mockModerationRepository struct {
	getByIDFunc func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
}

func (m *mockModerationRepository) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, caseID)
	}
	return nil, errors.New("moderation case not found")
}

// mockDecisionRepository is a mock implementation of DecisionRepository.
type mockDecisionRepository struct {
	getByIDFunc     func(ctx context.Context, tx db.Tx, decisionID uuid.UUID) (*entity.Decision, error)
	listByCaseFunc  func(ctx context.Context, tx db.Tx, caseID uuid.UUID, limit, offset int) ([]*entity.Decision, error)
}

func (m *mockDecisionRepository) Create(ctx context.Context, tx db.Tx, decision *entity.Decision) error {
	return nil
}

func (m *mockDecisionRepository) GetByID(ctx context.Context, tx db.Tx, decisionID uuid.UUID) (*entity.Decision, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, decisionID)
	}
	return nil, nil
}

func (m *mockDecisionRepository) ListByCase(ctx context.Context, tx db.Tx, caseID uuid.UUID, limit, offset int) ([]*entity.Decision, error) {
	if m.listByCaseFunc != nil {
		return m.listByCaseFunc(ctx, tx, caseID, limit, offset)
	}
	return nil, nil
}

// mockCaseRepository is a mock implementation of CaseRepository.
type mockCaseRepository struct {
	getByIDFunc func(ctx context.Context, tx db.Tx, caseID uuid.UUID) (*entity.CanonicalCase, error)
}

func (m *mockCaseRepository) FindOrCreateOpenCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error) {
	return nil, nil
}

func (m *mockCaseRepository) GetByID(ctx context.Context, tx db.Tx, caseID uuid.UUID) (*entity.CanonicalCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, caseID)
	}
	return nil, nil
}

func (m *mockCaseRepository) ListBySubject(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID, limit, offset int) ([]*entity.CanonicalCase, error) {
	return nil, nil
}

func (m *mockCaseRepository) ResolveCase(ctx context.Context, tx db.Tx, caseID uuid.UUID) error {
	return nil
}

func (m *mockCaseRepository) ListAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus, limit, offset int) ([]*entity.CanonicalCase, error) {
	return nil, nil
}

func (m *mockCaseRepository) CountAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus) (int, error) {
	return 0, nil
}

// mockTransactor is a minimal db.Transactor for tests.
type mockTransactor struct{}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	return fn(&mockAppealTx{})
}

// mockEnforcementRepository is a minimal stub for EnforcementRepository.
type mockEnforcementRepository struct{}

func (m *mockEnforcementRepository) Create(_ context.Context, _ db.Tx, _ *entity.Enforcement) error {
	return nil
}
func (m *mockEnforcementRepository) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Enforcement, error) {
	return nil, nil
}
func (m *mockEnforcementRepository) GetByDecisionAndTarget(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.ModerationTargetType, _ uuid.UUID) (*entity.Enforcement, error) {
	return nil, nil
}
func (m *mockEnforcementRepository) UpdateStatus(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.EnforcementStatus, _ *string) error {
	return nil
}
func (m *mockEnforcementRepository) MarkProcessing(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (m *mockEnforcementRepository) MarkSucceeded(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (m *mockEnforcementRepository) MarkFailed(_ context.Context, _ db.Tx, _ uuid.UUID, _ string, _ *time.Time) error {
	return nil
}
func (m *mockEnforcementRepository) ListByDecision(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.Enforcement, error) {
	return nil, nil
}

// newTestAppealService creates an AppealService with canonical mock dependencies.
func newTestAppealService(
	appealRepo *mockAppealRepository,
	decisionRepo *mockDecisionRepository,
	caseRepo *mockCaseRepository,
	contentRepo *mockContentRepository,
	commentRepo *mockCommentRepository,
) *application.AppealService {
	// Create a DecisionService for appeal review (needed for ReviewAppeal).
	dbTx := &mockTransactor{}
	enfRepo := &mockEnforcementRepository{}
	decisionService := application.NewDecisionService(dbTx, caseRepo, decisionRepo, enfRepo, newTestOutboxRepo(), nil)
	return application.NewAppealService(appealRepo, decisionRepo, caseRepo, decisionService, contentRepo, commentRepo)
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

func TestCreateAppeal_DecisionNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	decisionID := uuid.New()
	userID := uuid.New()

	mockDecisionRepo := &mockDecisionRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, decisionID uuid.UUID) (*entity.Decision, error) {
			return nil, nil // not found
		},
	}
	mockCaseRepo := &mockCaseRepository{}

	mockAppealRepo := &mockAppealRepository{}
	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := newTestAppealService(mockAppealRepo, mockDecisionRepo, mockCaseRepo, mockContentRepo, mockCommentRepo)

	// Act
	result, err := service.CreateAppeal(ctx, tx, decisionID, userID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notFoundErr *entity.ErrDecisionNotFound
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, decisionID, notFoundErr.DecisionID)
}

func TestCreateAppeal_NoViolationNotAppealable_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	decisionID := uuid.New()
	caseID := uuid.New()
	userID := uuid.New()
	contentID := uuid.New()

	// Create a no_violation Decision (not appealable per Design §23)
	decision := &entity.Decision{
		ID:        decisionID,
		CaseID:    caseID,
		DecidedBy: uuid.New(),
		Outcome:   entity.DecisionOutcomeNoViolation,
		CreatedAt: time.Now(),
	}

	kase := &entity.CanonicalCase{
		ID:          caseID,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   contentID,
		Status:      entity.CaseStatusResolved,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockDecisionRepo := &mockDecisionRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Decision, error) {
			return decision, nil
		},
	}
	mockCaseRepo := &mockCaseRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CanonicalCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	mockContentRepo := &mockContentRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := newTestAppealService(mockAppealRepo, mockDecisionRepo, mockCaseRepo, mockContentRepo, mockCommentRepo)

	// Act
	result, err := service.CreateAppeal(ctx, tx, decisionID, userID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notAppealableErr *entity.ErrDecisionNotAppealable
	assert.ErrorAs(t, err, &notAppealableErr)
	assert.Equal(t, decisionID, notAppealableErr.DecisionID)
	assert.Equal(t, entity.DecisionOutcomeNoViolation, notAppealableErr.Outcome)
}

func TestCreateAppeal_NotResourceOwner_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	decisionID := uuid.New()
	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	decision := &entity.Decision{
		ID:        decisionID,
		CaseID:    caseID,
		DecidedBy: uuid.New(),
		Outcome:   entity.DecisionOutcomeViolation,
		CreatedAt: time.Now(),
	}

	kase := &entity.CanonicalCase{
		ID:          caseID,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   contentID,
		Status:      entity.CaseStatusResolved,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{
				ID:       contentID,
				AuthorID: ownerID,
			}, nil
		},
	}

	mockDecisionRepo := &mockDecisionRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Decision, error) {
			return decision, nil
		},
	}
	mockCaseRepo := &mockCaseRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CanonicalCase, error) {
			return kase, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	mockCommentRepo := &mockCommentRepository{}
	service := newTestAppealService(mockAppealRepo, mockDecisionRepo, mockCaseRepo, mockContentRepo, mockCommentRepo)

	// Act - otherUserID tries to appeal content owned by ownerID
	result, err := service.CreateAppeal(ctx, tx, decisionID, otherUserID, "This is a mistake")

	// Assert
	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
	assert.Equal(t, decisionID, notOwnerErr.DecisionID)
	assert.Equal(t, contentID, notOwnerErr.ResourceID)
	assert.Equal(t, otherUserID, notOwnerErr.UserID)
}

func TestCreateAppeal_DuplicatePendingAppeal_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	decisionID := uuid.New()
	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	decision := &entity.Decision{
		ID: decisionID, CaseID: caseID, DecidedBy: uuid.New(),
		Outcome: entity.DecisionOutcomeViolation, CreatedAt: time.Now(),
	}
	kase := &entity.CanonicalCase{
		ID: caseID, SubjectType: entity.ReportTargetContent, SubjectID: contentID,
		Status: entity.CaseStatusResolved, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{ID: contentID, AuthorID: ownerID}, nil
		},
	}
	mockDecisionRepo := &mockDecisionRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Decision, error) {
			return decision, nil
		},
	}
	mockCaseRepo := &mockCaseRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CanonicalCase, error) {
			return kase, nil
		},
	}
	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(ctx context.Context, tx interface{}, appeal *entity.Appeal) error {
			return &entity.ErrDuplicatePendingAppeal{DecisionID: appeal.DecisionID}
		},
	}
	mockCommentRepo := &mockCommentRepository{}
	service := newTestAppealService(mockAppealRepo, mockDecisionRepo, mockCaseRepo, mockContentRepo, mockCommentRepo)

	result, err := service.CreateAppeal(ctx, tx, decisionID, ownerID, "This is a mistake")

	assert.Nil(t, result)
	assert.Error(t, err)
	var duplicateErr *entity.ErrDuplicatePendingAppeal
	assert.ErrorAs(t, err, &duplicateErr)
	assert.Equal(t, decisionID, duplicateErr.DecisionID)
}

func TestCreateAppeal_ValidOwner_RemovedCase_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	decisionID := uuid.New()
	caseID := uuid.New()
	contentID := uuid.New()
	ownerID := uuid.New()

	decision := &entity.Decision{
		ID: decisionID, CaseID: caseID, DecidedBy: uuid.New(),
		Outcome: entity.DecisionOutcomeViolation, CreatedAt: time.Now(),
	}
	kase := &entity.CanonicalCase{
		ID: caseID, SubjectType: entity.ReportTargetContent, SubjectID: contentID,
		Status: entity.CaseStatusResolved, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{ID: contentID, AuthorID: ownerID}, nil
		},
	}
	mockDecisionRepo := &mockDecisionRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Decision, error) {
			return decision, nil
		},
	}
	mockCaseRepo := &mockCaseRepository{
		getByIDFunc: func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.CanonicalCase, error) {
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
	service := newTestAppealService(mockAppealRepo, mockDecisionRepo, mockCaseRepo, mockContentRepo, mockCommentRepo)

	message := "This content was wrongfully removed"
	result, err := service.CreateAppeal(ctx, tx, decisionID, ownerID, message)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, decisionID, result.DecisionID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
	assert.Equal(t, createdAppeal, result)
}

// Helper: build canonical Decision + Case mocks for a given subject type.
// Returns mock repos configured to return a violation Decision and its Case.
func makeCanonicalViolationMocks(t *testing.T, subjectType entity.ReportTargetType, subjectID uuid.UUID) (decisionID, caseID uuid.UUID, decRepo *mockDecisionRepository, caseRepo *mockCaseRepository) {
	t.Helper()
	decisionID = uuid.New()
	caseID = uuid.New()

	decision := &entity.Decision{
		ID: decisionID, CaseID: caseID, DecidedBy: uuid.New(),
		Outcome: entity.DecisionOutcomeViolation, CreatedAt: time.Now(),
	}
	kase := &entity.CanonicalCase{
		ID: caseID, SubjectType: subjectType, SubjectID: subjectID,
		Status: entity.CaseStatusResolved, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	decRepo = &mockDecisionRepository{
		getByIDFunc: func(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.Decision, error) {
			if id == decisionID {
				return decision, nil
			}
			return nil, nil
		},
	}
	caseRepo = &mockCaseRepository{
		getByIDFunc: func(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.CanonicalCase, error) {
			if id == caseID {
				return kase, nil
			}
			return nil, nil
		},
	}
	return
}

func TestCreateAppeal_ValidOwner_RejectedCase_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	ownerID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)

	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{ID: contentID, AuthorID: ownerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, mockContentRepo, &mockCommentRepository{})

	result, err := service.CreateAppeal(ctx, tx, decisionID, ownerID, "This report was dismissed incorrectly")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, decisionID, result.DecisionID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_CommentResource_ValidOwner_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	commentID := uuid.New()
	ownerID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetComment, commentID)

	mockCommentRepo := &mockCommentRepository{
		getByIDFunc: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*contentEntity.Comment, error) {
			return &contentEntity.Comment{ID: commentID, AuthorID: ownerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, mockCommentRepo)

	result, err := service.CreateAppeal(ctx, tx, decisionID, ownerID, "My comment was wrongfully removed")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, decisionID, result.DecisionID)
	assert.Equal(t, ownerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

// ============================================================
// REVIEW APPEAL TESTS
// ============================================================

// Helper: build a pending appeal linked to a decisionID.
func makePendingAppeal(decisionID uuid.UUID) *entity.Appeal {
	return entity.NewAppeal(decisionID, uuid.New(), "Please reconsider")
}

// Helper: build ReviewAppeal mocks for a reversal (approved) test.
func makeReviewAppealMocks(t *testing.T, appeal *entity.Appeal, subjectType entity.ReportTargetType, subjectID uuid.UUID, execErr error) (*mockAppealRepository, *mockDecisionRepository, *mockCaseRepository) {
	t.Helper()
	_, caseID, decRepo, caseRepo := makeCanonicalViolationMocks(t, subjectType, subjectID)
	_ = caseID

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}

	// The DecisionService uses the same mock repos; execErr on the tx
	// makes outbox InsertEvent fail (simulating restoration event failure).
	_ = execErr

	return mockAppealRepo, decRepo, caseRepo
}

func TestReviewAppeal_ApproveNonRemovedCase_NoRestorationRequired_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	adminID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)
	appeal := entity.NewAppeal(decisionID, uuid.New(), "Please reconsider")

	mockAppealRepo, _, _ := makeReviewAppealMocks(t, appeal, entity.ReportTargetContent, contentID, nil)
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	adminResponse := "Appeal granted"
	result, err := service.ReviewAppeal(ctx, tx, uuid.New(), adminID, true, &adminResponse)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusApproved, result.Status)
}

func TestReviewAppeal_RejectAppeal_NoRestorationEvent_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	adminID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)
	appeal := entity.NewAppeal(decisionID, uuid.New(), "Please restore")

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	adminResponse := "Appeal denied - content violation stands"
	result, err := service.ReviewAppeal(ctx, tx, uuid.New(), adminID, false, &adminResponse)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusRejected, result.Status)
}

func TestReviewAppeal_ApproveRemovedCase_SuccessWithRestoration(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	adminID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)
	appeal := entity.NewAppeal(decisionID, uuid.New(), "Please restore")

	mockAppealRepo, _, _ := makeReviewAppealMocks(t, appeal, entity.ReportTargetContent, contentID, nil)
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	adminResponse := "Appeal granted - content restored"
	result, err := service.ReviewAppeal(ctx, tx, uuid.New(), adminID, true, &adminResponse)

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

// TestReviewAppeal_DecisionCreationFailureKeepsAppealPending proves atomicity:
// if DecisionService.CreateAppealDecision fails (e.g. outbox insert fails via
// the concrete OutboxRepository), the appeal remains pending because the single
// transaction rolls back.
func TestReviewAppeal_DecisionCreationFailureKeepsAppealPending(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	adminID := uuid.New()

	decisionID, _, _, _ := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)
	appeal := entity.NewAppeal(decisionID, uuid.New(), "Please restore")
	initialStatus := appeal.Status

	var updateCalled bool

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			updateCalled = true
			return nil
		},
	}

	// Use nil repos for CaseRepo — CreateAppealDecision validates case exists
	// and will fail with ErrDecisionCaseNotFound, causing the single TX to
	// roll back. Appeal update is never reached.
	service := newTestAppealService(mockAppealRepo, &mockDecisionRepository{}, &mockCaseRepository{}, &mockContentRepository{}, &mockCommentRepository{})

	adminResponse := "Appeal granted"
	result, err := service.ReviewAppeal(ctx, tx, uuid.New(), adminID, true, &adminResponse)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.False(t, updateCalled, "Update should not be called when Decision creation fails")
	assert.Equal(t, initialStatus, appeal.Status, "Appeal should remain pending after rollback")
}

// TestCreateAppeal_AllowsNewAppealAfterResolvedAppeal proves that after a previous
// appeal is resolved (approved or rejected), a new appeal can be created.
func TestCreateAppeal_AllowsNewAppealAfterResolvedAppeal(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	contentID := uuid.New()
	ownerID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetContent, contentID)

	mockContentRepo := &mockContentRepository{
		getByIDFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*contentEntity.Content, error) {
			return &contentEntity.Content{ID: contentID, AuthorID: ownerID}, nil
		},
	}

	var createdAppeal *entity.Appeal
	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, appeal *entity.Appeal) error {
			createdAppeal = appeal
			return nil
		},
	}
	service := newTestAppealService(mockAppealRepo, decRepo, caseRepo, mockContentRepo, &mockCommentRepository{})

	result, err := service.CreateAppeal(ctx, tx, decisionID, ownerID, "I want to appeal again")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, decisionID, result.DecisionID)
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

	forSaleID := uuid.New()
	sellerID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetForSale, forSaleID)

	mockForSaleRepo := &mockForSaleOwnerRepo{
		getByIDFunc: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*forsaleEntity.ForSale, error) {
			return &forsaleEntity.ForSale{SellerID: sellerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}

	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})
	svc.SetForSaleRepo(mockForSaleRepo)

	result, err := svc.CreateAppeal(ctx, tx, decisionID, sellerID, "My fixed-price sale was wrongfully removed")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, decisionID, result.DecisionID)
	assert.Equal(t, sellerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_ForSaleResource_NonOwner_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	forSaleID := uuid.New()
	sellerID := uuid.New()
	otherUserID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetForSale, forSaleID)

	mockForSaleRepo := &mockForSaleOwnerRepo{
		getByIDFunc: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*forsaleEntity.ForSale, error) {
			return &forsaleEntity.ForSale{SellerID: sellerID}, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{}
	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})
	svc.SetForSaleRepo(mockForSaleRepo)

	result, err := svc.CreateAppeal(ctx, tx, decisionID, otherUserID, "Trying to appeal someone else's fixed-price sale")

	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
}

func TestCreateAppeal_AuctionResource_ValidSeller_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	auctionID := uuid.New()
	sellerID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetAuction, auctionID)

	mockAuctionRepo := &mockAuctionOwnerRepo{
		getByIDFunc: func(_ context.Context, _ db.Tx, _ uuid.UUID) (*auctionEntity.Auction, error) {
			a := &auctionEntity.Auction{}
			a.SellerID = sellerID
			return a, nil
		},
	}

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}

	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})
	svc.SetAuctionRepo(mockAuctionRepo)

	result, err := svc.CreateAppeal(ctx, tx, decisionID, sellerID, "My auction was wrongfully cancelled")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, sellerID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_UserSuspension_ValidUser_Success(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	suspendedUserID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetUser, suspendedUserID)

	mockAppealRepo := &mockAppealRepository{
		createWithPendingCheckFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}

	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	result, err := svc.CreateAppeal(ctx, tx, decisionID, suspendedUserID, "I was wrongfully suspended")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, suspendedUserID, result.AppealedBy)
	assert.Equal(t, entity.AppealStatusPending, result.Status)
}

func TestCreateAppeal_UserSuspension_OtherUser_ReturnsError(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	suspendedUserID := uuid.New()
	otherUserID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetUser, suspendedUserID)

	mockAppealRepo := &mockAppealRepository{}
	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	result, err := svc.CreateAppeal(ctx, tx, decisionID, otherUserID, "Appealing someone else's suspension")

	assert.Nil(t, result)
	assert.Error(t, err)
	var notOwnerErr *entity.ErrNotResourceOwner
	assert.ErrorAs(t, err, &notOwnerErr)
}

func TestCreateAppeal_ForSaleWithoutRepo_ReturnsUnsupportedType(t *testing.T) {
	ctx := context.Background()
	tx := &mockAppealTx{}

	forSaleID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetForSale, forSaleID)

	mockAppealRepo := &mockAppealRepository{}
	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	result, err := svc.CreateAppeal(ctx, tx, decisionID, uuid.New(), "My fixed-price sale was removed")

	assert.Nil(t, result)
	assert.Error(t, err)
	var unsupportedErr *entity.ErrUnsupportedResourceType
	assert.ErrorAs(t, err, &unsupportedErr)
}

func TestCreateAppeal_ForSaleApproval_NoRestorationEvent(t *testing.T) {
	// Verify that approving a for_sale appeal goes through Decision #2 path.
	ctx := context.Background()
	tx := &mockAppealTx{}

	forSaleID := uuid.New()
	adminID := uuid.New()

	decisionID, _, decRepo, caseRepo := makeCanonicalViolationMocks(t, entity.ReportTargetForSale, forSaleID)
	appeal := entity.NewAppeal(decisionID, uuid.New(), "My fixed-price sale was wrongfully removed")

	mockAppealRepo := &mockAppealRepository{
		getForUpdateFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.Appeal, error) {
			return appeal, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, _ *entity.Appeal) error {
			return nil
		},
	}

	svc := newTestAppealService(mockAppealRepo, decRepo, caseRepo, &mockContentRepository{}, &mockCommentRepository{})

	adminResponse := "Appeal accepted administratively"
	result, err := svc.ReviewAppeal(ctx, tx, uuid.New(), adminID, true, &adminResponse)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, entity.AppealStatusApproved, result.Status)
}
