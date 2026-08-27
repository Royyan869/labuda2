package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	userRepo "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	userRepository "github.com/labuda/backend/internal/identity/user/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
)

type interactionAuthorityState struct {
	Found         bool
	EmailVerified bool
}

type interactionAuthorityService interface {
	GetInteractionAuthority(ctx context.Context, userID uuid.UUID) (interactionAuthorityState, error)
}

type dbInteractionAuthorityService struct {
	db   *db.DB
	repo userRepository.UserRepository
}

func RequireInteractionAuthority(database *db.DB) gin.HandlerFunc {
	return requireInteractionAuthorityWithService(&dbInteractionAuthorityService{
		db:   database,
		repo: userRepo.NewUserRepository(database),
	})
}

func RequireTransactionAuthority(database *db.DB) gin.HandlerFunc {
	return RequireInteractionAuthority(database)
}

func (s *dbInteractionAuthorityService) GetInteractionAuthority(ctx context.Context, userID uuid.UUID) (interactionAuthorityState, error) {
	var (
		state interactionAuthorityState
		user  *userEntity.User
	)

	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		user, err = s.repo.GetByID(ctx, tx, userID)
		return err
	})
	if err != nil {
		return interactionAuthorityState{}, err
	}
	if user == nil {
		return interactionAuthorityState{Found: false}, nil
	}

	state = interactionAuthorityState{
		Found:         true,
		EmailVerified: user.EmailVerified,
	}
	return state, nil
}

func requireInteractionAuthorityWithService(service interactionAuthorityService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			response.Unauthorized(c, "Authentication required")
			c.Abort()
			return
		}

		state, err := service.GetInteractionAuthority(c.Request.Context(), userID)
		if err != nil {
			response.InternalServerError(c, "Failed to verify interaction authority")
			c.Abort()
			return
		}
		if !state.Found {
			response.Error(c, http.StatusUnauthorized, response.ErrCodeUserNotFound, "User not found")
			c.Abort()
			return
		}

		emailVerified := state.EmailVerified
		if actor := GetActorFromContext(c); actor != nil {
			emailVerified = actor.EmailVerified
		}
		if !emailVerified {
			response.Error(c, http.StatusForbidden, "EMAIL_VERIFICATION_REQUIRED", "Email verification required")
			c.Abort()
			return
		}

		c.Next()
	}
}


