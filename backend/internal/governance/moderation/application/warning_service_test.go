package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	userentity "github.com/labuda/backend/internal/identity/user/domain/entity"
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

type warningServiceMockUserRepo struct {
	user     *userentity.User
	err      error
	called   bool
	lastUser uuid.UUID
}

func (m *warningServiceMockUserRepo) GetByID(_ context.Context, _ db.Tx, userID uuid.UUID) (*userentity.User, error) {
	m.called = true
	m.lastUser = userID
	return m.user, m.err
}

type warningServiceMockWarningRepo struct {
	createCalled    bool
	getForUpdateHit bool
	updateCalled    bool
	listAllCalled   bool

	createdWarning *entity.UserWarning
	warningToLock  *entity.UserWarning
	listWarnings   []*entity.UserWarning
	listTotal      int64
	listUserID     uuid.UUID
	listIsActive   *bool
}

func (m *warningServiceMockWarningRepo) Create(_ context.Context, _ interface{}, warning *entity.UserWarning) error {
	m.createCalled = true
	m.createdWarning = warning
	return nil
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

type warningServiceMockOutbox struct {
	called bool
}

func (m *warningServiceMockOutbox) InsertEvent(_ context.Context, _ db.Tx, _ string, _ uuid.UUID, _ []byte) error {
	m.called = true
	return nil
}

func TestIssueWarningRejectsMissingUser(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	warnRepo := &warningServiceMockWarningRepo{}
	userRepo := &warningServiceMockUserRepo{user: nil}
	outbox := &warningServiceMockOutbox{}
	svc := NewWarningService(warnRepo, userRepo, outbox)

	_, err := svc.IssueWarning(ctx, tx, uuid.New(), uuid.New(), entity.WarningLevelWarning, "Policy violation", nil)
	if err == nil {
		t.Fatal("expected error for missing target user")
	}

	var targetErr *entity.ErrWarningTargetNotFound
	if !errors.As(err, &targetErr) {
		t.Fatalf("expected ErrWarningTargetNotFound, got %T: %v", err, err)
	}
	if warnRepo.createCalled {
		t.Fatal("warning row must not be created for missing target user")
	}
	if outbox.called {
		t.Fatal("outbox must not emit for missing target user")
	}
	if !userRepo.called {
		t.Fatal("user lookup must be attempted")
	}
}

func TestIssueWarningSucceedsForExistingUser(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	userID := uuid.New()
	adminID := uuid.New()
	warnRepo := &warningServiceMockWarningRepo{}
	userRepo := &warningServiceMockUserRepo{user: &userentity.User{ID: userID, AccountStatus: "active"}}
	outbox := &warningServiceMockOutbox{}
	svc := NewWarningService(warnRepo, userRepo, outbox)

	warning, err := svc.IssueWarning(ctx, tx, userID, adminID, entity.WarningLevelWarning, "Policy violation", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !warnRepo.createCalled {
		t.Fatal("warning row must be created for an existing user")
	}
	if !outbox.called {
		t.Fatal("outbox must emit for a successful warning issue")
	}
	if warning == nil {
		t.Fatal("expected warning result")
	}
	if warning.UserID != userID {
		t.Fatalf("warning UserID = %s, want %s", warning.UserID, userID)
	}
	if warning.IssuedBy != adminID {
		t.Fatalf("warning IssuedBy = %s, want %s", warning.IssuedBy, adminID)
	}
	if warning.ExpiresAt != nil {
		t.Fatalf("warning ExpiresAt = %v, want nil for no-expiration warning", warning.ExpiresAt)
	}
	if warning.GetStatus() != entity.WarningStatusActive {
		t.Fatalf("warning status = %s, want active", warning.GetStatus())
	}
}

func TestIssueWarningWithExpirationKeepsExpiry(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	userID := uuid.New()
	adminID := uuid.New()
	warnRepo := &warningServiceMockWarningRepo{}
	userRepo := &warningServiceMockUserRepo{user: &userentity.User{ID: userID, AccountStatus: "active"}}
	svc := NewWarningService(warnRepo, userRepo, nil)

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	warning, err := svc.IssueWarning(ctx, tx, userID, adminID, entity.WarningLevelWarning, "Policy violation", &expiresAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if warning.ExpiresAt == nil {
		t.Fatal("warning ExpiresAt must be set when an expiry is provided")
	}
}

func TestRevokeWarning(t *testing.T) {
	ctx := context.Background()
	tx := &warningServiceMockTx{}
	warnRepo := &warningServiceMockWarningRepo{
		warningToLock: &entity.UserWarning{ID: uuid.New(), IsActive: true},
	}
	svc := NewWarningService(warnRepo, &warningServiceMockUserRepo{}, nil)

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
	svc := NewWarningService(warnRepo, &warningServiceMockUserRepo{}, nil)

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


