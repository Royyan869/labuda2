package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
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

	feedMediaURLs := make([]string, 0, len(item.Media))
	for _, m := range item.Media {
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
		"media":      item.Media,
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
