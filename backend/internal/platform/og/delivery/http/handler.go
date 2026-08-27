package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pkgdb "github.com/labuda/backend/pkg/db"
)

const (
	defaultTitle       = "Labuda"
	defaultDescription = "Labuda is a social commerce platform for trusted buying and selling."
)

type metadata struct {
	Title       string
	Description string
	ImageURL    string
	URL         string
}

type Handler struct {
	store store
}

type store interface {
	getForSale(ctx context.Context, id string) (*metadata, error)
	getAuction(ctx context.Context, id string) (*metadata, error)
	getProfile(ctx context.Context, id string) (*metadata, error)
	getContent(ctx context.Context, id string) (*metadata, error)
}

type pgStore struct {
	db *pkgdb.DB
}

func NewHandler(db *pkgdb.DB) *Handler {
	return &Handler{store: &pgStore{db: db}}
}

func (h *Handler) GetForSale(c *gin.Context) {
	h.serve(c, c.Param("id"), h.store.getForSale)
}

func (h *Handler) GetAuction(c *gin.Context) {
	h.serve(c, c.Param("id"), h.store.getAuction)
}

func (h *Handler) GetProfile(c *gin.Context) {
	h.serve(c, c.Param("id"), h.store.getProfile)
}

func (h *Handler) GetContent(c *gin.Context) {
	h.serve(c, c.Param("id"), h.store.getContent)
}

func (h *Handler) serve(c *gin.Context, id string, load func(context.Context, string) (*metadata, error)) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	page := fallbackMetadata(buildCanonicalURL(c))
	if strings.TrimSpace(id) != "" {
		if m, err := load(c.Request.Context(), id); err == nil && m != nil {
			page = withFallback(*m, page)
			page.URL = buildCanonicalURL(c)
		}
	}
	c.Status(http.StatusOK)
	_ = renderPage(c.Writer, page)
}

func fallbackMetadata(url string) metadata {
	return metadata{
		Title:       defaultTitle,
		Description: defaultDescription,
		URL:         url,
	}
}

func withFallback(in metadata, fallback metadata) metadata {
	out := fallback
	if strings.TrimSpace(in.Title) != "" {
		out.Title = in.Title
	}
	if strings.TrimSpace(in.Description) != "" {
		out.Description = in.Description
	}
	if strings.TrimSpace(in.ImageURL) != "" {
		out.ImageURL = in.ImageURL
	}
	if strings.TrimSpace(in.URL) != "" {
		out.URL = in.URL
	}
	return out
}

func buildCanonicalURL(c *gin.Context) string {
	scheme := "https"
	if c.Request.TLS == nil {
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "http") {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if host == "" {
		host = "labuda-79de2.web.app"
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, c.Request.URL.Path)
}

func (s *pgStore) getForSale(ctx context.Context, id string) (*metadata, error) {
	// Fixed-price sale preview: title/description/media live on products
	// (canonical item authority); price/status/published_at live on
	// for_sales. The legacy `forSales` table is write-dead — nothing
	// inserts or updates it anymore, so it must not be read here.
	// "Public" visibility is derived the same way the forsale repository
	// derives it: active status with a non-nil published_at.
	const q = `
SELECT
	p.title,
	fps.price_per_unit,
	p.description,
	p.media_urls
FROM for_sales fps
JOIN products p ON p.id = fps.product_id
JOIN users u ON u.id = fps.seller_id
WHERE fps.id = $1
  AND fps.status = 'active'
  AND fps.published_at IS NOT NULL
  AND u.account_status = 'active'
  AND u.deleted_at IS NULL
LIMIT 1
`
	var title string
	var price int64
	var description string
	var mediaRaw []byte
	if err := s.db.Pool().QueryRow(ctx, q, id).Scan(&title, &price, &description, &mediaRaw); err != nil {
		return nil, err
	}
	return &metadata{
		Title:       title,
		Description: compact(fmt.Sprintf("Rp %d. %s", price, description), 200),
		ImageURL:    extractFirstURL(mediaRaw),
	}, nil
}

func (s *pgStore) getAuction(ctx context.Context, id string) (*metadata, error) {
	// Auction preview: media/product identity comes from products via
	// auctions.product_id — the canonical link. auctions.for_sale_id was a
	// legacy column (never set for product-based auctions, dropped entirely
	// by migration 000010 / PASS_21C); joining through it (as this query
	// used to) silently returned zero rows for every real auction.
	const q = `
SELECT
	p.title,
	a.current_bid,
	a.start_price,
	a.status,
	p.description,
	p.media_urls
FROM auctions a
JOIN users u ON u.id = a.seller_id
JOIN products p ON p.id = a.product_id
WHERE a.id = $1
  AND a.status IN ('scheduled', 'active', 'waiting_settlement', 'ended')
  AND u.account_status = 'active'
  AND u.deleted_at IS NULL
LIMIT 1
`
	var title string
	var currentBid *int64
	var startPrice int64
	var status string
	var description string
	var mediaRaw []byte
	if err := s.db.Pool().QueryRow(ctx, q, id).Scan(&title, &currentBid, &startPrice, &status, &description, &mediaRaw); err != nil {
		return nil, err
	}
	bid := startPrice
	if currentBid != nil {
		bid = *currentBid
	}
	return &metadata{
		Title:       title,
		Description: compact(fmt.Sprintf("Status: %s. Current bid: Rp %d. %s", status, bid, description), 200),
		ImageURL:    extractFirstURL(mediaRaw),
	}, nil
}

func (s *pgStore) getProfile(ctx context.Context, id string) (*metadata, error) {
	const q = `
SELECT
	up.username,
	up.bio,
	up.avatar_url
FROM users u
JOIN user_profiles up ON up.user_id = u.id
WHERE u.id = $1
  AND u.account_status = 'active'
  AND u.deleted_at IS NULL
LIMIT 1
`
	var username string
	var bio *string
	var avatar *string
	if err := s.db.Pool().QueryRow(ctx, q, id).Scan(&username, &bio, &avatar); err != nil {
		return nil, err
	}
	description := fmt.Sprintf("See @%s on Labuda.", username)
	if bio != nil && strings.TrimSpace(*bio) != "" {
		description = compact(*bio, 200)
	}
	out := &metadata{
		Title:       "@" + username,
		Description: description,
	}
	if avatar != nil {
		out.ImageURL = *avatar
	}
	return out, nil
}

func (s *pgStore) getContent(ctx context.Context, id string) (*metadata, error) {
	const q = `
SELECT
	c.caption,
	up.username,
	(
		SELECT cm.media_url
		FROM content_media cm
		WHERE cm.content_id = c.id
		ORDER BY cm.position ASC
		LIMIT 1
	) AS media_url
FROM contents c
JOIN users u ON u.id = c.author_id
LEFT JOIN user_profiles up ON up.user_id = u.id
WHERE c.id = $1
  AND c.status = 'active'
  AND c.is_hidden = false
  AND c.deleted_at IS NULL
  AND u.account_status = 'active'
  AND u.deleted_at IS NULL
LIMIT 1
`
	var caption *string
	var username *string
	var media *string
	if err := s.db.Pool().QueryRow(ctx, q, id).Scan(&caption, &username, &media); err != nil {
		return nil, err
	}
	title := "Labuda Content"
	if username != nil && strings.TrimSpace(*username) != "" {
		title = "@" + *username
	}
	description := "See this post on Labuda."
	if caption != nil && strings.TrimSpace(*caption) != "" {
		description = compact(*caption, 200)
	}
	out := &metadata{Title: title, Description: description}
	if media != nil {
		out.ImageURL = *media
	}
	return out, nil
}

func extractFirstURL(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return ""
	}
	return strings.TrimSpace(arr[0])
}

func compact(s string, max int) string {
	clean := strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(clean) <= max {
		return clean
	}
	if max <= 3 {
		return clean[:max]
	}
	return clean[:max-3] + "..."
}
