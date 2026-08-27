package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// feedRepositoryImpl implements FeedRepository.
type feedRepositoryImpl struct{}

// NewFeedRepository creates a new FeedRepository.
func NewFeedRepository() FeedRepository {
	return &feedRepositoryImpl{}
}

// GetFeed retrieves content for the user's feed.
//
// Query contract (authoritative):
//
//	SELECT c.*
//	FROM contents c
//	JOIN user_follows f ON c.author_id = f.following_id
//	LEFT JOIN users u ON u.id = c.author_id        -- E2: lifecycle hydration
//	WHERE f.follower_id = $1
//	  AND c.status = 'active'
//	  AND c.is_hidden = false       -- F1-W1: moderation-flagged content stays off the wire
//	  AND c.deleted_at IS NULL      -- F1-W1: belt-and-suspenders with status='active'
//	  AND u.account_status = 'active'   -- F1-B1: exclude suspended/banned authors
//	  AND u.deleted_at IS NULL          -- F1-B1: exclude deleted authors
//	  AND ($2 IS NULL OR c.created_at < $3)
//	  AND NOT EXISTS (
//	    SELECT 1 FROM user_blocks b
//	    WHERE (b.blocker_id = $1 AND b.blocked_id = c.author_id)
//	       OR (b.blocker_id = c.author_id AND b.blocked_id = $1)
//	  )
//	  -- REPOST GOVERNANCE: exclude reposts whose target is no longer available.
//	  -- FIX-1 content, FIX-3 for_sale, FIX-4 auction (2026-05-28).
//	  -- Profile reposts intentionally unchecked.
//	  AND NOT (c.original_author_id IS NOT NULL AND targetType='content' AND orig unavailable)
//	  AND NOT (c.original_author_id IS NOT NULL AND targetType='for_sale' AND fixed-price sale not active)
//	  AND NOT (c.original_author_id IS NOT NULL AND targetType='auction' AND auction terminal)
//	ORDER BY c.created_at DESC
//	LIMIT $4;
//
// NO OFFSET - cursor-based pagination only.
//
// F1-W1 (audit closure 2026-05-15): is_hidden and deleted_at filters
// added at SQL authority so the default-shadow-mode wire no longer
// surfaces moderation-flagged or soft-deleted rows. Prior to this fix
// the SQL relied on the enforce-mode evaluator to drop these rows;
// shadow-mode responses (the production default) emitted them despite
// doctrine. Filter alignment with /search/content
// (search_repository_impl.go:265,354) preserves cross-surface parity.
//
// MEDIA INTEGRATION: Feed items include media from content_media table.
// Contract: media is fetched via LEFT JOIN and aggregated per content.
func (r *feedRepositoryImpl) GetFeed(ctx context.Context, tx interface{}, viewerID uuid.UUID, cursor *feedentity.FeedCursor, limit int) (*feedentity.FeedResult, error) {
	// Apply limit safety
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Get query executor
	q, ok := tx.(queryer)
	if !ok {
		return nil, fmt.Errorf("invalid tx type: expected queryer")
	}

	// Materialise cursor parameters. The query uses a single null-flag
	// parameter ($2) plus the three tuple components ($3, $4, $5) — when $2
	// is NULL the tuple values are ignored, so we can safely pass the
	// zero values in that branch.
	var cursorPresent *bool
	var cursorPriority *int
	var cursorTs *time.Time
	var cursorID *uuid.UUID
	if cursor != nil {
		t := true
		cursorPresent = &t
		pg := cursor.PriorityGroup
		cursorPriority = &pg
		ts := cursor.CreatedAt
		cursorTs = &ts
		id := cursor.ID
		cursorID = &id
	}

	// Probe one row beyond the requested limit so we can derive
	// HasMore precisely from the row count — never from the boundary
	// equality `len(items) == limit`. The extra row (if returned) is
	// dropped before mapping into the response.
	probeLimit := limit + 1

	// Authoritative query with media integration and Follow-First Discovery Bootstrap.
	//
	// Ordering: (feed_priority ASC, created_at DESC, id DESC) — stable across pages.
	// Priority Group 0: Content from followed users + own content.
	// Priority Group 1: Public discovery content from global users (cold-start bootstrap).
	// Cursor predicate: compound comparison over (feed_priority, created_at, id).
	//
	// Privacy & Visibility:
	// - Followed users: public and followers_only content.
	// - Global discovery (unfollowed): public content only (never followers_only or private).
	// - Own content: all visibility states.
	// - Excludes blocked users (both directions).
	// - Excludes suspended/banned/deleted authors.
	// - Excludes unavailable repost targets.
	query := `
		SELECT
			c.id, c.author_id,
			CASE WHEN c.original_author_id IS NOT NULL THEN 'repost' ELSE 'post' END AS type,
			c.status,
			COALESCE(c.caption, '') AS body,
			c.caption,
			c.city, c.province,
			c.is_hidden, c.created_at, c.updated_at,
			c.original_author_id,
			up.username AS author_username,
			up.avatar_url AS author_avatar,
			CASE
				WHEN COALESCE((up.privacy->>'show_location')::boolean, false) = true THEN up.city
				ELSE NULL
			END AS author_city,
			CASE
				WHEN COALESCE((up.privacy->>'show_location')::boolean, false) = true THEN up.province
				ELSE NULL
			END AS author_province,
			-- E2 — author lifecycle hydration. Raw enum values stay inside
			-- this projection; coarsening to the canonical public
			-- vocabulary {active, unavailable, removed} happens in Go via
			-- viewercontext.CoarsenLifecycle (NEVER in SQL).
			u.account_status AS author_account_status,
			(u.deleted_at IS NOT NULL) AS author_deleted,
			COALESCE(json_agg(json_build_object(
				'url', cm.media_url,
				'type', cm.media_type,
				'position', cm.position
			) ORDER BY cm.position) FILTER (WHERE cm.id IS NOT NULL), '[]') as media,
			CASE
				WHEN c.author_id = $1 OR f.follower_id IS NOT NULL THEN 0
				ELSE 1
			END AS feed_priority
		FROM contents c
		LEFT JOIN user_follows f ON f.follower_id = $1 AND f.following_id = c.author_id
		LEFT JOIN content_resource_occurrences occ ON occ.content_id = c.id
		LEFT JOIN user_profiles up ON up.user_id = c.author_id
		LEFT JOIN users u ON u.id = c.author_id
		LEFT JOIN content_media cm ON c.id = cm.content_id
		WHERE c.status = 'active'
		  AND c.is_hidden = false
		  AND c.deleted_at IS NULL
		  -- F1-B1 (2026-06-14): exclude content from suspended/banned/deleted authors.
		  AND u.account_status = 'active'
		  AND u.deleted_at IS NULL
		  -- Visibility authorization:
		  -- 1. Owner can see own content (public, followers_only, private)
		  -- 2. Follower can see followed author content (public, followers_only)
		  -- 3. Stranger sees global discovery content (public only)
		  AND (
		      c.author_id = $1
		      OR (f.follower_id IS NOT NULL AND c.visibility IN ('public', 'followers_only'))
		      OR (f.follower_id IS NULL AND c.visibility = 'public')
		  )
		  -- Compound cursor pagination:
		  AND (
		      $2::boolean IS NULL
		      OR (
		          (CASE WHEN c.author_id = $1 OR f.follower_id IS NOT NULL THEN 0 ELSE 1 END) > $3::int
		          OR (
		              (CASE WHEN c.author_id = $1 OR f.follower_id IS NOT NULL THEN 0 ELSE 1 END) = $3::int
		              AND (c.created_at, c.id) < ($4::timestamptz, $5::uuid)
		          )
		      )
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM user_blocks b
		    WHERE (b.blocker_id = $1 AND b.blocked_id = c.author_id)
		       OR (b.blocker_id = c.author_id AND b.blocked_id = $1)
		  )
		  -- REPOST GOVERNANCE: exclude reposts whose target is no longer available.
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
		        AND a.status NOT IN ('scheduled', 'active')
		    )
		  )
		GROUP BY c.id, c.author_id, c.status, c.caption,
		         c.city, c.province,
		         c.is_hidden, c.created_at, c.updated_at,
		         c.original_author_id, c.visibility,
		         up.username, up.avatar_url, up.city, up.province, up.privacy,
		         u.account_status, u.deleted_at,
		         f.follower_id
		ORDER BY feed_priority ASC, c.created_at DESC, c.id DESC
		LIMIT $6
	`

	// Execute query
	rows, err := q.Query(ctx, query, viewerID, cursorPresent, cursorPriority, cursorTs, cursorID, probeLimit)
	if err != nil {
		return nil, fmt.Errorf("query feed: %w", err)
	}
	defer rows.Close()

	// Scan results
	var items []*feedentity.FeedItem
	var priorities []int
	for rows.Next() {
		var item feedentity.FeedItem
		var originalAuthorID *uuid.UUID
		var mediaJSON []byte
		var city, province *string
		var authorUsername, authorAvatar, authorCity, authorProvince *string
		var authorAccountStatus *string
		var authorDeleted bool
		var feedPriority int

		err := rows.Scan(
			&item.ID,
			&item.AuthorID,
			&item.Type,
			&item.Status,
			&item.Body, // SCHEMA ALIGNMENT (Batch 3J): from COALESCE(c.caption, '')
			&item.Caption,
			&city,
			&province,
			&item.IsHidden,
			&item.CreatedAt,
			&item.UpdatedAt,
			&originalAuthorID,
			&authorUsername,
			&authorAvatar,
			&authorCity,
			&authorProvince,
			&authorAccountStatus,
			&authorDeleted,
			&mediaJSON,
			&feedPriority,
		)
		if err != nil {
			return nil, fmt.Errorf("scan feed item: %w", err)
		}

		// Set original author ID
		item.OriginalAuthorID = originalAuthorID

		// POST LOCATION: Set city and province
		item.City = city
		item.Province = province

		// AUTHOR LOCATION: Set author username, avatar, city, province
		item.AuthorUsername = authorUsername
		item.AuthorAvatar = authorAvatar
		item.AuthorCity = authorCity
		item.AuthorProvince = authorProvince

		// E2 — coarsen raw users.account_status + users.deleted_at into the
		// canonical public lifecycle vocabulary. The raw enum values never
		// leave this layer; the public lifecycle string is what flows into
		// the UserCard.Lifecycle wire slot via the handler call site.
		//
		// Defensive: a missing users row (impossible under the
		// contents.author_id → users(id) FK, but tolerated by LEFT JOIN)
		// yields an empty raw status and authorDeleted=false, which
		// CoarsenLifecycle maps to "active". This matches the
		// publiccard.UserCard fail-open default and preserves backward
		// behavior for any unexpected NULL row.
		rawStatus := ""
		if authorAccountStatus != nil {
			rawStatus = *authorAccountStatus
		}
		item.AuthorLifecycle = string(viewercontext.CoarsenLifecycle(rawStatus, authorDeleted))

		// Parse media from JSON aggregation
		// Contract: Empty array '[]' is valid for content without media
		if len(mediaJSON) > 0 {
			var media []feedentity.FeedMedia
			if err := json.Unmarshal(mediaJSON, &media); err == nil {
				item.Media = media
			} else {
				// If media JSON parsing fails, set to empty array
				item.Media = []feedentity.FeedMedia{}
			}
		} else {
			item.Media = []feedentity.FeedMedia{}
		}

		// PHASE C — MediaRef convergence (Option D). Mirror legacy Type
		// into the canonical-compatible Kind pointer for each hydrated
		// FeedMedia element. Width/Height stay nil — no DB column, no
		// inference. Legacy Type/Position remain populated as before.
		for idx := range item.Media {
			t := item.Media[idx].Type
			item.Media[idx].Kind = &t
		}

		items = append(items, &item)
		priorities = append(priorities, feedPriority)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feed rows: %w", err)
	}

	// LIMIT+1 probe → HasMore honesty. If the DB returned more than the
	// caller-requested `limit`, another page exists. Drop the extra
	// probe row before mapping so the caller never sees it.
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	result := &feedentity.FeedResult{
		Items:   items,
		HasMore: hasMore,
	}

	// NextCursor is the (feed_priority, created_at, id) tuple of the last returned
	// row, encoded opaquely at the HTTP boundary. Only emit it when
	// another page exists — terminal pages return NextCursor=nil so
	// the client cannot mistakenly request beyond the tail.
	if hasMore && len(items) > 0 {
		lastItem := items[len(items)-1]
		lastPriority := priorities[len(items)-1]
		result.NextCursor = &feedentity.FeedCursor{
			PriorityGroup: lastPriority,
			CreatedAt:     lastItem.CreatedAt,
			ID:            lastItem.ID,
		}
	}

	return result, nil
}

// queryer is the interface for executing queries.
// Matches db.Tx interface from pkg/db.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Verify tx implements queryer at compile time.
var _ queryer = (interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
})(nil)
