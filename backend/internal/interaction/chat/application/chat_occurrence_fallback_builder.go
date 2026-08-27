package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
)

// OccurrenceFallbackBuilders produces server-built immutable fallback snapshots
// for each resource type. Client never submits preview data.
type OccurrenceFallbackBuilders struct {
	profileBuilder ProfileFallbackBuilder
	contentBuilder ContentFallbackBuilder
	fpsBuilder     FPSFallbackBuilder
	auctionBuilder AuctionFallbackBuilder
}

// ProfileFallbackBuilder builds Profile fallbacks.
type ProfileFallbackBuilder interface {
	BuildProfileFallback(ctx context.Context, tx db.Tx, userID uuid.UUID) (json.RawMessage, error)
}

// ContentFallbackBuilder builds Content fallbacks.
type ContentFallbackBuilder interface {
	BuildContentFallback(ctx context.Context, tx db.Tx, contentID uuid.UUID) (json.RawMessage, error)
}

// FPSFallbackBuilder builds ForSale fallbacks.
type FPSFallbackBuilder interface {
	BuildFPSFallback(ctx context.Context, tx db.Tx, fpsID uuid.UUID) (json.RawMessage, error)
}

// AuctionFallbackBuilder builds Auction fallbacks.
type AuctionFallbackBuilder interface {
	BuildAuctionFallback(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (json.RawMessage, error)
}

// NewOccurrenceFallbackBuilders creates the builder set.
func NewOccurrenceFallbackBuilders(
	profileBuilder ProfileFallbackBuilder,
	contentBuilder ContentFallbackBuilder,
	fpsBuilder FPSFallbackBuilder,
	auctionBuilder AuctionFallbackBuilder,
) *OccurrenceFallbackBuilders {
	return &OccurrenceFallbackBuilders{
		profileBuilder: profileBuilder,
		contentBuilder: contentBuilder,
		fpsBuilder:     fpsBuilder,
		auctionBuilder: auctionBuilder,
	}
}

// BuildFallback dispatches to the correct builder based on resource type.
func (b *OccurrenceFallbackBuilders) BuildFallback(
	ctx context.Context,
	tx db.Tx,
	resourceType chatEntity.ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
) (json.RawMessage, error) {
	switch resourceType {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		if b.profileBuilder == nil {
			return nil, fmt.Errorf("profile fallback builder not configured")
		}
		return b.profileBuilder.BuildProfileFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		if b.contentBuilder == nil {
			return nil, fmt.Errorf("content fallback builder not configured")
		}
		return b.contentBuilder.BuildContentFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		if b.fpsBuilder == nil {
			return nil, fmt.Errorf("fps fallback builder not configured")
		}
		return b.fpsBuilder.BuildFPSFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		if b.auctionBuilder == nil {
			return nil, fmt.Errorf("auction fallback builder not configured")
		}
		return b.auctionBuilder.BuildAuctionFallback(ctx, tx, resourceID)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// ============================================================================
// Default SQL-based fallback builders
// ============================================================================

// defaultProfileFallbackBuilder builds Profile fallbacks from users +
// user_profiles + seller_profiles.
type defaultProfileFallbackBuilder struct{}

func (b *defaultProfileFallbackBuilder) BuildProfileFallback(ctx context.Context, tx db.Tx, userID uuid.UUID) (json.RawMessage, error) {
	type profileFallback struct {
		Username   string  `json:"username"`
		AvatarURL  *string `json:"avatar_url"`
		StoreName  *string `json:"store_name"`
		IsSeller   bool    `json:"is_seller"`
	}
	var fb profileFallback
	var avatarURL *string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(up.username, ''), up.avatar_url,
		       sp.store_name, sp.user_id IS NOT NULL AS is_seller
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id AND sp.status = 'active'
		WHERE u.id = $1
	`, userID).Scan(&fb.Username, &avatarURL, &fb.StoreName, &fb.IsSeller)
	if err != nil {
		return nil, fmt.Errorf("build profile fallback: %w", err)
	}
	if avatarURL != nil && *avatarURL != "" {
		fb.AvatarURL = avatarURL
	}
	return json.Marshal(fb)
}

// defaultContentFallbackBuilder builds Content fallbacks.
type defaultContentFallbackBuilder struct{}

func (b *defaultContentFallbackBuilder) BuildContentFallback(ctx context.Context, tx db.Tx, contentID uuid.UUID) (json.RawMessage, error) {
	type contentFallback struct {
		CaptionExcerpt     *string `json:"caption_excerpt"`
		FirstMediaURL      *string `json:"first_media_url"`
		AuthorUsername     string  `json:"author_username"`
		AuthorAvatarURL    *string `json:"author_avatar_url"`
	}
	var fb contentFallback
	var caption *string
	var firstMediaURL *string
	var authorUsername string
	var authorAvatar *string
	err := tx.QueryRow(ctx, `
		SELECT c.caption,
		       (SELECT cm.media_url FROM content_media cm
		         WHERE cm.content_id = c.id ORDER BY cm.position LIMIT 1) AS first_media_url,
		       COALESCE(up.username, '') AS author_username,
		       up.avatar_url AS author_avatar_url
		FROM contents c
		LEFT JOIN user_profiles up ON up.user_id = c.author_id
		WHERE c.id = $1
	`, contentID).Scan(&caption, &firstMediaURL, &authorUsername, &authorAvatar)
	if err != nil {
		return nil, fmt.Errorf("build content fallback: %w", err)
	}
	if caption != nil && *caption != "" {
		excerpt := truncateCaption(*caption, 200)
		fb.CaptionExcerpt = &excerpt
	}
	if firstMediaURL != nil && *firstMediaURL != "" {
		fb.FirstMediaURL = firstMediaURL
	}
	fb.AuthorUsername = authorUsername
	if authorAvatar != nil && *authorAvatar != "" {
		fb.AuthorAvatarURL = authorAvatar
	}
	return json.Marshal(fb)
}

func truncateCaption(caption string, maxLen int) string {
	runes := []rune(caption)
	if len(runes) <= maxLen {
		return caption
	}
	return string(runes[:maxLen]) + "..."
}

// defaultFPSFallbackBuilder builds ForSale fallbacks.
type defaultFPSFallbackBuilder struct{}

func (b *defaultFPSFallbackBuilder) BuildFPSFallback(ctx context.Context, tx db.Tx, fpsID uuid.UUID) (json.RawMessage, error) {
	type fpsFallback struct {
		Title            string  `json:"title"`
		ImageURL         *string `json:"image_url"`
		SellerStoreName  string  `json:"seller_store_name"`
		SellerStoreImage *string `json:"seller_store_image"`
	}
	var fb fpsFallback
	err := tx.QueryRow(ctx, `
		SELECT p.title,
		       p.media_urls->>0 AS image_url,
		       COALESCE(sp.store_name, '') AS seller_store_name,
		       NULL AS seller_store_image
		FROM for_sales fps
		JOIN products p ON p.id = fps.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id = fps.seller_id
		WHERE fps.id = $1
	`, fpsID).Scan(&fb.Title, &fb.ImageURL, &fb.SellerStoreName, &fb.SellerStoreImage)
	if err != nil {
		return nil, fmt.Errorf("build fps fallback: %w", err)
	}
	return json.Marshal(fb)
}

// defaultAuctionFallbackBuilder builds Auction fallbacks.
type defaultAuctionFallbackBuilder struct{}

func (b *defaultAuctionFallbackBuilder) BuildAuctionFallback(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (json.RawMessage, error) {
	type auctionFallback struct {
		Title            string  `json:"title"`
		ImageURL         *string `json:"image_url"`
		SellerStoreName  string  `json:"seller_store_name"`
		SellerStoreImage *string `json:"seller_store_image"`
	}
	var fb auctionFallback
	err := tx.QueryRow(ctx, `
		SELECT p.title,
		       p.media_urls->>0 AS image_url,
		       COALESCE(sp.store_name, '') AS seller_store_name,
		       NULL AS seller_store_image
		FROM auctions a
		JOIN products p ON p.id = a.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id = a.seller_id
		WHERE a.id = $1
	`, auctionID).Scan(&fb.Title, &fb.ImageURL, &fb.SellerStoreName, &fb.SellerStoreImage)
	if err != nil {
		return nil, fmt.Errorf("build auction fallback: %w", err)
	}
	return json.Marshal(fb)
}
