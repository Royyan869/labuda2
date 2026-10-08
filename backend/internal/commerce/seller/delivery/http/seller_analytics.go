package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// SellerAnalyticsWindow is the canonical analytics read window: 30 days.
// It is a READ/FILTER window only — it is NOT a retention policy and it never
// deletes historical Product View rows.
const SellerAnalyticsWindow = 30 * 24 * time.Hour

// SellerAnalyticsSummary is the seller-level rollup for the 30-day window.
type SellerAnalyticsSummary struct {
	TotalViews30d        int64 `json:"total_views_30d"`
	ProductsWithViews30d int64 `json:"products_with_views_30d"`
	ProductsSold30d      int64 `json:"products_sold_30d"`
}

// SellerAnalyticsProduct is one product's analytics row.
//
// `sold` is the product's current sold OUTCOME (state), while `ProductsSold30d`
// in the summary is a SALE EVENT (a completed order within the window). The two
// are intentionally distinct: Current State != Sale Event.
type SellerAnalyticsProduct struct {
	ProductID   string `json:"product_id"`
	Title       string `json:"title"`
	SurfaceType string `json:"surface_type"` // "for_sale" | "auction"
	State       string `json:"state"`        // canonical surface status
	Views30d    int64  `json:"views_30d"`
	Sold        bool   `json:"sold"`
	BidCount30d int64  `json:"bid_count_30d"` // auction only (0 for for_sale)
}

// SellerAnalyticsResponse is the canonical Seller Analytics read projection.
// It is a read model — NOT a new domain authority. All numbers are aggregated
// from canonical sources (product_view_events, orders, for_sales, auctions,
// auction_bids).
type SellerAnalyticsResponse struct {
	Summary  SellerAnalyticsSummary   `json:"summary"`
	Products []SellerAnalyticsProduct `json:"products"`
}

// sellerAnalyticsProductsQuery returns every product owned by the seller with
// its 30-day view count, canonical surface state, sold outcome, and 30-day bid
// count. Surface state is resolved from the product's exclusive selling surface
// (a product is either a for_sale OR an auction, never both).
const sellerAnalyticsProductsQuery = `
SELECT
    p.id,
    p.title,
    CASE
        WHEN fs.id IS NOT NULL THEN 'for_sale'
        WHEN a.id IS NOT NULL THEN 'auction'
        ELSE COALESCE(p.selling_surface::text, '')
    END AS surface_type,
    COALESCE(fs.status::text, a.status::text, '') AS state,
    COALESCE((
        SELECT COUNT(*)
        FROM product_view_events pv
        WHERE pv.product_id = p.id AND pv.viewed_at >= $2
    ), 0) AS views_30d,
    COALESCE(
        (fs.status = 'sold') OR (a.status = 'ended' AND a.current_winner_id IS NOT NULL),
        false
    ) AS sold,
    COALESCE((
        SELECT COUNT(*)
        FROM auction_bids ab
        JOIN auctions ax ON ax.id = ab.auction_id
        WHERE ax.product_id = p.id AND ab.created_at >= $2
    ), 0) AS bid_count_30d
FROM products p
LEFT JOIN for_sales fs ON fs.product_id = p.id
LEFT JOIN auctions a ON a.product_id = p.id
WHERE p.seller_id = $1
ORDER BY p.created_at DESC, p.id
`

// sellerAnalyticsSoldCountQuery counts distinct products that have a canonical
// completed sale (order) within the window. Sale occurrence authority is the
// completed order, never a surface status string alone.
const sellerAnalyticsSoldCountQuery = `
SELECT COUNT(DISTINCT oi.product_id)
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE o.seller_id = $1
  AND o.status = 'completed'
  AND o.completed_at >= $2
`

// GetAnalytics handles GET /api/v1/seller/analytics.
//
// It reads the authenticated seller's identity from the request context (never
// from a client-supplied seller_id) and returns a read projection of their
// products' Product Views and sale outcomes over the last 30 days.
//
// Authority boundaries:
//   - Product View count  → product_view_events (canonical)
//   - Sold outcome (state) → for_sales.status / auctions.status
//   - Sale occurrence     → orders.status='completed' + completed_at
//   - Bid activity        → auction_bids (auction only)
//
// Money is intentionally NOT an analytics metric — earnings remain owned by
// Finance/SELLER_PAYABLE.
func (h *SellerHandler) GetAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	windowStart := time.Now().UTC().Add(-SellerAnalyticsWindow)

	var products []SellerAnalyticsProduct
	var soldCount int64

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, sellerAnalyticsProductsQuery, userID, windowStart)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p SellerAnalyticsProduct
			var productID uuid.UUID
			if err := rows.Scan(
				&productID,
				&p.Title,
				&p.SurfaceType,
				&p.State,
				&p.Views30d,
				&p.Sold,
				&p.BidCount30d,
			); err != nil {
				return err
			}
			p.ProductID = productID.String()
			products = append(products, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		return tx.QueryRow(ctx, sellerAnalyticsSoldCountQuery, userID, windowStart).Scan(&soldCount)
	})

	if err != nil {
		h.log.Error("Failed to get seller analytics",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller analytics")
		return
	}

	var totalViews, productsWithViews int64
	for _, p := range products {
		totalViews += p.Views30d
		if p.Views30d > 0 {
			productsWithViews++
		}
	}

	resp := SellerAnalyticsResponse{
		Summary: SellerAnalyticsSummary{
			TotalViews30d:        totalViews,
			ProductsWithViews30d: productsWithViews,
			ProductsSold30d:      soldCount,
		},
		Products: products,
	}
	if resp.Products == nil {
		resp.Products = []SellerAnalyticsProduct{}
	}

	response.Success(c, resp)
}
