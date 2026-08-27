package serverboot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	"github.com/labuda/backend/internal/identity/auth"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// chatResourceAuthorizerAdapter implements chatApp.ResourceAuthorizer
// using canonical domain services wired in serverboot.
type chatResourceAuthorizerAdapter struct {
	db            *db.DB
	contentRepo   contentQuerier
	socialRepo    blockChecker
	roleChecker   auth.RoleChecker
	fpsRepo       fpsGetter
	auctionRepo   auctionGetter
}

// Narrow domain interfaces.
type contentQuerier interface {
	GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*contentEntity.Content, error)
}
type blockChecker interface {
	ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
}
type fpsGetter interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*fpsEntity.ForSale, error)
}
type auctionGetter interface {
	GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*auctionEntity.Auction, error)
}

func toDBTx(tx interface{}) db.Tx { return tx.(db.Tx) }

func newChatResourceAuthorizer(
	database *db.DB,
	contentQ contentQuerier,
	socialQ blockChecker,
	rc auth.RoleChecker,
	fpsQ fpsGetter,
	auctionQ auctionGetter,
) chatApp.ResourceAuthorizer {
	return &chatResourceAuthorizerAdapter{
		db:          database,
		contentRepo: contentQ,
		socialRepo:  socialQ,
		roleChecker: rc,
		fpsRepo:     fpsQ,
		auctionRepo: auctionQ,
	}
}

// AuthorizeShare delegates to typed share authorizers.
func (a *chatResourceAuthorizerAdapter) AuthorizeShare(
	ctx context.Context, tx interface{},
	viewerID uuid.UUID, resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID,
) (json.RawMessage, error) {
	switch resourceType {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		return a.authorizeProfileShare(ctx, tx, viewerID, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		return a.authorizeContentShare(ctx, tx, viewerID, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		return a.authorizeFPSShare(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		return a.authorizeAuctionShare(ctx, tx, resourceID)
	default:
		return nil, chatRepo.ErrResourceNotFound
	}
}

// AuthorizeDirect delegates to typed direct authorizers using canonical Commerce authority.
func (a *chatResourceAuthorizerAdapter) AuthorizeDirect(
	ctx context.Context, tx interface{},
	actorID uuid.UUID, resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID,
) (json.RawMessage, error) {
	switch resourceType {
	case chatEntity.ResourceOccurrenceResourceTypeProfile, chatEntity.ResourceOccurrenceResourceTypeContent:
		return nil, fmt.Errorf("direct_commerce_insert_chat not valid for profile or content")
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		return a.authorizeFPSDirect(ctx, tx, actorID, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		return a.authorizeAuctionDirect(ctx, tx, actorID, resourceID)
	default:
		return nil, chatRepo.ErrResourceNotFound
	}
}

// BuildFallback resolves canonical data for the fallback snapshot.
func (a *chatResourceAuthorizerAdapter) BuildFallback(
	ctx context.Context, tx interface{},
	resourceType chatEntity.ResourceOccurrenceResourceType, resourceID uuid.UUID,
) (json.RawMessage, error) {
	switch resourceType {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		return a.buildProfileFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		return a.buildContentFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		return a.buildFPSFallback(ctx, tx, resourceID)
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		return a.buildAuctionFallback(ctx, tx, resourceID)
	default:
		return nil, chatRepo.ErrResourceNotFound
	}
}

// --- Share authorization ---

func (a *chatResourceAuthorizerAdapter) authorizeProfileShare(ctx context.Context, tx interface{}, viewerID, profileID uuid.UUID) (json.RawMessage, error) {
	var deletedAt *interface{}
	err := toDBTx(tx).QueryRow(ctx, `SELECT deleted_at FROM users WHERE id=$1`, profileID).Scan(&deletedAt)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	if deletedAt != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	if viewerID != profileID {
		blocked, err := a.socialRepo.ExistsBlock(ctx, tx, viewerID, profileID)
		if err != nil {
			return nil, fmt.Errorf("block check: %w", err)
		}
		if blocked {
			return nil, chatRepo.ErrResourceNotAccessible
		}
	}
	return a.buildProfileFallback(ctx, tx, profileID)
}

func (a *chatResourceAuthorizerAdapter) authorizeContentShare(ctx context.Context, tx interface{}, viewerID, contentID uuid.UUID) (json.RawMessage, error) {
	c, err := a.contentRepo.GetByID(ctx, tx, contentID)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	if c.DeletedAt != nil || c.IsHidden || c.Status == "hidden" || c.Status == "moderated" {
		return nil, chatRepo.ErrResourceNotAccessible
	}
	switch string(c.Visibility) {
	case "public":
	case "private":
		if c.AuthorID != viewerID {
			return nil, chatRepo.ErrResourceNotAccessible
		}
	case "followers_only":
		if c.AuthorID != viewerID {
			blocked, _ := a.socialRepo.ExistsBlock(ctx, tx, viewerID, c.AuthorID)
			if blocked {
				return nil, chatRepo.ErrResourceNotAccessible
			}
			var follows bool
			_ = toDBTx(tx).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_follows WHERE follower_id=$1 AND followed_id=$2)`, viewerID, c.AuthorID).Scan(&follows)
			if !follows {
				return nil, chatRepo.ErrResourceNotAccessible
			}
		}
	default:
		return nil, chatRepo.ErrResourceNotAccessible
	}
	return a.buildContentFallback(ctx, tx, contentID)
}

func (a *chatResourceAuthorizerAdapter) authorizeFPSShare(ctx context.Context, tx interface{}, fpsID uuid.UUID) (json.RawMessage, error) {
	_, err := a.fpsRepo.GetByID(ctx, toDBTx(tx), fpsID)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	return a.buildFPSFallback(ctx, tx, fpsID)
}

func (a *chatResourceAuthorizerAdapter) authorizeAuctionShare(ctx context.Context, tx interface{}, auctionID uuid.UUID) (json.RawMessage, error) {
	_, err := a.auctionRepo.GetByID(ctx, toDBTx(tx), auctionID)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	return a.buildAuctionFallback(ctx, tx, auctionID)
}

// --- Direct authorization using canonical Commerce authority ---

func (a *chatResourceAuthorizerAdapter) authorizeFPSDirect(ctx context.Context, tx interface{}, actorID, fpsID uuid.UUID) (json.RawMessage, error) {
	fps, err := a.fpsRepo.GetByID(ctx, toDBTx(tx), fpsID)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	// Canonical ownership check
	if fps.SellerID != actorID {
		return nil, chatRepo.ErrNotResourceOwner
	}
	// Canonical market-increasing seller capability
	if a.roleChecker != nil {
		hasCap, err := a.roleChecker.HasActiveSellerCapability(ctx, actorID)
		if err != nil || !hasCap {
			return nil, chatRepo.ErrMarketAuthorityRequired
		}
	}
	// Canonical IsRepostable() — single source of truth
	if !fps.Status.IsRepostable() {
		return nil, chatRepo.ErrResourceNotPromotable
	}
	return a.buildFPSFallback(ctx, tx, fpsID)
}

func (a *chatResourceAuthorizerAdapter) authorizeAuctionDirect(ctx context.Context, tx interface{}, actorID, auctionID uuid.UUID) (json.RawMessage, error) {
	auc, err := a.auctionRepo.GetByID(ctx, toDBTx(tx), auctionID)
	if err != nil {
		return nil, chatRepo.ErrResourceNotFound
	}
	// Canonical ownership check
	if auc.SellerID != actorID {
		return nil, chatRepo.ErrNotResourceOwner
	}
	// Canonical market-increasing seller capability
	if a.roleChecker != nil {
		hasCap, err := a.roleChecker.HasActiveSellerCapability(ctx, actorID)
		if err != nil || !hasCap {
			return nil, chatRepo.ErrMarketAuthorityRequired
		}
	}
	// Canonical IsRepostable() — single source of truth
	if !auc.Status.IsRepostable() {
		return nil, chatRepo.ErrResourceNotPromotable
	}
	return a.buildAuctionFallback(ctx, tx, auctionID)
}

// --- Fallback builders ---

func (a *chatResourceAuthorizerAdapter) buildProfileFallback(ctx context.Context, tx interface{}, userID uuid.UUID) (json.RawMessage, error) {
	type pf struct {
		Username  string  `json:"username"`
		AvatarURL *string `json:"avatar_url"`
		StoreName *string `json:"store_name"`
		IsSeller  bool    `json:"is_seller"`
	}
	var fb pf
	var avatar *string
	err := toDBTx(tx).QueryRow(ctx, `
		SELECT COALESCE(up.username,''), up.avatar_url, sp.store_name, sp.user_id IS NOT NULL
		FROM users u LEFT JOIN user_profiles up ON up.user_id=u.id
		LEFT JOIN seller_profiles sp ON sp.user_id=u.id AND sp.status='active'
		WHERE u.id=$1`, userID).Scan(&fb.Username, &avatar, &fb.StoreName, &fb.IsSeller)
	if err != nil {
		return nil, fmt.Errorf("profile fallback: %w", err)
	}
	if avatar != nil && *avatar != "" {
		fb.AvatarURL = avatar
	}
	return json.Marshal(fb)
}

func (a *chatResourceAuthorizerAdapter) buildContentFallback(ctx context.Context, tx interface{}, contentID uuid.UUID) (json.RawMessage, error) {
	type cf struct {
		CaptionExcerpt  *string `json:"caption_excerpt"`
		FirstMediaURL   *string `json:"first_media_url"`
		AuthorUsername  string  `json:"author_username"`
		AuthorAvatarURL *string `json:"author_avatar_url"`
	}
	var fb cf
	var caption, firstMedia, authorUsername, authorAvatar *string
	err := toDBTx(tx).QueryRow(ctx, `
		SELECT c.caption,
			(SELECT cm.media_url FROM content_media cm WHERE cm.content_id=c.id ORDER BY cm.sort_order LIMIT 1),
			COALESCE(up.username,''), up.avatar_url
		FROM contents c LEFT JOIN user_profiles up ON up.user_id=c.author_id
		WHERE c.id=$1`, contentID).Scan(&caption, &firstMedia, &authorUsername, &authorAvatar)
	if err != nil {
		return nil, fmt.Errorf("content fallback: %w", err)
	}
	if caption != nil && *caption != "" {
		s := truncateStr(*caption, 200)
		fb.CaptionExcerpt = &s
	}
	if firstMedia != nil && *firstMedia != "" {
		fb.FirstMediaURL = firstMedia
	}
	if authorUsername != nil {
		fb.AuthorUsername = *authorUsername
	}
	if authorAvatar != nil && *authorAvatar != "" {
		fb.AuthorAvatarURL = authorAvatar
	}
	return json.Marshal(fb)
}

func (a *chatResourceAuthorizerAdapter) buildFPSFallback(ctx context.Context, tx interface{}, fpsID uuid.UUID) (json.RawMessage, error) {
	type ff struct {
		Title            string  `json:"title"`
		ImageURL         *string `json:"image_url"`
		SellerStoreName  string  `json:"seller_store_name"`
		SellerStoreImage *string `json:"seller_store_image"`
	}
	var fb ff
	err := toDBTx(tx).QueryRow(ctx, `
		SELECT p.title,
			p.media_urls->>0,
			COALESCE(sp.store_name,''), NULL
		FROM for_sales fps
		JOIN products p ON p.id=fps.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id=fps.seller_id
		WHERE fps.id=$1`, fpsID).Scan(&fb.Title, &fb.ImageURL, &fb.SellerStoreName, &fb.SellerStoreImage)
	if err != nil {
		return nil, fmt.Errorf("fps fallback: %w", err)
	}
	return json.Marshal(fb)
}

func (a *chatResourceAuthorizerAdapter) buildAuctionFallback(ctx context.Context, tx interface{}, auctionID uuid.UUID) (json.RawMessage, error) {
	type af struct {
		Title            string  `json:"title"`
		ImageURL         *string `json:"image_url"`
		SellerStoreName  string  `json:"seller_store_name"`
		SellerStoreImage *string `json:"seller_store_image"`
	}
	var fb af
	err := toDBTx(tx).QueryRow(ctx, `
		SELECT p.title,
			p.media_urls->>0,
			COALESCE(sp.store_name,''), NULL
		FROM auctions a
		JOIN products p ON p.id=a.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id=a.seller_id
		WHERE a.id=$1`, auctionID).Scan(&fb.Title, &fb.ImageURL, &fb.SellerStoreName, &fb.SellerStoreImage)
	if err != nil {
		return nil, fmt.Errorf("auction fallback: %w", err)
	}
	return json.Marshal(fb)
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

var _ chatApp.ResourceAuthorizer = (*chatResourceAuthorizerAdapter)(nil)
