package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/user/delivery/http/dto"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	outboxInfra "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

var (
	ErrUserNotProvisioned          = errors.New("user not provisioned")
	ErrVerificationRefreshUpstream = errors.New("verification refresh upstream error")
)

type firebaseUserFetcher interface {
	GetUser(ctx context.Context, uid string) (*firebaseauth.UserRecord, error)
}

type userProfileDB interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
	Pool() *pgxpool.Pool
}

type userProfileRepository interface {
	SoftDeleteUser(ctx context.Context, tx db.Tx, userID uuid.UUID) (alreadyDeleted bool, err error)
	GetPublicInfo(ctx context.Context, tx db.Tx, userID uuid.UUID, isOwnProfile bool) (*userEntity.UserPublicInfo, error)
	GetMyProfile(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.MyProfileResponse, error)
	GetByIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*userEntity.User, error)
	Update(ctx context.Context, tx db.Tx, user *userEntity.User) error
}

// SellerState represents the seller capability state for a user
type SellerState struct {
	HasProfile            bool
	HasActiveSubscription bool
	SubscriptionStatus    *string
	Tier                  *string
	HasMarketAuthority    bool
}

// UserProfileService handles cross-domain composition for user profiles
// It orchestrates between user, seller, and subscription domains
type UserProfileService struct {
	userRepo         userProfileRepository
	sellerRepo       sellerRepo.SellerRepository
	subscriptionRepo subscriptionRepo.SellerSubscriptionRepository
	outboxRepo       *outboxInfra.OutboxRepository
	firebaseClient   firebaseUserFetcher
	db               userProfileDB
}

// NewUserProfileService creates a new UserProfileService
func NewUserProfileService(
	userRepo userProfileRepository,
	sellerRepo sellerRepo.SellerRepository,
	subscriptionRepo subscriptionRepo.SellerSubscriptionRepository,
	outboxRepo *outboxInfra.OutboxRepository,
	firebaseClient firebaseUserFetcher,
	database userProfileDB,
) *UserProfileService {
	return &UserProfileService{
		userRepo:         userRepo,
		sellerRepo:       sellerRepo,
		subscriptionRepo: subscriptionRepo,
		outboxRepo:       outboxRepo,
		firebaseClient:   firebaseClient,
		db:               database,
	}
}

type VerificationSnapshot struct {
	PhoneVerified   bool
	PhoneNumber     *string
	PhoneVerifiedAt *time.Time
	EmailVerified   bool
	EmailVerifiedAt *time.Time
}

// RefreshVerificationSnapshot synchronizes user verification fields from
// Firebase-backed identity truth into PostgreSQL.
//
// Email sync is monotonic: once email_verified_at is set, it is not cleared.
// Phone verification is synced from Firebase Auth user phone number and keeps
// existing phone_verified_at on repeated truthy refreshes.
func (s *UserProfileService) RefreshVerificationSnapshot(
	ctx context.Context,
	userID uuid.UUID,
	firebaseEmailVerified *bool,
) (*VerificationSnapshot, error) {
	if s.firebaseClient == nil {
		return nil, fmt.Errorf("%w: firebase client unavailable", ErrVerificationRefreshUpstream)
	}

	var snapshot *VerificationSnapshot
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		user, err := s.userRepo.GetByIDForUpdate(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("get user for update: %w", err)
		}
		if user == nil {
			return ErrUserNotProvisioned
		}

		fbUser, err := s.firebaseClient.GetUser(ctx, user.FirebaseUID)
		if err != nil {
			return fmt.Errorf("%w: get firebase user: %v", ErrVerificationRefreshUpstream, err)
		}

		now := time.Now()
		hasPhone := fbUser.PhoneNumber != ""
		var phoneNumber *string
		if hasPhone {
			p := fbUser.PhoneNumber
			phoneNumber = &p
		}

		if user.PhoneVerifiedAt == nil && hasPhone {
			user.PhoneVerifiedAt = &now
		}
		user.PhoneNumber = phoneNumber
		user.PhoneVerified = hasPhone

		if firebaseEmailVerified != nil && *firebaseEmailVerified && user.EmailVerifiedAt == nil {
			user.EmailVerifiedAt = &now
			user.EmailVerified = true
		}

		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return fmt.Errorf("update user verification snapshot: %w", err)
		}

		snapshot = &VerificationSnapshot{
			PhoneVerified:   user.PhoneVerified,
			PhoneNumber:     user.PhoneNumber,
			PhoneVerifiedAt: user.PhoneVerifiedAt,
			EmailVerified:   user.EmailVerifiedAt != nil,
			EmailVerifiedAt: user.EmailVerifiedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// SelfDeleteAccount soft-deletes the caller's account and emits a user.deleted
// outbox event within the same transaction.  The caller is responsible for
// revoking the Firebase credential after this call returns successfully.
//
// Idempotency: if deleted_at is already set the method returns nil (no-op).
func (s *UserProfileService) SelfDeleteAccount(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	alreadyDeleted, err := s.userRepo.SoftDeleteUser(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	if alreadyDeleted {
		return nil // idempotent
	}

	payload, err := json.Marshal(map[string]string{"user_id": userID.String()})
	if err != nil {
		return fmt.Errorf("marshal user.deleted payload: %w", err)
	}
	if err := s.outboxRepo.InsertEvent(ctx, tx, events.EventUserDeleted, userID, payload); err != nil {
		return fmt.Errorf("insert user.deleted outbox event: %w", err)
	}

	return tx.Commit(ctx)
}

// GetPublicProfile retrieves a public user profile with seller state
// This is the service layer method that composes user + seller data
func (s *UserProfileService) GetPublicProfile(ctx context.Context, targetUserID uuid.UUID, isOwnProfile bool) (*dto.PublicUserResponse, error) {
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get user public info from user repository
	publicInfo, err := s.userRepo.GetPublicInfo(ctx, tx, targetUserID, isOwnProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get public info: %w", err)
	}
	if publicInfo == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get seller state (cross-domain composition)
	sellerState, err := s.getSellerState(ctx, tx, targetUserID)
	if err != nil {
		// Log but don't fail - seller state is not critical for public profile
		sellerState = &SellerState{}
	}

	// Parse CreatedAt from string to time.Time
	var createdAt time.Time
	if publicInfo.CreatedAt != "" {
		parsedTime, err := time.Parse(time.RFC3339, publicInfo.CreatedAt)
		if err == nil {
			createdAt = parsedTime
		}
	}

	// E5.1 — Coarsen raw lifecycle truth at the single canonical mapping site
	// per docs/05-rollout/search-lifecycle-overlay-topology.md §11. Raw
	// account_status enum values never leave this layer; only the coarsened
	// public lifecycle string ({active, unavailable, removed}) is emitted on
	// the wire via publiccard.NewWithLifecycle.
	//
	// Under the current GetPublicInfo SQL filter (WHERE u.deleted_at IS NULL)
	// IsDeleted always scans as false; suspended/banned accounts produce
	// "unavailable", everyone else "active". "removed" is reserved for a
	// future filter-relaxation batch (out of scope for E5.1).
	lifecycle := string(viewercontext.CoarsenLifecycle(publicInfo.AccountStatus, publicInfo.IsDeleted))
	identityCard := publiccard.NewWithLifecycle(
		publicInfo.UserID,
		publicInfo.Username,
		publicInfo.AvatarURL,
		lifecycle,
	)

	// Build response.
	//
	// PUBLIC BOUNDARY: KYC verification flags (is_id_verified,
	// is_farm_verified, is_email_verified) are intentionally NOT projected
	// onto the public profile. The repository still hydrates them on the
	// `publicInfo` value for downstream uses (e.g. self-profile lookups
	// elsewhere), but they do not cross this boundary.
	//
	// E5.1 — Identity is the canonical public-card seam per ADR-006. Flat
	// New consumers MUST read identity.* (in particular identity.lifecycle for
	// the public lifecycle state).
	resp := &dto.PublicUserResponse{
		UserID:         publicInfo.UserID,
		Username:       publicInfo.Username,
		Bio:            publicInfo.Bio,
		AvatarURL:      publicInfo.AvatarURL,
		CoverPhotoURL:  resolveMediaReadURL(publicInfo.CoverPhotoURL),
		Location:       publicInfo.Location,
		FollowersCount: publicInfo.FollowersCount,
		FollowingCount: publicInfo.FollowingCount,
		IsSeller:       sellerState.HasMarketAuthority, // derived; uses authority, not just role
		Roles:          publicInfo.Roles,
		CreatedAt:      createdAt,
		Identity:       &identityCard,
		SellerTier:     publicSellerTier(lifecycle, sellerState),
	}

	return resp, nil
}

// GetMyProfile retrieves the complete profile for the authenticated user
// This includes user, profile, roles, and seller state
func (s *UserProfileService) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get user profile from repository
	myProfile, err := s.userRepo.GetMyProfile(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get my profile: %w", err)
	}
	if myProfile == nil || myProfile.User == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get seller state (cross-domain composition)
	sellerState, err := s.getSellerState(ctx, tx, userID)
	if err != nil {
		// Log but don't fail - seller state is supplemental
		sellerState = &SellerState{}
	}

	// Convert entities to DTOs
	userDTO := s.entityToUserDTO(myProfile.User, myProfile.Roles, sellerState)
	profileDTO := s.entityToProfileDTO(myProfile.Profile)

	return &dto.UserResponse{
		User:    *userDTO,
		Profile: *profileDTO,
	}, nil
}

// getSellerState retrieves seller capability state from seller and subscription domains
// This is the cross-domain composition logic
func (s *UserProfileService) getSellerState(ctx context.Context, tx db.Tx, userID uuid.UUID) (*SellerState, error) {
	state := &SellerState{
		HasProfile:            false,
		HasActiveSubscription: false,
		HasMarketAuthority:    false,
	}

	// Check for seller profile
	sellerProfile, err := s.sellerRepo.GetByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check seller profile: %w", err)
	}
	if sellerProfile != nil {
		state.HasProfile = true
		if sellerProfile.Tier != "" {
			tier := string(sellerProfile.Tier)
			state.Tier = &tier
		}
	}

	// Check for subscription status
	subscription, err := s.subscriptionRepo.GetLatestByUserID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check seller subscription: %w", err)
	}

	if subscription != nil {
		status := string(subscription.Status)
		state.SubscriptionStatus = &status
		state.HasActiveSubscription = subscription.Status == subscriptionEntity.StatusActive
	}

	// Determine market authority: has profile + active subscription
	if state.HasProfile && state.HasActiveSubscription {
		state.HasMarketAuthority = true
	}

	return state, nil
}

// entityToUserDTO converts user entity to DTO with seller state
func (s *UserProfileService) entityToUserDTO(user *userEntity.User, roles []string, sellerState *SellerState) *dto.UserDTO {
	// Convert bool to *bool for DTO
	var idVerified *bool
	if user.IsIDVerified {
		idVerified = &user.IsIDVerified
	}
	var farmVerified *bool
	if user.IsFarmVerified {
		farmVerified = &user.IsFarmVerified
	}

	dto := &dto.UserDTO{
		ID:                       user.ID,
		Email:                    user.Email,
		PhoneNumber:              user.PhoneNumber,
		EmailVerified:            user.EmailVerified,
		PhoneVerified:            user.PhoneVerified,
		AccountStatus:            string(user.AccountStatus),
		Roles:                    roles,
		CreatedAt:                user.CreatedAt,
		UpdatedAt:                user.UpdatedAt,
		IsIDVerified:             idVerified,
		IsFarmVerified:           farmVerified,
		PhoneVerifiedAt:          user.PhoneVerifiedAt,
		EmailVerifiedAt:          user.EmailVerifiedAt,
		IDVerifiedAt:             user.IDVerifiedAt,
		FarmVerifiedAt:           user.FarmVerifiedAt,
		HasSellerProfile:         sellerState.HasProfile,
		SellerSubscriptionStatus: sellerState.SubscriptionStatus,
		HasMarketAuthority:       sellerState.HasMarketAuthority,
		SellerTier:               sellerState.Tier,
	}
	return dto
}

// entityToProfileDTO converts user profile entity to DTO
func (s *UserProfileService) entityToProfileDTO(profile *userEntity.UserProfile) *dto.ProfileDTO {
	if profile == nil {
		return &dto.ProfileDTO{}
	}

	// Convert *string to string for PreferredLang
	preferredLang := ""
	if profile.PreferredLang != nil {
		preferredLang = *profile.PreferredLang
	}

	// Convert bool to *bool for IsVerified
	var isVerified *bool
	if profile.IsVerified {
		isVerified = &profile.IsVerified
	}

	dto := &dto.ProfileDTO{
		ID:             profile.ID,
		UserID:         profile.UserID,
		Username:       stringValue(profile.Username),
		Bio:            profile.Bio,
		AvatarURL:      profile.AvatarURL,
		CoverPhotoURL:  resolveMediaReadURL(profile.CoverPhotoURL),
		DateOfBirth:    profile.DateOfBirth,
		Gender:         profile.Gender,
		Location:       profile.Location,
		City:           profile.City,
		Province:       profile.Province,
		PreferredLang:  preferredLang,
		LastActiveAt:   profile.LastActiveAt,
		FollowersCount: profile.FollowersCount,
		FollowingCount: profile.FollowingCount,
		IsVerified:     isVerified,
	}

	// TODO: Convert social_media and privacy JSON to DTOs

	return dto
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// resolveMediaReadURL resolves a persisted storage key (or absolute URL) to a
// client-facing read URL using the existing mediaresolve authority — the same
// proven pattern used by content/for-sale/auction projections. Nil or empty
// references resolve to nil (field omitted from the wire). On resolution
// failure (e.g. mediaresolve default config not configured in the running
// environment) the original reference is passed through unchanged so a raw
// storage key is never fabricated and an absolute URL is never mangled.
func resolveMediaReadURL(ref *string) *string {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return nil
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(*ref)
	if err != nil {
		return ref
	}
	return &resolved
}

// publicSellerTier returns the seller tier for public profile display, or nil
// when the badge should be hidden.
//
// Delegates to publiccard.GatedSellerTier — the single canonical 4-gate policy
// shared by profile, for_sale, and auction surfaces:
//  1. Feature flag ENABLE_PUBLIC_SELLER_TIER_PROFILE is set to true/1/yes
//  2. User-identity lifecycle is "active" (not suspended/banned/deleted)
//  3. Seller has market authority (HasProfile + HasActiveSubscription)
//  4. Tier is "pro" or "elite" (Basic is never shown publicly)
//
// Gate 3 maps HasMarketAuthority to the "active"/"unavailable" trust lifecycle
// string that GatedSellerTier expects. Behaviorally identical to the previous
// inline implementation; unified here to prevent future policy divergence.
func publicSellerTier(userLifecycle string, seller *SellerState) *string {
	// Coarsen seller state into the string primitives GatedSellerTier expects.
	tier := ""
	if seller != nil && seller.Tier != nil {
		tier = *seller.Tier
	}
	// Gate 3 for profile surface: market authority (profile + active subscription).
	// Coarsen to "active" / "unavailable" so the shared gate can evaluate it.
	trustLifecycle := "unavailable"
	if seller != nil && seller.HasMarketAuthority {
		trustLifecycle = "active"
	}
	return publiccard.GatedSellerTier(tier, userLifecycle, trustLifecycle)
}
