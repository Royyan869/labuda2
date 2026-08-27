package application

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

func projectPublicProfileIdentity(
	publicInfo *userEntity.UserPublicInfo,
	lifecycle string,
) (publiccard.UserCard, bool) {
	if publicInfo == nil || publicInfo.UserID == uuid.Nil {
		return publiccard.UserCard{}, false
	}

	card := publiccard.UserCard{ID: publicInfo.UserID}
	username := strings.TrimSpace(publicInfo.Username)

	switch lifecycle {
	case string(viewercontext.PublicLifecycleStateRemoved):
		return publiccard.UserCard{}, false
	case string(viewercontext.PublicLifecycleStateActive):
		if username == "" {
			break
		}

		card.Username = publicInfo.Username
		if publicInfo.AvatarURL != nil && strings.TrimSpace(*publicInfo.AvatarURL) != "" {
			card.AvatarURL = publicInfo.AvatarURL
		}
		v := string(viewercontext.PublicLifecycleStateActive)
		card.Lifecycle = &v
		return card, true
	}

	card.Username = ""
	v := string(viewercontext.PublicLifecycleStateUnavailable)
	card.Lifecycle = &v
	return card, true
}
