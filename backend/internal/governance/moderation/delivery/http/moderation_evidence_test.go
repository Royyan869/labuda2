package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

type evidenceCaseRepository struct {
	caseByID *entity.GovernanceCase
	err      error
}

var _ moderationRepo.ModerationRepository = (*evidenceCaseRepository)(nil)

func (r *evidenceCaseRepository) Create(context.Context, interface{}, *entity.GovernanceCase) error {
	return nil
}

func (r *evidenceCaseRepository) GetByID(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.caseByID, nil
}

func (r *evidenceCaseRepository) GetForUpdate(context.Context, interface{}, uuid.UUID) (*entity.GovernanceCase, error) {
	return nil, nil
}

func (r *evidenceCaseRepository) Update(context.Context, interface{}, *entity.GovernanceCase) error {
	return nil
}

func (r *evidenceCaseRepository) ListPending(context.Context, interface{}, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (r *evidenceCaseRepository) ListByResource(context.Context, interface{}, entity.ResourceType, uuid.UUID) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (r *evidenceCaseRepository) ListByReporter(context.Context, interface{}, uuid.UUID, int, int) ([]*entity.GovernanceCase, error) {
	return nil, nil
}

func (r *evidenceCaseRepository) ListWithStatus(context.Context, interface{}, *entity.GovernanceCaseStatus, *entity.ResourceType, int, int) ([]*entity.GovernanceCase, int64, error) {
	return nil, 0, nil
}

func (r *evidenceCaseRepository) ResourceExists(context.Context, interface{}, entity.ResourceType, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *evidenceCaseRepository) HasUserReportedResource(context.Context, interface{}, uuid.UUID, entity.ResourceType, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *evidenceCaseRepository) ValidateChatMessageReporter(context.Context, interface{}, uuid.UUID, uuid.UUID) (bool, string, error) {
	return false, "", nil
}

type evidenceWithTxTransactor struct {
	tx    db.Tx
	calls int
}

func (t *evidenceWithTxTransactor) WithTx(_ context.Context, fn func(tx db.Tx) error) error {
	t.calls++
	return fn(t.tx)
}

type evidenceAuditCall struct {
	actorID    uuid.UUID
	actionType string
	targetType string
	targetID   uuid.UUID
	metadata   map[string]interface{}
}

type evidenceAuditLogger struct {
	calls    []evidenceAuditCall
	logTxErr error
}

func (l *evidenceAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (l *evidenceAuditLogger) LogTx(_ context.Context, _ db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	l.calls = append(l.calls, evidenceAuditCall{
		actorID:    actorID,
		actionType: actionType,
		targetType: targetType,
		targetID:   targetID,
		metadata:   metadata,
	})
	return l.logTxErr
}

type evidenceTestAuditLogger struct{}

func (evidenceTestAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (evidenceTestAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func newEvidenceTestRouter(actor *capabilityEntity.Actor, handler *ModerationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if actor != nil {
			ctx := capability.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
			c.Set("user_id", actor.ID)
			c.Set("user_role", actor.Role)
		}
		c.Next()
	})
	router.GET("/admin/moderation/cases/:id/evidence", handler.GetCaseEvidence)
	return router
}

func TestFetchResourcePreviewTx_ChatMessage_RedactsBodyAndMarksEvidenceAvailable(t *testing.T) {
	handler := NewModerationHandler(nil, nil, zap.NewNop(), evidenceTestAuditLogger{})

	messageID := uuid.New()
	roomID := uuid.New()
	sentAt := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 7, 14, 9, 59, 0, 0, time.UTC)

	tx := &previewMockTx{
		row: previewMockRow{
			values: []any{
				uuid.New().String(),
				"hidden-author",
				"text",
				true,
				deletedAt,
				"moderated",
				roomID.String(),
				"normal",
				sentAt,
			},
		},
	}

	preview, err := handler.fetchResourcePreviewTx(context.Background(), tx, entity.ResourceTypeChatMessage, messageID)

	require.NoError(t, err)
	require.NotNil(t, preview)
	assert.NotContains(t, tx.lastSQL, "cm.body")
	assert.NotContains(t, tx.lastSQL, "attachment_json")
	assert.Equal(t, "hidden-author", preview.AuthorUsername)
	assert.Equal(t, "text", preview.ContentType)
	assert.Equal(t, "", preview.ContentText)
	assert.True(t, preview.IsDeleted)
	require.NotNil(t, preview.DeletedAt)
	assert.True(t, preview.DeletedAt.Equal(deletedAt))
	require.NotNil(t, preview.DeletionReason)
	assert.Equal(t, "moderated", *preview.DeletionReason)
	assert.True(t, preview.EvidenceAvailable)
	assert.Equal(t, capability.CapModerationEvidenceRead.String(), preview.EvidenceRequiresCapability)
	assert.Equal(t, roomID.String(), preview.RoomID)
	assert.Equal(t, "normal", preview.RoomType)
	assert.Equal(t, sentAt.Format(time.RFC3339), preview.SentAt)
}

func TestGetCaseEvidence_ChatMessage_ReturnsEvidenceAndAudits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caseID := uuid.New()
	messageID := uuid.New()
	roomID := uuid.New()
	senderID := uuid.New()
	reporterID := uuid.New()
	adminID := uuid.New()
	createdAt := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	body := "hidden original body"
	attachmentJSON := []byte(`{"media_url":"https://cdn.example.com/evidence.png"}`)

	kase := &entity.GovernanceCase{
		ID:           caseID,
		ResourceType: entity.ResourceTypeChatMessage,
		ResourceID:   messageID,
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   reporterID,
		Reason:       "spam",
		CreatedAt:    createdAt,
	}

	service := moderationApp.NewModerationService(&createCaseMockTransactor{}, &evidenceCaseRepository{caseByID: kase}, nil)

	tx := &previewMockTx{
		row: previewMockRow{
			values: []any{
				messageID.String(),
				roomID.String(),
				"normal",
				senderID.String(),
				"evidence-author",
				createdAt,
				deletedAt,
				"removed by moderation",
				body,
				attachmentJSON,
			},
		},
	}

	dbTransactor := &evidenceWithTxTransactor{tx: tx}
	auditLogger := &evidenceAuditLogger{}
	handler := NewModerationHandler(service, dbTransactor, zap.NewNop(), auditLogger)
	router := newEvidenceTestRouter(&capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationEvidenceRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, auditLogger.calls, 1)
	assert.Equal(t, "moderation.evidence.read", auditLogger.calls[0].actionType)
	assert.Equal(t, "moderation_case", auditLogger.calls[0].targetType)
	assert.Equal(t, caseID, auditLogger.calls[0].targetID)
	assert.Equal(t, adminID, auditLogger.calls[0].actorID)
	assert.NotContains(t, auditLogger.calls[0].metadata, "original_body")
	assert.NotContains(t, auditLogger.calls[0].metadata, "original_attachment")
	assert.Equal(t, messageID, tx.lastArgs[0])

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	evidence, ok := data["evidence"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, caseID.String(), evidence["case_id"])
	assert.Equal(t, messageID.String(), evidence["message_id"])
	assert.Equal(t, roomID.String(), evidence["room_id"])
	assert.Equal(t, "normal", evidence["room_type"])
	assert.Equal(t, senderID.String(), evidence["sender_id"])
	assert.Equal(t, "evidence-author", evidence["author_username"])
	assert.Equal(t, body, evidence["original_body"])
	attachment, ok := evidence["original_attachment"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "https://cdn.example.com/evidence.png", attachment["media_url"])
}

func TestGetCaseEvidence_AuditFailure_BlocksResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caseID := uuid.New()
	messageID := uuid.New()
	roomID := uuid.New()
	senderID := uuid.New()
	reporterID := uuid.New()
	adminID := uuid.New()
	createdAt := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)

	kase := &entity.GovernanceCase{
		ID:           caseID,
		ResourceType: entity.ResourceTypeChatMessage,
		ResourceID:   messageID,
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   reporterID,
		Reason:       "spam",
		CreatedAt:    createdAt,
	}

	service := moderationApp.NewModerationService(&createCaseMockTransactor{}, &evidenceCaseRepository{caseByID: kase}, nil)

	tx := &previewMockTx{
		row: previewMockRow{
			values: []any{
				messageID.String(),
				roomID.String(),
				"normal",
				senderID.String(),
				"evidence-author",
				createdAt,
				time.Time{},
				nil,
				"hidden original body",
				nil,
			},
		},
	}

	dbTransactor := &evidenceWithTxTransactor{tx: tx}
	auditLogger := &evidenceAuditLogger{logTxErr: errors.New("audit write failed")}
	handler := NewModerationHandler(service, dbTransactor, zap.NewNop(), auditLogger)
	router := newEvidenceTestRouter(&capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationEvidenceRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Len(t, auditLogger.calls, 1)
	assert.NotContains(t, w.Body.String(), "original_body")
}

func TestGetCaseEvidence_NonChatCase_ReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caseID := uuid.New()
	adminID := uuid.New()
	kase := &entity.GovernanceCase{
		ID:           caseID,
		ResourceType: entity.ResourceTypeContent,
		ResourceID:   uuid.New(),
		Status:       entity.GovernanceCaseStatusPending,
		ReportedBy:   uuid.New(),
		Reason:       "spam",
		CreatedAt:    time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC),
	}

	service := moderationApp.NewModerationService(&createCaseMockTransactor{}, &evidenceCaseRepository{caseByID: kase}, nil)
	dbTransactor := &evidenceWithTxTransactor{tx: &previewMockTx{}}
	auditLogger := &evidenceAuditLogger{}
	handler := NewModerationHandler(service, dbTransactor, zap.NewNop(), auditLogger)
	router := newEvidenceTestRouter(&capabilityEntity.Actor{
		ID:           adminID,
		Role:         "admin",
		Capabilities: []string{capability.CapModerationEvidenceRead.String()},
	}, handler)

	req := httptest.NewRequest(http.MethodGet, "/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, auditLogger.calls)
	assert.Equal(t, 0, dbTransactor.calls)
}
