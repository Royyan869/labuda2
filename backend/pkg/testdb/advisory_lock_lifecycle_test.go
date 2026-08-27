//go:build integration

package testdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/config"
)

// TestSetup_ReturnsLiveTestDatabase proves Setup returns a usable test pool
// bound to the configured test database.
func TestSetup_ReturnsLiveTestDatabase(t *testing.T) {
	loadDotEnvFromParents(t)
	cfg, err := config.Load()
	require.NoError(t, err)

	tdb, cleanup := Setup(t, cfg)
	defer cleanup()

	require.NotNil(t, tdb.Pool(), "test pool must exist")
	require.Equal(t, cfg.Database.TestName, tdb.DatabaseName())

	// The test pool works independently of the lock connection.
	var one int
	err = tdb.Pool().QueryRow(context.Background(), "SELECT 1").Scan(&one)
	require.NoError(t, err)
	require.Equal(t, 1, one)
}

// TestSetup_CleanupReleases proves a second Setup can run after cleanup.
func TestSetup_CleanupReleases(t *testing.T) {
	loadDotEnvFromParents(t)
	cfg, err := config.Load()
	require.NoError(t, err)

	tdb, cleanup := Setup(t, cfg)

	require.NotNil(t, tdb)
	cleanup()

	tdb2, cleanup2 := Setup(t, cfg)
	defer cleanup2()
	require.NotNil(t, tdb2.Pool())
	require.Equal(t, cfg.Database.TestName, tdb2.DatabaseName())
}
