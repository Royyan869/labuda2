package shared

import (
	"strings"
	"time"

	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// Canonical typed media wire block for commerce detail surfaces.
//
// OWNER CANONICAL: ForSale and Auction are siblings under the same Product
// authority, so both emit the IDENTICAL `media` shape (id, type, url,
// position, thumbnail_url, width, height, duration, created_at). Mobile
// renders both surfaces with one parser. Product.MediaURLs is the sole
// content authority; the typed block is a projection of it, never a
// competing source.

// ResolveReadableMediaReference projects a stored media reference onto the
// canonical CloudFront read URL. Fail-open per item: unresolvable references
// are emitted trimmed and unchanged so a persisted reference is never erased.
func ResolveReadableMediaReference(value string) string {
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

// ResolveReadableMediaReferences projects a stored reference list onto the
// canonical CloudFront read URLs in order. Fail-open per item; never nil so
// the wire shape stays stable.
func ResolveReadableMediaReferences(references []string) []string {
	if len(references) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(references))
	for _, ref := range references {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		if resolved, err := mediaresolve.ResolveMediaReadURL(trimmed); err == nil {
			out = append(out, resolved)
		} else {
			out = append(out, trimmed)
		}
	}
	return out
}

// MediaWireItems renders the typed media block from Product media
// references. createdAt anchors deterministic media IDs. Never nil — an
// empty list is emitted so the wire shape stays stable.
func MediaWireItems(references []string, createdAt time.Time) []map[string]interface{} {
	if len(references) == 0 {
		return []map[string]interface{}{}
	}
	items, err := mediaentity.NewListFromReferences(references, createdAt)
	if err != nil {
		return []map[string]interface{}{}
	}
	rendered := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		rendered = append(rendered, MediaWireItem(item))
	}
	return rendered
}

// MediaWireItem renders one typed media item.
func MediaWireItem(item mediaentity.Media) map[string]interface{} {
	thumbnail := ""
	if item.ThumbnailURL != nil {
		thumbnail = *item.ThumbnailURL
	}
	return map[string]interface{}{
		"id":            item.ID.String(),
		"type":          item.Type.String(),
		"url":           ResolveReadableMediaReference(item.URL),
		"position":      item.Position,
		"thumbnail_url": ResolveReadableMediaReference(thumbnail),
		"width":         item.Width,
		"height":        item.Height,
		"duration":      item.Duration,
		"created_at":    item.CreatedAt.Format(time.RFC3339),
	}
}
