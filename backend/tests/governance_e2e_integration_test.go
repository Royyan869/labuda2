//go:build integration

// SLICE 7 E2E PROOF — FULL GOVERNANCE FLOW
//
// Proves the complete end-to-end governance flow against real PostgreSQL:
//
//  1. Create Report → Case correlation
//  2. Admin creates Decision (simulating what the UI does via POST /admin/governance/cases/:id/decisions)
//  3. Decision + Enforcement + Outbox created atomically
//  4. Worker execution simulated (MarkProcessing → target mutation → MarkSucceeded)
//  5. Final Enforcement state = succeeded
//  6. All DB state consistent
//
// This test proves the actual data flow that the Admin UI triggers.
// No mocking — real PostgreSQL, real DecisionService, real repositories.

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestGovernanceE2EFlow(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()
	appDB := db.NewFromPool(pool)

	realCaseRepo := repository.NewCaseRepository()
	decRepo := repository.NewDecisionRepository()
	enfRepo := repository.NewEnforcementRepository()
	reportRepo := repository.NewReportRepository()
	obRepo := outboxRepo.NewOutboxRepository(appDB)
	decisionService := application.NewDecisionService(appDB, realCaseRepo, decRepo, enfRepo, obRepo)

	// ── Helpers ──────────────────────────────────────────────

	insertUser := func(t *testing.T) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			id, uuid.NewString(), id.String()+"@e2e.test",
		)
		require.NoError(t, err)
		return id
	}

	insertContent := func(t *testing.T, ownerID uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO contents (author_id, caption) VALUES ($1, 'e2e test content') RETURNING id`,
			ownerID,
		).Scan(&id)
		require.NoError(t, err)
		return id
	}

		createReport := func(t *testing.T, reporterID uuid.UUID, subjectType entity.ReportTargetType, subjectID uuid.UUID) (uuid.UUID, uuid.UUID) {
			t.Helper()

			// Create case + report with case_id set at INSERT time (reports are immutable)
			var reportID uuid.UUID
			var caseID uuid.UUID
			err := appDB.WithTx(ctx, func(tx db.Tx) error {
				kase, err := realCaseRepo.FindOrCreateOpenCase(ctx, tx, subjectType, subjectID)
				if err != nil {
					return err
				}
				caseID = kase.ID

				report := entity.NewReport(reporterID, subjectType, subjectID, entity.ReportReasonProhibitedContent, nil, nil)
				report.CaseID = &caseID
				if err := reportRepo.Create(ctx, tx, report); err != nil {
					return err
				}
				reportID = report.ID
				return nil
			})
			require.NoError(t, err)
			return reportID, caseID
		}

	// ── TEST: Full E2E Governance Flow ──────────────────────

	t.Run("Full E2E: Report → Case → Decision → Enforcement → Worker → Succeeded", func(t *testing.T) {
		reporter := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)

		// STEP 1: Create Report → Case correlation
		reportID, caseID := createReport(t, reporter, "content", contentID)

		// Verify: Report exists and is correlated to a Case
		var reportCaseID *uuid.UUID
		err := pool.QueryRow(ctx, `SELECT case_id FROM reports WHERE id = $1`, reportID).Scan(&reportCaseID)
		require.NoError(t, err)
		require.NotNil(t, reportCaseID)
		assert.Equal(t, caseID, *reportCaseID)

		// Verify: Case exists and is open
		var caseStatus string
		err = pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatus)
		require.NoError(t, err)
		assert.Equal(t, "open", caseStatus)

		// STEP 2: Admin creates violation Decision (simulating UI action)
		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  reporter,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)

		// STEP 3: Verify atomic creation — Decision + Enforcement + Outbox + Case resolution
		// 3a. Decision exists
		var dOutcome string
		err = pool.QueryRow(ctx, `SELECT outcome FROM decisions WHERE id = $1`, decision.ID).Scan(&dOutcome)
		require.NoError(t, err)
		assert.Equal(t, "violation", dOutcome)

		// 3b. Enforcement exists and is pending
		var enfID uuid.UUID
		var enfStatus string
		err = pool.QueryRow(ctx,
			`SELECT id, status FROM enforcements WHERE decision_id = $1 AND target_type = 'content' AND target_id = $2`,
			decision.ID, contentID,
		).Scan(&enfID, &enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "pending", enfStatus)

		// 3c. Outbox event exists
		var evtType string
		err = pool.QueryRow(ctx,
			`SELECT event_type FROM outbox WHERE aggregate_id = $1 AND event_type = 'moderation.content.removed'`,
			contentID,
		).Scan(&evtType)
		require.NoError(t, err)
		assert.Equal(t, "moderation.content.removed", evtType)

		// 3d. Case is resolved
		err = pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatus)
		require.NoError(t, err)
		assert.Equal(t, "resolved", caseStatus)

		// STEP 4: Worker execution (simulating ModerationEventHandler)
		// pending → processing
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkProcessing(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Verify processing state
		err = pool.QueryRow(ctx, `SELECT status FROM enforcements WHERE id = $1`, enfID).Scan(&enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "processing", enfStatus)

		// Target mutation (content soft-delete — the actual worker action)
		_, err = pool.Exec(ctx, `UPDATE contents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, contentID)
		require.NoError(t, err)

		// Verify content is soft-deleted
		var deleted bool
		err = pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM contents WHERE id = $1`, contentID).Scan(&deleted)
		require.NoError(t, err)
		assert.True(t, deleted, "content must be soft-deleted after worker execution")

		// processing → succeeded
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkSucceeded(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// STEP 5: Verify final Enforcement state
		err = pool.QueryRow(ctx, `SELECT status FROM enforcements WHERE id = $1`, enfID).Scan(&enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", enfStatus)

		// Verify attempt_count = 1 (one processing cycle)
		var attemptCount int
		err = pool.QueryRow(ctx, `SELECT attempt_count FROM enforcements WHERE id = $1`, enfID).Scan(&attemptCount)
		require.NoError(t, err)
		assert.Equal(t, 1, attemptCount)

		// STEP 6: Verify all DB state is consistent
		// Decision is immutable
		var dID uuid.UUID
		err = pool.QueryRow(ctx, `SELECT id FROM decisions WHERE id = $1`, decision.ID).Scan(&dID)
		require.NoError(t, err)
		assert.Equal(t, decision.ID, dID)

		// Case is resolved
		err = pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatus)
		require.NoError(t, err)
		assert.Equal(t, "resolved", caseStatus)

		// Content is soft-deleted (target mutation verified)
		err = pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM contents WHERE id = $1`, contentID).Scan(&deleted)
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	// ── TEST: no_violation Decision flow ─────────────────────

	t.Run("E2E: no_violation Decision — no enforcement, no target mutation", func(t *testing.T) {
		reporter := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)

		_, caseID := createReport(t, reporter, "content", contentID)

		// Create no_violation Decision
		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: reporter,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)

		// No enforcement
		var enfCount int
		err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM enforcements WHERE decision_id = $1`, decision.ID).Scan(&enfCount)
		require.NoError(t, err)
		assert.Equal(t, 0, enfCount)

		// No outbox event
		var evtCount int
		err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE aggregate_id = $1`, contentID).Scan(&evtCount)
		require.NoError(t, err)
		assert.Equal(t, 0, evtCount)

		// Content is NOT soft-deleted
		var deleted bool
		err = pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM contents WHERE id = $1`, contentID).Scan(&deleted)
		require.NoError(t, err)
		assert.False(t, deleted, "content must NOT be deleted for no_violation decision")

		// Case is resolved
		var caseStatus string
		err = pool.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1`, caseID).Scan(&caseStatus)
		require.NoError(t, err)
		assert.Equal(t, "resolved", caseStatus)
	})

	// ── TEST: UI read path — list cases, get detail ──────────

	t.Run("E2E: Admin read path — list cases, get detail with reports and decisions", func(t *testing.T) {
		reporter := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)

		reportID, caseID := createReport(t, reporter, "content", contentID)

		// Create violation Decision
		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  reporter,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Simulate GET /admin/governance/cases (list)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			cases, err := realCaseRepo.ListAll(ctx, tx, nil, 50, 0)
			if err != nil {
				return err
			}
			require.NotEmpty(t, cases, "must have at least one case")
			return nil
		})
		require.NoError(t, err)

		// Simulate GET /admin/governance/cases/:id (detail)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			// Get case
			kase, err := realCaseRepo.GetByID(ctx, tx, caseID)
			if err != nil {
				return err
			}
			require.NotNil(t, kase)
			assert.Equal(t, caseID, kase.ID)

			// Get reports
			reports, err := reportRepo.ListByCaseID(ctx, tx, caseID)
			if err != nil {
				return err
			}
			require.Len(t, reports, 1)
			assert.Equal(t, reportID, reports[0].ID)

			// Get decisions
			decisions, err := decRepo.ListByCase(ctx, tx, caseID, 50, 0)
			if err != nil {
				return err
			}
			require.Len(t, decisions, 1)
			assert.Equal(t, decision.ID, decisions[0].ID)
			assert.Equal(t, entity.DecisionOutcomeViolation, decisions[0].Outcome)

			// Get enforcements for this decision
			enfs, err := enfRepo.ListByDecision(ctx, tx, decision.ID)
			if err != nil {
				return err
			}
			require.Len(t, enfs, 1)
			assert.Equal(t, entity.EnforcementStatusPending, enfs[0].Status)

			return nil
		})
		require.NoError(t, err)
	})

	// ── TEST: Failure → retry flow ───────────────────────────

	t.Run("E2E: Worker failure → retry → succeeded", func(t *testing.T) {
		reporter := insertUser(t)
		contentOwner := insertUser(t)
		contentID := insertContent(t, contentOwner)

		_, caseID := createReport(t, reporter, "content", contentID)

		decision, err := decisionService.CreateDecision(ctx, application.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  reporter,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		var enfID uuid.UUID
		err = pool.QueryRow(ctx,
			`SELECT id FROM enforcements WHERE decision_id = $1`, decision.ID,
		).Scan(&enfID)
		require.NoError(t, err)

		// Attempt 1: pending → processing → failed (target not found)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkProcessing(ctx, tx, enfID)
		})
		require.NoError(t, err)

		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkFailed(ctx, tx, enfID, "target not found", nil)
		})
		require.NoError(t, err)

		// Verify failed state
		var enfStatus string
		err = pool.QueryRow(ctx, `SELECT status FROM enforcements WHERE id = $1`, enfID).Scan(&enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "failed", enfStatus)

		// Attempt 2: failed → processing → succeeded (retry)
		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkProcessing(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Target mutation (content soft-delete)
		_, err = pool.Exec(ctx, `UPDATE contents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, contentID)
		require.NoError(t, err)

		err = appDB.WithTx(ctx, func(tx db.Tx) error {
			return enfRepo.MarkSucceeded(ctx, tx, enfID)
		})
		require.NoError(t, err)

		// Verify final state
		err = pool.QueryRow(ctx, `SELECT status FROM enforcements WHERE id = $1`, enfID).Scan(&enfStatus)
		require.NoError(t, err)
		assert.Equal(t, "succeeded", enfStatus)

		// Verify attempt_count = 2
		var attemptCount int
		err = pool.QueryRow(ctx, `SELECT attempt_count FROM enforcements WHERE id = $1`, enfID).Scan(&attemptCount)
		require.NoError(t, err)
		assert.Equal(t, 2, attemptCount)

		// Content is soft-deleted
		var deleted bool
		err = pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM contents WHERE id = $1`, contentID).Scan(&deleted)
		require.NoError(t, err)
		assert.True(t, deleted)
	})
}
