package application

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// buildPublicUserCard builds a public-safe user card from raw truth.
func buildPublicUserCard(id uuid.UUID, username string, avatarURL *string, accountStatus string, deleted bool) publiccard.UserCard {
	lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, deleted))
	card := publiccard.UserCard{ID: id, Username: username, AvatarURL: avatarURL}
	if lifecycle != "" {
		card.Lifecycle = &lifecycle
	}
	return card
}

func resolveMediaRefs(urls []string) []mediaref.MediaRef {
	refs := make([]mediaref.MediaRef, 0, len(urls))
	for _, raw := range urls {
		if trimmed := resolveMediaReference(raw); trimmed != "" {
			refs = append(refs, mediaref.MediaRef{URL: trimmed})
		}
	}
	return refs
}

func firstResolvedMediaURL(urls []string) *string {
	for _, raw := range urls {
		if trimmed := resolveMediaReference(raw); trimmed != "" {
			v := trimmed
			return &v
		}
	}
	return nil
}

func resolveMediaReference(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		if resolved, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
			return resolved
		}
		return trimmed
	}
	return ""
}
