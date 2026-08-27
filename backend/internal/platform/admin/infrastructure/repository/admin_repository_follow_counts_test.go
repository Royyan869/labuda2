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
	"github.com/jackc/pgx/v5/pgtype"
	adminrepo "github.com/labuda/backend/internal/platform/admin/repository"
)

type adminFollowCountTx struct {
	lastQuery string
}

func (t *adminFollowCountTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *adminFollowCountTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *adminFollowCountTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.lastQuery = sql
	return &adminFollowCountRow{query: sql}
}

func (t *adminFollowCountTx) Commit(ctx context.Context) error   { return nil }
func (t *adminFollowCountTx) Rollback(ctx context.Context) error { return nil }

type adminFollowCountRow struct {
	query string
}

func (r *adminFollowCountRow) Scan(dest ...any) error {
	if !strings.Contains(r.query, "user_follows") {
		return fmt.Errorf("query does not use live follow counts: %s", r.query)
	}

	pgTextSeen := 0
	pgIntSeen := 0
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = uuid.MustParse("11111111-1111-1111-1111-111111111111")
		case *string:
			*v = "value"
		case *bool:
			*v = true
		case *int:
			*v = 0
		case *time.Time:
			*v = now
		case *interface{}:
			*v = now
		case *pgtype.Text:
			v.Valid = true
			switch pgTextSeen {
			case 0:
				v.String = "firebase-uid"
			case 1:
				v.String = "alice@example.com"
			case 2:
				v.String = "+6281234567890"
			case 3:
				v.String = "alice"
			case 4:
				v.String = "bio"
			case 5:
				v.String = "https://cdn.example.com/a.jpg"
			case 6:
				v.String = "Bandung"
			case 7:
				v.String = "Jabar"
			case 8:
				v.String = "settlement"
			case 9:
				v.String = "active"
			case 10:
				v.String = "recovery-payment-id"
			case 11:
				v.String = "pro"
			default:
				v.String = "value"
			}
			pgTextSeen++
		case *pgtype.Int8:
			v.Valid = true
			switch pgIntSeen {
			case 0:
				v.Int64 = 12
			case 1:
				v.Int64 = 7
			default:
				v.Int64 = 99
			}
			pgIntSeen++
		case *pgtype.Bool:
			v.Valid = true
			v.Bool = true
		default:
			return fmt.Errorf("unsupported scan destination %T", d)
		}
	}

	return nil
}

func TestAdminRepository_GetUserDetails_UsesLiveFollowCounts(t *testing.T) {
	tx := &adminFollowCountTx{}
	repo := &AdminRepositoryImpl{}
	userID := uuid.New()

	details, err := repo.GetUserDetails(context.Background(), tx, userID)
	if err != nil {
		t.Fatalf("GetUserDetails returned error: %v", err)
	}

	if details == nil {
		t.Fatal("expected user details")
	}
	if details.FollowersCount == nil || *details.FollowersCount != 12 {
		t.Fatalf("followers_count = %v; want 12", details.FollowersCount)
	}
	if details.FollowingCount == nil || *details.FollowingCount != 7 {
		t.Fatalf("following_count = %v; want 7", details.FollowingCount)
	}
	if !strings.Contains(tx.lastQuery, "FROM user_follows") {
		t.Fatalf("query does not use live follow table: %s", tx.lastQuery)
	}
	if strings.Contains(tx.lastQuery, "up.followers_count") {
		t.Fatalf("query still references stale profile projection: %s", tx.lastQuery)
	}
}

var _ adminrepo.AdminRepository = (*AdminRepositoryImpl)(nil)


