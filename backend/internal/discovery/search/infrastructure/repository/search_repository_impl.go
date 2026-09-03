package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	searchRepo "github.com/labuda/backend/internal/discovery/search/repository"
	"github.com/labuda/backend/pkg/db"
)

// SearchRepositoryImpl implements the search repository using PostgreSQL full-text search.
type SearchRepositoryImpl struct{}

// NewSearchRepository creates a new SearchRepositoryImpl.
func NewSearchRepository() searchRepo.SearchRepository {
	return &SearchRepositoryImpl{}
}

// ============================================================================
// SEARCH HISTORY
// ============================================================================

// AddSearchHistory adds a new search history entry.
func (r *SearchRepositoryImpl) AddSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, query string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO search_history (id, user_id, query, created_at)
		VALUES ($1, $2, $3, NOW())
	`, uuid.New(), userID, query)
	return err
}

// GetSearchHistory retrieves search history for a user, most recent first.
func (r *SearchRepositoryImpl) GetSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, limit int) ([]*entity.SearchHistory, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, query, created_at
		FROM search_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*entity.SearchHistory
	for rows.Next() {
		var h entity.SearchHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.Query, &h.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, &h)
	}
	return history, nil
}

// ClearSearchHistory deletes all search history entries for a user.
func (r *SearchRepositoryImpl) ClearSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM search_history
		WHERE user_id = $1
	`, userID)
	return err
}

// DeleteSearchHistory deletes a specific search history entry.
func (r *SearchRepositoryImpl) DeleteSearchHistory(ctx context.Context, tx db.Tx, id uuid.UUID, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM search_history
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	return err
}

// TrimSearchHistory keeps only the last 20 searches per user.
// Deletes the oldest entries if count exceeds 20.
func (r *SearchRepositoryImpl) TrimSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM search_history
		WHERE id IN (
			SELECT id FROM search_history
			WHERE user_id = $1
			ORDER BY created_at DESC
			OFFSET 20
		)
	`, userID)
	return err
}

// ============================================================================
// FOR SALE SEARCH
// ============================================================================

// SearchForSales performs full-text search on forSales.
//
// Matches: title, description, koi variety, breeder (per
// forSales.search_vector definition; seller display fields are NOT part
// of the search predicate).
//
// INVENTORY TRUTH: Filters out out-of-stock items (quantity_available > 0)
//
// Public seller identity is sourced exclusively from public profile
// columns: user_profiles.username, seller_profiles.store_name, and
// user_profiles.avatar_url. The auth-identity column users.email is
// NEVER projected as a public identity field and NEVER used as a
// search predicate — email is forbidden as public identity per
// viewer-context-contract.md §4.1 (same rule as /search/users).
//
// BANNED/DELETED SELLER SUPPRESSION (E2B1):
// ForSales whose seller has account_status='banned' OR deleted_at IS NOT NULL
// are excluded from discovery. SUSPENDED sellers are NOT excluded — suspension
// is a reversible governance state. The u.id IS NULL branch is a fail-open
// safety valve for orphaned forSales (data integrity violation; not expected).
func (r *SearchRepositoryImpl) SearchForSales(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ForSalePreview, int, error) {
	// Build query with full-text search.
	//
	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE (additive):
	// New explicit columns are projected with strict source separation:
	//   - seller_username   ← p.username  (NEVER store_name)
	//   - seller_farm_name  ← sp.store_name  (NEVER username)
	//   - seller_avatar_url ← p.avatar_url
	// Expired-seller visibility: surface raw user-identity + seller-trust
	// truth to the handler so it can coarsen and emit the SellerCard with
	// both lifecycle axes. The seller-trust axis (latest seller_subscriptions
	// row) is also used below to demote expired-seller results in the
	// relevance ranking.
	// Inline search_vector expression for products table (no stored tsvector column).
	const inlineFPSSearchVector = `(
		setweight(to_tsvector('simple', COALESCE(prod.title, '')), 'A') ||
		setweight(to_tsvector('simple', COALESCE(prod.description, '')), 'B') ||
		setweight(to_tsvector('simple', COALESCE(prod.variety, '')), 'A')
	)`

	baseQuery := `
		SELECT DISTINCT
			fps.id, prod.title, prod.description, prod.variety, fps.price_per_unit,
			prod.media_urls, fps.seller_id, fps.created_at,
			COALESCE(p.username, '')               as seller_username,
			COALESCE(sp.store_name, '')            as seller_farm_name,
			COALESCE(p.avatar_url, '')             as seller_avatar_url,
			COALESCE(u.account_status::text, '')   as seller_account_status,
			(u.deleted_at IS NOT NULL)             as seller_is_deleted,
			COALESCE(ss.status::text, '')          as seller_subscription_status
		FROM for_sales fps
		JOIN products prod ON prod.id = fps.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id = fps.seller_id
		LEFT JOIN user_profiles p ON p.user_id = fps.seller_id
		LEFT JOIN users u ON u.id = fps.seller_id
		LEFT JOIN LATERAL (
		    SELECT status, started_at, expires_at
		    FROM seller_subscriptions
		    WHERE user_id = fps.seller_id
		    ORDER BY created_at DESC
		    LIMIT 1
		) ss ON true
		WHERE fps.status = 'active'
			AND fps.quantity_available > 0
			AND (u.id IS NULL OR (u.account_status != 'banned' AND u.deleted_at IS NULL))
	`

	args := []interface{}{}
	argIdx := 1

	// Add full-text search if query provided
	if filters.Query != "" {
		baseQuery += fmt.Sprintf(" AND "+inlineFPSSearchVector+" @@ plainto_tsquery($%d)", argIdx)
		args = append(args, filters.Query)
		argIdx++
	}

	// Add ordering
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "relevance"
	}

	sortDir := strings.ToUpper(filters.SortDir)
	if sortDir != "ASC" && sortDir != "DESC" {
		sortDir = "DESC"
	}

	// Expired-seller demotion: prepend a two-tier CASE so forSales whose
	// latest seller subscription is currently within its entitlement interval
	// appear before forSales from expired/no-subscription sellers. Owner
	// doctrine: do not exclude, only demote — expired sellers' inventory
	// remains discoverable but strictly ranked below capable sellers' inventory.
	const sellerTrustDemotion = "CASE WHEN ss.status = 'active' AND ss.started_at <= NOW() AND NOW() < ss.expires_at THEN 0 ELSE 1 END ASC"

	switch sortBy {
	case "relevance":
		if filters.Query != "" {
			baseQuery += fmt.Sprintf(" ORDER BY %s, ts_rank("+inlineFPSSearchVector+", plainto_tsquery($%d)) %s", sellerTrustDemotion, argIdx, sortDir)
			args = append(args, filters.Query)
			argIdx++
		} else {
			baseQuery += fmt.Sprintf(" ORDER BY %s", sellerTrustDemotion)
		}
		baseQuery += fmt.Sprintf(", fps.created_at %s, fps.id ASC", sortDir)
	case "created_at":
		baseQuery += fmt.Sprintf(" ORDER BY %s, fps.created_at %s, fps.id ASC", sellerTrustDemotion, sortDir)
	default:
		baseQuery += fmt.Sprintf(" ORDER BY %s, fps.created_at DESC, fps.id ASC", sellerTrustDemotion)
	}

	// Add pagination
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := tx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search forSales failed: %w", err)
	}
	defer rows.Close()

	var forSales []*entity.ForSalePreview
	for rows.Next() {
		var l entity.ForSalePreview
		var mediaURLsJSON json.RawMessage
		var sellerUsername, sellerFarmName, sellerAvatarURL string
		var sellerAccountStatus, sellerSubscriptionStatus string
		var sellerIsDeleted bool

		err := rows.Scan(
			&l.ID, &l.Title, &l.Description, &l.Variety, &l.Price,
			&mediaURLsJSON, &l.SellerID, &l.CreatedAt,
			&sellerUsername, &sellerFarmName, &sellerAvatarURL,
			&sellerAccountStatus, &sellerIsDeleted, &sellerSubscriptionStatus,
		)
		if err != nil {
			return nil, 0, err
		}

		l.SellerUsername = sellerUsername
		l.SellerFarmName = sellerFarmName
		l.SellerAvatarURL = sellerAvatarURL
		l.SellerAccountStatus = sellerAccountStatus
		l.SellerIsDeleted = sellerIsDeleted
		l.SellerSubscriptionStatus = sellerSubscriptionStatus

		// Parse media URLs
		if len(mediaURLsJSON) > 0 && string(mediaURLsJSON) != "null" {
			json.Unmarshal(mediaURLsJSON, &l.MediaURLs)
		}

		forSales = append(forSales, &l)
	}

	// Get total count for pagination
	// NOTE: must mirror the banned/deleted filter from the base query (E2B1).
	countQuery := `
		SELECT COUNT(DISTINCT fps.id)
		FROM for_sales fps
		JOIN products prod ON prod.id = fps.product_id
		LEFT JOIN users u ON u.id = fps.seller_id
		WHERE fps.status = 'active'
			AND fps.quantity_available > 0
			AND (u.id IS NULL OR (u.account_status != 'banned' AND u.deleted_at IS NULL))
	`
	countArgs := []interface{}{}
	countArgIdx := 1

	if filters.Query != "" {
		countQuery += fmt.Sprintf(" AND "+inlineFPSSearchVector+" @@ plainto_tsquery($%d)", countArgIdx)
		countArgs = append(countArgs, filters.Query)
	}

	var total int
	if err := tx.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search forSales failed: %w", err)
	}

	return forSales, total, nil
}

// ============================================================================
// CONTENT SEARCH
// ============================================================================

// SearchContent performs full-text search on content.
//
// Matches: caption (via c.search_vector), hashtags.
//
// Author identity is sourced from user_profiles (username, avatar_url) —
// the canonical PUBLIC identity binding. PRIVACY: full_name is KYC/private
// data and is never projected on this surface. users.email is NEVER
// projected per viewer-context-contract.md §4.1.
//
// Username is COALESCEd to ” when the profile row is absent; avatar_url
// remains nullable. Mobile DTO already parses author.username/avatar_url
// (search_dto.dart ContentSearchResultDto.fromJson) and tolerates
// empty username and null avatar_url.
func (r *SearchRepositoryImpl) SearchContent(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ContentPreview, int, error) {
	// GROUP BY c.id (primary key) already deduplicates rows; SELECT DISTINCT
	// is therefore redundant and conflicts with ORDER BY ts_rank(...) under
	// the postgres rule "for SELECT DISTINCT, ORDER BY expressions must
	// appear in select list" (SQLSTATE 42P10). Visibility predicates and
	// row count are unchanged.
	baseQuery := `
		SELECT
			c.id, c.author_id,
			CASE WHEN c.original_author_id IS NOT NULL THEN 'repost' ELSE 'post' END AS type,
			c.caption, c.created_at,
			COALESCE(
				ARRAY_AGG(DISTINCT cm.media_url ORDER BY cm.media_url) FILTER (WHERE cm.media_url IS NOT NULL),
				ARRAY[]::TEXT[]
			) as media_urls,
			COALESCE(up.username, '') as author_username,
			up.avatar_url as author_avatar_url
		FROM contents c
		LEFT JOIN content_media cm ON cm.content_id = c.id
		LEFT JOIN content_resource_occurrences occ ON occ.content_id = c.id
		LEFT JOIN user_profiles up ON up.user_id = c.author_id
		JOIN users u ON u.id = c.author_id
		WHERE c.status = 'active' AND c.deleted_at IS NULL AND c.is_hidden = false
		  -- F1-B1 (2026-06-14): exclude content from suspended/banned/deleted authors.
		  -- Social content doctrine: suspended authors are also excluded (unlike commerce
		  -- forSale/auction search where suspended seller inventory is preserved).
		  -- account_status enum: 'active'|'suspended'|'banned'. Deletion = deleted_at IS NOT NULL.
		  AND u.account_status = 'active' AND u.deleted_at IS NULL
		  -- V-VISIBILITY: search only surfaces public content.
		  AND c.visibility = 'public'
		  -- SEARCH REPOST GOVERNANCE: exclude reposts whose target is no longer available.
		  -- Short-circuits on NULL original_author_id (non-reposts, most rows).
		  -- FIX-1 (2026-05-15): content-type reposts
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.content_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM contents orig
		      LEFT JOIN users orig_u ON orig_u.id = orig.author_id
		      WHERE orig.id = occ.content_source_id
		        AND (
		          orig.is_hidden = true
		          OR orig.deleted_at IS NOT NULL
		          OR orig.status != 'active'
		          OR orig_u.id IS NULL
		          OR orig_u.account_status != 'active'
		          OR orig_u.deleted_at IS NOT NULL
		        )
		    )
		  )
		  -- FIX-3 (2026-05-28): for_sale-type reposts — exclude when fixed-price sale not active
		  -- Fixed 2026-06-21: was querying FROM forSales (legacy table) — new FPS rows live in for_sales
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.for_sale_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM for_sales fps
		      WHERE fps.id = occ.for_sale_source_id
		        AND fps.status != 'active'
		    )
		  )
		  -- FIX-4 (2026-05-28): auction-type reposts — exclude when auction is terminal
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.auction_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM auctions a
		      WHERE a.id = occ.auction_source_id
		        AND (a.deleted_at IS NOT NULL OR a.status NOT IN ('scheduled', 'active'))
		    )
		  )
	`

	args := []interface{}{}
	argIdx := 1

	// Add full-text search on caption/body
	if filters.Query != "" {
		baseQuery += fmt.Sprintf(" AND (c.search_vector @@ plainto_tsquery($%d)", argIdx)
		args = append(args, filters.Query)
		argIdx++

		// Also search in hashtags
		baseQuery += fmt.Sprintf(" OR EXISTS (SELECT 1 FROM content_hashtags ch WHERE ch.content_id = c.id AND ch.hashtag ILIKE $%d)", argIdx)
		args = append(args, "%"+filters.Query+"%")
		argIdx++

		baseQuery += ")"
	}

	baseQuery += " GROUP BY c.id, c.author_id, c.caption, c.created_at, up.username, up.avatar_url"

	// Add ordering
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "relevance"
	}

	sortDir := strings.ToUpper(filters.SortDir)
	if sortDir != "ASC" && sortDir != "DESC" {
		sortDir = "DESC"
	}

	switch sortBy {
	case "relevance":
		if filters.Query != "" {
			baseQuery += fmt.Sprintf(" ORDER BY ts_rank(c.search_vector, plainto_tsquery($%d)) %s", argIdx, sortDir)
			args = append(args, filters.Query)
			argIdx++
		}
		baseQuery += fmt.Sprintf(", c.created_at %s, c.id ASC", sortDir)
	case "created_at":
		baseQuery += fmt.Sprintf(" ORDER BY c.created_at %s, c.id ASC", sortDir)
	default:
		baseQuery += " ORDER BY c.created_at DESC, c.id ASC"
	}

	// Add pagination
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := tx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search content failed: %w", err)
	}
	defer rows.Close()

	var contents []*entity.ContentPreview
	for rows.Next() {
		var c entity.ContentPreview
		var caption *string
		err := rows.Scan(
			&c.ID, &c.AuthorID, &c.Type, &caption, &c.CreatedAt,
			&c.MediaURLs,
			&c.AuthorUsername, &c.AuthorAvatarURL,
		)
		if err != nil {
			return nil, 0, err
		}

		if caption != nil {
			c.Caption = *caption
		}

		contents = append(contents, &c)
	}

	// Get total count for pagination
	countQuery := `
		SELECT COUNT(DISTINCT c.id)
		FROM contents c
		LEFT JOIN content_resource_occurrences occ ON occ.content_id = c.id
		JOIN users u ON u.id = c.author_id
		WHERE c.status = 'active' AND c.deleted_at IS NULL AND c.is_hidden = false
		  -- F1-B1 (2026-06-14): mirrors base query author lifecycle filter.
		  AND u.account_status = 'active' AND u.deleted_at IS NULL
		  -- V-VISIBILITY: search only surfaces public content.
		  AND c.visibility = 'public'
		  -- SEARCH REPOST GOVERNANCE (count query mirrors base query — FIX-1/3/4)
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.content_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM contents orig
		      LEFT JOIN users orig_u ON orig_u.id = orig.author_id
		      WHERE orig.id = occ.content_source_id
		        AND (
		          orig.is_hidden = true
		          OR orig.deleted_at IS NOT NULL
		          OR orig.status != 'active'
		          OR orig_u.id IS NULL
		          OR orig_u.account_status != 'active'
		          OR orig_u.deleted_at IS NOT NULL
		        )
		    )
		  )
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.for_sale_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM for_sales fps
		      WHERE fps.id = occ.for_sale_source_id
		        AND fps.status != 'active'
		    )
		  )
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND occ.auction_source_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM auctions a
		      WHERE a.id = occ.auction_source_id
		        AND (a.deleted_at IS NOT NULL OR a.status NOT IN ('scheduled', 'active'))
		    )
		  )
	`
	countArgs := []interface{}{}
	countArgIdx := 1

	if filters.Query != "" {
		countQuery += fmt.Sprintf(" AND (c.search_vector @@ plainto_tsquery($%d)", countArgIdx)
		countArgs = append(countArgs, filters.Query)
		countArgIdx++

		countQuery += fmt.Sprintf(" OR EXISTS (SELECT 1 FROM content_hashtags ch WHERE ch.content_id = c.id AND ch.hashtag ILIKE $%d)", countArgIdx)
		countArgs = append(countArgs, "%"+filters.Query+"%")
		countQuery += ")"
	}

	var total int
	if err := tx.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search content failed: %w", err)
	}

	return contents, total, nil
}

// ============================================================================
// USER SEARCH
// ============================================================================

// SearchUsers performs public user search.
//
// Public identity is sourced from user_profiles (username, avatar_url).
// PRIVACY: full_name is KYC/private data and is NOT projected as
// public identity on this surface and
// is NOT used as a search predicate (matching by full_name would
// allow third parties to enumerate users by their legal name). The
// auth-identity column users.email is NEVER projected to the response
// and NEVER used as a search predicate per viewer-context-contract.md §4.1.
//
// Username is COALESCEd to empty string when the profile row is
// absent or the username column is NULL; avatar_url remains nullable.
// The mobile DTO and the mention picker both tolerate empty username
// and null avatar_url.
//
// When the query is non-empty, only public username is matched
// (user_profiles.username); rows without a profile or with NULL
// username will not match.
func (r *SearchRepositoryImpl) SearchUsers(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.UserPreview, int, error) {
	baseQuery := `
		SELECT
			u.id,
			COALESCE(p.username, ''),
			p.avatar_url
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.account_status = 'active' AND u.deleted_at IS NULL
	`

	args := []interface{}{}
	argIdx := 1

	if filters.Query != "" {
		baseQuery += fmt.Sprintf(
			" AND p.username ILIKE $%d",
			argIdx,
		)
		pattern := "%" + filters.Query + "%"
		args = append(args, pattern)
		argIdx++
	}

	baseQuery += " ORDER BY p.username ASC NULLS LAST, u.id ASC"

	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := tx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search users failed: %w", err)
	}
	defer rows.Close()

	var users []*entity.UserPreview
	for rows.Next() {
		var u entity.UserPreview
		if err := rows.Scan(&u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}

	countQuery := `
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.account_status = 'active' AND u.deleted_at IS NULL
	`
	countArgs := []interface{}{}
	countArgIdx := 1

	if filters.Query != "" {
		countQuery += fmt.Sprintf(
			" AND p.username ILIKE $%d",
			countArgIdx,
		)
		pattern := "%" + filters.Query + "%"
		countArgs = append(countArgs, pattern)
	}

	var total int
	if err := tx.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search users failed: %w", err)
	}

	return users, total, nil
}

// ============================================================================
// AUCTION SEARCH
// ============================================================================

// SearchAuctions performs full-text search on auctions.
//
// AUCTION SEARCH ELIGIBILITY (Stage 6B): only searches auctions in public
// discovery states — scheduled or active. Draft, cancelled, waiting_settlement,
// ended (settled/no-winner) surfaces are non-public/historical
// and must not surface in anonymous discovery.
//
// Matches: title, description (ILIKE on prod.title / prod.description; seller
// display fields are NOT part of the search predicate).
//
// Joins with forSales to get thumbnail image.
//
// Public seller identity is sourced exclusively from public profile
// columns: user_profiles.username, seller_profiles.store_name, and
// user_profiles.avatar_url. The auth-identity column users.email is
// NEVER projected as a public identity field and NEVER used as a
// search predicate — email is forbidden as public identity per
// viewer-context-contract.md §4.1 (same rule as /search/users).
//
// BANNED/DELETED SELLER SUPPRESSION (E2B1):
// Auctions whose seller has account_status='banned' OR deleted_at IS NOT NULL
// are excluded from discovery. SUSPENDED sellers are NOT excluded — suspension
// is a reversible governance state. The u.id IS NULL branch is a fail-open
// safety valve for orphaned auctions (data integrity violation; not expected).
func (r *SearchRepositoryImpl) SearchAuctions(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.AuctionPreview, int, error) {
	// Build query with full-text search.
	//
	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE (additive):
	// New explicit columns are projected with strict source separation:
	//   - seller_username   ← p.username  (NEVER store_name)
	//   - seller_farm_name  ← sp.store_name  (NEVER username)
	//   - seller_avatar_url ← p.avatar_url
	baseQuery := `
		SELECT DISTINCT
			a.id, a.seller_id, a.product_id, prod.title, prod.description,
			a.start_price, a.current_bid, a.buy_now_price,
			a.start_at, a.end_at, a.status, a.created_at,
			COALESCE(prod.media_urls->>0, NULL) as thumbnail_url,
			COALESCE(bid_counts.bid_count, 0) as bid_count,
			COALESCE(p.username, '')                as seller_username,
			COALESCE(sp.store_name, '')             as seller_farm_name,
			COALESCE(p.avatar_url, '')              as seller_avatar_url,
			COALESCE(u.account_status::text, '')    as seller_account_status,
			(u.deleted_at IS NOT NULL)              as seller_is_deleted,
			COALESCE(ss.status::text, '')           as seller_subscription_status
		FROM auctions a
		JOIN products prod ON prod.id = a.product_id
		LEFT JOIN seller_profiles sp ON sp.user_id = a.seller_id
		LEFT JOIN user_profiles p ON p.user_id = a.seller_id
		LEFT JOIN users u ON u.id = a.seller_id
		LEFT JOIN LATERAL (
			SELECT status, started_at, expires_at
			FROM seller_subscriptions
			WHERE user_id = a.seller_id
			ORDER BY created_at DESC
			LIMIT 1
		) ss ON true
		LEFT JOIN (
			SELECT auction_id, COUNT(*) as bid_count
			FROM auction_bids
			GROUP BY auction_id
		) bid_counts ON bid_counts.auction_id = a.id
		WHERE a.status IN ('scheduled', 'active')
			AND (u.id IS NULL OR (u.account_status != 'banned' AND u.deleted_at IS NULL))
	`

	args := []interface{}{}
	argIdx := 1

	// Add full-text search if query provided — Product is the canonical source for title/description
	if filters.Query != "" {
		baseQuery += fmt.Sprintf(" AND (prod.title ILIKE $%d OR prod.description ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, "%"+filters.Query+"%", "%"+filters.Query+"%")
		argIdx += 2
	}

	// Add ordering
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "relevance"
	}

	sortDir := strings.ToUpper(filters.SortDir)
	if sortDir != "ASC" && sortDir != "DESC" {
		sortDir = "DESC"
	}

	// Expired-seller demotion: prepend a two-tier CASE so auctions whose
	// latest seller subscription is currently within its entitlement interval
	// appear before auctions from expired/no-subscription sellers. Owner
	// doctrine: do not exclude, only demote.
	const auctionSellerTrustDemotion = "CASE WHEN ss.status = 'active' AND ss.started_at <= NOW() AND NOW() < ss.expires_at THEN 0 ELSE 1 END ASC"

	switch sortBy {
	case "relevance":
		// For relevance, prioritize active-seller auctions first, then active
		// auctions, then by bid count, then by created_at.
		baseQuery += fmt.Sprintf(" ORDER BY %s", auctionSellerTrustDemotion)
		baseQuery += fmt.Sprintf(", CASE WHEN a.status = 'active' THEN 0 ELSE 1 END %s", sortDir)
		baseQuery += fmt.Sprintf(", bid_count %s", sortDir)
		baseQuery += fmt.Sprintf(", a.created_at %s, a.id ASC", sortDir)
	case "created_at":
		baseQuery += fmt.Sprintf(" ORDER BY %s, a.created_at %s, a.id ASC", auctionSellerTrustDemotion, sortDir)
	case "end_at":
		baseQuery += fmt.Sprintf(" ORDER BY %s, a.end_at %s, a.id ASC", auctionSellerTrustDemotion, sortDir)
	default:
		baseQuery += fmt.Sprintf(" ORDER BY %s, a.created_at DESC, a.id ASC", auctionSellerTrustDemotion)
	}

	// Add pagination
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := tx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search auctions failed: %w", err)
	}
	defer rows.Close()

	var auctions []*entity.AuctionPreview
	for rows.Next() {
		var a entity.AuctionPreview
		var thumbnailURL *string
		var sellerUsername, sellerFarmName, sellerAvatarURL string
		var sellerAccountStatus, sellerSubscriptionStatus string
		var sellerIsDeleted bool
		err := rows.Scan(
			&a.ID, &a.SellerID, &a.ProductID, &a.Title, &a.Description,
			&a.StartPrice, &a.CurrentBid, &a.BuyNowPrice,
			&a.StartAt, &a.EndAt, &a.Status, &a.CreatedAt,
			&thumbnailURL,
			&a.BidCount,
			&sellerUsername, &sellerFarmName, &sellerAvatarURL,
			&sellerAccountStatus, &sellerIsDeleted, &sellerSubscriptionStatus,
		)
		if err != nil {
			return nil, 0, err
		}

		a.ThumbnailURL = thumbnailURL
		a.SellerUsername = sellerUsername
		a.SellerFarmName = sellerFarmName
		a.SellerAvatarURL = sellerAvatarURL
		a.SellerAccountStatus = sellerAccountStatus
		a.SellerIsDeleted = sellerIsDeleted
		a.SellerSubscriptionStatus = sellerSubscriptionStatus
		auctions = append(auctions, &a)
	}

	// Get total count for pagination
	// NOTE: must mirror the banned/deleted filter from the base query (E2B1).
	countQuery := `
		SELECT COUNT(DISTINCT a.id)
		FROM auctions a
		JOIN products prod ON prod.id = a.product_id
		LEFT JOIN users u ON u.id = a.seller_id
		WHERE a.status IN ('scheduled', 'active')
			AND (u.id IS NULL OR (u.account_status != 'banned' AND u.deleted_at IS NULL))
	`
	countArgs := []interface{}{}
	countArgIdx := 1

	if filters.Query != "" {
		countQuery += fmt.Sprintf(" AND (prod.title ILIKE $%d OR prod.description ILIKE $%d)", countArgIdx, countArgIdx+1)
		countArgs = append(countArgs, "%"+filters.Query+"%", "%"+filters.Query+"%")
	}

	var total int
	if err := tx.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search auctions failed: %w", err)
	}

	return auctions, total, nil
}

// Ensure SearchRepositoryImpl implements the interface
var _ searchRepo.SearchRepository = (*SearchRepositoryImpl)(nil)
