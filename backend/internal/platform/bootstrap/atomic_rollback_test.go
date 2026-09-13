package bootstrap_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/bootstrap"
	capInfra "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// failingAuditLogger always fails LogTx to trigger rollback of the whole bootstrap tx.
type failingAuditLogger struct{}

func (f *failingAuditLogger) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return nil
}
func (f *failingAuditLogger) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {}
func (f *failingAuditLogger) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	return assertAnError("audit failure injected")
}

func assertAnError(msg string) error { return &fakeErr{msg} }
type fakeErr struct{ m string }
func (e *fakeErr) Error() string { return e.m }

func TestBootstrap_Atomic_RollbackOnAuditFailure(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	pgxDB := db.NewFromPool(tdb.Pool())
	capRepo := capInfra.NewCapabilityRepository(pgxDB)
	svc := bootstrap.NewService(pgxDB, capRepo, &failingAuditLogger{})

	uid := insertUser(t, tdb, "rollback@test.local", true, "active", "user")
	_, err := svc.BootstrapByID(context.Background(), uid)
	require.Error(t, err)
	require.Contains(t, err.Error(), "audit")
	// Must NOT be left as admin — entire tx rolled back
	require.Equal(t, "user", hasRole(t, tdb, uid))
	require.Len(t, listActiveCaps(t, tdb, uid), 0)
}

var _ audit.AdminAuditLogger = (*failingAuditLogger)(nil)
