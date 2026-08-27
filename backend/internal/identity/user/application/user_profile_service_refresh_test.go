package application

import (
	"context"
	"errors"
	"testing"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	outboxInfra "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

type fakeTx struct{}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (f *fakeTx) Commit(ctx context.Context) error                              { return nil }
func (f *fakeTx) Rollback(ctx context.Context) error                            { return nil }

type fakeDB struct{}

func (f *fakeDB) WithTx(ctx context.Context, fn func(tx db.Tx) error) error { return fn(&fakeTx{}) }
func (f *fakeDB) Pool() *pgxpool.Pool                                       { return nil }

type fakeFirebase struct {
	user *firebaseauth.UserRecord
	err  error
}

func (f *fakeFirebase) GetUser(ctx context.Context, uid string) (*firebaseauth.UserRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

type fakeUserRepo struct {
	user   *userEntity.User
	update *userEntity.User
}

func (f *fakeUserRepo) SoftDeleteUser(ctx context.Context, tx db.Tx, userID uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeUserRepo) GetPublicInfo(ctx context.Context, tx db.Tx, userID uuid.UUID, isOwnProfile bool) (*userEntity.UserPublicInfo, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetMyProfile(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.MyProfileResponse, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetByIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.User, error) {
	return f.user, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, tx db.Tx, user *userEntity.User) error {
	cp := *user
	f.update = &cp
	return nil
}

type noopSellerRepo struct{ sellerRepo.SellerRepository }
type noopSubRepo struct {
	subscriptionRepo.SellerSubscriptionRepository
}

func newRefreshService(repo *fakeUserRepo, fb *fakeFirebase) *UserProfileService {
	return NewUserProfileService(
		repo,
		&noopSellerRepo{},
		&noopSubRepo{},
		&outboxInfra.OutboxRepository{},
		fb,
		&fakeDB{},
	)
}

func TestRefreshVerificationSnapshot_FirstTimePhoneVerification(t *testing.T) {
	nowUser := &userEntity.User{
		ID:          uuid.New(),
		FirebaseUID: "firebase-uid-1",
	}
	repo := &fakeUserRepo{user: nowUser}
	fb := &fakeFirebase{user: &firebaseauth.UserRecord{UserInfo: &firebaseauth.UserInfo{PhoneNumber: "+6281234567890"}}}
	svc := newRefreshService(repo, fb)
	v := true

	snap, err := svc.RefreshVerificationSnapshot(context.Background(), nowUser.ID, &v)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !snap.PhoneVerified {
		t.Fatalf("expected phone verified")
	}
	if snap.PhoneNumber == nil || *snap.PhoneNumber == "" {
		t.Fatalf("expected phone number set")
	}
	if snap.PhoneVerifiedAt == nil {
		t.Fatalf("expected phone_verified_at set")
	}
	if !snap.EmailVerified || snap.EmailVerifiedAt == nil {
		t.Fatalf("expected email monotonic set when firebase email_verified=true")
	}
}

func TestRefreshVerificationSnapshot_IdempotentPreservesPhoneTimestamp(t *testing.T) {
	ts := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Second)
	nowUser := &userEntity.User{
		ID:              uuid.New(),
		FirebaseUID:     "firebase-uid-2",
		PhoneVerified:   true,
		PhoneVerifiedAt: &ts,
	}
	repo := &fakeUserRepo{user: nowUser}
	fb := &fakeFirebase{user: &firebaseauth.UserRecord{UserInfo: &firebaseauth.UserInfo{PhoneNumber: "+628111111111"}}}
	svc := newRefreshService(repo, fb)
	v := false

	snap, err := svc.RefreshVerificationSnapshot(context.Background(), nowUser.ID, &v)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if snap.PhoneVerifiedAt == nil || !snap.PhoneVerifiedAt.Equal(ts) {
		t.Fatalf("expected existing timestamp preserved")
	}
	if repo.update == nil || repo.update.PhoneVerifiedAt == nil || !repo.update.PhoneVerifiedAt.Equal(ts) {
		t.Fatalf("expected repository update to preserve timestamp")
	}
}

func TestRefreshVerificationSnapshot_FirebaseFailure(t *testing.T) {
	nowUser := &userEntity.User{
		ID:          uuid.New(),
		FirebaseUID: "firebase-uid-3",
	}
	repo := &fakeUserRepo{user: nowUser}
	fb := &fakeFirebase{err: errors.New("upstream down")}
	svc := newRefreshService(repo, fb)

	_, err := svc.RefreshVerificationSnapshot(context.Background(), nowUser.ID, nil)
	if err == nil || !errors.Is(err, ErrVerificationRefreshUpstream) {
		t.Fatalf("expected ErrVerificationRefreshUpstream, got %v", err)
	}
}

func TestRefreshVerificationSnapshot_UnprovisionedUser(t *testing.T) {
	repo := &fakeUserRepo{user: nil}
	fb := &fakeFirebase{user: &firebaseauth.UserRecord{}}
	svc := newRefreshService(repo, fb)

	_, err := svc.RefreshVerificationSnapshot(context.Background(), uuid.New(), nil)
	if err == nil || !errors.Is(err, ErrUserNotProvisioned) {
		t.Fatalf("expected ErrUserNotProvisioned, got %v", err)
	}
}

func TestRefreshVerificationSnapshot_EmailVerifiedMonotonic(t *testing.T) {
	ts := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	nowUser := &userEntity.User{
		ID:              uuid.New(),
		FirebaseUID:     "firebase-uid-4",
		EmailVerifiedAt: &ts,
	}
	repo := &fakeUserRepo{user: nowUser}
	fb := &fakeFirebase{user: &firebaseauth.UserRecord{UserInfo: &firebaseauth.UserInfo{PhoneNumber: ""}}}
	svc := newRefreshService(repo, fb)
	v := false

	snap, err := svc.RefreshVerificationSnapshot(context.Background(), nowUser.ID, &v)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if snap.EmailVerifiedAt == nil || !snap.EmailVerifiedAt.Equal(ts) {
		t.Fatalf("expected email_verified_at to remain monotonic")
	}
}


