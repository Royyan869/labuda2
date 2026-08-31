//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestCanonicalModerationFoundation verifies SLICE 1 schema foundation after
// a clean migration replay from zero:
//
//  1. canonical tables exist (reports, cases, decisions, enforcements)
//  2. obsolete moderation_cases is gone
//  3. canonical enums exist and carry canonical vocabulary
//  4. required indexes exist
//  5. constraints are enforced by PostgreSQL (not application-only):
//     - two active cases for same subject → rejected
//     - invalid target type → rejected
//     - invalid Decision relation (FK) → rejected
//     - invalid Enforcement relation (FK) → rejected
//     - Warning without Decision → rejected
//     - Appeal without Decision → rejected
func TestCanonicalModerationFoundation(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	exists := func(q string) bool {
		var ok bool
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&ok))
		return ok
	}

	// ── 1. Canonical tables exist ───────────────────────────────
	for _, table := range []string{"reports", "cases", "decisions", "enforcements"} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='`+table+`')`),
			"canonical table %s must exist", table)
	}
	// user_warnings and appeals are modified in place (retained tables)
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='user_warnings')`),
		"user_warnings must exist")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='appeals')`),
		"appeals must exist")

	// ── 2. Obsolete moderation_cases is gone ────────────────────
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='moderation_cases')`),
		"moderation_cases must be gone (rejected GovernanceCase schema)")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='moderation_status_enum')`),
		"moderation_status_enum must be gone")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='moderation_resource_enum')`),
		"moderation_resource_enum must be gone")

	// ── 3. Canonical enums exist with canonical vocabulary ──────
	for _, enum := range []string{
		"moderation_target_type_enum",
		"case_status_enum",
		"decision_outcome_enum",
		"enforcement_status_enum",
	} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='`+enum+`')`),
			"canonical enum %s must exist", enum)
	}

	// chat_message is NOT part of canonical moderation scope
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='moderation_target_type_enum' AND enumlabel='chat_message')`),
		"moderation_target_type_enum must NOT contain chat_message")
	// canonical target values
	for _, label := range []string{"content", "comment", "for_sale", "auction", "user"} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='moderation_target_type_enum' AND enumlabel='`+label+`')`),
			"moderation_target_type_enum must contain %s", label)
	}
	// case status: open/resolved only — no enforced/approved/rejected
	for _, label := range []string{"open", "resolved"} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='case_status_enum' AND enumlabel='`+label+`')`),
			"case_status_enum must contain %s", label)
	}
	for _, forbidden := range []string{"enforced", "approved", "rejected", "pending", "removed"} {
		require.Falsef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='case_status_enum' AND enumlabel='`+forbidden+`')`),
			"case_status_enum must NOT contain %s", forbidden)
	}
	// decision outcome: no_violation/violation
	for _, label := range []string{"no_violation", "violation"} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='decision_outcome_enum' AND enumlabel='`+label+`')`),
			"decision_outcome_enum must contain %s", label)
	}
	// enforcement lifecycle
	for _, label := range []string{"pending", "processing", "succeeded", "failed"} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='enforcement_status_enum' AND enumlabel='`+label+`')`),
			"enforcement_status_enum must contain %s", label)
	}

	// ── 4. Required indexes exist ───────────────────────────────
	for _, idx := range []string{
		"uniq_active_case_per_subject",
		"uniq_reports_one_per_reporter_subject",
		"idx_reports_reporter",
		"idx_reports_subject",
		"idx_reports_case_id",
		"idx_cases_subject",
		"idx_cases_status",
		"idx_decisions_case",
		"idx_enforcements_decision",
		"idx_enforcements_status",
		"idx_enforcements_target",
		"idx_appeals_decision_id",
	} {
		require.Truef(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='`+idx+`')`),
			"required index %s must exist", idx)
	}

	// ── 5. Constraint proof ─────────────────────────────────────

	// Fixture: two users (reporter + moderator).
	reporterID := insertModerationUser(t, ctx, pool)
	moderatorID := insertModerationUser(t, ctx, pool)

	// ── 5a. Two active cases for same subject → rejected ────────
	subjectID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO cases (subject_type, subject_id, status)
		VALUES ('content', $1, 'open')`, subjectID)
	require.NoError(t, err, "first active case must succeed")

	_, err = pool.Exec(ctx, `
		INSERT INTO cases (subject_type, subject_id, status)
		VALUES ('content', $1, 'open')`, subjectID)
	require.Error(t, err, "second active case for same subject must be rejected")
	require.Contains(t, err.Error(), "uniq_active_case_per_subject",
		"rejection must come from the partial unique index")

	// resolved case for same subject is allowed (active invariant only)
	_, err = pool.Exec(ctx, `
		INSERT INTO cases (subject_type, subject_id, status)
		VALUES ('content', $1, 'resolved')`, subjectID)
	require.NoError(t, err, "resolved case for same subject must be allowed")

	// ── 5b. Invalid target type → rejected ──────────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO reports (reporter_id, subject_type, subject_id, reason_code)
		VALUES ($1, 'chat_message', $2, 'spam')`, reporterID, uuid.New())
	require.Error(t, err, "invalid target type (chat_message) must be rejected")
	require.Contains(t, err.Error(), "moderation_target_type_enum",
		"rejection must come from the enum type constraint")

	_, err = pool.Exec(ctx, `
		INSERT INTO cases (subject_type, subject_id, status)
		VALUES ('chat_message', $1, 'open')`, uuid.New())
	require.Error(t, err, "invalid case subject_type (chat_message) must be rejected")

	// ── 5c. Valid report + case + decision chain ────────────────
	// Use a fresh subject (subjectID already has an open case from 5a).
	chainSubjectID := uuid.New()
	caseID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO cases (id, subject_type, subject_id, status)
		VALUES ($1, 'content', $2, 'open')`, caseID, chainSubjectID)
	require.NoError(t, err)

	reportID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO reports (id, reporter_id, subject_type, subject_id, reason_code, case_id)
		VALUES ($1, $2, 'content', $3, 'scam_or_fraud', $4)`, reportID, reporterID, chainSubjectID, caseID)
	require.NoError(t, err, "valid report with case correlation must succeed")

	// ── 5d. Invalid Decision relation → rejected ────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO decisions (case_id, decided_by, outcome, decision_note)
		VALUES ($1, $2, 'violation', 'note')`, uuid.New(), moderatorID)
	require.Error(t, err, "decision with non-existent case_id must be rejected (FK)")

	// immutable decision: UPDATE rejected by trigger
	decisionID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO decisions (id, case_id, decided_by, outcome, decision_note)
		VALUES ($1, $2, $3, 'violation', 'note')`, decisionID, caseID, moderatorID)
	require.NoError(t, err, "valid decision must succeed")

	_, err = pool.Exec(ctx, `UPDATE decisions SET decision_note = 'hacked' WHERE id = $1`, decisionID)
	require.Error(t, err, "decision UPDATE must be rejected (immutable append-only)")
	require.Contains(t, err.Error(), "immutable",
		"rejection must come from the decisions immutable trigger")

	// invalid outcome value rejected by enum
	_, err = pool.Exec(ctx, `
		INSERT INTO decisions (case_id, decided_by, outcome)
		VALUES ($1, $2, 'reversed')`, caseID, moderatorID)
	require.Error(t, err, "invalid decision outcome must be rejected")

	// ── 5e. Invalid Enforcement relation → rejected ─────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO enforcements (decision_id, target_type, target_id, status)
		VALUES ($1, 'content', $2, 'pending')`, uuid.New(), uuid.New())
	require.Error(t, err, "enforcement with non-existent decision_id must be rejected (FK)")

	enforcementID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO enforcements (id, decision_id, target_type, target_id, status)
		VALUES ($1, $2, 'content', $3, 'pending')`, enforcementID, decisionID, subjectID)
	require.NoError(t, err, "valid enforcement must succeed")

	// duplicate enforcement for same (decision, target) → rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO enforcements (decision_id, target_type, target_id, status)
		VALUES ($1, 'content', $2, 'pending')`, decisionID, subjectID)
	require.Error(t, err, "duplicate enforcement for same decision+target must be rejected")
	require.Contains(t, err.Error(), "enforcements_decision_target_unique",
		"rejection must come from the unique constraint")

	// invalid enforcement status rejected by enum
	_, err = pool.Exec(ctx, `
		INSERT INTO enforcements (decision_id, target_type, target_id, status)
		VALUES ($1, 'content', $2, 'dead_letter')`, decisionID, uuid.New())
	require.Error(t, err, "invalid enforcement status must be rejected")

	// ── 5f. Warning without Decision → rejected ─────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO user_warnings (user_id, issued_by, level, reason, decision_id)
		VALUES ($1, $2, 'warning', 'reason', NULL)`, reporterID, moderatorID)
	require.Error(t, err, "warning without decision_id must be rejected")

	// warning with valid decision succeeds
	_, err = pool.Exec(ctx, `
		INSERT INTO user_warnings (user_id, issued_by, level, reason, decision_id)
		VALUES ($1, $2, 'warning', 'reason', $3)`, reporterID, moderatorID, decisionID)
	require.NoError(t, err, "warning with valid decision provenance must succeed")

	// duplicate warning for same (decision, user) → rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO user_warnings (user_id, issued_by, level, reason, decision_id)
		VALUES ($1, $2, 'warning', 'reason', $3)`, reporterID, moderatorID, decisionID)
	require.Error(t, err, "duplicate warning for same (decision, user) must be rejected")
	require.Contains(t, err.Error(), "user_warnings_decision_unique",
		"rejection must come from the unique constraint")

	// ── 5g. Appeal without Decision → rejected ──────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO appeals (appealed_by, message, status, decision_id)
		VALUES ($1, 'appeal message', 'pending', NULL)`, reporterID)
	require.Error(t, err, "appeal without decision_id must be rejected")

	// appeal with valid decision succeeds
	_, err = pool.Exec(ctx, `
		INSERT INTO appeals (appealed_by, message, status, decision_id)
		VALUES ($1, 'appeal message', 'pending', $2)`, reporterID, decisionID)
	require.NoError(t, err, "appeal with valid decision provenance must succeed")
}

// insertModerationUser creates a minimal users row for FK integrity.
func insertModerationUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email)
		VALUES ($1, $2, $3)`,
		id, uuid.NewString(), id.String()+"@test.local")
	require.NoError(t, err)
	return id
}
