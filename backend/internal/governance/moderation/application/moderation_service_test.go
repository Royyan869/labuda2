//go:build integration

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

// mockTransactor is a mock implementation of Transactor for testing.
type mockTransactor struct {
	executeFn func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, fn)
	}
	return fn(&mockTxImpl{})
}

// mockTx is a mock implementation of db.Tx for testing.
type mockTxImpl struct{}

func (m *mockTxImpl) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockTxImpl) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockTxImpl) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &mockRow{}
}

func (m *mockTxImpl) Commit(ctx context.Context) error {
	return nil
}

func (m *mockTxImpl) Rollback(ctx context.Context) error {
	return nil
}

// mockRow is a mock implementation of db.Row.
type mockRow struct{}

func (m *mockRow) Scan(dest ...interface{}) error {
	return errors.New("not found")
}

// mockModerationRepository is a mock implementation of ModerationRepository.
type mockModerationRepository struct {
	createFunc         func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error
	getByIDFunc        func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
	getForUpdateFunc   func(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
	updateFunc         func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error
	listPendingFunc    func(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error)
	listByResource     func(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error)
	listWithStatusFunc func(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error)
}

func (m *mockModerationRepository) Create(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, kase)
	}
	return nil
}

func (m *mockModerationRepository) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}

func (m *mockModerationRepository) GetForUpdate(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}

func (m *mockModerationRepository) Update(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, kase)
	}
	return nil
}

func (m *mockModerationRepository) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error) {
	if m.listPendingFunc != nil {
		return m.listPendingFunc(ctx, tx, limit, offset)
	}
	return nil, nil
}

func (m *mockModerationRepository) ListByResource(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error) {
	if m.listByResource != nil {
		return m.listByResource(ctx, tx, resourceType, resourceID)
	}
	return nil, nil
}

func (m *mockModerationRepository) ListWithStatus(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error) {
	if m.listWithStatusFunc != nil {
		return m.listWithStatusFunc(ctx, tx, statusFilter, resourceTypeFilter, limit, offset)
	}
	return nil, 0, nil
}

// TestModerationService_CreateCase tests case creation.
func TestModerationService_CreateCase(t *testing.T) {
	t.Run("creates new moderation case", func(t *testing.T) {
		resourceID := uuid.New()
		reporterID := uuid.New()
		reason := "Inappropriate content"
		var createdCase *entity.GovernanceCase

		repo := &mockModerationRepository{
			createFunc: func(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
				createdCase = kase
				return nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		kase, err := service.CreateCase(context.Background(), entity.ResourceTypeContent, resourceID, reporterID, reason)

		require.NoError(t, err)
		assert.NotNil(t, kase)
		assert.Equal(t, entity.ResourceTypeContent, kase.ResourceType)
		assert.Equal(t, resourceID, kase.ResourceID)
		assert.Equal(t, reporterID, kase.ReportedBy)
		assert.Equal(t, reason, kase.Reason)
		assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
		assert.NotNil(t, createdCase)
	})

	t.Run("rejects invalid resource type", func(t *testing.T) {
		repo := &mockModerationRepository{}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		_, err := service.CreateCase(context.Background(), entity.ResourceType("invalid"), uuid.New(), uuid.New(), "test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource type")
	})
}

// TestModerationService_ReviewCase tests case review.
func TestModerationService_ReviewCase(t *testing.T) {
	adminID := uuid.New()
	note := "Review complete"

	t.Run("approves pending case", func(t *testing.T) {
		caseID := uuid.New()
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")
		var updatedCase *entity.GovernanceCase

		repo := &mockModerationRepository{
			getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
				return kase, nil
			},
			updateFunc: func(ctx context.Context, tx interface{}, c *entity.GovernanceCase) error {
				updatedCase = c
				return nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		result, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionApprove, &note)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, entity.GovernanceCaseStatusApproved, result.Status)
		assert.Equal(t, &adminID, result.ReviewedBy)
		assert.NotNil(t, result.ReviewedAt)
		assert.NotNil(t, updatedCase)
	})

	t.Run("rejects pending case", func(t *testing.T) {
		caseID := uuid.New()
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")
		var updatedCase *entity.GovernanceCase

		repo := &mockModerationRepository{
			getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
				return kase, nil
			},
			updateFunc: func(ctx context.Context, tx interface{}, c *entity.GovernanceCase) error {
				updatedCase = c
				return nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		result, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionReject, &note)

		require.NoError(t, err)
		assert.Equal(t, entity.GovernanceCaseStatusRejected, result.Status)
		assert.NotNil(t, updatedCase)
	})

	t.Run("removes pending case", func(t *testing.T) {
		caseID := uuid.New()
		resourceID := uuid.New()
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, resourceID, uuid.New(), "test")
		var updatedCase *entity.GovernanceCase

		repo := &mockModerationRepository{
			getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
				return kase, nil
			},
			updateFunc: func(ctx context.Context, tx interface{}, c *entity.GovernanceCase) error {
				updatedCase = c
				return nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		result, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionEnforce, &note)

		require.NoError(t, err)
		assert.Equal(t, entity.GovernanceCaseStatusEnforced, result.Status)
		assert.NotNil(t, updatedCase)
		assert.Equal(t, resourceID, result.ResourceID)
	})

	t.Run("rejects double review", func(t *testing.T) {
		caseID := uuid.New()
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")
		// First review
		_ = kase.Approve(adminID, nil)

		repo := &mockModerationRepository{
			getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
				return kase, nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		_, err := service.ReviewCase(context.Background(), caseID, adminID, entity.DecisionReject, &note)

		assert.Error(t, err)
		var alreadyReviewed *entity.ErrAlreadyReviewed
		assert.ErrorAs(t, err, &alreadyReviewed)
	})

	t.Run("rejects invalid decision", func(t *testing.T) {
		caseID := uuid.New()
		kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")

		repo := &mockModerationRepository{
			getForUpdateFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
				return kase, nil
			},
		}
		transactor := &mockTransactor{}

		service := application.NewModerationService(transactor, repo, nil)

		_, err := service.ReviewCase(context.Background(), caseID, adminID, entity.Decision("invalid"), nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid decision")
	})
}

// TestModerationService_ListPendingCases tests listing pending cases.
func TestModerationService_ListPendingCases(t *testing.T) {
	cases := []*entity.GovernanceCase{
		entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test1"),
		entity.NewGovernanceCase(entity.ResourceTypeComment, uuid.New(), uuid.New(), "test2"),
	}

	repo := &mockModerationRepository{
		listPendingFunc: func(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error) {
			return cases, nil
		},
	}
	transactor := &mockTransactor{}

	service := application.NewModerationService(transactor, repo, nil)

	result, err := service.ListPendingCases(context.Background(), 10, 0)

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

// TestModerationService_GetCasesByResource tests listing cases by resource.
func TestModerationService_GetCasesByResource(t *testing.T) {
	resourceID := uuid.New()
	cases := []*entity.GovernanceCase{
		entity.NewGovernanceCase(entity.ResourceTypeContent, resourceID, uuid.New(), "test1"),
	}

	repo := &mockModerationRepository{
		listByResource: func(ctx context.Context, tx interface{}, resourceType entity.ResourceType, rid uuid.UUID) ([]*entity.GovernanceCase, error) {
			return cases, nil
		},
	}
	transactor := &mockTransactor{}

	service := application.NewModerationService(transactor, repo, nil)

	result, err := service.GetCasesByResource(context.Background(), entity.ResourceTypeContent, resourceID)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// TestModerationService_ListCases tests listing cases with filters and authoritative count.
func TestModerationService_ListCases(t *testing.T) {
	status := entity.GovernanceCaseStatusPending
	resourceType := entity.ResourceTypeContent
	cases := []*entity.GovernanceCase{
		entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test1"),
	}

	repo := &mockModerationRepository{
		listWithStatusFunc: func(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error) {
			require.NotNil(t, statusFilter)
			require.NotNil(t, resourceTypeFilter)
			assert.Equal(t, status, *statusFilter)
			assert.Equal(t, resourceType, *resourceTypeFilter)
			return cases, int64(len(cases)), nil
		},
	}
	transactor := &mockTransactor{}

	service := application.NewModerationService(transactor, repo, nil)

	result, total, err := service.ListCases(context.Background(), &status, &resourceType, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
}

// TestModerationService_GetCase tests getting a case by ID.
func TestModerationService_GetCase(t *testing.T) {
	caseID := uuid.New()
	kase := entity.NewGovernanceCase(entity.ResourceTypeContent, uuid.New(), uuid.New(), "test")

	repo := &mockModerationRepository{
		getByIDFunc: func(ctx context.Context, tx interface{}, cid uuid.UUID) (*entity.GovernanceCase, error) {
			return kase, nil
		},
	}
	transactor := &mockTransactor{}

	service := application.NewModerationService(transactor, repo, nil)

	result, err := service.GetCase(context.Background(), caseID)

	require.NoError(t, err)
	assert.Equal(t, kase.ID, result.ID)
}


