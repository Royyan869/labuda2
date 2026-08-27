//go:build integration

package testdb

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/migration"
	"github.com/stretchr/testify/require"
)

func TestConcurrentBootstrapSerialization(t *testing.T) {
	loadDotEnvFromParents(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	ctx := context.Background()
	monitorDB, err := db.New(ctx, db.Config{ConnString: cfg.Database.GetTestDSN(), MaxConns: 2})
	require.NoError(t, err)
	defer monitorDB.Close()

	releaseFirst := make(chan struct{})
	firstEntered := make(chan struct{})
	var entered atomic.Int32

	testDBBootstrapBeforeReset = func() {
		if entered.Add(1) == 1 {
			close(firstEntered)
			<-releaseFirst
		}
	}
	defer func() {
		testDBBootstrapBeforeReset = nil
	}()

	errCh := make(chan error, 2)
	started := make(chan struct{}, 2)

	run := func() {
		started <- struct{}{}
		errCh <- runMigrationsWithLogger(cfg, nil)
	}

	go run()
	<-started
	<-firstEntered

	go run()
	<-started

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	require.Eventually(t, func() bool {
		var granted, waiting int
		err := monitorDB.Pool().QueryRow(waitCtx, fmt.Sprintf(`
			WITH lock_key AS (
				SELECT hashtextextended($1, 0)::bigint AS key64
			)
			SELECT
				COUNT(*) FILTER (WHERE l.granted) AS granted_count,
				COUNT(*) FILTER (WHERE NOT l.granted) AS waiting_count
			FROM pg_locks l, lock_key
			WHERE l.locktype = 'advisory'
			  AND l.classid::bigint = ((lock_key.key64 >> 32) & 4294967295)
			  AND l.objid::bigint = (lock_key.key64 & 4294967295)
		`), testDBMigrationLockKey).Scan(&granted, &waiting)
		return err == nil && granted == 1 && waiting == 1
	}, 30*time.Second, 200*time.Millisecond, "expected one granted and one waiting advisory lock for bootstrap")

	close(releaseFirst)

	firstErr := <-errCh
	secondErr := <-errCh
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)

	migrationsDir, err := testMigrationDir()
	require.NoError(t, err)
	migrations, err := migration.LoadMigrations(migrationsDir)
	require.NoError(t, err)
	require.NotEmpty(t, migrations)

	current, err := migration.CurrentVersion(ctx, monitorDB.Pool())
	require.NoError(t, err)
	require.Equal(t, migrations[len(migrations)-1].Version, current)
}

func testMigrationDir() (string, error) {
	candidates := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"../../../migrations",
		"../../../../migrations",
		"../../../../../migrations",
		"../../../../../../migrations",
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("migration directory not found")
}
