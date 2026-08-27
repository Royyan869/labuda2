//go:build integration

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type moderationEvidenceIntegrationFixture struct {
	tdb      *testdb.TestDB
	handler  *ModerationHandler
	router   *gin.Engine
	appDB    *db.DB
	adminID  uuid.UUID
	reporter uuid.UUID
	sender   uuid.UUID
	roomID   uuid.UUID
	messageA uuid.UUID
	messageB uuid.UUID
	caseID   uuid.UUID
}

func newModerationEvidenceIntegrationRouter(actor *capabilityEntity.Actor, handler *ModerationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		if actor != nil {
			ctx := capability.WithActor(c.Request.Context(), actor)
			c.Request = c.Request.WithContext(ctx)
			c.Set("user_id", actor.ID)
			c.Set("user_role", actor.Role)
		}
		c.Next()
	})

	admin := router.Group("/api/v1/admin")
	admin.GET("/moderation/cases/:id", handler.GetCase)
	admin.GET("/moderation/cases/:id/evidence",
		middleware.RequireCapability("moderation.evidence.read"),
		handler.GetCaseEvidence)

	return router
}

func setupModerationEvidenceIntegrationFixture(t *testing.T, actor *capabilityEntity.Actor) *moderationEvidenceIntegrationFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())

	service := moderationApp.NewModerationService(appDB, moderationRepo.NewModerationRepository(), nil)
	handler := NewModerationHandler(service, appDB, zap.NewNop(), audit.NewAdminAuditLoggerDB(tdb.Pool()))

	return &moderationEvidenceIntegrationFixture{
		tdb:     tdb,
		handler: handler,
		router:  newModerationEvidenceIntegrationRouter(actor, handler),
		appDB:   appDB,
		adminID: actor.ID,
	}
}

func insertEvidenceTestUser(t *testing.T, ctx context.Context, pool *db.DB, role string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at, phone_verified,
			account_status, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW(), $4)
	`, userID, userID.String(), userID.String()+"@test.invalid", role)
	require.NoError(t, err)
	return userID
}

func insertEvidenceTestProfile(t *testing.T, ctx context.Context, pool *db.DB, userID uuid.UUID, username string) string {
	t.Helper()

	uniqueUsername := fmt.Sprintf("%s-%s", username, userID.String()[:8])
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO user_profiles (
			user_id, username, followers_count, following_count, created_at, updated_at
		)
		VALUES ($1, $2, 0, 0, NOW(), NOW())
	`, userID, uniqueUsername)
	require.NoError(t, err)
	return uniqueUsername
}

func insertEvidenceTestChatFixture(t *testing.T, ctx context.Context, pool *db.DB, reporterID, senderID uuid.UUID) (roomID, messageA, messageB, caseID uuid.UUID) {
	t.Helper()

	participantA := reporterID
	participantB := senderID
	if participantA.String() > participantB.String() {
		participantA, participantB = participantB, participantA
	}

	roomID = uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO chat_rooms (
			id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at
		)
		VALUES ($1, 'direct', $2, $3, NOW(), NOW(), NOW())
	`, roomID, participantA, participantB)
	require.NoError(t, err)

	messageA = uuid.New()
	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, created_at, deleted_at, deleted_by, deletion_reason
		)
		VALUES ($1, $2, $3, 'text', $4, $5, $6, NOW(), NOW(), NULL, $7)
	`, messageA, roomID, senderID, "hidden original body", []byte(`{"media_url":"https://cdn.example.com/evidence.png"}`), uuid.NewString(), "deleted for moderation")
	require.NoError(t, err)

	messageB = uuid.New()
	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, created_at
		)
		VALUES ($1, $2, $3, 'text', $4, NULL, $5, NOW())
	`, messageB, roomID, senderID, "unrelated message", uuid.NewString())
	require.NoError(t, err)

	caseID = uuid.New()
	_, err = pool.Pool().Exec(ctx, `
		INSERT INTO moderation_cases (
			id, resource_type, resource_id, status, reported_by, reason, created_at, updated_at
		)
		VALUES ($1, 'chat_message', $2, 'pending', $3, 'spam', NOW(), NOW())
	`, caseID, messageA, reporterID)
	require.NoError(t, err)

	return roomID, messageA, messageB, caseID
}

func insertEvidenceTestContentCase(t *testing.T, ctx context.Context, pool *db.DB, reporterID uuid.UUID) uuid.UUID {
	t.Helper()

	caseID := uuid.New()
	_, err := pool.Pool().Exec(ctx, `
		INSERT INTO moderation_cases (
			id, resource_type, resource_id, status, reported_by, reason, created_at, updated_at
		)
		VALUES ($1, 'content', $2, 'pending', $3, 'spam', NOW(), NOW())
	`, caseID, uuid.New(), reporterID)
	require.NoError(t, err)
	return caseID
}

func countAdminAuditRows(t *testing.T, ctx context.Context, pool *db.DB) int {
	t.Helper()

	var count int
	require.NoError(t, pool.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM admin_audit_logs`).Scan(&count))
	return count
}

func queryAdminAuditRow(t *testing.T, ctx context.Context, pool *db.DB) map[string]any {
	t.Helper()

	var actorID, targetID uuid.UUID
	var actionType, targetType string
	var metadataBytes []byte
	var createdAt time.Time
	require.NoError(t, pool.Pool().QueryRow(ctx, `
		SELECT actor_id, action_type, target_type, target_id, metadata, created_at
		FROM admin_audit_logs
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&actorID, &actionType, &targetType, &targetID, &metadataBytes, &createdAt))

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))

	return map[string]any{
		"actor_id":    actorID.String(),
		"action_type": actionType,
		"target_type": targetType,
		"target_id":   targetID.String(),
		"metadata":    metadata,
		"created_at":  createdAt,
	}
}

func actorWithCapabilities(id uuid.UUID, caps []string) *capabilityEntity.Actor {
	return &capabilityEntity.Actor{ID: id, Role: "admin", Capabilities: caps}
}

func assertEvidencePayload(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var resp map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "expected data envelope, got %s", string(body))
	return data
}

func TestModerationEvidence_RuntimeClosure_PostgresBacked(t *testing.T) {
	ctx := context.Background()
	adminActor := actorWithCapabilities(uuid.New(), []string{capability.CapModerationEvidenceRead.String()})
	fixture := setupModerationEvidenceIntegrationFixture(t, adminActor)

	t.Run("hidden normal case read is tombstone-only", func(t *testing.T) {
		router := newModerationEvidenceIntegrationRouter(actorWithCapabilities(uuid.New(), []string{"moderation.case.read"}), fixture.handler)

		reporterID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		senderID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		_ = insertEvidenceTestProfile(t, ctx, fixture.appDB, senderID, "hidden-sender")
		_, _, _, caseID := insertEvidenceTestChatFixture(t, ctx, fixture.appDB, reporterID, senderID)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+caseID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		data := assertEvidencePayload(t, w.Body.Bytes())
		caseObj, ok := data["case"].(map[string]any)
		require.True(t, ok)
		preview, ok := caseObj["resource_preview"].(map[string]any)
		require.True(t, ok)
		_, hasContentText := preview["content_text"]
		assert.False(t, hasContentText, "content_text must be omitted for hidden chat preview")
		_, hasAttachment := preview["attachment_json"]
		assert.False(t, hasAttachment, "attachment_json must never appear on normal preview")
		assert.Equal(t, true, preview["is_deleted"])
		assert.Equal(t, true, preview["evidence_available"])
		assert.Equal(t, "moderation.evidence.read", preview["evidence_requires_capability"])
		assert.Equal(t, "direct", preview["room_type"])
		assert.Equal(t, senderID.String(), preview["author_id"])
		assert.Equal(t, before, countAdminAuditRows(t, ctx, fixture.appDB))
	})

	t.Run("case-read only cannot access evidence", func(t *testing.T) {
		router := newModerationEvidenceIntegrationRouter(actorWithCapabilities(uuid.New(), []string{"moderation.case.read"}), fixture.handler)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+uuid.New().String()+"/evidence", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Equal(t, before, countAdminAuditRows(t, ctx, fixture.appDB))
	})

	t.Run("generic admin without evidence capability is denied", func(t *testing.T) {
		router := newModerationEvidenceIntegrationRouter(actorWithCapabilities(uuid.New(), []string{}), fixture.handler)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+uuid.New().String()+"/evidence", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Equal(t, before, countAdminAuditRows(t, ctx, fixture.appDB))
	})

	t.Run("evidence access returns case-bound message and audit row", func(t *testing.T) {
		reporterID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		senderID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		senderUsername := insertEvidenceTestProfile(t, ctx, fixture.appDB, senderID, "evidence-sender")
		_, messageA, messageB, caseID := insertEvidenceTestChatFixture(t, ctx, fixture.appDB, reporterID, senderID)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		data := assertEvidencePayload(t, w.Body.Bytes())
		evidence, ok := data["evidence"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, caseID.String(), evidence["case_id"])
		assert.Equal(t, "chat_message", evidence["resource_type"])
		assert.Equal(t, messageA.String(), evidence["message_id"])
		assert.Equal(t, messageA.String(), evidence["resource_id"])
		assert.NotEqual(t, messageB.String(), evidence["message_id"], "endpoint must not allow arbitrary message substitution")
		assert.Equal(t, senderID.String(), evidence["sender_id"])
		assert.Equal(t, senderUsername, evidence["author_username"])
		assert.Equal(t, "hidden original body", evidence["original_body"])
		attachment, ok := evidence["original_attachment"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://cdn.example.com/evidence.png", attachment["media_url"])

		require.Equal(t, before+1, countAdminAuditRows(t, ctx, fixture.appDB))
		row := queryAdminAuditRow(t, ctx, fixture.appDB)
		assert.Equal(t, fixture.adminID.String(), row["actor_id"])
		assert.Equal(t, "moderation.evidence.read", row["action_type"])
		assert.Equal(t, "moderation_case", row["target_type"])
		assert.Equal(t, caseID.String(), row["target_id"])

		metadata := row["metadata"].(map[string]any)
		assert.Equal(t, caseID.String(), metadata["case_id"])
		assert.Equal(t, "chat_message", metadata["resource_type"])
		assert.Equal(t, messageA.String(), metadata["resource_id"])
		assert.Equal(t, messageA.String(), metadata["message_id"])
		assert.Equal(t, senderID.String(), metadata["sender_id"])
		_, hasBody := metadata["original_body"]
		assert.False(t, hasBody)
		_, hasAttachment := metadata["original_attachment"]
		assert.False(t, hasAttachment)
	})

	t.Run("non-chat moderation case is rejected safely", func(t *testing.T) {
		reporterID := insertEvidenceTestUser(t, ctx, fixture.appDB, "admin")
		caseID := insertEvidenceTestContentCase(t, ctx, fixture.appDB, reporterID)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, before, countAdminAuditRows(t, ctx, fixture.appDB))
	})

	t.Run("audit persistence failure fails closed", func(t *testing.T) {
		reporterID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		senderID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		_, _, _, caseID := insertEvidenceTestChatFixture(t, ctx, fixture.appDB, reporterID, senderID)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		_, err := fixture.appDB.Pool().Exec(ctx, `
			CREATE OR REPLACE FUNCTION moderation_evidence_audit_fail()
			RETURNS trigger AS $$
			BEGIN
				IF NEW.action_type = 'moderation.evidence.read' THEN
					RAISE EXCEPTION 'forced audit insert failure';
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;
		`)
		require.NoError(t, err)
		_, err = fixture.appDB.Pool().Exec(ctx, `
			DROP TRIGGER IF EXISTS moderation_evidence_audit_fail_trigger ON admin_audit_logs;
			CREATE TRIGGER moderation_evidence_audit_fail_trigger
			BEFORE INSERT ON admin_audit_logs
			FOR EACH ROW
			EXECUTE FUNCTION moderation_evidence_audit_fail();
		`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = fixture.appDB.Pool().Exec(context.Background(), `
				DROP TRIGGER IF EXISTS moderation_evidence_audit_fail_trigger ON admin_audit_logs;
				DROP FUNCTION IF EXISTS moderation_evidence_audit_fail();
			`)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), "hidden original body")
		assert.Equal(t, before, countAdminAuditRows(t, ctx, fixture.appDB))
	})

	t.Run("successful repeated reads each create their own audit row", func(t *testing.T) {
		reporterID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		senderID := insertEvidenceTestUser(t, ctx, fixture.appDB, "user")
		_, _, _, caseID := insertEvidenceTestChatFixture(t, ctx, fixture.appDB, reporterID, senderID)
		before := countAdminAuditRows(t, ctx, fixture.appDB)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation/cases/"+caseID.String()+"/evidence", nil)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		}

		assert.Equal(t, before+2, countAdminAuditRows(t, ctx, fixture.appDB))
	})
}
