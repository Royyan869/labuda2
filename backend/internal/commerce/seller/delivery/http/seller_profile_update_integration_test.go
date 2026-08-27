//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *SellerHandlerTestSuite) queryPersistedStoreImageUpdatedAt(
	ctx context.Context,
	userID uuid.UUID,
) *time.Time {
	t := s.T()
	t.Helper()

	var updatedAt *time.Time
	err := s.testDB.Pool().QueryRow(
		ctx,
		`SELECT store_image_updated_at FROM seller_profiles WHERE user_id = $1`,
		userID,
	).Scan(&updatedAt)
	require.NoError(t, err)
	return updatedAt
}

func (s *SellerHandlerTestSuite) seedSellerUpdateFixture(
	ctx context.Context,
	userID uuid.UUID,
	storeName string,
	storeImageURL *string,
	avatarURL *string,
	coverURL *string,
) error {
	if err := s.seedSellerProfileFixture(ctx, userID, storeName, storeImageURL); err != nil {
		return err
	}

	return s.db.WithTx(ctx, func(tx db.Tx) error {
		_, err := s.userRepo.UpdateProfile(ctx, tx, userID, &entity.UpdateProfileInput{
			AvatarURL:     avatarURL,
			CoverPhotoURL: coverURL,
		})
		return err
	})
}

func (s *SellerHandlerTestSuite) TestSellerProfileUpdate_CanonicalMutationMatrix() {
	ctx := context.Background()

	testCases := []struct {
		name             string
		body             map[string]any
		initialStoreName string
		initialImage     *string
		wantStoreName    string
		wantStoreImage   *string
		wantImageBump    bool
	}{
		{
			name:             "store name-only update succeeds",
			body:             map[string]any{"store_name": "Seller Prime"},
			initialStoreName: "Seller Farm",
			initialImage:     strPtr("https://example.com/store-old.jpg"),
			wantStoreName:    "Seller Prime",
			wantStoreImage:   strPtr("https://example.com/store-old.jpg"),
			wantImageBump:    false,
		},
		{
			name:             "store image-only update succeeds",
			body:             map[string]any{"store_image_url": "https://example.com/store-new.jpg"},
			initialStoreName: "Seller Farm",
			initialImage:     strPtr("https://example.com/store-old.jpg"),
			wantStoreName:    "Seller Farm",
			wantStoreImage:   strPtr("https://example.com/store-new.jpg"),
			wantImageBump:    true,
		},
		{
			name: "store name and store image update atomically",
			body: map[string]any{
				"store_name":      "Seller Prime",
				"store_image_url": "https://example.com/store-new.jpg",
			},
			initialStoreName: "Seller Farm",
			initialImage:     strPtr("https://example.com/store-old.jpg"),
			wantStoreName:    "Seller Prime",
			wantStoreImage:   strPtr("https://example.com/store-new.jpg"),
			wantImageBump:    true,
		},
		{
			name:             "store image removal yields null",
			body:             map[string]any{"store_image_url": ""},
			initialStoreName: "Seller Farm",
			initialImage:     strPtr("https://example.com/store-old.jpg"),
			wantStoreName:    "Seller Farm",
			wantStoreImage:   nil,
			wantImageBump:    true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			userID := uuid.New()
			avatar := strPtr("https://example.com/avatar-old.jpg")
			cover := strPtr("https://example.com/cover-old.jpg")
			require.NoError(
				s.T(),
				s.seedSellerUpdateFixture(ctx, userID, tc.initialStoreName, tc.initialImage, avatar, cover),
			)
			beforeStoreImageUpdatedAt := s.queryPersistedStoreImageUpdatedAt(ctx, userID)
			require.NotNil(s.T(), beforeStoreImageUpdatedAt)

			payload, err := json.Marshal(tc.body)
			require.NoError(s.T(), err)

			w := s.performSellerProfileRequest(userID, bytes.NewReader(payload))
			require.Equal(s.T(), http.StatusOK, w.Code, w.Body.String())

			var resp struct {
				Data SellerProfileResponse `json:"data"`
			}
			require.NoError(s.T(), json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(s.T(), userID, resp.Data.UserID)
			assert.Equal(s.T(), tc.wantStoreName, resp.Data.StoreName)
			assert.Equal(s.T(), tc.wantStoreImage, resp.Data.StoreImageURL)

			err = s.db.WithTx(ctx, func(tx db.Tx) error {
				profile, err := s.sellerRepo.GetByUserID(ctx, tx, userID)
				require.NoError(s.T(), err)
				require.NotNil(s.T(), profile)

				assert.Equal(s.T(), tc.wantStoreName, profile.StoreName)
				assert.Equal(s.T(), tc.wantStoreImage, profile.StoreImageURL)
				afterStoreImageUpdatedAt := profile.StoreImageUpdatedAt
				require.NotNil(s.T(), afterStoreImageUpdatedAt)
				if tc.wantImageBump {
					assert.False(
						s.T(),
						beforeStoreImageUpdatedAt.Equal(*afterStoreImageUpdatedAt),
						"store_image_updated_at should bump for image mutations",
					)
				} else {
					assert.True(
						s.T(),
						beforeStoreImageUpdatedAt.Equal(*afterStoreImageUpdatedAt),
						"store_image_updated_at should remain stable for this mutation",
					)
				}

				myProfile, err := s.userRepo.GetMyProfile(ctx, tx, userID)
				require.NoError(s.T(), err)
				require.NotNil(s.T(), myProfile)
				require.NotNil(s.T(), myProfile.Profile)
				assert.Equal(s.T(), avatar, myProfile.Profile.AvatarURL)
				assert.Equal(s.T(), cover, myProfile.Profile.CoverPhotoURL)
				return nil
			})
			require.NoError(s.T(), err)
		})
	}
}

func (s *SellerHandlerTestSuite) TestSellerProfileUpdate_InvalidStoreImageRejectedWithoutBumpingAuthority() {
	ctx := context.Background()
	userID := uuid.New()
	avatar := strPtr("https://example.com/avatar-old.jpg")
	cover := strPtr("https://example.com/cover-old.jpg")
	require.NoError(
		s.T(),
		s.seedSellerUpdateFixture(ctx, userID, "Seller Farm", strPtr("https://example.com/store-old.jpg"), avatar, cover),
	)

	beforeStoreImageUpdatedAt := s.queryPersistedStoreImageUpdatedAt(ctx, userID)
	require.NotNil(s.T(), beforeStoreImageUpdatedAt)

	payload, err := json.Marshal(map[string]any{
		"store_image_url": "../not-allowed.jpg",
	})
	require.NoError(s.T(), err)

	w := s.performSellerProfileRequest(userID, bytes.NewReader(payload))
	require.Equal(s.T(), http.StatusBadRequest, w.Code, w.Body.String())

	afterStoreImageUpdatedAt := s.queryPersistedStoreImageUpdatedAt(ctx, userID)
	require.NotNil(s.T(), afterStoreImageUpdatedAt)
	assert.True(
		s.T(),
		beforeStoreImageUpdatedAt.Equal(*afterStoreImageUpdatedAt),
		"store_image_updated_at should not change when validation rejects the request",
	)
}

func (s *SellerHandlerTestSuite) TestSellerProfileUpdate_AuthorityAndIdentityGuards() {
	ctx := context.Background()

	s.Run("cross-user update leaves the other seller untouched", func() {
		userA := uuid.New()
		userB := uuid.New()
		avatarA := strPtr("https://example.com/avatar-a.jpg")
		coverA := strPtr("https://example.com/cover-a.jpg")
		avatarB := strPtr("https://example.com/avatar-b.jpg")
		coverB := strPtr("https://example.com/cover-b.jpg")

		require.NoError(
			s.T(),
			s.seedSellerUpdateFixture(ctx, userA, "Store A", strPtr("https://example.com/store-a.jpg"), avatarA, coverA),
		)
		require.NoError(
			s.T(),
			s.seedSellerUpdateFixture(ctx, userB, "Store B", strPtr("https://example.com/store-b.jpg"), avatarB, coverB),
		)

		payload, err := json.Marshal(map[string]any{
			"store_name":      "Store A Prime",
			"store_image_url": "https://example.com/store-a-new.jpg",
		})
		require.NoError(s.T(), err)

		w := s.performSellerProfileRequest(userA, bytes.NewReader(payload))
		require.Equal(s.T(), http.StatusOK, w.Code, w.Body.String())

		err = s.db.WithTx(ctx, func(tx db.Tx) error {
			profileA, err := s.sellerRepo.GetByUserID(ctx, tx, userA)
			require.NoError(s.T(), err)
			require.NotNil(s.T(), profileA)
			assert.Equal(s.T(), "Store A Prime", profileA.StoreName)

			profileB, err := s.sellerRepo.GetByUserID(ctx, tx, userB)
			require.NoError(s.T(), err)
			require.NotNil(s.T(), profileB)
			assert.Equal(s.T(), "Store B", profileB.StoreName)
			assert.Equal(
				s.T(),
				strPtr("https://example.com/store-b.jpg"),
				profileB.StoreImageURL,
			)

			myProfileB, err := s.userRepo.GetMyProfile(ctx, tx, userB)
			require.NoError(s.T(), err)
			require.NotNil(s.T(), myProfileB)
			require.NotNil(s.T(), myProfileB.Profile)
			assert.Equal(s.T(), avatarB, myProfileB.Profile.AvatarURL)
			assert.Equal(s.T(), coverB, myProfileB.Profile.CoverPhotoURL)
			return nil
		})
		require.NoError(s.T(), err)
	})

	s.Run("non-seller update is rejected by authority middleware", func() {
		userID := uuid.New()
		email := "noseller+" + userID.String()[:8] + "@example.com"
		now := time.Now()
		err := s.db.WithTx(ctx, func(tx db.Tx) error {
			return s.userRepo.Create(ctx, tx, &entity.User{
				ID:              userID,
				FirebaseUID:     "test-" + userID.String(),
				Email:           &email,
				AccountStatus:   "active",
				EmailVerifiedAt: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			})
		})
		require.NoError(s.T(), err)

		payload, err := json.Marshal(map[string]any{"store_name": "Should Not Save"})
		require.NoError(s.T(), err)

		w := s.performSellerProfileRequest(userID, bytes.NewReader(payload))
		require.Equal(s.T(), http.StatusForbidden, w.Code, w.Body.String())
	})
}
