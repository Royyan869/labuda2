//go:build integration

package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/labuda/backend/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMigration000001NoLongerContainsChatMediaReplyAuthority(t *testing.T) {
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "migrations", "000001_canonical_schema.up.sql"))
	require.NoError(t, err)

	data, err := os.ReadFile(p)
	require.NoError(t, err)
	text := string(data)

	for _, needle := range []string{
		"chat_media_asset_status_enum",
		"chat_media_asset_type_enum",
		"reply_to_message_id",
		"reply_preview_json",
		"chat_media_assets",
		"chat_message_media_assets",
	} {
		require.False(t, strings.Contains(text, needle), "000001 still contains %q", needle)
	}
}

func TestMigration000027AppliesOnExistingSchema(t *testing.T) {
	loadDotEnvFromParentsForMigrationTest(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.Database.GetTestDSN())
	require.NoError(t, err)
	defer conn.Close(ctx)

	schemaName := "chat_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schemaName))
	require.NoError(t, err)
	defer func() {
		_, _ = conn.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schemaName))
	}()

	_, err = conn.Exec(ctx, fmt.Sprintf(`SET search_path TO %s`, schemaName))
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `CREATE TABLE users (id uuid PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE TABLE chat_rooms (id uuid PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE chat_messages (
			id uuid PRIMARY KEY,
			room_id uuid NOT NULL,
			sender_id uuid NOT NULL,
			message_type text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	upgradePath, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "migrations", "000027_chat_media_reply_authority.up.sql"))
	require.NoError(t, err)

	sqlBytes, err := os.ReadFile(upgradePath)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	_, err = conn.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

func loadDotEnvFromParentsForMigrationTest(t *testing.T) {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		return
	}

	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
