package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/internal/identity/user/repository"
	"github.com/labuda/backend/internal/util"
	"github.com/labuda/backend/pkg/db"
)

// userRepositoryImpl implements UserRepository interface.
type userRepositoryImpl struct {
	db *db.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(database *db.DB) repository.UserRepository {
	return &userRepositoryImpl{db: database}
}

// ============================================================================
// USER QUERIES
// ============================================================================

func (r *userRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.User, error) {
	var user entity.User
	var phoneNumber sql.NullString
	var emailVerifiedAt, phoneVerifiedAt, idVerifiedAt, farmVerifiedAt, deletedAt sql.NullTime
	query := `
		SELECT
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	err := tx.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.FirebaseUID, &user.Email, &phoneNumber,
		&user.PhoneVerified, &user.AccountStatus,
		&user.IsIDVerified, &user.IsFarmVerified,
		&emailVerifiedAt, &phoneVerifiedAt, &idVerifiedAt, &farmVerifiedAt,
		&user.CreatedAt, &user.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	// Convert sql.NullString to *string
	user.PhoneNumber = nullStringToPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimeToPtr(emailVerifiedAt)
	user.EmailVerified = user.EmailVerifiedAt != nil
	user.PhoneVerifiedAt = nullTimeToPtr(phoneVerifiedAt)
	user.IDVerifiedAt = nullTimeToPtr(idVerifiedAt)
	user.FarmVerifiedAt = nullTimeToPtr(farmVerifiedAt)
	user.DeletedAt = nullTimeToPtr(deletedAt)
	return &user, nil
}

func (r *userRepositoryImpl) GetByIDForUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.User, error) {
	var user entity.User
	var phoneNumber sql.NullString
	var emailVerifiedAt, phoneVerifiedAt, idVerifiedAt, farmVerifiedAt, deletedAt sql.NullTime
	query := `
		SELECT
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`
	err := tx.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.FirebaseUID, &user.Email, &phoneNumber,
		&user.PhoneVerified, &user.AccountStatus,
		&user.IsIDVerified, &user.IsFarmVerified,
		&emailVerifiedAt, &phoneVerifiedAt, &idVerifiedAt, &farmVerifiedAt,
		&user.CreatedAt, &user.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID for update: %w", err)
	}
	user.PhoneNumber = nullStringToPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimeToPtr(emailVerifiedAt)
	user.EmailVerified = user.EmailVerifiedAt != nil
	user.PhoneVerifiedAt = nullTimeToPtr(phoneVerifiedAt)
	user.IDVerifiedAt = nullTimeToPtr(idVerifiedAt)
	user.FarmVerifiedAt = nullTimeToPtr(farmVerifiedAt)
	user.DeletedAt = nullTimeToPtr(deletedAt)
	return &user, nil
}

func (r *userRepositoryImpl) GetByFirebaseUID(ctx context.Context, tx db.Tx, firebaseUID string) (*entity.User, error) {
	var user entity.User
	var phoneNumber sql.NullString
	var emailVerifiedAt, phoneVerifiedAt, idVerifiedAt, farmVerifiedAt, deletedAt sql.NullTime
	query := `
		SELECT
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at, deleted_at
		FROM users
		WHERE firebase_uid = $1 AND deleted_at IS NULL
	`
	err := tx.QueryRow(ctx, query, firebaseUID).Scan(
		&user.ID, &user.FirebaseUID, &user.Email, &phoneNumber,
		&user.PhoneVerified, &user.AccountStatus,
		&user.IsIDVerified, &user.IsFarmVerified,
		&emailVerifiedAt, &phoneVerifiedAt, &idVerifiedAt, &farmVerifiedAt,
		&user.CreatedAt, &user.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by Firebase UID: %w", err)
	}
	user.PhoneNumber = nullStringToPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimeToPtr(emailVerifiedAt)
	user.EmailVerified = user.EmailVerifiedAt != nil
	user.PhoneVerifiedAt = nullTimeToPtr(phoneVerifiedAt)
	user.IDVerifiedAt = nullTimeToPtr(idVerifiedAt)
	user.FarmVerifiedAt = nullTimeToPtr(farmVerifiedAt)
	user.DeletedAt = nullTimeToPtr(deletedAt)
	return &user, nil
}

func (r *userRepositoryImpl) GetByEmail(ctx context.Context, tx db.Tx, email string) (*entity.User, error) {
	var user entity.User
	var phoneNumber sql.NullString
	var emailVerifiedAt, phoneVerifiedAt, idVerifiedAt, farmVerifiedAt, deletedAt sql.NullTime
	query := `
		SELECT
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at, deleted_at
		FROM users
		WHERE LOWER(BTRIM(email)) = LOWER(BTRIM($1)) AND deleted_at IS NULL
		LIMIT 1
	`
	err := tx.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.FirebaseUID, &user.Email, &phoneNumber,
		&user.PhoneVerified, &user.AccountStatus,
		&user.IsIDVerified, &user.IsFarmVerified,
		&emailVerifiedAt, &phoneVerifiedAt, &idVerifiedAt, &farmVerifiedAt,
		&user.CreatedAt, &user.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	user.PhoneNumber = nullStringToPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimeToPtr(emailVerifiedAt)
	user.EmailVerified = user.EmailVerifiedAt != nil
	user.PhoneVerifiedAt = nullTimeToPtr(phoneVerifiedAt)
	user.IDVerifiedAt = nullTimeToPtr(idVerifiedAt)
	user.FarmVerifiedAt = nullTimeToPtr(farmVerifiedAt)
	user.DeletedAt = nullTimeToPtr(deletedAt)
	return &user, nil
}

func (r *userRepositoryImpl) GetMultipleByIDs(ctx context.Context, tx db.Tx, userIDs []uuid.UUID) (map[uuid.UUID]*entity.User, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID]*entity.User), nil
	}

	query := `
		SELECT
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at, deleted_at
		FROM users
		WHERE id = ANY($1) AND deleted_at IS NULL
	`

	rows, err := tx.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple users: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*entity.User)
	for rows.Next() {
		var user entity.User
		var phoneNumber sql.NullString
		var emailVerifiedAt, phoneVerifiedAt, idVerifiedAt, farmVerifiedAt, deletedAt sql.NullTime
		err := rows.Scan(
			&user.ID, &user.FirebaseUID, &user.Email, &phoneNumber,
			&user.PhoneVerified, &user.AccountStatus,
			&user.IsIDVerified, &user.IsFarmVerified,
			&emailVerifiedAt, &phoneVerifiedAt, &idVerifiedAt, &farmVerifiedAt,
			&user.CreatedAt, &user.UpdatedAt, &deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		user.PhoneNumber = nullStringToPtr(phoneNumber)
		user.EmailVerifiedAt = nullTimeToPtr(emailVerifiedAt)
		user.EmailVerified = user.EmailVerifiedAt != nil
		user.PhoneVerifiedAt = nullTimeToPtr(phoneVerifiedAt)
		user.IDVerifiedAt = nullTimeToPtr(idVerifiedAt)
		user.FarmVerifiedAt = nullTimeToPtr(farmVerifiedAt)
		user.DeletedAt = nullTimeToPtr(deletedAt)
		result[user.ID] = &user
	}

	return result, nil
}

// ============================================================================
// USER PROFILE QUERIES
// ============================================================================

func (r *userRepositoryImpl) GetProfileByID(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.UserProfile, error) {
	var profile entity.UserProfile

	var username, bio, avatarURL, coverPhotoURL, location, city, province sql.NullString
	var isVerified sql.NullBool
	var followersCount, followingCount sql.NullInt64
	var createdAt, updatedAt sql.NullTime

	query := `
		SELECT
			user_id, username, bio, avatar_url, cover_photo_url,
			location, city, province, is_verified,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.following_id = user_profiles.user_id
			), 0) AS followers_count,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.follower_id = user_profiles.user_id
			), 0) AS following_count,
			created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1
	`

	err := tx.QueryRow(ctx, query, userID).Scan(
		&profile.UserID,
		&username,
		&bio,
		&avatarURL,
		&coverPhotoURL,
		&location,
		&city,
		&province,
		&isVerified,
		&followersCount,
		&followingCount,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get profile by ID: %w", err)
	}

	profile.ID = userID
	profile.Username = nullStringToPtr(username)
	profile.Bio = nullStringToPtr(bio)
	profile.AvatarURL = nullStringToPtr(avatarURL)
	profile.CoverPhotoURL = nullStringToPtr(coverPhotoURL)
	profile.Location = nullStringToPtr(location)
	profile.City = nullStringToPtr(city)
	profile.Province = nullStringToPtr(province)
	profile.IsVerified = isVerified.Bool
	profile.FollowersCount = int(followersCount.Int64)
	profile.FollowingCount = int(followingCount.Int64)
	profile.CreatedAt = createdAt.Time
	profile.UpdatedAt = updatedAt.Time

	return &profile, nil
}

func (r *userRepositoryImpl) GetPublicInfo(ctx context.Context, tx db.Tx, userID uuid.UUID, isOwnProfile bool) (*entity.UserPublicInfo, error) {
	var info entity.UserPublicInfo
	var location sql.NullString
	var showLocation bool

	// PRIVACY: public-facing identity MUST mirror username only.
	// p.full_name is KYC/private data and MUST NOT be projected to any
	// public viewer surface (/users/{id} and any consumer of GetPublicInfo).
	//
	// E5.1 — u.account_status + (u.deleted_at IS NOT NULL) are projected so
	// the application layer can coarsen via viewercontext.CoarsenLifecycle
	// at the single canonical mapping site. is_deleted always scans as
	// false under the current WHERE u.deleted_at IS NULL filter; the
	// projection is preserved for forward-compat with a later batch that
	// may relax the filter (out of scope for E5.1). Raw account_status
	// values do not leave the service boundary — only the coarsened
	// public lifecycle string is emitted on the wire.
	query := `
		SELECT
			u.id,
			COALESCE(p.username, ''),
			p.bio,
			p.avatar_url,
			p.cover_photo_url,
			p.location,
			COALESCE(u.is_id_verified, false),
			COALESCE(u.is_farm_verified, false),
			(u.email_verified_at IS NOT NULL),
			EXTRACT(EPOCH FROM u.created_at) AS created_at_epoch,
			COALESCE(p.privacy->>'show_location' = 'true', false),
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON u.id = p.user_id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`
	err := tx.QueryRow(ctx, query, userID).Scan(
		&info.UserID,
		&info.Username,
		&info.Bio,
		&info.AvatarURL,
		&info.CoverPhotoURL,
		&location,
		&info.IsIDVerified,
		&info.IsFarmVerified,
		&info.IsEmailVerified,
		&info.CreatedAt,
		&showLocation,
		&info.AccountStatus,
		&info.IsDeleted,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get public user info: %w", err)
	}

	// Get follower/following counts from derived queries
	followersCount, err := r.GetFollowersCount(ctx, tx, userID)
	if err == nil {
		info.FollowersCount = followersCount
	}
	followingCount, err := r.GetFollowingCount(ctx, tx, userID)
	if err == nil {
		info.FollowingCount = followingCount
	}

	// Only show location if user allows it or is own profile
	if showLocation || isOwnProfile {
		info.Location = nullStringPtr(location)
	}

	// Get user roles (public info)
	roles, err := r.GetRoles(ctx, tx, userID)
	if err == nil {
		info.Roles = roles
	}

	// NOTE: Seller state should be retrieved from seller domain (SellerRepository)
	// This repository only handles identity + profile
	//
	// ⚠️ ROLE IS NON-AUTHORITATIVE FOR SELLER OPERATIONS ⚠️
	// This field is for UI/display purposes only.
	// For authoritative seller capability checks, use:
	// - RoleChecker.HasActiveSellerCapability() (subscription-based)
	// - SellerRepository for profile existence
	info.IsSeller = false
	info.HasSellerProfile = false

	return &info, nil
}

func (r *userRepositoryImpl) GetPublicInfoMultiple(ctx context.Context, tx db.Tx, userIDs []uuid.UUID) (map[uuid.UUID]*entity.UserPublicInfo, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID]*entity.UserPublicInfo), nil
	}

	// PRIVACY: this is the batch variant of GetPublicInfo. The selected
	// columns derive only from p.username; p.full_name never enters the
	// public surface.
	query := `
		SELECT
			u.id,
			COALESCE(p.username, '') as username,
			p.avatar_url
		FROM users u
		LEFT JOIN user_profiles p ON u.id = p.user_id
		WHERE u.id = ANY($1) AND u.deleted_at IS NULL
	`

	rows, err := tx.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get public info for multiple users: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*entity.UserPublicInfo)
	for rows.Next() {
		var info entity.UserPublicInfo
		var username, avatarURL string
		err := rows.Scan(&info.UserID, &username, &avatarURL)
		if err != nil {
			continue
		}

		info.Username = username
		if avatarURL != "" {
			info.AvatarURL = &avatarURL
		}

		result[info.UserID] = &info
	}

	return result, nil
}

func (r *userRepositoryImpl) GetMyProfile(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.MyProfileResponse, error) {
	// Get user
	user, err := r.GetByID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get profile
	profile, err := r.GetProfileByID(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Get roles
	roles, err := r.GetRoles(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}

	return &entity.MyProfileResponse{
		User:    user,
		Profile: profile,
		Roles:   roles,
	}, nil
}

// ============================================================================
// USER MUTATIONS
// ============================================================================

func (r *userRepositoryImpl) Create(ctx context.Context, tx db.Tx, user *entity.User) error {
	// Normalize email before database write
	normalizedEmail := util.NormalizeEmail(user.Email)
	if normalizedEmail != nil {
		user.Email = normalizedEmail
	} else {
		user.Email = nil
	}

	query := `
		INSERT INTO users (
			id, firebase_uid, email, phone_number,
			phone_verified, account_status,
			is_id_verified, is_farm_verified,
			email_verified_at, phone_verified_at, id_verified_at, farm_verified_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW()
		)
		ON CONFLICT (firebase_uid) DO NOTHING
	`
	_, err := tx.Exec(ctx, query,
		user.ID, user.FirebaseUID, user.Email, user.PhoneNumber,
		user.PhoneVerified, user.AccountStatus,
		user.IsIDVerified, user.IsFarmVerified,
		user.EmailVerifiedAt, user.PhoneVerifiedAt, user.IDVerifiedAt, user.FarmVerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *userRepositoryImpl) Update(ctx context.Context, tx db.Tx, user *entity.User) error {
	// Normalize email before database write
	normalizedEmail := util.NormalizeEmail(user.Email)
	if normalizedEmail != nil {
		user.Email = normalizedEmail
	} else {
		user.Email = nil
	}

	query := `
		UPDATE users
		SET
			email = $2,
			phone_number = $3,
			phone_verified = $4,
			account_status = $5,
			is_id_verified = $6,
			is_farm_verified = $7,
			email_verified_at = $8,
			phone_verified_at = $9,
			id_verified_at = $10,
			farm_verified_at = $11,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := tx.Exec(ctx, query,
		user.ID, user.Email, user.PhoneNumber,
		user.PhoneVerified, user.AccountStatus,
		user.IsIDVerified, user.IsFarmVerified,
		user.EmailVerifiedAt, user.PhoneVerifiedAt, user.IDVerifiedAt, user.FarmVerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (r *userRepositoryImpl) SoftDeleteUser(ctx context.Context, tx db.Tx, userID uuid.UUID) (alreadyDeleted bool, err error) {
	query := `UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := tx.Exec(ctx, query, userID)
	if err != nil {
		return false, fmt.Errorf("failed to soft-delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return true, nil
	}
	return false, nil
}

func (r *userRepositoryImpl) UpdateProfile(ctx context.Context, tx db.Tx, userID uuid.UUID, input *entity.UpdateProfileInput) (*entity.UserProfile, error) {
	// Build update query dynamically based on provided fields
	updateFields := []string{}
	updateValues := []interface{}{}
	argCount := 1

	if input.Bio != nil {
		updateFields = append(updateFields, fmt.Sprintf("bio = $%d", argCount))
		updateValues = append(updateValues, *input.Bio)
		argCount++
	}
	if input.Location != nil {
		updateFields = append(updateFields, fmt.Sprintf("location = $%d", argCount))
		updateValues = append(updateValues, *input.Location)
		argCount++
	}
	if input.City != nil {
		updateFields = append(updateFields, fmt.Sprintf("city = $%d", argCount))
		updateValues = append(updateValues, *input.City)
		argCount++
	}
	if input.Province != nil {
		updateFields = append(updateFields, fmt.Sprintf("province = $%d", argCount))
		updateValues = append(updateValues, *input.Province)
		argCount++
	}
	if input.AvatarURL != nil {
		updateFields = append(updateFields, fmt.Sprintf("avatar_url = $%d", argCount))
		updateValues = append(updateValues, *input.AvatarURL)
		argCount++
	}
	if input.CoverPhotoURL != nil {
		updateFields = append(updateFields, fmt.Sprintf("cover_photo_url = $%d", argCount))
		// Empty string is the canonical "clear the cover" signal → store NULL.
		updateValues = append(updateValues, nullableStringValue(input.CoverPhotoURL))
		argCount++
	}
	if input.Username != nil {
		updateFields = append(updateFields, fmt.Sprintf("username = $%d", argCount))
		updateValues = append(updateValues, *input.Username)
		argCount++
	}
	if input.Gender != nil {
		updateFields = append(updateFields, fmt.Sprintf("gender = $%d", argCount))
		updateValues = append(updateValues, *input.Gender)
		argCount++
	}

	if len(updateFields) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Add updated_at and user_id
	updateFields = append(updateFields, fmt.Sprintf("updated_at = NOW()"))
	updateValues = append(updateValues, userID)

	query := fmt.Sprintf(`
		UPDATE user_profiles
		SET %s
		WHERE user_id = $%d
		RETURNING id, username, bio, avatar_url, cover_photo_url, location, city, province
	`, joinFields(updateFields), argCount)

	var profileID uuid.UUID
	var username, bio, avatarURL, coverPhotoURL, location, city, province sql.NullString
	err := tx.QueryRow(ctx, query, updateValues...).Scan(
		&profileID, &username, &bio, &avatarURL, &coverPhotoURL, &location, &city, &province,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Profile doesn't exist, create it
			return r.createProfileFromUpdate(ctx, tx, userID, input)
		}
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Return updated profile
	return &entity.UserProfile{
		ID:            profileID,
		UserID:        userID,
		Username:      nullStringToPtr(username),
		Bio:           nullStringToPtr(bio),
		AvatarURL:     nullStringToPtr(avatarURL),
		CoverPhotoURL: nullStringToPtr(coverPhotoURL),
		Location:      nullStringToPtr(location),
		City:          nullStringToPtr(city),
		Province:      nullStringToPtr(province),
	}, nil
}

func (r *userRepositoryImpl) createProfileFromUpdate(ctx context.Context, tx db.Tx, userID uuid.UUID, input *entity.UpdateProfileInput) (*entity.UserProfile, error) {
	insertQuery := `
		INSERT INTO user_profiles (user_id, bio, location, city, province, avatar_url, cover_photo_url, username, gender, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, username, bio, avatar_url, cover_photo_url, location, city, province
	`

	var profileID uuid.UUID
	var username, bio, avatarURL, coverPhotoURL, location, city, province sql.NullString

	// Extract values from input, using nil pointers for missing fields
	bioInput := sqlToNullString(input.Bio)
	locationInput := sqlToNullString(input.Location)
	cityInput := sqlToNullString(input.City)
	provinceInput := sqlToNullString(input.Province)
	avatarInput := sqlToNullString(input.AvatarURL)
	coverInput := sqlToNullString(input.CoverPhotoURL)
	usernameInput := sqlToNullString(input.Username)
	genderInput := sqlToNullString(input.Gender)

	_ = genderInput // Gender is stored but not returned in this query

	err := tx.QueryRow(ctx, insertQuery,
		userID, bioInput, locationInput, cityInput, provinceInput,
		avatarInput, coverInput, usernameInput, genderInput,
	).Scan(&profileID, &username, &bio, &avatarURL, &coverPhotoURL, &location, &city, &province)

	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	return &entity.UserProfile{
		ID:            profileID,
		UserID:        userID,
		Username:      nullStringToPtr(username),
		Bio:           nullStringToPtr(bio),
		AvatarURL:     nullStringToPtr(avatarURL),
		CoverPhotoURL: nullStringToPtr(coverPhotoURL),
		Location:      nullStringToPtr(location),
		City:          nullStringToPtr(city),
		Province:      nullStringToPtr(province),
	}, nil
}

// ============================================================================
// ROLE QUERIES
// ============================================================================

func (r *userRepositoryImpl) GetRoles(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]string, error) {
	// FIXED: Query role from users table, not user_roles table
	// The users table has a 'role' column that stores the user's role
	var role string
	query := `SELECT role FROM users WHERE id = $1`
	err := tx.QueryRow(ctx, query, userID).Scan(&role)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// Return role as a slice for compatibility with existing code
	return []string{role}, nil
}

func (r *userRepositoryImpl) HasRole(ctx context.Context, tx db.Tx, userID uuid.UUID, role string) (bool, error) {
	// FIXED: Query role from users table, not user_roles table
	var userRole string
	query := `SELECT role FROM users WHERE id = $1`
	err := tx.QueryRow(ctx, query, userID).Scan(&userRole)
	if err != nil {
		return false, fmt.Errorf("failed to check role: %w", err)
	}
	return userRole == role, nil
}

// ============================================================================
// FOLLOW COUNT OPERATIONS (Derived from user_follows)
// ============================================================================

func (r *userRepositoryImpl) GetFollowersCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_follows WHERE following_id = $1`
	err := tx.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get followers count: %w", err)
	}
	return count, nil
}

func (r *userRepositoryImpl) GetFollowingCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_follows WHERE follower_id = $1`
	err := tx.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get following count: %w", err)
	}
	return count, nil
}

// ============================================================================
// USERNAME OPERATIONS
// ============================================================================

func (r *userRepositoryImpl) GetUsername(ctx context.Context, tx db.Tx, userID uuid.UUID) (string, error) {
	var username sql.NullString
	err := tx.QueryRow(ctx, `
		SELECT username FROM user_profiles WHERE user_id = $1
	`, userID).Scan(&username)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get username: %w", err)
	}
	return username.String, nil
}

func (r *userRepositoryImpl) IsUsernameTaken(ctx context.Context, tx db.Tx, username string, excludeUserID uuid.UUID) (bool, error) {
	var existingUserID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT user_id FROM user_profiles WHERE LOWER(username) = $1
	`, username).Scan(&existingUserID)
	if err == nil {
		// Username exists, but check if it's the same user (for updates)
		if existingUserID != excludeUserID {
			return true, nil
		}
	}
	if err != nil && err != pgx.ErrNoRows {
		return false, fmt.Errorf("failed to check username: %w", err)
	}
	return false, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func joinFields(fields []string) string {
	result := ""
	for i, f := range fields {
		if i > 0 {
			result += ", "
		}
		result += f
	}
	return result
}

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// nullableStringValue maps a pointer to a bindable value: nil for empty
// (clear → SQL NULL) or the string itself. Used for optional text columns
// where an explicit "" means "clear this field".
func nullableStringValue(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

// sqlToNullString maps a pointer to a SQL nullable string. An empty-string
// pointer maps to NULL: an explicit "" is the canonical "clear this field"
// signal for optional text columns (avatar_url, cover_photo_url, bio, ...),
// so the persisted value is a true NULL rather than an empty string.
func sqlToNullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}
