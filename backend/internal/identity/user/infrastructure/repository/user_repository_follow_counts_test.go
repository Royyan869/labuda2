package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type followCountTx struct {
	lastQuery string
}

func (t *followCountTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *followCountTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *followCountTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.lastQuery = sql
	return &followCountRow{query: sql}
}

func (t *followCountTx) Commit(ctx context.Context) error   { return nil }
func (t *followCountTx) Rollback(ctx context.Context) error { return nil }

type followCountRow struct {
	query string
}

func (r *followCountRow) Scan(dest ...any) error {
	if !strings.Contains(r.query, "user_follows") {
		return fmt.Errorf("query does not use live follow counts: %s", r.query)
	}

	nullIntSeen := 0
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = uuid.MustParse("11111111-1111-1111-1111-111111111111")
		case *sql.NullString:
			v.Valid = true
			v.String = "value"
		case *sql.NullBool:
			v.Valid = true
			v.Bool = true
		case *sql.NullInt64:
			v.Valid = true
			switch nullIntSeen {
			case 0:
				v.Int64 = 12
			case 1:
				v.Int64 = 7
			default:
				v.Int64 = 0
			}
			nullIntSeen++
		case *sql.NullTime:
			v.Valid = true
			v.Time = now
		case *string:
			*v = "value"
		default:
			return fmt.Errorf("unsupported scan destination %T", d)
		}
	}

	return nil
}

func TestUserRepository_GetProfileByID_UsesLiveFollowCounts(t *testing.T) {
	tx := &followCountTx{}
	repo := &userRepositoryImpl{}
	userID := uuid.New()

	profile, err := repo.GetProfileByID(context.Background(), tx, userID)
	if err != nil {
		t.Fatalf("GetProfileByID returned error: %v", err)
	}

	if profile == nil {
		t.Fatal("expected profile")
	}
	if profile.FollowersCount != 12 {
		t.Fatalf("followers_count = %d; want 12", profile.FollowersCount)
	}
	if profile.FollowingCount != 7 {
		t.Fatalf("following_count = %d; want 7", profile.FollowingCount)
	}
	if !strings.Contains(tx.lastQuery, "FROM user_follows") {
		t.Fatalf("query does not use live follow table: %s", tx.lastQuery)
	}
	if strings.Contains(tx.lastQuery, "up.followers_count") {
		t.Fatalf("query still references stale profile projection: %s", tx.lastQuery)
	}
}


