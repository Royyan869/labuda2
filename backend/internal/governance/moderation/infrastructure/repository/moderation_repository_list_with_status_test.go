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

type moderationListFixture struct {
	ID           uuid.UUID
	ResourceType entity.ResourceType
	ResourceID   uuid.UUID
	Status       entity.GovernanceCaseStatus
	ReportedBy   uuid.UUID
	Reason       string
	CreatedAt    time.Time
}

type moderationListMockTx struct {
	t             *testing.T
	fixtures      []moderationListFixture
	lastCountSQL  string
	lastCountArgs []any
	lastDataSQL   string
	lastDataArgs  []any
}

var _ db.Tx = (*moderationListMockTx)(nil)

func (t *moderationListMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *moderationListMockTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.lastCountSQL = sql
	t.lastCountArgs = append([]any(nil), args...)
	if !strings.Contains(sql, "COUNT(*) FROM moderation_cases") {
		t.t.Fatalf("unexpected count SQL: %s", sql)
	}

	statusFilter, resourceTypeFilter := moderationListParseFilters(sql, args)
	if statusFilter != nil && !strings.Contains(sql, "status =") {
		t.t.Fatalf("count query missing status filter: %s", sql)
	}
	if resourceTypeFilter != nil && !strings.Contains(sql, "resource_type =") {
		t.t.Fatalf("count query missing resource_type filter: %s", sql)
	}

	return moderationListMockRow{count: int64(len(t.matchingFixtures(statusFilter, resourceTypeFilter)))}
}

func (t *moderationListMockTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.lastDataSQL = sql
	t.lastDataArgs = append([]any(nil), args...)
	if !strings.Contains(sql, "FROM moderation_cases") {
		t.t.Fatalf("unexpected data SQL: %s", sql)
	}
	if len(args) < 2 {
		t.t.Fatalf("pagination args missing")
	}

	limit := args[len(args)-2].(int)
	offset := args[len(args)-1].(int)
	filterArgs := args[:len(args)-2]
	statusFilter, resourceTypeFilter := moderationListParseFilters(sql, filterArgs)
	if statusFilter != nil && !strings.Contains(sql, "status =") {
		t.t.Fatalf("data query missing status filter: %s", sql)
	}
	if resourceTypeFilter != nil && !strings.Contains(sql, "resource_type =") {
		t.t.Fatalf("data query missing resource_type filter: %s", sql)
	}

	matches := t.matchingFixtures(statusFilter, resourceTypeFilter)
	if offset > len(matches) {
		return &moderationListMockRows{}, nil
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}

	rows := make([]moderationListFixture, end-offset)
	copy(rows, matches[offset:end])
	return &moderationListMockRows{rows: rows}, nil
}

func (t *moderationListMockTx) Commit(_ context.Context) error   { return nil }
func (t *moderationListMockTx) Rollback(_ context.Context) error { return nil }

func (t *moderationListMockTx) matchingFixtures(statusFilter *entity.GovernanceCaseStatus, resourceTypeFilter *entity.ResourceType) []moderationListFixture {
	var matches []moderationListFixture
	for _, fx := range t.fixtures {
		if statusFilter != nil && fx.Status != *statusFilter {
			continue
		}
		if resourceTypeFilter != nil && fx.ResourceType != *resourceTypeFilter {
			continue
		}
		matches = append(matches, fx)
	}
	return matches
}

func moderationListParseFilters(sql string, args []any) (*entity.GovernanceCaseStatus, *entity.ResourceType) {
	var statusFilter *entity.GovernanceCaseStatus
	var resourceTypeFilter *entity.ResourceType
	argIdx := 0

	if strings.Contains(sql, "status =") && argIdx < len(args) {
		status := entity.GovernanceCaseStatus(args[argIdx].(string))
		statusFilter = &status
		argIdx++
	}
	if strings.Contains(sql, "resource_type =") && argIdx < len(args) {
		resourceType := entity.ResourceType(args[argIdx].(string))
		resourceTypeFilter = &resourceType
	}

	return statusFilter, resourceTypeFilter
}

type moderationListMockRow struct {
	count int64
}

func (r moderationListMockRow) Scan(dest ...any) error {
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

type moderationListMockRows struct {
	rows []moderationListFixture
	idx  int
}

func (r *moderationListMockRows) Close() {}

func (r *moderationListMockRows) Err() error { return nil }

func (r *moderationListMockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *moderationListMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *moderationListMockRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *moderationListMockRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("scan called out of sequence")
	}
	fx := r.rows[r.idx-1]
	values := []any{
		fx.ID,
		string(fx.ResourceType),
		fx.ResourceID,
		string(fx.Status),
		fx.ReportedBy,
		nil,
		fx.Reason,
		nil,
		fx.CreatedAt,
		nil,
	}
	return scanModerationListValues(dest, values)
}

func (r *moderationListMockRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, fmt.Errorf("values called out of sequence")
	}
	fx := r.rows[r.idx-1]
	return []any{
		fx.ID,
		string(fx.ResourceType),
		fx.ResourceID,
		string(fx.Status),
		fx.ReportedBy,
		nil,
		fx.Reason,
		nil,
		fx.CreatedAt,
		nil,
	}, nil
}

func (r *moderationListMockRows) RawValues() [][]byte { return nil }

func (r *moderationListMockRows) Conn() *pgx.Conn { return nil }

func scanModerationListValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("unexpected scan dest count: got %d want %d", len(dest), len(values))
	}
	for i := range dest {
		if err := assignModerationListValue(dest[i], values[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignModerationListValue(dest any, value any) error {
	switch d := dest.(type) {
	case *uuid.UUID:
		*d = value.(uuid.UUID)
	case *string:
		*d = value.(string)
	case *time.Time:
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

func TestModerationRepositoryListWithStatusFiltersAndCounts(t *testing.T) {
	repo := NewModerationRepository()
	now := time.Now()
	contentPendingA := moderationListFixture{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: uuid.New(), Reason: "content-a", CreatedAt: now.Add(-3 * time.Minute)}
	commentPending := moderationListFixture{ID: uuid.New(), ResourceType: entity.ResourceTypeComment, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: uuid.New(), Reason: "comment", CreatedAt: now.Add(-2 * time.Minute)}
	contentPendingB := moderationListFixture{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusPending, ReportedBy: uuid.New(), Reason: "content-b", CreatedAt: now.Add(-time.Minute)}
	contentApproved := moderationListFixture{ID: uuid.New(), ResourceType: entity.ResourceTypeContent, ResourceID: uuid.New(), Status: entity.GovernanceCaseStatusApproved, ReportedBy: uuid.New(), Reason: "content-approved", CreatedAt: now}

	tx := &moderationListMockTx{
		t:        t,
		fixtures: []moderationListFixture{contentPendingA, commentPending, contentPendingB, contentApproved},
	}

	status := entity.GovernanceCaseStatusPending
	resourceType := entity.ResourceTypeContent

	cases, total, err := repo.ListWithStatus(context.Background(), tx, &status, &resourceType, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}
	if cases[0].Reason != "content-a" {
		t.Fatalf("first page reason = %s, want content-a", cases[0].Reason)
	}
	if !strings.Contains(tx.lastCountSQL, "status =") || !strings.Contains(tx.lastCountSQL, "resource_type =") {
		t.Fatal("count query must include both status and resource_type filters")
	}
	if !strings.Contains(tx.lastDataSQL, "status =") || !strings.Contains(tx.lastDataSQL, "resource_type =") {
		t.Fatal("data query must include both status and resource_type filters")
	}

	cases, total, err = repo.ListWithStatus(context.Background(), tx, &status, &resourceType, 1, 1)
	if err != nil {
		t.Fatalf("unexpected paginated error: %v", err)
	}
	if total != 2 {
		t.Fatalf("paginated total = %d, want 2", total)
	}
	if len(cases) != 1 || cases[0].Reason != "content-b" {
		t.Fatalf("expected second content case after pagination, got %#v", cases)
	}

	cases, total, err = repo.ListWithStatus(context.Background(), tx, &status, nil, 10, 0)
	if err != nil {
		t.Fatalf("unexpected no-resource-type error: %v", err)
	}
	if total != 3 {
		t.Fatalf("status-only total = %d, want 3", total)
	}
	if len(cases) != 3 {
		t.Fatalf("status-only len = %d, want 3", len(cases))
	}

	resourceTypeOnly := entity.ResourceTypeComment
	cases, total, err = repo.ListWithStatus(context.Background(), tx, nil, &resourceTypeOnly, 10, 0)
	if err != nil {
		t.Fatalf("unexpected resource-type-only error: %v", err)
	}
	if total != 1 {
		t.Fatalf("resource-type-only total = %d, want 1", total)
	}
	if len(cases) != 1 {
		t.Fatalf("resource-type-only len = %d, want 1", len(cases))
	}
	if cases[0].Reason != "comment" {
		t.Fatalf("resource-type-only first reason = %s, want comment", cases[0].Reason)
	}
	if strings.Contains(tx.lastCountSQL, "status =") {
		t.Fatal("resource-type-only count query must not include status filter")
	}
	if !strings.Contains(tx.lastCountSQL, "resource_type =") {
		t.Fatal("resource-type-only count query must include resource_type filter")
	}
	if strings.Contains(tx.lastDataSQL, "status =") {
		t.Fatal("resource-type-only data query must not include status filter")
	}
	if !strings.Contains(tx.lastDataSQL, "resource_type =") {
		t.Fatal("resource-type-only data query must include resource_type filter")
	}
}


