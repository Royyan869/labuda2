package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// â”€â”€â”€ minimal mock infrastructure â”€â”€â”€

type enforceNotesMockTx struct {
	execCalls int
}

var _ db.Tx = (*enforceNotesMockTx)(nil)

func (m *enforceNotesMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	m.execCalls++
	return pgconn.CommandTag{}, nil
}
func (m *enforceNotesMockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *enforceNotesMockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return enforceNotesNoRow{}
}
func (m *enforceNotesMockTx) Commit(_ context.Context) error   { return nil }
func (m *enforceNotesMockTx) Rollback(_ context.Context) error { return nil }

type enforceNotesNoRow struct{}

func (enforceNotesNoRow) Scan(_ ...any) error { return pgx.ErrNoRows }

type enforceNotesMockTransactor struct {
	tx *enforceNotesMockTx
}

func (m *enforceNotesMockTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	if m.tx == nil {
		m.tx = &enforceNotesMockTx{}
	}
	return fn(m.tx)
}

// enforceNotesMockRepo is a minimal ModerationRepository that lets tests control
// GetByID (for GetCase pre-fetch), GetForUpdate, and Update on a per-test basis.
type enforceNotesMockRepo struct {
	getByIDFunc      func(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	getForUpdateFunc func(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error)
	updateFunc       func(context.Context, interface{}, *entity.GovernanceCase) error
}

var _ moderationRepo.ModerationRepository = (*enforceNotesMockRepo)(nil)

func (m *enforceNotesMockRepo) Create(_ context.Context, _ interface{}, _ *entity.GovernanceCase) error {
	return nil
}
func (m *enforceNotesMockRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, tx, id)
	}
	return nil, fmt.Errorf("not found")
}
func (m *enforceNotesMockRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
	if m.getForUpdateFunc != nil {
		return m.getForUpdateFunc(ctx, tx, id)
	}
	return nil, nil
}
func (m *enforceNotesMockRepo) Update(ctx context.Context, tx interface{}, kase *entity.GovernanceCase) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, tx, kase)
	}
	return nil
}
func (m *enforceNotesMockRepo) ListPending(_ context.Context, _ interface{}, _, _ int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *enforceNotesMockRepo) ListByResource(_ context.Context, _ interface{}, _ entity.ResourceType, _ uuid.UUID) ([]*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *enforceNotesMockRepo) ListByReporter(_ context.Context, _ interface{}, _ uuid.UUID, _, _ int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}
func (m *enforceNotesMockRepo) ListWithStatus(_ context.Context, _ interface{}, _ *entity.GovernanceCaseStatus, _ *entity.ResourceType, _, _ int) ([]*entity.GovernanceCase, int64, error) {
	return nil, 0, nil
}
func (m *enforceNotesMockRepo) ResourceExists(_ context.Context, _ interface{}, _ entity.ResourceType, _ uuid.UUID) (bool, error) {
	return true, nil
}
func (m *enforceNotesMockRepo) HasUserReportedResource(_ context.Context, _ interface{}, _ uuid.UUID, _ entity.ResourceType, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (m *enforceNotesMockRepo) ValidateChatMessageReporter(_ context.Context, _ interface{}, _ uuid.UUID, _ uuid.UUID) (bool, string, error) {
	return true, "", nil
}

// â”€â”€â”€ no-op audit logger â”€â”€â”€

type enforceNotesNoopAuditLogger struct{}

func (enforceNotesNoopAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (enforceNotesNoopAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

// â”€â”€â”€ router helpers â”€â”€â”€

func newEnforceNotesRouter(handler *ModerationHandler, adminID uuid.UUID) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &capabilityEntity.Actor{
			ID:           adminID,
			Role:         "admin",
			Capabilities: []string{capability.CapModerationCaseResolve.String()},
		}
		ctx := capability.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", adminID)
		c.Next()
	})
	router.POST("/admin/moderation/cases/:id/action", handler.ApplyAction)
	return router
}

func newEnforceNotesHandler(repo moderationRepo.ModerationRepository, dbtx db.Transactor) *ModerationHandler {
	service := moderationApp.NewModerationService(dbtx, repo, nil)
	return NewModerationHandler(service, dbtx, zap.NewNop(), enforceNotesNoopAuditLogger{})
}

func TestApplyAction_Enforce_RejectsBlankNotes(t *testing.T) {
	adminID := uuid.New()
	caseID := uuid.New()
	caseRow := &entity.GovernanceCase{
		ID:           caseID,
		ResourceType: entity.ResourceTypeChatMessage,
		ResourceID:   uuid.New(),
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   uuid.New(),
		Reason:       "spam",
	}

	repo := &enforceNotesMockRepo{
		getByIDFunc: func(_ context.Context, _ interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			if id != caseID {
				return nil, fmt.Errorf("unexpected case id %s", id)
			}
			return caseRow, nil
		},
		getForUpdateFunc: func(_ context.Context, _ interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			if id != caseID {
				return nil, fmt.Errorf("unexpected case id %s", id)
			}
			return caseRow, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, kase *entity.GovernanceCase) error {
			caseRow = kase
			return nil
		},
	}

	handler := newEnforceNotesHandler(repo, &enforceNotesMockTransactor{})
	router := newEnforceNotesRouter(handler, adminID)

	body, _ := json.Marshal(ApplyActionRequest{Action: "enforce"})
	req := httptest.NewRequest(http.MethodPost, "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "non-empty notes")
}

func TestApplyAction_Enforce_AllowsNonEmptyNotes(t *testing.T) {
	adminID := uuid.New()
	caseID := uuid.New()
	caseRow := &entity.GovernanceCase{
		ID:           caseID,
		ResourceType: entity.ResourceTypeChatMessage,
		ResourceID:   uuid.New(),
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   uuid.New(),
		Reason:       "spam",
	}

	repo := &enforceNotesMockRepo{
		getByIDFunc: func(_ context.Context, _ interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			if id != caseID {
				return nil, fmt.Errorf("unexpected case id %s", id)
			}
			return caseRow, nil
		},
		getForUpdateFunc: func(_ context.Context, _ interface{}, id uuid.UUID) (*entity.GovernanceCase, error) {
			if id != caseID {
				return nil, fmt.Errorf("unexpected case id %s", id)
			}
			return caseRow, nil
		},
		updateFunc: func(_ context.Context, _ interface{}, kase *entity.GovernanceCase) error {
			caseRow = kase
			return nil
		},
	}

	handler := newEnforceNotesHandler(repo, &enforceNotesMockTransactor{})
	router := newEnforceNotesRouter(handler, adminID)

	body, _ := json.Marshal(ApplyActionRequest{Action: "enforce", Notes: "policy violation"})
	req := httptest.NewRequest(http.MethodPost, "/admin/moderation/cases/"+caseID.String()+"/action", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), "requires notes")
}
