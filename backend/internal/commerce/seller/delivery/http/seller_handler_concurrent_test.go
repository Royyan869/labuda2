//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/seller/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentOnboarding_StrictMode tests that concurrent onboarding requests
// are handled deterministically with row-level locking.
//
// STRICT MODE BEHAVIOR:
// - First request to complete validation wins
// - Subsequent requests see existing profile and return it (idempotency)
// - No race conditions, no duplicate profiles
func (s *SellerHandlerTestSuite) TestConcurrentOnboarding_StrictMode() {
	suite := s

	suite.Run("concurrent onboarding with different store names - first wins", func() {
		// Setup: Create a test user with all onboarding requirements
		userID := uuid.New()
		ctx := context.Background()

		err := suite.db.WithTx(ctx, func(tx db.Tx) error {
			// Create user
			now := time.Now()
			user := &userEntity.User{
				ID:              userID,
				FirebaseUID:     fmt.Sprintf("test-concurrent-%s", userID.String()),
				Email:           strPtr(fmt.Sprintf("concurrent-test-%s@example.com", userID.String())),
				AccountStatus:   "active",
				PhoneNumber:     strPtr("+1234567890"),
				EmailVerifiedAt: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := suite.userRepo.Create(ctx, tx, user); err != nil {
				return err
			}

			if err := suite.seedCompleteOnboardingFixture(ctx, tx, userID, "Store Seed"); err != nil {
				return err
			}

			return nil
		})
		require.NoError(suite.T(), err, "Failed to setup test user")
		suite.currentUserID = userID

		// Test: Launch 10 concurrent onboarding requests with different store names
		numRequests := 10
		storeNames := make([]string, numRequests)
		for i := 0; i < numRequests; i++ {
			storeNames[i] = fmt.Sprintf("Store-%d", i)
		}

		results := make([]OnboardingResponse, numRequests)
		errors := make([]error, numRequests)
		var wg sync.WaitGroup

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Create onboarding request
				req := OnboardingRequest{
					StoreName: storeNames[idx],
				}
				body, _ := json.Marshal(req)

				// Create HTTP request
				w := suite.performRequest("POST", "/api/v1/seller/onboarding", bytes.NewBuffer(body))

				// Parse response
				var resp response.Response
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					errors[idx] = err
					return
				}

				if w.Code != 200 {
					errors[idx] = fmt.Errorf("unexpected status code: %d, body: %s", w.Code, w.Body.String())
					return
				}

				// Parse onboarding response
				data, _ := json.Marshal(resp.Data)
				var onboardingResp OnboardingResponse
				if err := json.Unmarshal(data, &onboardingResp); err != nil {
					errors[idx] = err
					return
				}

				results[idx] = onboardingResp
			}(i)
		}

		wg.Wait()

		// Verify: All requests should succeed
		for i := 0; i < numRequests; i++ {
			assert.NoError(suite.T(), errors[i], "Request %d should succeed", i)
			assert.Equal(suite.T(), userID, results[i].UserID, "Request %d should return correct user ID", i)
		}

		// Verify: All requests should return the SAME profile ID (idempotency)
		profileIDs := make(map[uuid.UUID]int)
		for i := 0; i < numRequests; i++ {
			profileIDs[results[i].ProfileID]++
		}
		assert.Equal(suite.T(), 1, len(profileIDs), "All requests should return the same profile ID")

		// Verify: Only ONE profile should exist in database
		err = suite.db.WithTx(ctx, func(tx db.Tx) error {
			profile, err := suite.sellerRepo.GetByUserID(ctx, tx, userID)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), profile, "Profile should exist")

			// Verify profile matches one of the requests (we don't know which one won the race)
			found := false
			for i := 0; i < numRequests; i++ {
				if storeNames[i] == profile.StoreName {
					found = true
					break
				}
			}
			assert.True(suite.T(), found, "Profile store name should match one of the requests")

			return nil
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("concurrent onboarding with missing requirements - deterministic rejection", func() {
		// Setup: Create a test user WITHOUT phone number (incomplete onboarding)
		userID := uuid.New()
		ctx := context.Background()

		err := suite.db.WithTx(ctx, func(tx db.Tx) error {
			// Create user without phone number
			now := time.Now()
			user := &userEntity.User{
				ID:              userID,
				FirebaseUID:     fmt.Sprintf("test-incomplete-%s", userID.String()),
				Email:           strPtr(fmt.Sprintf("incomplete-test-%s@example.com", userID.String())),
				AccountStatus:   "active",
				PhoneNumber:     nil, // Missing phone number
				EmailVerifiedAt: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := suite.userRepo.Create(ctx, tx, user); err != nil {
				return err
			}

			username := fmt.Sprintf("incomplete-%s", userID.String()[:8])
			bio := "Test seller bio"
			if _, err := suite.userRepo.UpdateProfile(ctx, tx, userID, &userEntity.UpdateProfileInput{
				Username: &username,
				Bio:      &bio,
			}); err != nil {
				return err
			}

			return nil
		})
		require.NoError(suite.T(), err, "Failed to setup test user")
		suite.currentUserID = userID

		// Test: Launch 10 concurrent onboarding requests
		numRequests := 10
		results := make([]int, numRequests) // status codes
		var wg sync.WaitGroup

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Create onboarding request
				req := OnboardingRequest{
					StoreName: fmt.Sprintf("Store-%d", idx),
				}
				body, _ := json.Marshal(req)

				// Create HTTP request
				w := suite.performRequest("POST", "/api/v1/seller/onboarding", bytes.NewBuffer(body))

				results[idx] = w.Code
			}(i)
		}

		wg.Wait()

		// Verify: All requests should fail with 400 (missing requirements)
		for i := 0; i < numRequests; i++ {
			assert.Equal(suite.T(), 400, results[i], "Request %d should fail with 400", i)
		}

		// Verify: NO profile should exist in database
		err = suite.db.WithTx(ctx, func(tx db.Tx) error {
			profile, err := suite.sellerRepo.GetByUserID(ctx, tx, userID)
			require.NoError(suite.T(), err)
			assert.Nil(suite.T(), profile, "No profile should exist for incomplete onboarding")
			return nil
		})
		require.NoError(suite.T(), err)
	})

	suite.Run("concurrent onboarding with existing profile - idempotent response", func() {
		// Setup: Create a test user with existing seller profile
		userID := uuid.New()
		ctx := context.Background()
		existingStoreName := "Original Store"

		err := suite.db.WithTx(ctx, func(tx db.Tx) error {
			// Create user
			now := time.Now()
			user := &userEntity.User{
				ID:              userID,
				FirebaseUID:     fmt.Sprintf("test-existing-%s", userID.String()),
				Email:           strPtr(fmt.Sprintf("existing-test-%s@example.com", userID.String())),
				AccountStatus:   "active",
				PhoneNumber:     strPtr("+1234567890"),
				EmailVerifiedAt: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := suite.userRepo.Create(ctx, tx, user); err != nil {
				return err
			}

			if err := suite.seedCompleteOnboardingFixture(ctx, tx, userID, existingStoreName); err != nil {
				return err
			}

			// Create seller profile
			sellerProfile := &entity.SellerProfile{
				ID:        uuid.New(),
				UserID:    userID,
				StoreName: existingStoreName,
				Tier:      entity.TierBasic,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := suite.sellerRepo.InsertProfileTx(ctx, tx, sellerProfile); err != nil {
				return err
			}

			return nil
		})
		require.NoError(suite.T(), err, "Failed to setup test user")
		suite.currentUserID = userID

		// Test: Launch 10 concurrent onboarding requests with different store names
		numRequests := 10
		results := make([]OnboardingResponse, numRequests)
		errors := make([]error, numRequests)
		var wg sync.WaitGroup

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Create onboarding request with DIFFERENT store name
				req := OnboardingRequest{
					StoreName: fmt.Sprintf("New-Store-%d", idx),
				}
				body, _ := json.Marshal(req)

				// Create HTTP request
				w := suite.performRequest("POST", "/api/v1/seller/onboarding", bytes.NewBuffer(body))

				// Parse response
				var resp response.Response
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				if err != nil {
					errors[idx] = err
					return
				}

				if w.Code != 200 {
					errors[idx] = fmt.Errorf("unexpected status code: %d", w.Code)
					return
				}

				// Parse onboarding response
				data, _ := json.Marshal(resp.Data)
				var onboardingResp OnboardingResponse
				if err := json.Unmarshal(data, &onboardingResp); err != nil {
					errors[idx] = err
					return
				}

				results[idx] = onboardingResp
			}(i)
		}

		wg.Wait()

		// Verify: All requests should succeed
		for i := 0; i < numRequests; i++ {
			assert.NoError(suite.T(), errors[i], "Request %d should succeed", i)
		}

		// Verify: All requests should return the EXISTING profile (idempotency)
		// Store name should be the ORIGINAL, not the new ones
		for i := 0; i < numRequests; i++ {
			assert.Equal(suite.T(), existingStoreName, results[i].StoreName,
				"Request %d should return original store name", i)
		}

		// Verify: All profile IDs should match
		profileIDs := make(map[uuid.UUID]int)
		for i := 0; i < numRequests; i++ {
			profileIDs[results[i].ProfileID]++
		}
		assert.Equal(suite.T(), 1, len(profileIDs), "All requests should return the same profile ID")

		// Verify: Store name in database should remain unchanged
		err = suite.db.WithTx(ctx, func(tx db.Tx) error {
			profile, err := suite.sellerRepo.GetByUserID(ctx, tx, userID)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), profile)
			assert.Equal(suite.T(), existingStoreName, profile.StoreName,
				"Store name in database should remain unchanged")
			return nil
		})
		require.NoError(suite.T(), err)
	})
}

// Helper function
func strPtr(s string) *string {
	return &s
}


