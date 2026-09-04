package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

type warningServiceMockTx struct{}

func (t *warningServiceMockTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *warningServiceMockTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *warningServiceMockTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return warningServiceMockRow{}
}
func (t *warningServiceMockTx) Commit(context.Context) error   { return nil }
func (t *warningServiceMockTx) Rollback(context.Context) error { return nil }

type warningServiceMockRow struct{}

func (warningServiceMockRow) Scan(_ ...any) error { return nil }

var _ db.Tx = (*warningServiceMockTx)(nil)

type warningServiceMockWarningRepo struct {
	getForUpdateHit bool
	updateCalled    bool
	listAllCalled   bool

	warningToLock *entity.UserWarning
	listWarnings  []*entity.UserWarning
	listTotal     int64
	listUserID    uuid.UUID
	listIsActive  *bool
}

func (m *warningServiceMockWarningRepo) Create(_ context.Context, _ interface{}, _ *entity.UserWarning) error {
	panic("Create should not be called — standalone warning creation removed")
}

func (m *warningServiceMockWarningRepo) GetByID(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.UserWarning, error) {
	return nil, nil
}

func (m *warningServiceMockWarningRepo) GetForUpdate(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.UserWarning, error) {
	m.getForUpdateHit = true
	return m.warningToLock, nil
}

func (m *warningServiceMockWarningRepo) Update(_ context.Context, _ interface{}, warning *entity.UserWarning) error {
	m.updateCalled = true
	m.warningToLock = warning
	return nil
}

func (m *warningServiceMockWarningRepo) ListByUser(_ context.Context, _ interface{}, _ uuid.UUID, limit, offset int) ([]*entity.UserWarning, error) {
	return nil, nil
}

func (m *warningServiceMockWarningRepo) ListActiveByUser(_ context.Context, _ interface{}, _ uuid.UUID) ([]*entity.UserWarning, error) {
	return nil, nil
}

func (m *warningServiceMockWarningRepo) ListAll(_ context.Context, _ interface{}, userID *uuid.UUID, isActive *bool, limit, offset int) ([]*entity.UserWarning, int64, error) {
	m.listAllCalled = true
	if userID != nil {
		m.listUserID = *userID
	}
	m.listIsActive = isActive
	return m.listWarnings, m.listTotal, nil
}

func TestRevokeWarning(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	warnRepo := &warningServiceMockWarningRepo{
		warningToLock: &entity.UserWarning{ID: uuid.New(), IsActive: true},
	}
	svc := NewWarningService(warnRepo)

	warning, err := svc.RevokeWarning(ctx, tx, warnRepo.warningToLock.ID, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !warnRepo.getForUpdateHit {
		t.Fatal("GetForUpdate must be used for revocation")
	}
	if !warnRepo.updateCalled {
		t.Fatal("Update must be called for revocation")
	}
	if warning == nil || warning.IsActive {
		t.Fatal("revoked warning must be returned as inactive")
	}
}

func TestListAllWarningsForwardsActiveFilterAndCount(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	active := true
	userID := uuid.New()
	warnRepo := &warningServiceMockWarningRepo{
		listWarnings: []*entity.UserWarning{{ID: uuid.New(), IsActive: true}},
		listTotal:    1,
	}
	svc := NewWarningService(warnRepo)

	warnings, total, err := svc.ListAllWarnings(ctx, tx, &userID, &active, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !warnRepo.listAllCalled {
		t.Fatal("ListAll must be called")
	}
	if warnRepo.listIsActive == nil || *warnRepo.listIsActive != active {
		t.Fatal("is_active filter must be forwarded to repository")
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings length = %d, want 1", len(warnings))
	}
}
