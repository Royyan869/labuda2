package shared

import (
	"net/url"
	"strings"
	"time"

	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/platform/mediaresolve"
)

// Canonical typed media wire block for commerce detail surfaces.
//
// OWNER CANONICAL: ForSale and Auction are siblings under the same Product
// authority, so both emit the IDENTICAL `media` shape (id, type, url,
// position, thumbnail_url, blurhash, width, height, duration, status,
// created_at). Mobile renders both surfaces with one parser.
// Product.MediaURLs is the sole content authority; the typed block is a
// projection of it, never a competing source.

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

// ThumbnailVariantKey derives the Lambda thumbnail variant key from a stored
// media reference using the SAME rule as aws-lambda/image-processor
// getVariantKey(key, 'thumbnail'): {dir}/thumbnail/{file}.
//
// Absolute bucket/CDN URLs are reduced to their object path first;
// external absolute URLs pass through untouched (no variant exists for
// content the pipeline never processed). Empty stays empty.
func ThumbnailVariantKey(reference string) string {
	return VariantKey(reference, ListVariantDir)
}

// ListVariantDir is the Lambda variant served on every list surface
// (feed, marketplace cards, search rows): 600px q85 — sharp on phones,
// ~30x lighter than the original. The 150px thumbnail/ variant is too small
// for cards (it rendered blurry); detail/viewer always use the original.
const ListVariantDir = "medium"

// VariantKey derives a Lambda variant key from a stored media reference
// using the SAME rule as aws-lambda/image-processor getVariantKey:
// {dir}/{variant}/{file}.
func VariantKey(reference, variant string) string {
	trimmed := strings.TrimSpace(reference)
	if trimmed == "" {
		return ""
	}
	path := trimmed
	if u, err := url.Parse(trimmed); err == nil && u.IsAbs() {
		if u.Path == "" || u.Path == "/" {
			return trimmed
		}
		path = strings.Trim(u.Path, "/")
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	// Idempotent: a reference that already names a pipeline variant is
	// returned unchanged so the rule can never stack (medium/medium).
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "thumbnail", "medium", "large", "webp":
			return trimmed
		}
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return variant + "/" + path
	}
	return path[:idx] + "/" + variant + "/" + path[idx+1:]
}

// ResolveReadableThumbnailURL projects a stored media reference onto the
// canonical CloudFront URL of a Lambda variant (ListVariantDir by default).
// Fail-open: unresolvable references fall back to the trimmed raw reference,
// never erased.
func ResolveReadableThumbnailURL(reference string) string {
	return ResolveReadableVariantURL(reference, ListVariantDir)
}

// ResolveReadableVariantURL projects a stored media reference onto the
// canonical CloudFront URL of the named Lambda variant.
func ResolveReadableVariantURL(reference, variant string) string {
	key := VariantKey(reference, variant)
	if key == "" {
		return ""
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(key)
	if err != nil {
		return strings.TrimSpace(reference)
	}
	return resolved
}

// FirstImageRef returns the first stored reference that is an image.
// Videos carry a poster frame instead of an image variant (see
// VideoPosterKey), so list thumbnails derive only from images; empty when
// the list holds video alone.
func FirstImageRef(references []string) string {
	for _, ref := range references {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		if mediaentity.InferMediaType(trimmed) == mediaentity.MediaTypeVideo {
			continue
		}
		return trimmed
	}
	return ""
}

// VideoPosterKey derives the poster-frame key for a stored video reference:
// {dir}/{name}_poster.jpg next to the mp4. Produced by the remux Lambda
// (faststart + middle frame) at upload time; the mediaupload contract
// already authorizes the _poster.jpg suffix.
func VideoPosterKey(reference string) string {
	trimmed := strings.TrimSpace(reference)
	if trimmed == "" {
		return ""
	}
	path := trimmed
	if u, err := url.Parse(trimmed); err == nil && u.IsAbs() {
		if u.Path == "" || u.Path == "/" {
			return trimmed
		}
		path = strings.Trim(u.Path, "/")
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	// Idempotent: an existing poster path is returned unchanged.
	if strings.HasSuffix(path, "_poster.jpg") {
		return trimmed
	}
	if idx := strings.LastIndex(path, "."); idx > strings.LastIndex(path, "/") {
		path = path[:idx]
	}
	return path + "_poster.jpg"
}

// ResolveReadablePosterURL projects a stored video reference onto the
// canonical CloudFront URL of its poster frame. Fail-open like the rest.
func ResolveReadablePosterURL(reference string) string {
	key := VideoPosterKey(reference)
	if key == "" {
		return ""
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(key)
	if err != nil {
		return strings.TrimSpace(reference)
	}
	return resolved
}

// ResolveReadableCardThumbnailURL resolves a product media list's display
// thumbnail: the Lambda image variant of the first image, else the poster
// frame of the first video, else "". One rule for every card/row/list slot.
func ResolveReadableCardThumbnailURL(media []productentity.ProductMedia) string {
	if first := FirstImageRef(productentity.URLs(media)); first != "" {
		return ResolveReadableThumbnailURL(first)
	}
	for _, m := range media {
		trimmed := strings.TrimSpace(m.URL)
		if trimmed == "" {
			continue
		}
		if mediaentity.InferMediaType(trimmed) == mediaentity.MediaTypeVideo {
			return ResolveReadablePosterURL(trimmed)
		}
	}
	return ""
}

// MediaWireItems renders the typed media block from Product media.
// createdAt anchors deterministic media IDs. Never nil — an
// empty list is emitted so the wire shape stays stable.
//
// Persisted per-item metadata (thumbnail, dimensions, duration) is
// overlaid onto the inferred items by URL: the wire emits what is stored,
// and MediaWireItem only derives the thumbnail/poster fallback when the
// slot carries no explicit thumbnail.
func MediaWireItems(media []productentity.ProductMedia, createdAt time.Time) []map[string]interface{} {
	if len(media) == 0 {
		return []map[string]interface{}{}
	}
	items, err := mediaentity.NewListFromReferences(productentity.URLs(media), createdAt)
	if err != nil {
		return []map[string]interface{}{}
	}
	byURL := make(map[string]productentity.ProductMedia, len(media))
	hashes := make(map[string]string, len(media))
	for _, m := range media {
		trimmed := strings.TrimSpace(m.URL)
		if trimmed == "" {
			continue
		}
		byURL[trimmed] = m
		if m.Blurhash != nil && *m.Blurhash != "" {
			hashes[trimmed] = *m.Blurhash
		}
	}
	rendered := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if persisted, ok := byURL[item.URL]; ok {
			item.ThumbnailURL = persisted.ThumbnailURL
			item.Width = persisted.Width
			item.Height = persisted.Height
			item.Duration = persisted.DurationMs
			item.Status = persisted.Status
		}
		rendered = append(rendered, MediaWireItem(item, hashes[item.URL]))
	}
	return rendered
}

// MediaWireItem renders one typed media item.
//
// `thumbnail_url` is the Lambda thumbnail variant derived from the item URL
// by deterministic rule — an explicitly provided item ThumbnailURL wins when
// present. Videos resolve to their poster frame. List surfaces render the
// thumbnail; detail/viewer render `url` (untouched original).
func MediaWireItem(item mediaentity.Media, blurhash string) map[string]interface{} {
	thumbnail := ""
	if item.ThumbnailURL != nil {
		thumbnail = ResolveReadableMediaReference(*item.ThumbnailURL)
	} else if item.Type == mediaentity.MediaTypeVideo {
		thumbnail = ResolveReadablePosterURL(item.URL)
	} else {
		thumbnail = ResolveReadableThumbnailURL(item.URL)
	}
	out := map[string]interface{}{
		"id":            item.ID.String(),
		"type":          item.Type.String(),
		"url":           ResolveReadableMediaReference(item.URL),
		"position":      item.Position,
		"thumbnail_url": thumbnail,
		"width":         item.Width,
		"height":        item.Height,
		"duration":      item.Duration,
		// Always a vocabulary string, never null: legacy slots without a
		// persisted status read as ready (NormalizeStatus).
		"status": string(mediaref.NormalizeStatus(item.Status)),
		"created_at": item.CreatedAt.Format(time.RFC3339),
	}
	if blurhash != "" {
		out["blurhash"] = blurhash
	}
	return out
}
