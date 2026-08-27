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
	"github.com/labuda/backend/pkg/db"
)

type warningListFixture struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	IssuedBy  uuid.UUID
	Level     string
	Reason    string
	IsActive  bool
	RevokedAt *time.Time
	RevokedBy *uuid.UUID
	CreatedAt time.Time
	ExpiresAt *time.Time
}

type warningListMockTx struct {
	t             *testing.T
	fixtures      []warningListFixture
	lastCountSQL  string
	lastCountArgs []any
	lastDataSQL   string
	lastDataArgs  []any
}

var _ db.Tx = (*warningListMockTx)(nil)

func (t *warningListMockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *warningListMockTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.lastCountSQL = sql
	t.lastCountArgs = append([]any(nil), args...)
	if !strings.Contains(sql, "COUNT(*) FROM user_warnings") {
		t.t.Fatalf("unexpected count SQL: %s", sql)
	}
	userID, isActive := parseWarningListFilters(args)
	if userID != nil && !strings.Contains(sql, "user_id =") {
		t.t.Fatalf("count query must include user filter: %s", sql)
	}
	if isActive != nil && !strings.Contains(sql, "is_active =") {
		t.t.Fatalf("count query must include active filter: %s", sql)
	}
	return warningListMockRow{count: int64(len(t.matchingFixtures(userID, isActive)))}
}

func (t *warningListMockTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.lastDataSQL = sql
	t.lastDataArgs = append([]any(nil), args...)
	if !strings.Contains(sql, "FROM user_warnings") {
		t.t.Fatalf("unexpected data SQL: %s", sql)
	}
	if len(args) == 0 {
		t.t.Fatalf("pagination args missing")
	}

	limit := args[len(args)-2].(int)
	offset := args[len(args)-1].(int)
	filterArgs := args[:len(args)-2]
	userID, isActive := parseWarningListFilters(filterArgs)
	if userID != nil && !strings.Contains(sql, "user_id =") {
		t.t.Fatalf("data query must include user filter: %s", sql)
	}
	if isActive != nil && !strings.Contains(sql, "is_active =") {
		t.t.Fatalf("data query must include active filter: %s", sql)
	}
	matches := t.matchingFixtures(userID, isActive)

	if offset > len(matches) {
		return &warningListMockRows{}, nil
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}

	rows := make([]warningListFixture, end-offset)
	copy(rows, matches[offset:end])
	return &warningListMockRows{rows: rows}, nil
}

func (t *warningListMockTx) Commit(_ context.Context) error   { return nil }
func (t *warningListMockTx) Rollback(_ context.Context) error { return nil }

func (t *warningListMockTx) matchingFixtures(userID *uuid.UUID, isActive *bool) []warningListFixture {
	var matches []warningListFixture
	for _, fx := range t.fixtures {
		if userID != nil && fx.UserID != *userID {
			continue
		}
		if isActive != nil && fx.IsActive != *isActive {
			continue
		}
		matches = append(matches, fx)
	}
	return matches
}

func parseWarningListFilters(args []any) (*uuid.UUID, *bool) {
	var userID *uuid.UUID
	var isActive *bool
	for _, arg := range args {
		switch v := arg.(type) {
		case uuid.UUID:
			uid := v
			userID = &uid
		case bool:
			active := v
			isActive = &active
		}
	}
	return userID, isActive
}

type warningListMockRow struct {
	count int64
}

func (r warningListMockRow) Scan(dest ...any) error {
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

type warningListMockRows struct {
	rows []warningListFixture
	idx  int
}

func (r *warningListMockRows) Close() {}

func (r *warningListMockRows) Err() error { return nil }

func (r *warningListMockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *warningListMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *warningListMockRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *warningListMockRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("scan called out of sequence")
	}
	fx := r.rows[r.idx-1]
	values := []any{
		fx.ID,
		fx.UserID,
		fx.IssuedBy,
		fx.Level,
		fx.Reason,
		fx.IsActive,
		fx.RevokedAt,
		fx.RevokedBy,
		fx.CreatedAt,
		fx.ExpiresAt,
	}
	return scanWarningListValues(dest, values)
}

func (r *warningListMockRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, fmt.Errorf("values called out of sequence")
	}
	fx := r.rows[r.idx-1]
	return []any{
		fx.ID,
		fx.UserID,
		fx.IssuedBy,
		fx.Level,
		fx.Reason,
		fx.IsActive,
		fx.RevokedAt,
		fx.RevokedBy,
		fx.CreatedAt,
		fx.ExpiresAt,
	}, nil
}

func (r *warningListMockRows) RawValues() [][]byte { return nil }

func (r *warningListMockRows) Conn() *pgx.Conn { return nil }

func scanWarningListValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("unexpected scan dest count: got %d want %d", len(dest), len(values))
	}
	for i := range dest {
		if err := assignWarningListValue(dest[i], values[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignWarningListValue(dest any, value any) error {
	switch d := dest.(type) {
	case *uuid.UUID:
		*d = value.(uuid.UUID)
	case *string:
		*d = value.(string)
	case *bool:
		*d = value.(bool)
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
	default:
		return fmt.Errorf("unexpected scan dest type %T", dest)
	}
	return nil
}

func TestWarningRepositoryListAllActiveFilterAndPagination(t *testing.T) {
	repo := NewWarningRepository()
	userID := uuid.New()
	otherUser := uuid.New()
	adminID := uuid.New()
	now := time.Now()

	fixtures := []warningListFixture{
		{ID: uuid.New(), UserID: userID, IssuedBy: adminID, Level: "warning", Reason: "active 1", IsActive: true, CreatedAt: now},
		{ID: uuid.New(), UserID: userID, IssuedBy: adminID, Level: "info", Reason: "inactive 1", IsActive: false, CreatedAt: now.Add(-time.Minute)},
		{ID: uuid.New(), UserID: otherUser, IssuedBy: adminID, Level: "warning", Reason: "active other", IsActive: true, CreatedAt: now.Add(-2 * time.Minute)},
	}

	tx := &warningListMockTx{t: t, fixtures: fixtures}

	active := true
	warnings, total, err := repo.ListAll(context.Background(), tx, &userID, &active, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(warnings))
	}
	if !warnings[0].IsActive {
		t.Fatal("expected only active warnings")
	}
	if !strings.Contains(tx.lastCountSQL, "is_active =") || !strings.Contains(tx.lastDataSQL, "is_active =") {
		t.Fatal("active filter must be applied in the query layer")
	}
	if warnings[0].Reason != "active 1" {
		t.Fatalf("unexpected first page warning: %s", warnings[0].Reason)
	}

	warnings, total, err = repo.ListAll(context.Background(), tx, &userID, &active, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error on paginated active query: %v", err)
	}
	if total != 1 {
		t.Fatalf("paginated active total = %d, want 1", total)
	}
	if len(warnings) != 0 {
		t.Fatalf("paginated active len = %d, want 0", len(warnings))
	}

	inactive := false
	warnings, total, err = repo.ListAll(context.Background(), tx, &userID, &inactive, 20, 0)
	if err != nil {
		t.Fatalf("unexpected error on inactive query: %v", err)
	}
	if total != 1 {
		t.Fatalf("inactive total = %d, want 1", total)
	}
	if len(warnings) != 1 {
		t.Fatalf("inactive len = %d, want 1", len(warnings))
	}
	if warnings[0].IsActive {
		t.Fatal("expected only inactive warnings")
	}
	if warnings[0].Reason != "inactive 1" {
		t.Fatalf("unexpected inactive warning: %s", warnings[0].Reason)
	}
}


