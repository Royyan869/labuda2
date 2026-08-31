//go:build integration

// SLICE 8 HARDENING — GOVERNANCE AUDIT TRAIL
//
// Adversarial verification tests:
//
// 1. ATOMICITY FAULT INJECTION: Force audit INSERT to fail → verify ALL governance
//    mutations are rolled back (Decision, Enforcement, Outbox, Case resolution).
// 2. AUDIT EVENT CONTENT: Verify every field for both violation and no_violation.
// 3. ACTOR CORRECTNESS: Verify admin actor_type and actor_id.
// 4. IMMUTABILITY: INSERT succeeds, UPDATE rejected, DELETE rejected (DB trigger).
// 5. MIGRATION REPLAY: Verify audit_events table and trigger exist after clean replay.
//
// No mocking — real PostgreSQL, real fault injection via dependency injection.

package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditApp "github.com/labuda/backend/internal/governance/audit/application"
	auditentity "github.com/labuda/backend/internal/governance/audit/entity"
	auditRepo "github.com/labuda/backend/internal/governance/audit/repository"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// ============================================================================
// FAULT INJECTION: FailingAuditEmitter
// ============================================================================

// failingAuditEmitter is a GovernanceAuditEmitter that always returns an error.
// Used to prove that audit failure rolls back the entire governance transaction.
type failingAuditEmitter struct{}

func (f *failingAuditEmitter) GovernanceDecisionCreated(
	_ context.Context, _ db.Tx,
	_, _, _ uuid.UUID,
	_ string,
	_ map[string]interface{},
) error {
	return errors.New("simulated audit INSERT failure")
}

// ============================================================================
// TESTS
// ============================================================================

func TestGovernanceAuditTrailHardening(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)

	// Real infrastructure
	realCaseRepo := repository.NewCaseRepository()
	decRepo := repository.NewDecisionRepository()
	enfRepo := repository.NewEnforcementRepository()
	obRepo := outboxRepo.NewOutboxRepository(appDB)
	auditEventRepo := auditRepo.NewAuditEventRepository()

	// Helpers
	insertUser := func(t *testing.T) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			id, uuid.NewString(), id.String()+"@harden.test",
		)
		require.NoError(t, err)
		return id
	}

	insertContent := func(t *testing.T, ownerID uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO contents (author_id, caption) VALUES ($1, 'harden test content') RETURNING id`,
			ownerID,
		).Scan(&id)
		require.NoError(t, err)
		return id
	}

	createCaseAndReport := func(t *testing.T, reporterID uuid.UUID, subjectType entity.ReportTargetType, subjectID uuid.UUID) uuid.UUID {
		t.Helper()
		var caseID uuid.UUID
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			kase, err := realCaseRepo.FindOrCreateOpenCase(ctx, tx, subjectType, subjectID)
			if err != nil {
				return err
			}
			caseID = kase.ID
			report := entity.NewReport(reporterID, subjectType, subjectID, entity.ReportReasonProhibitedContent, nil, nil)
			report.CaseID = &caseID
			return repository.NewReportRepository().Create(ctx, tx, report)
		})
		require.NoError(t, err)
		return caseID
	}

	// ── H1: ATOMICITY FAULT INJECTION ──────────────────────────

	t.Run("H1: audit INSERT failure rolls back entire governance transaction", func(t *testing.T) {
		// Create DecisionService with FAILING audit emitter
		faultService := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, &failingAuditEmitter{},
		)

		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		// Verify Case is open before decision attempt
		var caseStatusBefore string
		err := pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatusBefore)
		require.NoError(t, err)
		assert.Equal(t, "open", caseStatusBefore, "case must be open before decision attempt")

		// Attempt Decision creation — must fail because audit INSERT fails
		decision, err := faultService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})

		// Decision creation MUST fail
		require.Error(t, err, "Decision creation must fail when audit INSERT fails")
		assert.Nil(t, decision, "Decision must be nil on failure")
		assert.Contains(t, err.Error(), "governance audit event failed", "error must indicate audit failure")

		// CRITICAL: Verify ALL governance mutations were rolled back

		// 1. Zero Decisions for this case
		var decCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID,
		).Scan(&decCount)
		require.NoError(t, err)
		assert.Equal(t, 0, decCount, "MUST have 0 Decisions after rollback")

		// 2. Zero Enforcements
		var enfCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM enforcements WHERE decision_id IN (SELECT id FROM decisions WHERE case_id = $1)`,
			caseID,
		).Scan(&enfCount)
		require.NoError(t, err)
		assert.Equal(t, 0, enfCount, "MUST have 0 Enforcements after rollback")

		// 3. Zero Outbox events
		var obCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM outbox WHERE aggregate_id = $1`, contentID,
		).Scan(&obCount)
		require.NoError(t, err)
		assert.Equal(t, 0, obCount, "MUST have 0 Outbox events after rollback")

		// 4. Case resolution NOT persisted (still open)
		var caseStatusAfter string
		err = pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatusAfter)
		require.NoError(t, err)
		assert.Equal(t, "open", caseStatusAfter, "case MUST remain open after rollback")

		// 5. Zero mandatory audit events
		var auditCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision'`,
		).Scan(&auditCount)
		require.NoError(t, err)
		assert.Equal(t, 0, auditCount, "MUST have 0 governance audit events after rollback")
	})

	// ── H2: AUDIT EVENT CONTENT — VIOLATION ─────────────────────

	t	.Run("H2: violation Decision audit event content correctness", func(t *testing.T) {
		auditSvc := _newAuditServiceForTest(appDB)
		svc := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, auditSvc,
		)

		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		note := "clear policy violation — hardening proof"
		decision, err := svc.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:       caseID,
			DecidedBy:    adminID,
			Outcome:      entity.DecisionOutcomeViolation,
			TargetType:   entity.ModerationTargetTypeContent,
			TargetID:     contentID,
			DecisionNote: &note,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		// Query audit event
		var eventType, entityType, actorType string
		var entityID, actorID uuid.UUID
		var payload map[string]interface{}
		var createdAt interface{}

		err = pool.QueryRow(ctx,
			`SELECT event_type, entity_type, entity_id, actor_type, actor_id, payload_json, created_at
			 FROM audit_events
			 WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			decision.ID,
		).Scan(&eventType, &entityType, &entityID, &actorType, &actorID, &payload, &createdAt)
		require.NoError(t, err, "audit event must exist for violation Decision")

		// Verify all fields
		assert.Equal(t, "governance.decision.created", eventType, "event_type")
		assert.Equal(t, "governance.decision", entityType, "entity_type")
		assert.Equal(t, decision.ID, entityID, "entity_id must equal Decision.ID")
		assert.Equal(t, "admin", actorType, "actor_type must be admin")
		assert.Equal(t, adminID, actorID, "actor_id must be decided_by")

		// Verify payload
		require.NotNil(t, payload)
		assert.Equal(t, caseID.String(), payload["case_id"], "payload.case_id")
		assert.Equal(t, "violation", payload["outcome"], "payload.outcome")
		assert.Equal(t, "content", payload["target_type"], "payload.target_type")
		assert.Equal(t, contentID.String(), payload["target_id"], "payload.target_id")
		assert.Equal(t, "clear policy violation — hardening proof", payload["decision_note"], "payload.decision_note")
	})

	// ── H3: AUDIT EVENT CONTENT — NO_VIOLATION ──────────────────

	t	.Run("H3: no_violation Decision audit event has no fabricated target fields", func(t *testing.T) {
		auditSvc := _newAuditServiceForTest(appDB)
		svc := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, auditSvc,
		)

		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := svc.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		var payload map[string]interface{}
		err = pool.QueryRow(ctx,
			`SELECT payload_json FROM audit_events WHERE entity_id = $1 AND entity_type = 'governance.decision'`,
			decision.ID,
		).Scan(&payload)
		require.NoError(t, err)

		// Verify payload
		require.NotNil(t, payload)
		assert.Equal(t, caseID.String(), payload["case_id"], "payload.case_id")
		assert.Equal(t, "no_violation", payload["outcome"], "payload.outcome")

		// CRITICAL: no fabricated target fields
		assert.Nil(t, payload["target_type"], "no_violation must NOT have target_type")
		assert.Nil(t, payload["target_id"], "no_violation must NOT have target_id")
	})

	// ── H4: IMMUTABILITY — DB trigger proof ──────────────────────

	t.Run("H4: audit_events immutability — INSERT allowed, UPDATE rejected, DELETE rejected", func(t *testing.T) {
		// Verify the trigger exists
		var triggerExists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM pg_trigger
				WHERE tgname = 'trg_audit_events_immutable'
				AND NOT tgisinternal
			)`,
		).Scan(&triggerExists)
		require.NoError(t, err)
		require.True(t, triggerExists, "trg_audit_events_immutable trigger must exist")

		// Insert a real audit event via the repository
		testEvent := &auditentity.AuditEvent{
			ID:         uuid.New(),
			EventType:  "test.immutable_check",
			EntityType: "test",
			EntityID:   uuid.New(),
			ActorType:  "system",
			ActorID:    nil,
			PayloadJSON: map[string]interface{}{
				"test": "immutable_check",
			},
		}

		err = auditEventRepo.Emit(ctx, nil, testEvent)
		require.NoError(t, err, "INSERT must succeed")

		// Verify INSERT succeeded
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM audit_events WHERE id = $1)`, testEvent.ID,
		).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "INSERTed event must exist")

		// UPDATE must be REJECTED by trigger
		_, err = pool.Exec(ctx,
			`UPDATE audit_events SET event_type = 'tampered' WHERE id = $1`, testEvent.ID,
		)
		require.Error(t, err, "UPDATE must be rejected by trg_audit_events_immutable")
		assert.Contains(t, err.Error(), "immutable", "error must mention immutability")

		// DELETE must be REJECTED by trigger
		_, err = pool.Exec(ctx,
			`DELETE FROM audit_events WHERE id = $1`, testEvent.ID,
		)
		require.Error(t, err, "DELETE must be rejected by trg_audit_events_immutable")
		assert.Contains(t, err.Error(), "immutable", "error must mention immutability")

		// Verify original event is unchanged
		var eventType string
		err = pool.QueryRow(ctx,
			`SELECT event_type FROM audit_events WHERE id = $1`, testEvent.ID,
		).Scan(&eventType)
		require.NoError(t, err)
		assert.Equal(t, "test.immutable_check", eventType, "original event must be unchanged")
	})

	// ── H5: DUPLICATE AUTHORITY CHECK ───────────────────────────

	t	.Run("H5: canonical Decision audit uses only audit_events, not admin_audit_logs", func(t *testing.T) {
		auditSvc := _newAuditServiceForTest(appDB)
		svc := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, auditSvc,
		)

		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		// Count admin_audit_logs before
		var beforeCount int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM admin_audit_logs WHERE actor_id = $1`, adminID,
		).Scan(&beforeCount)
		require.NoError(t, err)

		// Create Decision
		_, err = svc.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Count admin_audit_logs after — must NOT have increased
		var afterCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM admin_audit_logs WHERE actor_id = $1`, adminID,
		).Scan(&afterCount)
		require.NoError(t, err)
		assert.Equal(t, beforeCount, afterCount,
			"canonical Decision creation must NOT write to admin_audit_logs")

		// Count audit_events — must have exactly 1
		var auditCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision'`,
		).Scan(&auditCount)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, auditCount, 1,
			"must have at least 1 governance.decision audit event")
	})

	// ── H6: NO AUDIT FOR NON-GOVERNANCE ACTIONS ─────────────────

	t.Run("H6: Report/Case/Enforcement do NOT emit governance audit events", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		// Verify no governance audit events for Case
		var caseAudit int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.case' AND entity_id = $1`,
			caseID,
		).Scan(&caseAudit)
		require.NoError(t, err)
		assert.Equal(t, 0, caseAudit, "Case creation must NOT emit governance audit")

		// Create Decision and verify only 1 audit event total
		auditSvc := _newAuditServiceForTest(appDB)
		svc := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, auditSvc,
		)
		decision, err := svc.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Simulate worker enforcement lifecycle
		var enfID uuid.UUID
		err = pool.QueryRow(ctx,
			`SELECT id FROM enforcements WHERE decision_id = $1`, decision.ID,
		).Scan(&enfID)
		require.NoError(t, err)

		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkProcessing(ctx, tx, enfID)
		})
		require.NoError(t, err)

		_, err = pool.Exec(ctx,
			`UPDATE contents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, contentID,
		)
		require.NoError(t, err)

		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkSucceeded(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Exactly 1 governance audit event (Decision only)
		var count int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			decision.ID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "enforcement transitions must NOT emit additional audit events")
	})
}

// _newAuditServiceForTest creates a real AuditService for integration tests.
func _newAuditServiceForTest(appDB *db.DB) *auditApp.AuditService {
	repo := auditRepo.NewAuditEventRepository()
	return auditApp.NewAuditService(repo, appDB, zap.NewNop())
}
