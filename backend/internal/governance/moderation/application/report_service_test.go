package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// mockReportTransactor is a test transactor that runs fn directly.
type mockReportTransactor struct {
	fn func(ctx context.Context, fn func(tx db.Tx) error) error
}

func (m *mockReportTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if m.fn != nil {
		return m.fn(ctx, fn)
	}
	return fn(&mockReportTx{})
}

// mockReportTx is a minimal db.Tx stub (used only as a pass-through token).
type mockReportTx struct{}

var _ db.Tx = (*mockReportTx)(nil)

func (m *mockReportTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockReportTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (m *mockReportTx) QueryRow(context.Context, string, ...any) pgx.Row         { return nil }
func (m *mockReportTx) Commit(context.Context) error                            { return nil }
func (m *mockReportTx) Rollback(context.Context) error                          { return nil }

// mockReportRepo is a configurable ReportRepository.
type mockReportRepo struct {
	createFn        func(ctx context.Context, tx db.Tx, report *entity.Report) error
	validateFn      func(ctx context.Context, tx db.Tx, st entity.ReportTargetType, id uuid.UUID) (*entity.EvidenceSnapshot, error)
	getByIDFn       func(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Report, error)
	listFn          func(ctx context.Context, tx db.Tx, reporter uuid.UUID, limit, offset int) ([]*entity.Report, error)
	hasReportedFn   func(ctx context.Context, tx db.Tx, reporter uuid.UUID, st entity.ReportTargetType, id uuid.UUID) (bool, error)
}

func (m *mockReportRepo) Create(ctx context.Context, tx db.Tx, report *entity.Report) error {
	if m.createFn != nil {
		return m.createFn(ctx, tx, report)
	}
	return nil
}

func (m *mockReportRepo) ValidateTarget(ctx context.Context, tx db.Tx, st entity.ReportTargetType, id uuid.UUID) (*entity.EvidenceSnapshot, error) {
	if m.validateFn != nil {
		return m.validateFn(ctx, tx, st, id)
	}
	return &entity.EvidenceSnapshot{AuthorID: uuid.NewString()}, nil
}

func (m *mockReportRepo) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Report, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, tx, id)
	}
	return nil, nil
}

func (m *mockReportRepo) ListByReporter(ctx context.Context, tx db.Tx, reporter uuid.UUID, limit, offset int) ([]*entity.Report, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tx, reporter, limit, offset)
	}
	return nil, nil
}

func (m *mockReportRepo) HasUserReported(ctx context.Context, tx db.Tx, reporter uuid.UUID, st entity.ReportTargetType, id uuid.UUID) (bool, error) {
	if m.hasReportedFn != nil {
		return m.hasReportedFn(ctx, tx, reporter, st, id)
	}
	return false, nil
}

func TestReportService_CreateReport_RejectsInvalidTarget(t *testing.T) {
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  uuid.New(),
		SubjectType: "chat_message",
		SubjectID:   uuid.New(),
		ReasonCode:  entity.ReportReasonOther,
	})
	require.Error(t, err)
	var invalidTarget *entity.ErrInvalidReportTarget
	require.True(t, errors.As(err, &invalidTarget))
}

func TestReportService_CreateReport_RejectsInvalidReason(t *testing.T) {
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  uuid.New(),
		SubjectType: entity.ReportTargetContent,
		SubjectID:   uuid.New(),
		ReasonCode:  "spam", // not in locked taxonomy
	})
	require.Error(t, err)
	var invalidReason *entity.ErrInvalidReasonCode
	require.True(t, errors.As(err, &invalidReason))
}

func TestReportService_CreateReport_TargetNotFound(t *testing.T) {
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{
		validateFn: func(context.Context, db.Tx, entity.ReportTargetType, uuid.UUID) (*entity.EvidenceSnapshot, error) {
			return nil, &repository.ErrReportTargetNotFound{SubjectID: uuid.New()}
		},
	})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  uuid.New(),
		SubjectType: entity.ReportTargetContent,
		SubjectID:   uuid.New(),
		ReasonCode:  entity.ReportReasonOther,
	})
	require.Error(t, err)
	var notFound *repository.ErrReportTargetNotFound
	require.True(t, errors.As(err, &notFound))
}

func TestReportService_CreateReport_SelfReportDenied(t *testing.T) {
	owner := uuid.New()
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{
		validateFn: func(context.Context, db.Tx, entity.ReportTargetType, uuid.UUID) (*entity.EvidenceSnapshot, error) {
			return &entity.EvidenceSnapshot{AuthorID: owner.String()}, nil
		},
	})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  owner,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   uuid.New(),
		ReasonCode:  entity.ReportReasonOther,
	})
	require.Error(t, err)
	var selfReport *ErrSelfReportDenied
	require.True(t, errors.As(err, &selfReport))
}

func TestReportService_CreateReport_DuplicateRejected(t *testing.T) {
	reporter := uuid.New()
	subjectID := uuid.New()
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{
		hasReportedFn: func(context.Context, db.Tx, uuid.UUID, entity.ReportTargetType, uuid.UUID) (bool, error) {
			return true, nil
		},
	})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  reporter,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   subjectID,
		ReasonCode:  entity.ReportReasonOther,
	})
	require.Error(t, err)
	var dup *repository.ErrDuplicateReport
	require.True(t, errors.As(err, &dup))
}

func TestReportService_CreateReport_ConcurrentDuplicateFromDB(t *testing.T) {
	// The repository may raise ErrDuplicateReport from the unique index
	// even if the pre-check passed (race). The service must propagate it.
	reporter := uuid.New()
	subjectID := uuid.New()
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{
		createFn: func(context.Context, db.Tx, *entity.Report) error {
			return &repository.ErrDuplicateReport{ReporterID: reporter, SubjectType: entity.ReportTargetContent, SubjectID: subjectID}
		},
	})
	_, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  reporter,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   subjectID,
		ReasonCode:  entity.ReportReasonOther,
	})
	require.Error(t, err)
	var dup *repository.ErrDuplicateReport
	require.True(t, errors.As(err, &dup))
}

func TestReportService_CreateReport_Success(t *testing.T) {
	reporter := uuid.New()
	subjectID := uuid.New()
	var created *entity.Report
	svc := NewReportService(&mockReportTransactor{}, &mockReportRepo{
		createFn: func(_ context.Context, _ db.Tx, report *entity.Report) error {
			created = report
			return nil
		},
	})
	report, err := svc.CreateReport(context.Background(), CreateReportInput{
		ReporterID:  reporter,
		SubjectType: entity.ReportTargetContent,
		SubjectID:   subjectID,
		ReasonCode:  entity.ReportReasonScamOrFraud,
	})
	require.NoError(t, err)
	require.NotNil(t, report)
	require.NotNil(t, created)
	require.Equal(t, reporter, created.ReporterID)
	require.Equal(t, entity.ReportTargetContent, created.SubjectType)
	require.Equal(t, subjectID, created.SubjectID)
	require.Equal(t, entity.ReportReasonScamOrFraud, created.ReasonCode)
	require.NotNil(t, created.EvidenceSnapshot)
}
