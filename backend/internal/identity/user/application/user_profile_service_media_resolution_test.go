package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/labuda/backend/pkg/db"
)

type profileMediaUserRepo struct {
	publicInfo *userEntity.UserPublicInfo
	myProfile  *userEntity.MyProfileResponse
}

func (r *profileMediaUserRepo) SoftDeleteUser(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *profileMediaUserRepo) GetPublicInfo(context.Context, db.Tx, uuid.UUID, bool) (*userEntity.UserPublicInfo, error) {
	return r.publicInfo, nil
}

func (r *profileMediaUserRepo) GetMyProfile(context.Context, db.Tx, uuid.UUID) (*userEntity.MyProfileResponse, error) {
	return r.myProfile, nil
}

func (r *profileMediaUserRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*userEntity.User, error) {
	return nil, nil
}

func (r *profileMediaUserRepo) Update(context.Context, db.Tx, *userEntity.User) error {
	return nil
}

type profileMediaSellerRepo struct {
	profile *sellerEntity.SellerProfile
}

func (r *profileMediaSellerRepo) InsertProfileTx(context.Context, db.Tx, *sellerEntity.SellerProfile) error {
	return nil
}

func (r *profileMediaSellerRepo) GetByUserID(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

func (r *profileMediaSellerRepo) GetByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

func (r *profileMediaSellerRepo) EnsureProfileExistsTx(ctx context.Context, tx db.Tx, userID uuid.UUID, storeName string) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

func (r *profileMediaSellerRepo) UpdateStoreIdentityTx(context.Context, db.Tx, uuid.UUID, *string, *string) error {
	return nil
}

func (r *profileMediaSellerRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

func (r *profileMediaSellerRepo) UpdateStoreImageTx(context.Context, db.Tx, uuid.UUID, *string) error {
	return nil
}

func (r *profileMediaSellerRepo) UpdateTierTx(context.Context, db.Tx, uuid.UUID, sellerEntity.Tier) error {
	return nil
}

func (r *profileMediaSellerRepo) InsertMonthlyMetricTx(context.Context, db.Tx, *sellerEntity.SellerMonthlyMetric) error {
	return nil
}

func (r *profileMediaSellerRepo) UpsertReputationStateTx(context.Context, db.Tx, *sellerEntity.SellerReputationState) error {
	return nil
}

func (r *profileMediaSellerRepo) GetReputationStateForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerReputationState, error) {
	return nil, nil
}

type profileMediaSubRepo struct {
	sub *subscriptionEntity.SellerSubscription
}

func (r *profileMediaSubRepo) InsertTx(context.Context, db.Tx, *subscriptionEntity.SellerSubscription) error {
	return nil
}

func (r *profileMediaSubRepo) UpdateStatusTx(context.Context, db.Tx, uuid.UUID, subscriptionEntity.Status, subscriptionEntity.Status) error {
	return nil
}

func (r *profileMediaSubRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.sub, nil
}

func (r *profileMediaSubRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.sub, nil
}

func (r *profileMediaSubRepo) GetLatestByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.sub, nil
}

func (r *profileMediaSubRepo) GetLatestByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.sub, nil
}

func (r *profileMediaSubRepo) GetActiveByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.sub, nil
}

func (r *profileMediaSubRepo) FetchActiveExpiredBatch(context.Context, db.Tx, time.Time, int) ([]*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *profileMediaSubRepo) FetchActiveExpiredBatchIDs(context.Context, db.Tx, time.Time, int) ([]uuid.UUID, error) {
	return nil, nil
}

func (r *profileMediaSubRepo) ExistsActiveByUserID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *profileMediaSubRepo) GetActiveConfig(context.Context, db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	return nil, nil
}

func (r *profileMediaSubRepo) UpdateConfigTx(context.Context, db.Tx, uuid.UUID, int64, int, int, bool) error {
	return nil
}

func setReadableMediaDefaultsForTest(t *testing.T) {
	t.Helper()
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.com/media",
		ReadTTL:    time.Minute,
	})
	t.Cleanup(func() {
		mediaresolve.SetDefaultConfig(mediaresolve.Config{})
	})
}

func TestUserProfileService_ResolvesCoverPhotoURLForPublicAndSelfProfiles(t *testing.T) {
	setReadableMediaDefaultsForTest(t)

	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	coverKey := "images/profile-covers/" + userID.String() + ".jpg"
	coverReadURL := "https://cdn.example.com/media/images/profile-covers/" + userID.String() + ".jpg"
	avatarURL := "https://cdn.example.com/avatar.jpg"

	publicInfo := &userEntity.UserPublicInfo{
		UserID:        userID,
		Username:      "selleralice",
		AvatarURL:     &avatarURL,
		CoverPhotoURL: &coverKey,
		CreatedAt:     now.Format(time.RFC3339),
		AccountStatus: "active",
		IsDeleted:     false,
		FollowersCount: 12,
		FollowingCount: 7,
		Roles:         []string{"user"},
	}
	myProfile := &userEntity.MyProfileResponse{
		User: &userEntity.User{
			ID:            userID,
			Email:         strPtr("selleralice@example.com"),
			AccountStatus: "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		Profile: &userEntity.UserProfile{
			ID:            uuid.New(),
			UserID:        userID,
			Username:      strPtr("selleralice"),
			CoverPhotoURL: &coverKey,
			FollowersCount: 12,
			FollowingCount: 7,
		},
		Roles: []string{"user"},
	}
	sellerProfile := &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    userID,
		StoreName: "Alice Store",
		Tier:      sellerEntity.TierPro,
		CreatedAt: now,
		UpdatedAt: now,
	}
	activeSub := &subscriptionEntity.SellerSubscription{
		Status: subscriptionEntity.StatusActive,
	}

	svc := NewUserProfileService(
		&profileMediaUserRepo{publicInfo: publicInfo, myProfile: myProfile},
		&profileMediaSellerRepo{profile: sellerProfile},
		&profileMediaSubRepo{sub: activeSub},
		nil,
		&fakeFirebase{},
		&fakeDB{},
	)

	publicResp, err := svc.GetPublicProfile(context.Background(), userID, false)
	if err != nil {
		t.Fatalf("GetPublicProfile returned error: %v", err)
	}
	if publicResp.CoverPhotoURL == nil {
		t.Fatal("expected public cover_photo_url to resolve")
	}
	if got := *publicResp.CoverPhotoURL; got != coverReadURL {
		t.Fatalf("public cover_photo_url = %q, want %q", got, coverReadURL)
	}
	if publicResp.AvatarURL == nil || *publicResp.AvatarURL != avatarURL {
		t.Fatalf("public avatar_url = %v, want %q", publicResp.AvatarURL, avatarURL)
	}
	if publicResp.CoverPhotoURL != nil && *publicResp.CoverPhotoURL == coverKey {
		t.Fatal("public cover_photo_url must not remain a raw key")
	}

	selfResp, err := svc.GetMyProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetMyProfile returned error: %v", err)
	}
	if selfResp.Profile.CoverPhotoURL == nil {
		t.Fatal("expected self cover_photo_url to resolve")
	}
	if got := *selfResp.Profile.CoverPhotoURL; got != coverReadURL {
		t.Fatalf("self cover_photo_url = %q, want %q", got, coverReadURL)
	}
	if selfResp.Profile.CoverPhotoURL != nil && *selfResp.Profile.CoverPhotoURL == coverKey {
		t.Fatal("self cover_photo_url must not remain a raw key")
	}
}

func TestUserProfileService_PreservesNilCoverPhotoURL(t *testing.T) {
	setReadableMediaDefaultsForTest(t)

	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	avatarURL := "https://cdn.example.com/avatar.jpg"

	publicInfo := &userEntity.UserPublicInfo{
		UserID:        userID,
		Username:      "sellerbob",
		AvatarURL:     &avatarURL,
		CreatedAt:     now.Format(time.RFC3339),
		AccountStatus: "active",
		IsDeleted:     false,
	}
	myProfile := &userEntity.MyProfileResponse{
		User: &userEntity.User{
			ID:            userID,
			Email:         strPtr("sellerbob@example.com"),
			AccountStatus: "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		Profile: &userEntity.UserProfile{
			ID:       uuid.New(),
			UserID:   userID,
			Username: strPtr("sellerbob"),
		},
		Roles: []string{"user"},
	}
	sellerProfile := &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    userID,
		StoreName: "Bob Store",
		Tier:      sellerEntity.TierBasic,
		CreatedAt: now,
		UpdatedAt: now,
	}
	activeSub := &subscriptionEntity.SellerSubscription{
		Status: subscriptionEntity.StatusActive,
	}

	svc := NewUserProfileService(
		&profileMediaUserRepo{publicInfo: publicInfo, myProfile: myProfile},
		&profileMediaSellerRepo{profile: sellerProfile},
		&profileMediaSubRepo{sub: activeSub},
		nil,
		&fakeFirebase{},
		&fakeDB{},
	)

	publicResp, err := svc.GetPublicProfile(context.Background(), userID, false)
	if err != nil {
		t.Fatalf("GetPublicProfile returned error: %v", err)
	}
	if publicResp.CoverPhotoURL != nil {
		t.Fatalf("public cover_photo_url = %v, want nil", publicResp.CoverPhotoURL)
	}

	selfResp, err := svc.GetMyProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetMyProfile returned error: %v", err)
	}
	if selfResp.Profile.CoverPhotoURL != nil {
		t.Fatalf("self cover_photo_url = %v, want nil", selfResp.Profile.CoverPhotoURL)
	}
}

// TestResolveMediaReadURLHelper proves the resolve helper used by the profile
// projection: a persisted storage key resolves to a CDN read URL through the
// existing mediaresolve authority, an absolute URL passes through unchanged,
// and a nil/empty reference stays nil. This is the deterministic unit proof of
// the read-resolution contract without requiring a database.
func TestResolveMediaReadURLHelper(t *testing.T) {
	setReadableMediaDefaultsForTest(t)

	userID := uuid.New()
	coverKey := "images/profile-covers/" + userID.String() + ".jpg"
	coverReadURL := "https://cdn.example.com/media/images/profile-covers/" + userID.String() + ".jpg"

	// Storage key → CDN read URL.
	got := resolveMediaReadURL(&coverKey)
	if got == nil || *got != coverReadURL {
		t.Fatalf("resolveMediaReadURL(key) = %v, want %q", got, coverReadURL)
	}

	// Absolute URL passes through unchanged.
	abs := "https://example.com/legacy-cover.jpg"
	got = resolveMediaReadURL(&abs)
	if got == nil || *got != abs {
		t.Fatalf("resolveMediaReadURL(abs) = %v, want %q", got, abs)
	}

	// Nil and empty stay nil.
	if got := resolveMediaReadURL(nil); got != nil {
		t.Fatalf("resolveMediaReadURL(nil) = %v, want nil", got)
	}
	empty := ""
	if got := resolveMediaReadURL(&empty); got != nil {
		t.Fatalf("resolveMediaReadURL(empty) = %v, want nil", got)
	}
}
