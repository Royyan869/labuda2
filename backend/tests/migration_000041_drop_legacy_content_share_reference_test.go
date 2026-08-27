//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

func TestMigration000041_DropsLegacyContentShareReferenceAndKeepsOccurrenceAuthority(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	pool := tdb.Pool()

	var applied bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = 41)`).Scan(&applied))
	require.True(t, applied, "migration 000041 must be recorded in schema_migrations")

	var shareRefExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'contents'
			  AND column_name = 'share_reference'
		)
	`).Scan(&shareRefExists))
	require.False(t, shareRefExists, "contents.share_reference must be dropped by migration 000041")

	var occTableExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'content_resource_occurrences'
		)
	`).Scan(&occTableExists))
	require.True(t, occTableExists, "content_resource_occurrences must exist after migration 000039")

	for _, constraintName := range []string{
		"content_resource_occurrence_exactly_one_source",
		"content_resource_occurrence_operation_compatibility",
		"content_resource_occurrence_no_self_reference",
	} {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM information_schema.table_constraints
				WHERE table_schema = 'public'
				  AND table_name = 'content_resource_occurrences'
				  AND constraint_name = $1
			)
		`, constraintName).Scan(&exists))
		require.True(t, exists, "expected constraint %s to exist", constraintName)
	}

	authorID := uuid.New()
	profileTargetID := uuid.New()
	contentID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at,
			phone_verified, account_status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			true, 'active', $4, $4
		)
	`, authorID, authorID.String(), authorID.String()+"@test.invalid", time.Now().UTC())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at,
			phone_verified, account_status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			true, 'active', $4, $4
		)
	`, profileTargetID, profileTargetID.String(), profileTargetID.String()+"@test.invalid", time.Now().UTC())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption, is_hidden,
			visibility, created_at, updated_at
		) VALUES (
			$1, $2, 'active', 'migration proof', false,
			'public', $3, $3
		)
	`, contentID, authorID, time.Now().UTC())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO content_resource_occurrences (
			content_id, actor_id, operation, profile_source_id, created_at
		) VALUES (
			$1, $2, 'share_to_feed', $3, NOW()
		)
	`, contentID, authorID, profileTargetID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO content_resource_occurrences (
			content_id, actor_id, operation, created_at
		) VALUES (
			$1, $2, 'share_to_feed', NOW()
		)
	`, uuid.New(), authorID)
	require.Error(t, err, "zero-source content_resource_occurrences row must be rejected")
	require.Contains(t, err.Error(), "content_resource_occurrence_exactly_one_source")
}
