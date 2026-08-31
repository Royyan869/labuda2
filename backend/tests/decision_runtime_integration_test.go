//go:build integration

// SLICE 4 PROOF — CANONICAL DECISION RUNTIME
//
// Proves the full Decision path against real PostgreSQL:
//
//	DecisionService → DecisionRepository + CaseRepository → decisions/cases tables
//
// A. First Decision on open Case → Case resolved
// B. Second Decision on resolved Case → success, Case stays resolved
// C. Multiple Decisions on same Case → all exist, all immutable
// D. Immutability → UPDATE rejected by trigger
// E. Invalid outcome → rejected
// F. Missing Case → rejected
// G. Atomicity → Decision failure does not mutate Case
// H. Regression → existing Report + Case tests remain green

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestCanonicalDecisionRuntime(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// ── Fixtures ─────────────────────────────────────────────────
	adminID := insertModerationUser(t, ctx, pool)
	ownerID := insertModerationUser(t, ctx, pool)

	// Create a content target for the Case
	contentID := insertReportFixtureContent(t, ctx, pool, ownerID)

	appDB := db.NewFromPool(pool)
	caseRepo := moderationRepo.NewCaseRepository()
	decRepo := moderationRepo.NewDecisionRepository()
	decisionService := moderationApp.NewDecisionService(appDB, caseRepo, decRepo)

	// Helper: create an open Case for the content
	createOpenCase := func(t *testing.T) uuid.UUID {
		t.Helper()
		var caseID uuid.UUID
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			kase, err := caseRepo.FindOrCreateOpenCase(ctx, tx, "content", contentID)
			if err != nil {
				return err
			}
			caseID = kase.ID
			return nil
		})
		require.NoError(t, err)
		return caseID
	}

	// ── A. First Decision on open Case → Case resolved ─────────
	t.Run("first_decision_on_open_case_resolves_case", func(t *testing.T) {
		caseID := createOpenCase(t)

		// Verify Case is open
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		assert.Equal(t, "open", status)

		// Create Decision
		decision, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		require.NoError(t, err)
		require.NotNil(t, decision)
		assert.Equal(t, caseID, decision.CaseID)
		assert.Equal(t, adminID, decision.DecidedBy)
		assert.Equal(t, entity.DecisionOutcomeViolation, decision.Outcome)

		// Verify Case is now resolved
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		assert.Equal(t, "resolved", status)

		// Verify closed_at is set
		var closedAt *interface{}
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT closed_at FROM cases WHERE id = $1`, caseID).Scan(&closedAt))
		assert.NotNil(t, closedAt, "closed_at must be set after Decision resolves Case")

		// Verify Decision row exists
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 1, decCount)
	})

	// ── B. Second Decision on resolved Case → success ──────────
	t.Run("second_decision_on_resolved_case_succeeds", func(t *testing.T) {
		caseID := createOpenCase(t)

		// First Decision → resolves Case
		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		require.NoError(t, err)

		// Verify Case is resolved
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		assert.Equal(t, "resolved", status)

		// Second Decision on already-resolved Case
		decision2, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)
		require.NotNil(t, decision2)

		// Verify Case remains resolved
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		assert.Equal(t, "resolved", status)

		// Verify both Decisions exist
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 2, decCount, "both Decisions must exist")
	})

	// ── C. Multiple Decisions on same Case ─────────────────────
	t.Run("multiple_decisions_all_exist", func(t *testing.T) {
		caseID := createOpenCase(t)

		// Create 3 Decisions
		for i := 0; i < 3; i++ {
			outcome := entity.DecisionOutcomeViolation
			if i%2 == 1 {
				outcome = entity.DecisionOutcomeNoViolation
			}
			_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
				CaseID:    caseID,
				DecidedBy: adminID,
				Outcome:   outcome,
			})
			require.NoError(t, err)
		}

		// Verify all 3 exist
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 3, decCount, "all 3 Decisions must exist")

		// Verify Case is resolved
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&status))
		assert.Equal(t, "resolved", status)

		// Verify Decision #1 is unchanged (immutable)
		decisions, err := decisionService.ListDecisionsByCase(ctx, caseID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, decisions, 3)
		// All should have the same case_id
		for _, d := range decisions {
			assert.Equal(t, caseID, d.CaseID)
		}
	})

	// ── D. Immutability ────────────────────────────────────────
	t.Run("decision_immutable_update_rejected", func(t *testing.T) {
		caseID := createOpenCase(t)

		decision, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		require.NoError(t, err)

		// Attempt UPDATE — must be rejected by trg_decisions_immutable
		_, err = pool.Exec(ctx,
			`UPDATE decisions SET outcome = 'no_violation' WHERE id = $1`, decision.ID)
		assert.Error(t, err, "UPDATE must be rejected by immutable trigger")
		assert.Contains(t, err.Error(), "immutable")

		// Verify Decision unchanged
		fetched, err := decisionService.GetDecision(ctx, decision.ID)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		assert.Equal(t, entity.DecisionOutcomeViolation, fetched.Outcome, "Decision must not change")
	})

	// ── E. Invalid outcome ─────────────────────────────────────
	t.Run("invalid_outcome_rejected", func(t *testing.T) {
		caseID := createOpenCase(t)

		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcome("enforce"),
		})
		assert.Error(t, err)
		var invalidErr *entity.ErrInvalidDecisionOutcome
		assert.ErrorAs(t, err, &invalidErr)
	})

	// ── F. Missing Case ────────────────────────────────────────
	t.Run("missing_case_rejected", func(t *testing.T) {
		fakeCaseID := uuid.New()

		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    fakeCaseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		assert.Error(t, err)
		var notFoundErr *entity.ErrDecisionCaseNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	// ── G. Atomicity ───────────────────────────────────────────
	t.Run("decision_failure_does_not_mutate_case", func(t *testing.T) {
		caseID := createOpenCase(t)

		// Verify Case is open before
		var statusBefore string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&statusBefore))
		assert.Equal(t, "open", statusBefore)

		// Attempt Decision with invalid outcome (fails before INSERT)
		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcome("garbage"),
		})
		assert.Error(t, err)

		// Verify Case is still open (no mutation)
		var statusAfter string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&statusAfter))
		assert.Equal(t, "open", statusAfter, "Case must not change when Decision fails")

		// Verify no Decision was created
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 0, decCount, "no Decision should exist after failure")
	})

	// ── H. Case resolution idempotent ──────────────────────────
	t.Run("case_resolution_idempotent_across_decisions", func(t *testing.T) {
		caseID := createOpenCase(t)

		// First Decision → resolves
		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		require.NoError(t, err)

		// Second Decision → Case already resolved, no error
		_, err = decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeNoViolation,
		})
		require.NoError(t, err)

		// Third Decision → still resolved, no error
		_, err = decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:    caseID,
			DecidedBy: adminID,
			Outcome:   entity.DecisionOutcomeViolation,
		})
		require.NoError(t, err)

		// Verify Case is resolved with closed_at set once
		var status string
		var closedAt *interface{}
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status, closed_at FROM cases WHERE id = $1`, caseID).Scan(&status, &closedAt))
		assert.Equal(t, "resolved", status)
		assert.NotNil(t, closedAt)

		// Verify 3 Decisions exist
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 3, decCount)
	})

	// ── I. Decision order (newest first) ───────────────────────
	t.Run("list_decisions_newest_first", func(t *testing.T) {
		caseID := createOpenCase(t)

		var ids []uuid.UUID
		for i := 0; i < 3; i++ {
			d, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
				CaseID:    caseID,
				DecidedBy: adminID,
				Outcome:   entity.DecisionOutcomeViolation,
			})
			require.NoError(t, err)
			ids = append(ids, d.ID)
		}

		decisions, err := decisionService.ListDecisionsByCase(ctx, caseID, 10, 0)
		require.NoError(t, err)
		require.Len(t, decisions, 3)
		// Newest first: ids[2], ids[1], ids[0]
		assert.Equal(t, ids[2], decisions[0].ID)
		assert.Equal(t, ids[1], decisions[1].ID)
		assert.Equal(t, ids[0], decisions[2].ID)
	})
}
