package http

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

func feedItemToResponseCanonical(item *feedentity.FeedItem, lifecycleOverrides map[uuid.UUID]string, origAuthorLifecycles map[uuid.UUID]string) map[string]interface{} {
	resp, _ := feedItemToResponseCanonicalWithProjection(item, lifecycleOverrides, origAuthorLifecycles, nil)
	return resp
}

func feedItemToResponseCanonicalStrict(
	item *feedentity.FeedItem,
	lifecycleOverrides map[uuid.UUID]string,
	origAuthorLifecycles map[uuid.UUID]string,
	projection *contentApp.ContentResourceProjection,
) (map[string]interface{}, error) {
	resp, err := feedItemToResponseCanonicalWithProjection(item, lifecycleOverrides, origAuthorLifecycles, projection)
	if resp == nil {
		resp = map[string]interface{}{}
	}
	return resp, err
}

func feedItemToResponseCanonicalWithProjection(
	item *feedentity.FeedItem,
	lifecycleOverrides map[uuid.UUID]string,
	origAuthorLifecycles map[uuid.UUID]string,
	projection *contentApp.ContentResourceProjection,
) (map[string]interface{}, error) {
	if item == nil {
		return map[string]interface{}{}, nil
	}

	cardLifecycle := contententity.PublicLifecycleFromString(item.Status)
	if lifecycleOverrides != nil {
		if v, ok := lifecycleOverrides[item.ID]; ok && v != "" {
			cardLifecycle = v
		}
	}

	attribution := buildFeedAttributionContext(item)
	authorCard := buildFeedAuthorCard(item, attribution)

	var captionPtr *string
	if item.Caption != nil && *item.Caption != "" {
		c := *item.Caption
		captionPtr = &c
	}

	// MEDIA READ RESOLUTION: the flat `media` array is the surface the mobile
	// feed card renders, so both that array and the canonical card's media refs
	// are projected through the shared mediaresolve authority, from the same
	// resolved values (they can never disagree). Fail-open: an unresolvable
	// reference is emitted unchanged, never erased.
	feedMedia := resolveReadableFeedMedia(item.Media)
	feedMediaURLs := make([]string, 0, len(feedMedia))
	for _, m := range feedMedia {
		if m.URL != "" {
			feedMediaURLs = append(feedMediaURLs, m.URL)
		}
	}

	resp := map[string]interface{}{
		"id":         item.ID.String(),
		"author_id":  item.AuthorID.String(),
		"type":       item.Type,
		"status":     contententity.PublicLifecycleFromString(item.Status),
		"lifecycle":  cardLifecycle,
		"body":       item.Body,
		"created_at": item.CreatedAt.Format(time.RFC3339),
		"updated_at": item.UpdatedAt.Format(time.RFC3339),
		"media":      feedMedia,
		"author":     authorCard,
		"card": publiccard.NewContentCard(
			item.ID,
			item.Type,
			captionPtr,
			feedMediaURLs,
			cardLifecycle,
			item.CreatedAt,
			&authorCard,
		),
	}

	if item.Caption != nil {
		resp["caption"] = *item.Caption
	}
	if item.AuthorUsername != nil {
		resp["author_username"] = *item.AuthorUsername
	}
	if item.AuthorAvatar != nil {
		resp["author_avatar"] = *item.AuthorAvatar
	}
	if item.AuthorCity != nil || item.AuthorProvince != nil {
		city := ""
		province := ""
		if item.AuthorCity != nil {
			city = *item.AuthorCity
		}
		if item.AuthorProvince != nil {
			province = *item.AuthorProvince
		}
		resp["author_city"] = city
		resp["author_province"] = province
	}
	if hasNonEmptyValue(item.City) || hasNonEmptyValue(item.Province) {
		resp["location"] = map[string]interface{}{
			"city":     derefOrEmpty(item.City),
			"province": derefOrEmpty(item.Province),
		}
	}

	if attribution.OriginalAuthorID != nil {
		resp["original_author_id"] = attribution.OriginalAuthorID.String()
		if origAuthorLifecycles != nil {
			if lc, ok := origAuthorLifecycles[*attribution.OriginalAuthorID]; ok && lc != "" {
				resp["original_author_lifecycle"] = lc
			}
		}
	}

	if projection != nil {
		resp["resource_projection"] = projection
	}

	return resp, nil
}

func buildFeedAttributionContext(item *feedentity.FeedItem) contentApp.ShareAttributionContext {
	if item == nil {
		return contentApp.ShareAttributionContext{}
	}

	ctx := contentApp.ShareAttributionContext{
		ActorID:         item.AuthorID,
		DisplayName:     feedDisplayName(item),
		Username:        feedUsername(item),
		LifecycleState:  item.AuthorLifecycle,
		VisibilityState: "public",
	}

	if item.IsHidden {
		ctx.VisibilityState = "private"
	}

	if item.OriginalAuthorID != nil && *item.OriginalAuthorID != uuid.Nil {
		original := *item.OriginalAuthorID
		ctx.OriginalAuthorID = &original
		ctx.TargetOwnerID = &original
	} else {
		owner := item.AuthorID
		ctx.TargetOwnerID = &owner
	}

	return ctx
}

func buildFeedSnapshotContext(item *feedentity.FeedItem) contentApp.ShareSnapshotContext {
	return contentApp.ShareSnapshotContext{}
}

func buildFeedAuthorCard(item *feedentity.FeedItem, attribution contentApp.ShareAttributionContext) publiccard.UserCard {
	username := attribution.Username
	if username == "" && item != nil && item.AuthorUsername != nil {
		username = *item.AuthorUsername
	}
	return publiccard.NewWithLifecycle(
		item.AuthorID,
		username,
		item.AuthorAvatar,
		attribution.LifecycleState,
	)
}

// resolveReadableFeedMediaReference projects a persisted feed media reference
// (content_media.media_url, surfaced through the feed projection) onto the
// canonical readable URL using the shared mediaresolve authority.
//
// Fail-open: empty stays empty and an unresolvable reference is returned
// trimmed and unchanged, so a persisted reference is never erased by a
// resolution failure.
func resolveReadableFeedMediaReference(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(trimmed)
	if err != nil {
		return trimmed
	}
	return resolved
}

// resolveReadableFeedMedia projects the feed media projection onto readable
// references. The additive canonical fields (Kind / Width / Height) are
// preserved verbatim — only URL is resolved.
func resolveReadableFeedMedia(in []feedentity.FeedMedia) []feedentity.FeedMedia {
	out := make([]feedentity.FeedMedia, 0, len(in))
	for _, m := range in {
		resolved := m
		resolved.URL = resolveReadableFeedMediaReference(m.URL)
		out = append(out, resolved)
	}
	return out
}

func feedDisplayName(item *feedentity.FeedItem) string {
	if item == nil || item.AuthorUsername == nil {
		return ""
	}
	return *item.AuthorUsername
}

func feedUsername(item *feedentity.FeedItem) string {
	if item == nil || item.AuthorUsername == nil {
		return ""
	}
	return *item.AuthorUsername
}
