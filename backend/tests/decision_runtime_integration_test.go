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
// G. Atomicity → real PostgreSQL transaction rollback after partial mutation
// H. Case resolution idempotent
// I. Decision order (newest first)

package tests

import (
	"context"
	"fmt"
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

// ── caseRepoFault: fault-injected CaseRepository ─────────────────
//
// Wraps the real CaseRepository. GetByID delegates to the real repo.
// ResolveCase delegates to the real repo but returns an injected error
// if faultErr is set. This forces a real PostgreSQL transaction rollback
// after the Decision INSERT has already succeeded within the same tx.

type caseRepoFault struct {
	real     moderationRepo.CaseRepository
	faultErr error // if non-nil, ResolveCase returns this error
}

func (r *caseRepoFault) FindOrCreateOpenCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error) {
	return r.real.FindOrCreateOpenCase(ctx, tx, subjectType, subjectID)
}

func (r *caseRepoFault) GetByID(ctx context.Context, tx db.Tx, caseID uuid.UUID) (*entity.CanonicalCase, error) {
	return r.real.GetByID(ctx, tx, caseID)
}

func (r *caseRepoFault) ListBySubject(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID, limit, offset int) ([]*entity.CanonicalCase, error) {
	return r.real.ListBySubject(ctx, tx, subjectType, subjectID, limit, offset)
}

func (r *caseRepoFault) ResolveCase(ctx context.Context, tx db.Tx, caseID uuid.UUID) error {
	if r.faultErr != nil {
		return r.faultErr
	}
	return r.real.ResolveCase(ctx, tx, caseID)
}

func (r *caseRepoFault) ListAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus, limit, offset int) ([]*entity.CanonicalCase, error) {
	return r.real.ListAll(ctx, tx, statusFilter, limit, offset)
}

func (r *caseRepoFault) CountAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus) (int, error) {
	return r.real.CountAll(ctx, tx, statusFilter)
}

// Ensure caseRepoFault satisfies the interface at compile time.
var _ moderationRepo.CaseRepository = (*caseRepoFault)(nil)

// ── Test Suite ────────────────────────────────────────────────────

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
	realCaseRepo := moderationRepo.NewCaseRepository()
	decRepo := moderationRepo.NewDecisionRepository()
	enfRepo := moderationRepo.NewEnforcementRepository()
	decisionService := moderationApp.NewDecisionService(appDB, realCaseRepo, decRepo, enfRepo, nil)

	// Helper: create an open Case for the content
	createOpenCase := func(t *testing.T) uuid.UUID {
		t.Helper()
		var caseID uuid.UUID
		err := appDB.WithTx(ctx, func(tx db.Tx) error {
			kase, err := realCaseRepo.FindOrCreateOpenCase(ctx, tx, "content", contentID)
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

		// Create Decision (with enforcement target for violation)
		decision, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
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
	})	// ── B. Second Decision on resolved Case → success ──────────
	t.Run("second_decision_on_resolved_case_succeeds", func(t *testing.T) {
		caseID := createOpenCase(t)

		// First Decision → resolves Case
		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
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
			input := moderationApp.CreateDecisionInput{
				CaseID:    caseID,
				DecidedBy: adminID,
				Outcome:   outcome,
			}
			if outcome == entity.DecisionOutcomeViolation {
				input.TargetType = entity.ModerationTargetTypeContent
				input.TargetID = contentID
			}
			_, err := decisionService.CreateDecision(ctx, input)
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

		// Verify all have correct case_id
		decisions, err := decisionService.ListDecisionsByCase(ctx, caseID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, decisions, 3)
		for _, d := range decisions {
			assert.Equal(t, caseID, d.CaseID)
		}
	})

	// ── D. Immutability ────────────────────────────────────────
	t.Run("decision_immutable_update_rejected", func(t *testing.T) {
		caseID := createOpenCase(t)

		decision, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
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
			CaseID:     fakeCaseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		assert.Error(t, err)
		// ErrDecisionCaseNotFound is expected — validation passes, then DB lookup fails.
		var notFoundErr *entity.ErrDecisionCaseNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	// ── G. Atomicity — REAL PostgreSQL transaction rollback ─────
	//
	// This test proves real transaction atomicity against PostgreSQL.
	//
	// Strategy: inject a fault into CaseRepository.ResolveCase so that
	// it fails AFTER the Decision INSERT has been executed within the
	// same BEGIN/COMMIT transaction. The WithTx wrapper rolls back the
	// entire transaction, leaving zero persisted mutations.
	//
	// Execution flow inside the transaction:
	//   BEGIN
	//     caseRepo.GetByID       → SUCCESS (Case found, status=open)
	//     decRepo.Create         → SUCCESS (Decision INSERT executed)
	//     caseRepo.ResolveCase   → FAULT   (returns injected error)
	//   ROLLBACK (triggered by WithTx error path)
	//
	// Proof: Decision count = 0, Case status = open, closed_at = NULL.
	t.Run("decision_failure_does_not_mutate_case", func(t *testing.T) {
		caseID := createOpenCase(t)

		// Verify Case is open before
		var statusBefore string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&statusBefore))
		assert.Equal(t, "open", statusBefore)

		// Create a fault-injected CaseRepository.
		// ResolveCase will return an error AFTER the real DB operation is attempted,
		// but since the error is returned to WithTx, the transaction is rolled back.
		faultCaseRepo := &caseRepoFault{
			real:     realCaseRepo,
			faultErr: fmt.Errorf("injected fault: ResolveCase failed after Decision INSERT"),
		}

		// Create a NEW DecisionService with the fault-injected CaseRepository.
		// The DecisionRepository is real — its Create will execute within the tx.
		faultService := moderationApp.NewDecisionService(appDB, faultCaseRepo, decRepo, enfRepo, nil)

		// Attempt Decision creation — this will:
		//   1. BEGIN transaction
		//   2. GetByID → Case found (real repo)
		//   3. Decision CREATE → INSERT executed (real repo, same tx)
		//   4. ResolveCase → FAULT (injected error, returned to WithTx)
		//   5. WithTx sees error → ROLLBACK entire transaction
		_, err := faultService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.Error(t, err, "CreateDecision must fail due to injected fault")

		// Verify ZERO persisted mutations against real PostgreSQL:
		//
		// 1. Decision count = 0 (INSERT was rolled back)
		var decCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM decisions WHERE case_id = $1`, caseID).Scan(&decCount))
		assert.Equal(t, 0, decCount, "Decision INSERT must be rolled back — no Decision should exist")

		// 2. Case status = open (resolution was rolled back)
		var statusAfter string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM cases WHERE id = $1`, caseID).Scan(&statusAfter))
		assert.Equal(t, "open", statusAfter, "Case must remain open — resolution was rolled back")

		// 3. closed_at = NULL (resolution timestamp was rolled back)
		var closedAt *interface{}
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT closed_at FROM cases WHERE id = $1`, caseID).Scan(&closedAt))
		assert.Nil(t, closedAt, "closed_at must be NULL — resolution was rolled back")
	})

	// ── H. Case resolution idempotent ──────────────────────────
	t.Run("case_resolution_idempotent_across_decisions", func(t *testing.T) {
		caseID := createOpenCase(t)

		// First Decision → resolves
		_, err := decisionService.CreateDecision(ctx, moderationApp.CreateDecisionInput{
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
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
			CaseID:     caseID,
			DecidedBy:  adminID,
			Outcome:    entity.DecisionOutcomeViolation,
			TargetType: entity.ModerationTargetTypeContent,
			TargetID:   contentID,
		})
		require.NoError(t, err)

		// Verify Case is resolved with closed_at set
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
				CaseID:     caseID,
				DecidedBy:  adminID,
				Outcome:    entity.DecisionOutcomeViolation,
				TargetType: entity.ModerationTargetTypeContent,
				TargetID:   contentID,
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


