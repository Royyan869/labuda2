package http

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	identityusername "github.com/labuda/backend/internal/identity/username"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

type feedAuthorProjection struct {
	AuthorCard publiccard.UserCard
	Username   string
	AvatarURL  *string
}

var feedAnonymousUsernamePattern = regexp.MustCompile(`^user_[0-9a-f]{8}$`)

func buildFeedCurrentAuthorProjection(item *feedentity.FeedItem) (feedAuthorProjection, error) {
	if item == nil {
		return feedAuthorProjection{}, nil
	}
	if item.AuthorID == uuid.Nil {
		return feedAuthorProjection{}, fmt.Errorf("feed current author id is nil")
	}

	username, ok := canonicalFeedAuthorUsername(item.AuthorUsername)
	avatarURL := canonicalFeedAuthorAvatar(item.AuthorAvatar)
	if !ok {
		unavailable := "unavailable"
		return feedAuthorProjection{
			AuthorCard: publiccard.UserCard{
				ID:        item.AuthorID,
				Username:  "",
				Lifecycle: &unavailable,
			},
			Username:  "",
			AvatarURL: nil,
		}, nil
	}

	return feedAuthorProjection{
		AuthorCard: publiccard.NewWithLifecycle(
			item.AuthorID,
			username,
			avatarURL,
			item.AuthorLifecycle,
		),
		Username:  username,
		AvatarURL: avatarURL,
	}, nil
}

func canonicalFeedAuthorUsername(raw *string) (string, bool) {
	if raw == nil {
		return "", false
	}

	username := identityusername.Normalize(*raw)
	if username == "" {
		return "", false
	}
	if err := identityusername.ValidateFormat(username); err != nil {
		return "", false
	}
	if identityusername.IsReserved(username) {
		return "", false
	}
	if feedAnonymousUsernamePattern.MatchString(username) {
		return "", false
	}

	return username, true
}

func canonicalFeedAuthorAvatar(raw *string) *string {
	if raw == nil {
		return nil
	}

	avatarURL := strings.TrimSpace(*raw)
	if avatarURL == "" {
		return nil
	}
	if resolved, err := mediaresolve.ResolveMediaReadURL(avatarURL); err == nil {
		return &resolved
	}
	return &avatarURL
}
