package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// Mock Tx for ListByReporter — follows existing moderationListMockTx pattern
// =============================================================================

type listByReporterFixture struct {
	ID           uuid.UUID
	ResourceType entity.ResourceType
	ResourceID   uuid.UUID
	Status       entity.GovernanceCaseStatus
	ReportedBy   uuid.UUID
	Reason       string
	CreatedAt    time.Time
}

type listByReporterMockTx struct {
	t        *testing.T
	fixtures []listByReporterFixture

	// Inspection
	lastCountSQL  string
	lastCountArgs []any
	lastDataSQL   string
	lastDataArgs  []any

	// Failure injection
	countErr   error
	queryErr   error
	rowsErr    error
	rowScanErr error
}

var _ db.Tx = (*listByReporterMockTx)(nil)

func (m *listByReporterMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *listByReporterMockTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	m.lastCountSQL = sql
	m.lastCountArgs = append([]any(nil), args...)

	if !strings.Contains(sql, "COUNT(*) FROM moderation_cases") {
		m.t.Fatalf("expected COUNT query, got: %s", sql)
	}

	// Verify reporter predicate is always present in count query
	if !strings.Contains(sql, "reported_by =") {
		m.t.Fatalf("count query must contain reported_by predicate: %s", sql)
	}

	// Parse args: reporterID is always first, optional status filter
	reporterID := args[0].(uuid.UUID)
	var statusFilter *entity.GovernanceCaseStatus
	if strings.Contains(sql, "status =") && len(args) >= 2 {
		s := entity.GovernanceCaseStatus(args[1].(string))
		statusFilter = &s
	}

	if m.countErr != nil {
		return listByReporterErrorRow{err: m.countErr}
	}

	return listByReporterMockRow{count: int64(len(m.matchingFixtures(reporterID, statusFilter)))}
}

func (m *listByReporterMockTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.lastDataSQL = sql
	m.lastDataArgs = append([]any(nil), args...)

	if !strings.Contains(sql, "FROM moderation_cases") {
		m.t.Fatalf("expected data query, got: %s", sql)
	}
	if !strings.Contains(sql, "reported_by =") {
		m.t.Fatalf("data query must contain reported_by predicate: %s", sql)
	}

	// Parse pagination args (limit, offset are last two)
	if len(args) < 3 {
		m.t.Fatalf("expected at least reporter + limit + offset args")
	}
	limit := args[len(args)-2].(int)
	offset := args[len(args)-1].(int)

	reporterID := args[0].(uuid.UUID)
	var statusFilter *entity.GovernanceCaseStatus
	if strings.Contains(sql, "status =") {
		s := entity.GovernanceCaseStatus(args[1].(string))
		statusFilter = &s
	}

	// Verify ordering
	if !strings.Contains(sql, "ORDER BY created_at DESC") {
		m.t.Fatalf("data query must order by created_at DESC: %s", sql)
	}
	if !strings.Contains(sql, "id DESC") {
		m.t.Fatalf("data query must include id DESC tie-breaker: %s", sql)
	}

	if m.queryErr != nil {
		return nil, m.queryErr
	}

	matches := m.matchingFixtures(reporterID, statusFilter)
	if offset > len(matches) {
		return &listByReporterMockRows{err: m.rowsErr}, nil
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}

	rows := make([]listByReporterFixture, end-offset)
	copy(rows, matches[offset:end])
	return &listByReporterMockRows{rows: rows, err: m.rowsErr, scanErr: m.rowScanErr}, nil
}

func (m *listByReporterMockTx) Commit(_ context.Context) error   { return nil }
func (m *listByReporterMockTx) Rollback(_ context.Context) error { return nil }

func (m *listByReporterMockTx) matchingFixtures(reporterID uuid.UUID, statusFilter *entity.GovernanceCaseStatus) []listByReporterFixture {
	var matches []listByReporterFixture
	for _, fx := range m.fixtures {
		if fx.ReportedBy != reporterID {
			continue
		}
		if statusFilter != nil && fx.Status != *statusFilter {
			continue
		}
		matches = append(matches, fx)
	}
	// Simulate created_at DESC, id DESC ordering
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].CreatedAt.Before(matches[j].CreatedAt) ||
				(matches[i].CreatedAt.Equal(matches[j].CreatedAt) &&
					matches[i].ID.String() < matches[j].ID.String()) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	return matches
}

// =============================================================================
// Mock Rows
// =============================================================================

type listByReporterMockRow struct{ count int64 }

type listByReporterErrorRow struct{ err error }

func (r listByReporterMockRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return fmt.Errorf("unexpected scan target count: %d", len(dest))
	}
	switch d := dest[0].(type) {
	case *int64:
		*d = r.count
		return nil
	default:
		return fmt.Errorf("unexpected scan dest type %T", dest[0])
	}
}

func (r listByReporterErrorRow) Scan(_ ...any) error {
	return r.err
}

type listByReporterMockRows struct {
	rows    []listByReporterFixture
	idx     int
	err     error
	scanErr error
}

func (r *listByReporterMockRows) Close()                                       {}
func (r *listByReporterMockRows) Err() error                                   { return r.err }
func (r *listByReporterMockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *listByReporterMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *listByReporterMockRows) Conn() *pgx.Conn                              { return nil }
func (r *listByReporterMockRows) RawValues() [][]byte                          { return nil }

func (r *listByReporterMockRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *listByReporterMockRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("scan out of sequence")
	}
	if r.scanErr != nil {
		return r.scanErr
	}
	fx := r.rows[r.idx-1]
	values := []any{
		fx.ID, string(fx.ResourceType), fx.ResourceID, string(fx.Status),
		fx.ReportedBy, (*uuid.UUID)(nil), fx.Reason, (*string)(nil),
		fx.CreatedAt, (*time.Time)(nil),
	}
	return scanListByReporterValues(dest, values)
}

func (r *listByReporterMockRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, fmt.Errorf("values out of sequence")
	}
	fx := r.rows[r.idx-1]
	return []any{
		fx.ID, string(fx.ResourceType), fx.ResourceID, string(fx.Status),
		fx.ReportedBy, nil, fx.Reason, nil, fx.CreatedAt, nil,
	}, nil
}

func scanListByReporterValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan dest count mismatch: %d vs %d", len(dest), len(values))
	}
	for i := range dest {
		if err := assignListByReporterValue(dest[i], values[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignListByReporterValue(dest any, value any) error {
	switch d := dest.(type) {
	case *uuid.UUID:
		if value == nil {
			*d = uuid.Nil
			return nil
		}
		*d = value.(uuid.UUID)
	case *string:
		if value == nil {
			*d = ""
			return nil
		}
		*d = value.(string)
	case *time.Time:
		if value == nil {
			*d = time.Time{}
			return nil
		}
		*d = value.(time.Time)
	case **time.Time:
		if value == nil {
			*d = nil
			return nil
		}
		t := value.(*time.Time)
		*d = t
	case **uuid.UUID:
		if value == nil {
			*d = nil
			return nil
		}
		u := value.(*uuid.UUID)
		*d = u
	case **string:
		if value == nil {
			*d = nil
			return nil
		}
		s := value.(*string)
		*d = s
	default:
		return fmt.Errorf("unexpected scan dest type %T", dest)
	}
	return nil
}

// =============================================================================
// Tests
// =============================================================================

func TestListByReporter_ReporterIsolation(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterA := uuid.New()
	reporterB := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterA, Reason: "a1", CreatedAt: now},
	}}

	// Reporter A: sees 1 case
	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterA, nil, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 for reporter A", total)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}

	// Reporter B: sees 0 cases
	cases, total, err = repo.ListByReporter(context.Background(), tx, reporterB, nil, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0 for reporter B", total)
	}
	if len(cases) != 0 {
		t.Fatalf("len(cases) = %d, want 0", len(cases))
	}

	// Both queries contain reported_by predicate
	if !strings.Contains(tx.lastCountSQL, "reported_by =") {
		t.Fatal("count query must contain reported_by")
	}
	if !strings.Contains(tx.lastDataSQL, "reported_by =") {
		t.Fatal("data query must contain reported_by")
	}
}

func TestListByReporter_NoStatusFilter_AllMatch(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "p", CreatedAt: now},
		{ID: uuid.New(), ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusEnforced, ReportedBy: reporterID, Reason: "e", CreatedAt: now.Add(-time.Minute)},
	}}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (no status filter)", total)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
}

func TestListByReporter_StatusFiltered_ExactMatch(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "p", CreatedAt: now},
		{ID: uuid.New(), ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusEnforced, ReportedBy: reporterID, Reason: "e", CreatedAt: now.Add(-time.Minute)},
	}}

	statusFilter := entity.GovernanceCaseStatusPending
	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, &statusFilter, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 (only pending)", total)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}
	if cases[0].Status != entity.GovernanceCaseStatusPending {
		t.Fatalf("case status = %s, want pending", cases[0].Status)
	}

	// Both queries contain status filter
	if !strings.Contains(tx.lastCountSQL, "status =") {
		t.Fatal("count query must contain status predicate when filtered")
	}
	if !strings.Contains(tx.lastDataSQL, "status =") {
		t.Fatal("data query must contain status predicate when filtered")
	}
}

func TestListByReporter_IdenticalPredicatesForItemsAndCount(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()
	statusFilter := entity.GovernanceCaseStatusPending

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "p", CreatedAt: now},
		{ID: uuid.New(), ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusEnforced, ReportedBy: reporterID, Reason: "e", CreatedAt: now.Add(-time.Minute)},
	}}

	_, total, err := repo.ListByReporter(context.Background(), tx, reporterID, &statusFilter, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}

	// Count and data queries must both have reported_by AND status
	if !strings.Contains(tx.lastCountSQL, "reported_by =") || !strings.Contains(tx.lastCountSQL, "status =") {
		t.Fatal("count query missing reporter or status predicate")
	}
	if !strings.Contains(tx.lastDataSQL, "reported_by =") || !strings.Contains(tx.lastDataSQL, "status =") {
		t.Fatal("data query missing reporter or status predicate")
	}
}

func TestListByReporter_LimitAndOffset(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "1", CreatedAt: now},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "2", CreatedAt: now.Add(-time.Minute)},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "3", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "4", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000005"), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "5", CreatedAt: now.Add(-4 * time.Minute)},
	}}

	// Page 1: limit 2 → 2 items
	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5 (limit does not affect count)", total)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}

	// Page 2: limit 2, offset 2
	cases, total, err = repo.ListByReporter(context.Background(), tx, reporterID, nil, 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5 (still 5)", total)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
}

func TestListByReporter_CountUnaffectedByPageSize(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "1", CreatedAt: now},
		{ID: uuid.New(), ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "2", CreatedAt: now.Add(-time.Minute)},
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "3", CreatedAt: now.Add(-2 * time.Minute)},
	}}

	// limit=1, total still 3
	_, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (limit=1 does not change total)", total)
	}

	// limit=100, total still 3
	_, total, err = repo.ListByReporter(context.Background(), tx, reporterID, nil, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (limit=100 does not change total)", total)
	}
}

func TestListByReporter_DeterministicOrdering(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	// Create cases with deterministic IDs for ordering test
	idNewest := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	idOlder := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: idOlder, ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "older", CreatedAt: now.Add(-time.Minute)},
		{ID: idNewest, ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "newest", CreatedAt: now},
	}}

	cases, _, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
	// Newest first
	if cases[0].Reason != "newest" {
		t.Fatalf("first case reason = %s, want 'newest' (newest first)", cases[0].Reason)
	}
	if cases[1].Reason != "older" {
		t.Fatalf("second case reason = %s, want 'older'", cases[1].Reason)
	}

	// Data query must have ORDER BY
	if !strings.Contains(tx.lastDataSQL, "id DESC") {
		t.Fatal("data query must order by id DESC as tie-breaker")
	}
}

func TestListByReporter_OutOfRangePage_EmptyItemsTotalPreserved(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "only", CreatedAt: now},
	}}

	// Page 10 (way beyond data): empty items, total=1
	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 180)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 (preserved for out-of-range page)", total)
	}
	if len(cases) != 0 {
		t.Fatalf("len(cases) = %d, want 0 (no items on out-of-range page)", len(cases))
	}
}

func TestListByReporter_ScanMapsRowsCorrectly(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()
	caseID := uuid.New()
	resourceID := uuid.New()

	tx := &listByReporterMockTx{t: t, fixtures: []listByReporterFixture{
		{ID: caseID, ResourceType: entity.ResourceTypeContent, ResourceID: resourceID, Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "test reason", CreatedAt: now},
	}}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}

	c := cases[0]
	if c.ID != caseID {
		t.Fatalf("case ID = %s, want %s", c.ID, caseID)
	}
	if c.ResourceType != entity.ResourceTypeContent {
		t.Fatalf("resource_type = %s, want content", c.ResourceType)
	}
	if c.ResourceID != resourceID {
		t.Fatalf("resource_id = %s, want %s", c.ResourceID, resourceID)
	}
	if c.Status != entity.GovernanceCaseStatusPending {
		t.Fatalf("status = %s, want pending", c.Status)
	}
	if c.ReportedBy != reporterID {
		t.Fatalf("reported_by = %s, want %s", c.ReportedBy, reporterID)
	}
	if c.Reason != "test reason" {
		t.Fatalf("reason = %s, want 'test reason'", c.Reason)
	}
}

func TestListByReporter_CountQueryError(t *testing.T) {
	repo := NewModerationRepository()
	reporterID := uuid.New()
	tx := &listByReporterMockTx{
		t:        t,
		fixtures: []listByReporterFixture{},
		countErr: fmt.Errorf("count failed"),
	}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err == nil {
		t.Fatal("expected count query error")
	}
	if cases != nil {
		t.Fatalf("cases = %#v, want nil", cases)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
}

func TestListByReporter_ItemsQueryError(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()
	tx := &listByReporterMockTx{
		t: t,
		fixtures: []listByReporterFixture{
			{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "reason", CreatedAt: now},
		},
		queryErr: fmt.Errorf("items failed"),
	}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err == nil {
		t.Fatal("expected items query error")
	}
	if cases != nil {
		t.Fatalf("cases = %#v, want nil", cases)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
}

func TestListByReporter_RowsScanError(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()
	tx := &listByReporterMockTx{
		t: t,
		fixtures: []listByReporterFixture{
			{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "reason", CreatedAt: now},
		},
		rowScanErr: fmt.Errorf("scan failed"),
	}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err == nil {
		t.Fatal("expected rows scan error")
	}
	if cases != nil {
		t.Fatalf("cases = %#v, want nil", cases)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
}

func TestListByReporter_RowsIterationError(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	reporterID := uuid.New()
	tx := &listByReporterMockTx{
		t: t,
		fixtures: []listByReporterFixture{
			{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: reporterID, Reason: "reason", CreatedAt: now},
		},
		rowsErr: fmt.Errorf("rows failed"),
	}

	cases, total, err := repo.ListByReporter(context.Background(), tx, reporterID, nil, 20, 0)
	if err == nil {
		t.Fatal("expected rows iteration error")
	}
	if cases != nil {
		t.Fatalf("cases = %#v, want nil", cases)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
}
