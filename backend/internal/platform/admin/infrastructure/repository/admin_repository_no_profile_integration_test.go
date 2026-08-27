//go:build integration

// This is the direct regression test for the admin Users page HTTP 500
// reported in PASS_20F: ListUsers/CountUsers/GetUserDetails all LEFT JOIN
// user_profiles, and up.is_verified was scanned into a plain (non-nullable)
// bool. A user with no user_profiles row at all — e.g. a directly seeded or
// bootstrapped admin account, which never went through the normal signup
// flow that creates a profile row — makes every up.* column NULL for that
// row, and pgx refuses to scan NULL into *bool. That crashed the query for
// the whole page the moment any such user existed, not just on empty data.
package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/platform/admin/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func insertUserWithoutProfile(t *testing.T, tdb *testdb.TestDB, ctx context.Context) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email) VALUES ($1, $2, $3)`,
			id, "fb-"+id.String(), id.String()+"@no-profile.test",
		)
		return err
	})
	if err != nil {
		t.Fatalf("insertUserWithoutProfile: %v", err)
	}
	return id
}

func TestAdminRepository_ListUsers_UserWithoutProfileDoesNotCrash(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUserWithoutProfile(t, tdb, ctx)

	repo := &AdminRepositoryImpl{}
	filters := repository.UserListFilters{Page: 1, PageSize: 20}

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		users, err := repo.ListUsers(ctx, tx, filters)
		if err != nil {
			return err
		}
		found := false
		for _, u := range users {
			if u.ID == userID {
				found = true
				if u.IsVerified {
					t.Fatalf("expected IsVerified=false default for a user with no profile row")
				}
			}
		}
		if !found {
			t.Fatalf("expected user %s in list results", userID)
		}
		_, err = repo.CountUsers(ctx, tx, filters)
		return err
	})
	if err != nil {
		t.Fatalf("ListUsers/CountUsers with a profile-less user: %v (NULL scan regression if this mentions is_verified or converting NULL)", err)
	}
}

func TestAdminRepository_GetUserDetails_UserWithoutProfileDoesNotCrash(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	userID := insertUserWithoutProfile(t, tdb, ctx)

	repo := &AdminRepositoryImpl{}

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		details, err := repo.GetUserDetails(ctx, tx, userID)
		if err != nil {
			return err
		}
		if details.ID != userID {
			t.Fatalf("ID mismatch: got %v want %v", details.ID, userID)
		}
		if details.IsVerified {
			t.Fatalf("expected IsVerified=false default for a user with no profile row")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("GetUserDetails with a profile-less user: %v (NULL scan regression if this mentions is_verified or converting NULL)", err)
	}
}
