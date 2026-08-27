package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// TestUserRepository_CoverPhotoRoundTrip proves the canonical cover-photo
// persistence contract against a real PostgreSQL test database:
//
//  1. UpdateProfile accepts and stores the canonical storage key
//     (images/profile-covers/{userId}.jpg).
//  2. GetProfileByID hydrates the persisted key back into the entity.
//  3. GetPublicInfo projects the persisted key into the public projection.
//  4. Clearing the reference (empty string) removes the DB reference.
//  5. No migration was required — the column already exists (000020/000022).
func TestUserRepository_CoverPhotoRoundTrip(t *testing.T) {
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	repo := NewUserRepository(db.NewFromPool(testDB.Pool()))
	ctx := context.Background()

	userID := uuid.New()
	coverKey := "images/profile-covers/" + userID.String() + ".jpg"

	// Seed a minimal user row (user_profiles.user_id is FK to users.id).
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at)
			VALUES ($1, $2, $3, 'active', NOW(), NOW())
		`, userID, "firebase-"+userID.String(), "cover-roundtrip@example.com")
		return err
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// 1. UpdateProfile stores the canonical cover key (profile row created).
	var updated *entity.UserProfile
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		u, err := repo.UpdateProfile(ctx, tx, userID, &entity.UpdateProfileInput{
			Username:      strPtr("coveruser"),
			CoverPhotoURL: &coverKey,
		})
		updated = u
		return err
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated == nil || updated.CoverPhotoURL == nil || *updated.CoverPhotoURL != coverKey {
		t.Fatalf("UpdateProfile returned cover = %v, want %q", updatedCover(updated), coverKey)
	}

	// 2. GetProfileByID hydrates the key back.
	var fetched *entity.UserProfile
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		p, err := repo.GetProfileByID(ctx, tx, userID)
		fetched = p
		return err
	})
	if err != nil {
		t.Fatalf("GetProfileByID: %v", err)
	}
	if fetched == nil || fetched.CoverPhotoURL == nil || *fetched.CoverPhotoURL != coverKey {
		t.Fatalf("GetProfileByID cover = %v, want %q", updatedCover(fetched), coverKey)
	}

	// 3. GetPublicInfo projects the key into the public projection.
	var publicInfo *entity.UserPublicInfo
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		p, err := repo.GetPublicInfo(ctx, tx, userID, false)
		publicInfo = p
		return err
	})
	if err != nil {
		t.Fatalf("GetPublicInfo: %v", err)
	}
	if publicInfo == nil || publicInfo.CoverPhotoURL == nil || *publicInfo.CoverPhotoURL != coverKey {
		t.Fatalf("GetPublicInfo cover = %v, want %q", updatedCoverPublic(publicInfo), coverKey)
	}

	// 4. Clearing the reference removes the DB value.
	empty := ""
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := repo.UpdateProfile(ctx, tx, userID, &entity.UpdateProfileInput{CoverPhotoURL: &empty})
		return err
	})
	if err != nil {
		t.Fatalf("UpdateProfile clear: %v", err)
	}
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		p, err := repo.GetProfileByID(ctx, tx, userID)
		fetched = p
		return err
	})
	if err != nil {
		t.Fatalf("GetProfileByID after clear: %v", err)
	}
	if fetched == nil || fetched.CoverPhotoURL != nil {
		t.Fatalf("GetProfileByID cover after clear = %v, want nil", updatedCover(fetched))
	}
}

// compile-time guard: testdb.TestDB exposes the pgx pool the repository needs.
var _ = pgxpool.Pool{}

func strPtr(s string) *string { return &s }

func updatedCover(p *entity.UserProfile) any {
	if p == nil || p.CoverPhotoURL == nil {
		return nil
	}
	return *p.CoverPhotoURL
}

func updatedCoverPublic(p *entity.UserPublicInfo) any {
	if p == nil || p.CoverPhotoURL == nil {
		return nil
	}
	return *p.CoverPhotoURL
}
