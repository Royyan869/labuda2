package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type intakeMockTx struct {
	execCalls []intakeMockExecCall
}

type intakeMockExecCall struct {
	sql  string
	args []any
}

var _ db.Tx = (*intakeMockTx)(nil)

func (m *intakeMockTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, intakeMockExecCall{
		sql:  sql,
		args: append([]any(nil), args...),
	})
	return pgconn.CommandTag{}, nil
}

func (m *intakeMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *intakeMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return intakeNoRowsRow{}
}

func (m *intakeMockTx) Commit(_ context.Context) error   { return nil }
func (m *intakeMockTx) Rollback(_ context.Context) error { return nil }

type intakeNoRowsRow struct{}

func (intakeNoRowsRow) Scan(_ ...any) error {
	return pgx.ErrNoRows
}

type intakeMockTransactor struct {
	tx db.Tx
}

func (m *intakeMockTransactor) WithTx(_ context.Context, fn func(tx db.Tx) error) error {
	if m.tx == nil {
		m.tx = &intakeMockTx{}
	}
	return fn(m.tx)
}

type intakeMockRepo struct {
	createFunc                    func(context.Context, interface{}, *entity.GovernanceCase) error
	getByIDFunc                   func(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	getForUpdateFunc              func(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	updateFunc                    func(context.Context, interface{}, *entity.GovernanceCase) error
	listPendingFunc               func(context.Context, interface{}, int, int) ([]*entity.GovernanceCase, error)
	listByResourceFunc            func(context.Context, interface{}, entity.ResourceType, uuid.UUID) ([]*entity.GovernanceCase, error)
	listByReporterFunc            func(context.Context, interface{}, uuid.UUID, int, int) ([]*entity.GovernanceCase, error)
	listWithStatusFunc            func(context.Context, interface{}, *entity.GovernanceCaseStatus, *entity.ResourceType, int, int) ([]*entity.GovernanceCase, int64, error)
	resourceExistsFunc            func(context.Context, interface{}, entity.ResourceType, uuid.UUID) (bool, error)
	hasUserReportedResourceFunc   func(context.Context, interface{}, uuid.UUID, entity.ResourceType, uuid.UUID) (bool, error)
	validateChatMessageReporterFn func(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, string, error)
}

var _ interface {
	Create(context.Context, interface{}, *entity.GovernanceCase) error
	GetByID(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	GetForUpdate(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	Update(context.Context, interface{}, *entity.GovernanceCase) error
	ListPending(context.Context, interface{}, int, int) ([]*entity.GovernanceCase, error)
	ListByResource(context.Context, interface{}, entity.ResourceType, uuid.UUID) ([]*entity.GovernanceCase, error)
	ListByReporter(context.Context, interface{}, uuid.UUID, int, int) ([]*entity.GovernanceCase, error)
	ListWithStatus(context.Context, interface{}, *entity.GovernanceCaseStatus, *entity.ResourceType, int, int) ([]*entity.GovernanceCase, int64, error)
	ResourceExists(context.Context, interface{}, entity.ResourceType, uuid.UUID) (bool, error)
	HasUserReportedResource(context.Context, interface{}, uuid.UUID, entity.ResourceType, uuid.UUID) (bool, error)
	ValidateChatMessageReporter(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, string, error)
} = (*intakeMockRepo)(nil)

func (m *intakeMockRepo) Create(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, tx, kase)
	}
	return nil
}

func (m *intakeMockRepo) GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}

func (m *intakeMockRepo) GetForUpdate(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, caseID)
	}
	return nil, errors.New("not found")
}

func (m *intakeMockRepo) Update(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, kase)
	}
	return nil
}

func (m *intakeMockRepo) ListPending(ctx context.Context, tx interface{}, limit, offset int) ([]*entity.GovernanceCase, error) {
	if m.listPendingFunc != nil {
		return m.listPendingFunc(ctx, tx, limit, offset)
	}
	return nil, nil
}

func (m *intakeMockRepo) ListByResource(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) ([]*entity.GovernanceCase, error) {
	if m.listByResourceFunc != nil {
		return m.listByResourceFunc(ctx, tx, resourceType, resourceID)
	}
	return nil, nil
}

func (m *intakeMockRepo) ListByReporter(ctx context.Context, tx interface{}, reporterID uuid.UUID, limit, offset int) ([]*entity.GovernanceCase, error) {
	if m.listByReporterFunc != nil {
		return m.listByReporterFunc(ctx, tx, reporterID, limit, offset)
	}
	return nil, nil
}

func (m *intakeMockRepo) ListWithStatus(ctx context.Context, tx interface{}, statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType, limit, offset int) ([]*entity.GovernanceCase, int64, error) {
	if m.listWithStatusFunc != nil {
		return m.listWithStatusFunc(ctx, tx, statusFilter, resourceTypeFilter, limit, offset)
	}
	return nil, 0, nil
}

func (m *intakeMockRepo) ResourceExists(ctx context.Context, tx interface{}, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.resourceExistsFunc != nil {
		return m.resourceExistsFunc(ctx, tx, resourceType, resourceID)
	}
	return true, nil
}

func (m *intakeMockRepo) HasUserReportedResource(ctx context.Context, tx interface{}, reporterID uuid.UUID, resourceType entity.ResourceType, resourceID uuid.UUID) (bool, error) {
	if m.hasUserReportedResourceFunc != nil {
		return m.hasUserReportedResourceFunc(ctx, tx, reporterID, resourceType, resourceID)
	}
	return false, nil
}

func (m *intakeMockRepo) ValidateChatMessageReporter(ctx context.Context, tx interface{}, messageID uuid.UUID, reporterID uuid.UUID) (bool, string, error) {
	if m.validateChatMessageReporterFn != nil {
		return m.validateChatMessageReporterFn(ctx, tx, messageID, reporterID)
	}
	return true, "", nil
}

func TestModerationService_CreateCase_AllowsContentForSaleAuction(t *testing.T) {
	cases := []struct {
		name         string
		resourceType entity.ResourceType
	}{
		{name: "content", resourceType: entity.ResourceTypeContent},
		{name: "for_sale", resourceType: entity.ResourceTypeForSale},
		{name: "auction", resourceType: entity.ResourceTypeAuction},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			resourceID := uuid.New()
			reporterID := uuid.New()
			reason := "intake test"

			var createdCase *entity.GovernanceCase
			repo := &intakeMockRepo{
				createFunc: func(_ context.Context, _ interface{}, kase *entity.GovernanceCase) error {
					createdCase = kase
					return nil
				},
				resourceExistsFunc: func(_ context.Context, _ interface{}, resourceType entity.ResourceType, id uuid.UUID) (bool, error) {
					assert.Equal(t, tc.resourceType, resourceType)
					assert.Equal(t, resourceID, id)
					return true, nil
				},
				hasUserReportedResourceFunc: func(_ context.Context, _ interface{}, reporter uuid.UUID, resourceType entity.ResourceType, id uuid.UUID) (bool, error) {
					assert.Equal(t, reporterID, reporter)
					assert.Equal(t, tc.resourceType, resourceType)
					assert.Equal(t, resourceID, id)
					return false, nil
				},
			}

			service := application.NewModerationService(&intakeMockTransactor{}, repo, nil)
			kase, err := service.CreateCase(ctx, tc.resourceType, resourceID, reporterID, reason)

			require.NoError(t, err)
			require.NotNil(t, kase)
			require.NotNil(t, createdCase)
			assert.Equal(t, tc.resourceType, kase.ResourceType)
			assert.Equal(t, resourceID, kase.ResourceID)
			assert.Equal(t, reporterID, kase.ReportedBy)
			assert.Equal(t, reason, kase.Reason)
			assert.Equal(t, entity.GovernanceCaseStatusPending, kase.Status)
		})
	}
}

func TestModerationService_CreateCase_RejectsMissingForSaleAndAuctionResources(t *testing.T) {
	for _, resourceType := range []entity.ResourceType{entity.ResourceTypeForSale, entity.ResourceTypeAuction} {
		t.Run(string(resourceType), func(t *testing.T) {
			repo := &intakeMockRepo{
				resourceExistsFunc: func(_ context.Context, _ interface{}, rt entity.ResourceType, _ uuid.UUID) (bool, error) {
					assert.Equal(t, resourceType, rt)
					return false, nil
				},
			}

			service := application.NewModerationService(&intakeMockTransactor{}, repo, nil)
			_, err := service.CreateCase(context.Background(), resourceType, uuid.New(), uuid.New(), "missing resource")

			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "resource not found"), err.Error())
		})
	}
}

func TestModerationService_ReviewCase_EmitsMarketplaceRemovalEvents(t *testing.T) {
	for _, tc := range []struct {
		name         string
		resourceType entity.ResourceType
		wantEvent    string
	}{
		{name: "for_sale", resourceType: entity.ResourceTypeForSale, wantEvent: "moderation.for_sale.removed"},
		{name: "auction", resourceType: entity.ResourceTypeAuction, wantEvent: "moderation.auction.removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &intakeMockTx{}
			transactor := &intakeMockTransactor{tx: tx}

			resourceID := uuid.New()
			kase := entity.NewGovernanceCase(tc.resourceType, resourceID, uuid.New(), "remove this")
			var updatedCase *entity.GovernanceCase

			repo := &intakeMockRepo{
				getForUpdateFunc: func(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.GovernanceCase, error) {
					return kase, nil
				},
				updateFunc: func(_ context.Context, _ interface{}, c *entity.GovernanceCase) error {
					updatedCase = c
					return nil
				},
			}

			service := application.NewModerationService(transactor, repo, &outboxRepo.OutboxRepository{})
			note := "moderation required"
			result, err := service.ReviewCase(context.Background(), kase.ID, uuid.New(), entity.DecisionEnforce, &note)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, updatedCase)
			require.Len(t, tx.execCalls, 1)
			assert.Equal(t, tc.wantEvent, tx.execCalls[0].args[3])

			payloadBytes, ok := tx.execCalls[0].args[4].([]byte)
			require.True(t, ok)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(payloadBytes, &payload))
			assert.Equal(t, string(tc.resourceType), payload["resource_type"])
			assert.Equal(t, resourceID.String(), payload["resource_id"])
			assert.Equal(t, entity.GovernanceCaseStatusEnforced, result.Status)
		})
	}
}
