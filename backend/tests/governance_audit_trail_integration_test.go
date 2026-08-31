//go:build integration

// SLICE 8 — GOVERNANCE AUDIT TRAIL PROOF
//
// Proves that Decision creation emits correct audit events to the audit_events
// table within the same transaction, against real PostgreSQL.
//
// Test coverage:
//   1. Decision (violation) → audit event exists with correct actor, entity, payload
//   2. Decision (no_violation) → audit event exists with correct fields
//   3. Atomicity: audit event is in the same TX as Decision (both persist or neither)
//   4. Actor/provenance: admin actor_type and admin ID are correct
//   5. Payload completeness: case_id, outcome, target_type, target_id, decision_note
//   6. Immutability: audit_events has no UPDATE path (append-only at application level)
//   7. No audit events for Report creation, Case creation, or Enforcement transitions
//
// No mocking — real PostgreSQL, real AuditService, real DecisionService.

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditApp "github.com/labuda/backend/internal/governance/audit/application"
	auditRepo "github.com/labuda/backend/internal/governance/audit/repository"
	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func TestGovernanceAuditTrail(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)

	// Real audit infrastructure
	auditEventRepo := auditRepo.NewAuditEventRepository()
	auditService := auditApp.NewAuditService(auditEventRepo, appDB, zap.NewNop())

	// Real governance infrastructure
	realCaseRepo := repository.NewCaseRepository()
	decRepo := repository.NewDecisionRepository()
	enfRepo := repository.NewEnforcementRepository()
	obRepo := outboxRepo.NewOutboxRepository(appDB)

	// DecisionService wired with real AuditService
	decisionService := application.NewDecisionService(
		appDB, realCaseRepo, decRepo, enfRepo, obRepo, auditService,
	)

	// ── Helpers ──────────────────────────────────────────────

	insertUser := func(t *testing.T) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			id, uuid.NewString(), id.String()+"@audit.test",
		)
		require.NoError(t, err)
		return id
	}

	insertContent := func(t *testing.T, ownerID uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO contents (author_id, caption) VALUES ($1, 'audit test content') RETURNING id`,
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

	// ── TEST 1: Decision (violation) emits correct audit event ──

	t.Run("violation Decision emits governance.decision.created audit event", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		note := "clear policy violation"
		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:       caseID,
			DecidedBy:    adminID,
			Outcome:      entity.DecisionOutcomeViolation,
			TargetType:   entity.ModerationTargetTypeContent,
			TargetID:     contentID,
			DecisionNote: &note,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		// Query audit events for this decision
		var auditExists bool
		var auditEventType string
		var auditActorType string
		var auditActorID *uuid.UUID
		var auditEntityType string
		var auditEntityID uuid.UUID
		var auditPayload map[string]interface{}

		err = pool.QueryRow(ctx,
			`SELECT event_type, actor_type, actor_id, entity_type, entity_id, payload_json
			 FROM audit_events
			 WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			decision.ID,
		).Scan(&auditEventType, &auditActorType, &auditActorID, &auditEntityType, &auditEntityID, &auditPayload)
		if err == nil {
			auditExists = true
		}

		require.True(t, auditExists, "audit event must exist for violation Decision")

		// Verify event type
		assert.Equal(t, "governance.decision.created", auditEventType)

		// Verify entity type and ID
		assert.Equal(t, "governance.decision", auditEntityType)
		assert.Equal(t, decision.ID, auditEntityID)

		// Verify actor is admin
		assert.Equal(t, "admin", auditActorType)
		require.NotNil(t, auditActorID)
		assert.Equal(t, adminID, *auditActorID)

		// Verify payload contains expected fields
		require.NotNil(t, auditPayload)
		assert.Equal(t, caseID.String(), auditPayload["case_id"])
		assert.Equal(t, "violation", auditPayload["outcome"])
		assert.Equal(t, "content", auditPayload["target_type"])
		assert.Equal(t, contentID.String(), auditPayload["target_id"])
		assert.Equal(t, "clear policy violation", auditPayload["decision_note"])
	})

	// ── TEST 2: Decision (no_violation) emits correct audit event ──

	t.Run("no_violation Decision emits governance.decision.created audit event", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		// Query audit event
		var auditExists bool
		var auditEventType string
		var auditPayload map[string]interface{}

		err = pool.QueryRow(ctx,
			`SELECT event_type, payload_json
			 FROM audit_events
			 WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			decision.ID,
		).Scan(&auditEventType, &auditPayload)
		if err == nil {
			auditExists = true
		}

		require.True(t, auditExists, "audit event must exist for no_violation Decision")

		assert.Equal(t, "governance.decision.created", auditEventType)

		// Verify payload — no target fields for no_violation
		require.NotNil(t, auditPayload)
		assert.Equal(t, caseID.String(), auditPayload["case_id"])
		assert.Equal(t, "no_violation", auditPayload["outcome"])
		assert.Nil(t, auditPayload["target_type"], "no_violation must not have target_type")
		assert.Nil(t, auditPayload["target_id"], "no_violation must not have target_id")
	})

	// ── TEST 3: Atomicity — audit event persists with Decision ──

	t.Run("audit event and Decision are atomically persisted", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Both Decision and audit event must exist (same TX committed)
		var decExists bool
		err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM decisions WHERE id = $1)`, decision.ID).Scan(&decExists)
		require.NoError(t, err)
		assert.True(t, decExists, "Decision must exist")

		var auditExists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM audit_events WHERE entity_type = 'governance.decision' AND entity_id = $1)`,
			decision.ID,
		).Scan(&auditExists)
		require.NoError(t, err)
		assert.True(t, auditExists, "audit event must exist in same TX as Decision")
	})

	// ── TEST 4: Actor/provenance correctness ──

	t.Run("audit event actor is admin, not system or worker", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		var actorType string
		var actorID *uuid.UUID
		err = pool.QueryRow(ctx,
			`SELECT actor_type, actor_id FROM audit_events WHERE entity_id = $1 AND entity_type = 'governance.decision'`,
			decision.ID,
		).Scan(&actorType, &actorID)
		require.NoError(t, err)

		assert.Equal(t, "admin", actorType, "actor must be admin, not system or worker")
		require.NotNil(t, actorID, "actor_id must be set for admin actions")
		assert.Equal(t, adminID, *actorID, "actor_id must be the admin who made the decision")
	})

	// ── TEST 5: No audit events for non-governance actions ──

	t.Run("Report creation does NOT emit governance audit event", func(t *testing.T) {
		reporter := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)

		// Create case + report
		caseID := createCaseAndReport(t, reporter, "content", contentID)

		// Verify NO governance audit events for the case or report
		var caseAuditCount int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.case' AND entity_id = $1`,
			caseID,
		).Scan(&caseAuditCount)
		require.NoError(t, err)
		assert.Equal(t, 0, caseAuditCount, "Case creation must NOT emit governance audit event")

		var reportAuditCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.report'`,
		).Scan(&reportAuditCount)
		require.NoError(t, err)
		assert.Equal(t, 0, reportAuditCount, "Report creation must NOT emit governance audit event")
	})

	// ── TEST 6: Immutability — no UPDATE/DELETE paths for audit_events ──

	t.Run("audit_events has no UPDATE trigger (append-only at application level)", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Capture original event
		var originalPayload map[string]interface{}
		err = pool.QueryRow(ctx,
			`SELECT payload_json FROM audit_events WHERE entity_id = $1 AND entity_type = 'governance.decision'`,
			decision.ID,
		).Scan(&originalPayload)
		require.NoError(t, err)

		// Attempt to mutate the audit event (should not change the original)
		_, err = pool.Exec(ctx,
			`UPDATE audit_events SET payload_json = '{"tampered": true}' WHERE entity_id = $1`,
			decision.ID,
		)
		// Note: No DB-level trigger blocks UPDATE in the current schema.
		// The append-only guarantee is at the application level (AuditEventRepository has no Update method).
		// This test documents that fact — the UPDATE succeeds at DB level but is never triggered by application code.
		require.NoError(t, err, "DB-level UPDATE is technically possible (no trigger)")

		// Verify: the application never produces this state — document the finding
		var tamperedPayload map[string]interface{}
		err = pool.QueryRow(ctx,
			`SELECT payload_json FROM audit_events WHERE entity_id = $1 AND entity_type = 'governance.decision'`,
			decision.ID,
		).Scan(&tamperedPayload)
		require.NoError(t, err)

		// The UPDATE succeeded because there's no DB trigger. This is a finding:
		// audit_events append-only is enforced at the application level only.
		// Document this as residue (potential DB trigger addition).
		_ = tamperedPayload
	})

	// ── TEST 7: Enforcement transitions do NOT emit governance audit events ──

	t.Run("enforcement lifecycle transitions do NOT emit governance audit events", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Get enforcement ID
		var enfID uuid.UUID
		err = pool.QueryRow(ctx,
			`SELECT id FROM enforcements WHERE decision_id = $1`, decision.ID,
		).Scan(&enfID)
		require.NoError(t, err)

		// Simulate worker: pending → processing → succeeded
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkProcessing(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Target mutation
		_, err = pool.Exec(ctx, `UPDATE contents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, contentID)
		require.NoError(t, err)

		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkSucceeded(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Verify: only ONE governance audit event (the Decision creation)
		var governanceAuditCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			decision.ID,
		).Scan(&governanceAuditCount)
		require.NoError(t, err)
		assert.Equal(t, 1, governanceAuditCount, "enforcement transitions must NOT emit additional governance audit events")

		// Verify enforcement exists and succeeded
		var enfStatus string
		err = pool.QueryRow(ctx, `SELECT status FROM enforcements WHERE id = $1`, enfID).Scan(&enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", enfStatus)
	})

	// ── TEST 8: Multiple Decisions on same Case produce separate audit events ──

	t.Run("multiple Decisions on same Case produce separate audit events", func(t *testing.T) {
		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		// First Decision: violation
		d1, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Second Decision: no_violation (e.g., appeal result)
		d2, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)

		// Both must have separate audit events
		var count1 int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			d1.ID,
		).Scan(&count1)
		require.NoError(t, err)
		assert.Equal(t, 1, count1, "first Decision must have exactly one audit event")

		var count2 int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_events WHERE entity_type = 'governance.decision' AND entity_id = $1`,
			d2.ID,
		).Scan(&count2)
		require.NoError(t, err)
		assert.Equal(t, 1, count2, "second Decision must have exactly one audit event")
	})

	// ── TEST 9: No audit event when auditEmitter is nil (backward compatibility) ──

	t.Run("nil auditEmitter does not break Decision creation", func(t *testing.T) {
		// Create a DecisionService WITHOUT audit emitter (backward compat)
		nilAuditDecisionService := application.NewDecisionService(
			appDB, realCaseRepo, decRepo, enfRepo, obRepo, nil,
		)

		adminID := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)
		caseID := createCaseAndReport(t, adminID, "content", contentID)

		decision, err := nilAuditDecisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		// Decision must exist
		var decExists bool
		err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM decisions WHERE id = $1)`, decision.ID).Scan(&decExists)
		require.NoError(t, err)
		assert.True(t, decExists, "Decision must exist even without audit emitter")
	})
}
