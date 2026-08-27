package application

import (
	"testing"

	"github.com/google/uuid"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
)

func TestUserProfileService_entityToProfileDTO_UsesProfileCounts(t *testing.T) {
	svc := &UserProfileService{}
	username := "alice"
	profile := &userEntity.UserProfile{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Username:       &username,
		FollowersCount: 19,
		FollowingCount: 8,
	}

	dto := svc.entityToProfileDTO(profile)

	if dto.FollowersCount != 19 {
		t.Fatalf("followers_count = %d; want 19", dto.FollowersCount)
	}
	if dto.FollowingCount != 8 {
		t.Fatalf("following_count = %d; want 8", dto.FollowingCount)
	}
}


