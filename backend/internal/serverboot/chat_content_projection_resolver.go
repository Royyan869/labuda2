package serverboot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

type contentProjectionBatchResolver struct {
	db contentProjectionDB
}

type contentProjectionDB interface {
	WithTx(ctx context.Context, fn func(db.Tx) error) error
}

type contentSourceRow struct {
	contentID  uuid.UUID
	authorID   uuid.UUID
	status     string
	visibility string
	isHidden   bool
	caption    sql.NullString
	createdAt  time.Time
	deletedAt  sql.NullTime
}

type contentResourceOccurrenceRow struct {
	contentID              uuid.UUID
	profileSourceID        sql.NullString
	contentSourceID        sql.NullString
	forSaleSourceID sql.NullString
	auctionSourceID        sql.NullString
}

type contentAuthorRow struct {
	userID        uuid.UUID
	username      string
	avatarURL     sql.NullString
	accountStatus string
	deletedAt     sql.NullTime
}

type contentMediaRow struct {
	contentID uuid.UUID
	mediaURL  string
	mediaType string
	position  int
}

type nestedShareReferenceTarget struct {
	targetType entity.ShareTargetType
	targetID   uuid.UUID
}

func newContentProjectionBatchResolver(database contentProjectionDB) *contentProjectionBatchResolver {
	return &contentProjectionBatchResolver{db: database}
}

func (r *contentProjectionBatchResolver) ResolveContents(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	if len(occurrences) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("chat: content projection resolver not configured")
	}

	contentIDs := make([]uuid.UUID, 0, len(occurrences))
	seenContentIDs := make(map[uuid.UUID]struct{}, len(occurrences))
	occurrenceByMsgID := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(occurrences))

	for messageID, occ := range occurrences {
		if occ == nil {
			return nil, fmt.Errorf("chat: nil occurrence for message %s", messageID)
		}
		if !occ.ResourceType().IsValid() {
			return nil, fmt.Errorf("chat: malformed occurrence identity for message %s", messageID)
		}
		if occ.ResourceType() != chatEntity.ResourceOccurrenceResourceTypeContent {
			return nil, fmt.Errorf("chat: unsupported resource type %q in content resolver", occ.ResourceType())
		}
		if occ.ContentSourceID == nil {
			return nil, fmt.Errorf("chat: content occurrence for message %s requires non-nil content source id", messageID)
		}
		if _, seen := seenContentIDs[*occ.ContentSourceID]; !seen {
			contentIDs = append(contentIDs, *occ.ContentSourceID)
			seenContentIDs[*occ.ContentSourceID] = struct{}{}
		}
		occurrenceByMsgID[messageID] = occ
	}

	if len(contentIDs) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}

	contentByID := make(map[uuid.UUID]contentSourceRow, len(contentIDs))
	occurrenceByContentID := make(map[uuid.UUID]contentResourceOccurrenceRow, len(contentIDs))
	authorIDs := make([]uuid.UUID, 0, len(contentIDs))
	seenAuthorIDs := make(map[uuid.UUID]struct{}, len(contentIDs))
	var authorRows map[uuid.UUID]contentAuthorRow
	var mediaByContentID map[uuid.UUID][]mediaref.MediaRef
	var blockedSet map[uuid.UUID]bool
	var followedSet map[uuid.UUID]bool
	var result map[uuid.UUID]*chatApp.ResourceProjection

	err := r.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				c.id,
				c.author_id,
				c.status,
				c.visibility,
				c.is_hidden,
				c.caption,
				c.created_at,
				c.deleted_at
			FROM contents c
			WHERE c.id = ANY($1)
		`, contentIDs)
		if err != nil {
			return fmt.Errorf("chat: content source batch query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row contentSourceRow
			if err := rows.Scan(
				&row.contentID,
				&row.authorID,
				&row.status,
				&row.visibility,
				&row.isHidden,
				&row.caption,
				&row.createdAt,
				&row.deletedAt,
			); err != nil {
				return fmt.Errorf("chat: content source batch scan failed: %w", err)
			}
			contentByID[row.contentID] = row
			if row.authorID == uuid.Nil {
				return fmt.Errorf("chat: content source row missing author id for %s", row.contentID)
			}
			if _, seen := seenAuthorIDs[row.authorID]; !seen {
				authorIDs = append(authorIDs, row.authorID)
				seenAuthorIDs[row.authorID] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: content source batch iteration failed: %w", err)
		}

		for _, contentID := range contentIDs {
			if _, ok := contentByID[contentID]; !ok {
				return fmt.Errorf("chat: content source row missing for %s", contentID)
			}
		}

		occurrenceByContentID, err = loadContentResourceOccurrences(ctx, tx, contentIDs)
		if err != nil {
			return err
		}

		rows, err = tx.Query(ctx, `
			SELECT
				u.id,
				COALESCE(up.username, '') AS username,
				up.avatar_url,
				u.account_status,
				u.deleted_at
			FROM users u
			LEFT JOIN user_profiles up ON up.user_id = u.id
			WHERE u.id = ANY($1)
		`, authorIDs)
		if err != nil {
			return fmt.Errorf("chat: content author batch query failed: %w", err)
		}
		defer rows.Close()

		authorRows = make(map[uuid.UUID]contentAuthorRow, len(authorIDs))
		for rows.Next() {
			var row contentAuthorRow
			if err := rows.Scan(
				&row.userID,
				&row.username,
				&row.avatarURL,
				&row.accountStatus,
				&row.deletedAt,
			); err != nil {
				return fmt.Errorf("chat: content author batch scan failed: %w", err)
			}
			authorRows[row.userID] = row
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: content author batch iteration failed: %w", err)
		}

		for _, authorID := range authorIDs {
			if _, ok := authorRows[authorID]; !ok {
				return fmt.Errorf("chat: content author row missing for %s", authorID)
			}
		}

		blockedSet = map[uuid.UUID]bool{}
		if viewerID != uuid.Nil {
			var blockErr error
			blockedSet, blockErr = blockcheck.BlockedSet(ctx, tx, viewerID, authorIDs)
			if blockErr != nil {
				return fmt.Errorf("chat: content block batch query failed: %w", blockErr)
			}
		}

		followTargets := make([]uuid.UUID, 0, len(authorIDs))
		seenFollowTargets := make(map[uuid.UUID]struct{}, len(authorIDs))
		for _, contentID := range contentIDs {
			row := contentByID[contentID]
			if entity.Visibility(row.visibility) != entity.VisibilityFollowersOnly {
				continue
			}
			if viewerID == uuid.Nil || viewerID == row.authorID {
				continue
			}
			if _, seen := seenFollowTargets[row.authorID]; seen {
				continue
			}
			followTargets = append(followTargets, row.authorID)
			seenFollowTargets[row.authorID] = struct{}{}
		}

		followedSet = map[uuid.UUID]bool{}
		if len(followTargets) > 0 {
			rows, err = tx.Query(ctx, `
				SELECT following_id
				FROM user_follows
				WHERE follower_id = $1
				  AND following_id = ANY($2)
			`, viewerID, followTargets)
			if err != nil {
				return fmt.Errorf("chat: content follow batch query failed: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var followingID uuid.UUID
				if err := rows.Scan(&followingID); err != nil {
					return fmt.Errorf("chat: content follow batch scan failed: %w", err)
				}
				followedSet[followingID] = true
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("chat: content follow batch iteration failed: %w", err)
			}
		}

		mediaByContentID = make(map[uuid.UUID][]mediaref.MediaRef, len(contentIDs))
		rows, err = tx.Query(ctx, `
			SELECT
				content_id,
				media_url,
				media_type,
				position
			FROM content_media
			WHERE content_id = ANY($1)
			ORDER BY content_id, position ASC
		`, contentIDs)
		if err != nil {
			return fmt.Errorf("chat: content media batch query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row contentMediaRow
			if err := rows.Scan(&row.contentID, &row.mediaURL, &row.mediaType, &row.position); err != nil {
				return fmt.Errorf("chat: content media batch scan failed: %w", err)
			}
			resolvedURL := resolveReadableMediaReference(row.mediaURL)
			if resolvedURL == "" {
				continue
			}
			kind := row.mediaType
			mediaByContentID[row.contentID] = append(mediaByContentID[row.contentID], mediaref.MediaRef{
				URL:  resolvedURL,
				Kind: &kind,
			})
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: content media batch iteration failed: %w", err)
		}

		nestedTargetsByContentID := make(map[uuid.UUID]nestedShareReferenceTarget, len(occurrenceByMsgID))
		for messageID, occ := range occurrenceByMsgID {
			contentID := occ.ContentSourceID
			if contentID == nil {
				return fmt.Errorf("chat: content occurrence for message %s requires non-nil content source id", messageID)
			}

			row := contentByID[*contentID]
			authorRow := authorRows[row.authorID]
			lifecycle := viewercontext.CoarsenLifecycle(authorRow.accountStatus, authorRow.deletedAt.Valid)
			if !contentCanResolveToLive(viewerID, row, lifecycle, blockedSet, followedSet) {
				continue
			}

			occurrence, ok := occurrenceByContentID[*contentID]
			if !ok {
				continue
			}
			if target, ok, err := occurrence.nestedTarget(); err != nil {
				return err
			} else if ok {
				nestedTargetsByContentID[*contentID] = target
			}
		}

		nestedIndicators, err := r.resolveNestedResourceIndicators(ctx, tx, viewerID, nestedTargetsByContentID)
		if err != nil {
			return err
		}

		result = make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrenceByMsgID))
		for messageID, occ := range occurrenceByMsgID {
			contentID := occ.ContentSourceID
			if contentID == nil {
				return fmt.Errorf("chat: content occurrence for message %s requires non-nil content source id", messageID)
			}

			row := contentByID[*contentID]
			authorRow := authorRows[row.authorID]
			lifecycle := viewercontext.CoarsenLifecycle(authorRow.accountStatus, authorRow.deletedAt.Valid)
			if !contentCanResolveToLive(viewerID, row, lifecycle, blockedSet, followedSet) {
				proj, projErr := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeContent)
				if projErr != nil {
					return projErr
				}
				result[messageID] = &proj
				continue
			}

			caption := nullStringPtr(row.caption)
			avatarURL := resolvedAvatarURL(authorRow.avatarURL)
			author := publiccard.NewWithLifecycle(
				row.authorID,
				authorRow.username,
				avatarURL,
				string(lifecycle),
			)

			payload := chatApp.ContentLivePayload{
				Caption:   caption,
				Media:     cloneMediaRefs(mediaByContentID[row.contentID]),
				Lifecycle: string(lifecycle),
				CreatedAt: row.createdAt.UTC().Format(time.RFC3339),
				Author:    author,
			}
			if nested, ok := nestedIndicators[*contentID]; ok {
				payload.NestedResource = nested
			}

			proj, projErr := chatApp.NewLiveProjection(
				chatEntity.ResourceOccurrenceResourceTypeContent,
				row.contentID,
				payload,
				chatApp.ProjectionViewerCapabilities{
					CanView:            true,
					CanInteract:        false,
					BlockedByTombstone: false,
				},
				nil,
			)
			if projErr != nil {
				return projErr
			}
			result[messageID] = &proj
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func contentCanResolveToLive(
	viewerID uuid.UUID,
	row contentSourceRow,
	lifecycle viewercontext.PublicLifecycleState,
	blockedSet map[uuid.UUID]bool,
	followedSet map[uuid.UUID]bool,
) bool {
	if row.status != string(entity.StatusActive) {
		return false
	}
	if row.deletedAt.Valid || row.isHidden {
		return false
	}
	if lifecycle != viewercontext.PublicLifecycleStateActive {
		return false
	}
	if viewerID != uuid.Nil && viewerID != row.authorID && blockedSet[row.authorID] {
		return false
	}

	visibility := entity.Visibility(row.visibility)
	switch visibility {
	case entity.VisibilityPublic:
		return true
	case entity.VisibilityFollowersOnly:
		if viewerID == row.authorID {
			return true
		}
		if viewerID == uuid.Nil {
			return false
		}
		return followedSet[row.authorID]
	case entity.VisibilityPrivate:
		return viewerID == row.authorID
	default:
		return false
	}
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func resolvedAvatarURL(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(v.String)
	if trimmed == "" {
		return nil
	}
	resolved, err := mediaresolve.ResolveMediaReadURL(trimmed)
	if err != nil {
		v := trimmed
		return &v
	}
	return &resolved
}

func cloneMediaRefs(in []mediaref.MediaRef) []mediaref.MediaRef {
	if len(in) == 0 {
		return []mediaref.MediaRef{}
	}
	out := make([]mediaref.MediaRef, len(in))
	copy(out, in)
	return out
}

func resolveReadableMediaReference(value string) string {
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

func (r *contentProjectionBatchResolver) resolveNestedResourceIndicators(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetsByContentID map[uuid.UUID]nestedShareReferenceTarget,
) (map[uuid.UUID]*chatApp.NestedResourceIndicator, error) {
	if len(targetsByContentID) == 0 {
		return map[uuid.UUID]*chatApp.NestedResourceIndicator{}, nil
	}

	contentTargets := make(map[uuid.UUID][]uuid.UUID)
	profileTargets := make(map[uuid.UUID][]uuid.UUID)
	forSaleTargets := make(map[uuid.UUID][]uuid.UUID)
	auctionTargets := make(map[uuid.UUID][]uuid.UUID)

	for sourceContentID, target := range targetsByContentID {
		switch target.targetType {
		case entity.ShareTargetTypeContent:
			contentTargets[target.targetID] = append(contentTargets[target.targetID], sourceContentID)
		case entity.ShareTargetTypeProfile:
			profileTargets[target.targetID] = append(profileTargets[target.targetID], sourceContentID)
		case entity.ShareTargetTypeForSale:
			forSaleTargets[target.targetID] = append(forSaleTargets[target.targetID], sourceContentID)
		case entity.ShareTargetTypeAuction:
			auctionTargets[target.targetID] = append(auctionTargets[target.targetID], sourceContentID)
		}
	}

	result := make(map[uuid.UUID]*chatApp.NestedResourceIndicator, len(targetsByContentID))

	if len(contentTargets) > 0 {
		resolved, err := r.resolveNestedContentIndicators(ctx, tx, viewerID, contentTargets)
		if err != nil {
			return nil, err
		}
		for k, v := range resolved {
			result[k] = v
		}
	}

	if len(profileTargets) > 0 {
		resolved, err := r.resolveNestedProfileIndicators(ctx, tx, viewerID, profileTargets)
		if err != nil {
			return nil, err
		}
		for k, v := range resolved {
			result[k] = v
		}
	}

	if len(forSaleTargets) > 0 {
		resolved, err := r.resolveNestedForSaleIndicators(ctx, tx, viewerID, forSaleTargets)
		if err != nil {
			return nil, err
		}
		for k, v := range resolved {
			result[k] = v
		}
	}

	if len(auctionTargets) > 0 {
		resolved, err := r.resolveNestedAuctionIndicators(ctx, tx, viewerID, auctionTargets)
		if err != nil {
			return nil, err
		}
		for k, v := range resolved {
			result[k] = v
		}
	}

	return result, nil
}

func (r *contentProjectionBatchResolver) resolveNestedContentIndicators(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetsByContentID map[uuid.UUID][]uuid.UUID,
) (map[uuid.UUID]*chatApp.NestedResourceIndicator, error) {
	if len(targetsByContentID) == 0 {
		return map[uuid.UUID]*chatApp.NestedResourceIndicator{}, nil
	}

	contentIDs := make([]uuid.UUID, 0, len(targetsByContentID))
	for contentID := range targetsByContentID {
		contentIDs = append(contentIDs, contentID)
	}

	contentByID := make(map[uuid.UUID]contentSourceRow, len(contentIDs))
	authorIDs := make([]uuid.UUID, 0, len(contentIDs))
	seenAuthorIDs := make(map[uuid.UUID]struct{}, len(contentIDs))
	var authorRows map[uuid.UUID]contentAuthorRow
	var blockedSet map[uuid.UUID]bool
	var followedSet map[uuid.UUID]bool

	rows, err := tx.Query(ctx, `
		SELECT
			c.id,
			c.author_id,
			c.status,
			c.visibility,
			c.is_hidden,
			c.caption,
			c.created_at,
			c.deleted_at
		FROM contents c
		WHERE c.id = ANY($1)
	`, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: nested content source batch query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row contentSourceRow
		if err := rows.Scan(
			&row.contentID,
			&row.authorID,
			&row.status,
			&row.visibility,
			&row.isHidden,
			&row.caption,
			&row.createdAt,
			&row.deletedAt,
		); err != nil {
			return nil, fmt.Errorf("chat: nested content source batch scan failed: %w", err)
		}
		contentByID[row.contentID] = row
		if row.authorID == uuid.Nil {
			continue
		}
		if _, seen := seenAuthorIDs[row.authorID]; !seen {
			authorIDs = append(authorIDs, row.authorID)
			seenAuthorIDs[row.authorID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: nested content source batch iteration failed: %w", err)
	}

	authorRows = make(map[uuid.UUID]contentAuthorRow, len(authorIDs))
	if len(authorIDs) > 0 {
		rows, err = tx.Query(ctx, `
			SELECT
				u.id,
				COALESCE(up.username, '') AS username,
				up.avatar_url,
				u.account_status,
				u.deleted_at
			FROM users u
			LEFT JOIN user_profiles up ON up.user_id = u.id
			WHERE u.id = ANY($1)
		`, authorIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested content author batch query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row contentAuthorRow
			if err := rows.Scan(
				&row.userID,
				&row.username,
				&row.avatarURL,
				&row.accountStatus,
				&row.deletedAt,
			); err != nil {
				return nil, fmt.Errorf("chat: nested content author batch scan failed: %w", err)
			}
			authorRows[row.userID] = row
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("chat: nested content author batch iteration failed: %w", err)
		}
	}

	blockedSet = map[uuid.UUID]bool{}
	if viewerID != uuid.Nil && len(authorIDs) > 0 {
		blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, authorIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested content block batch query failed: %w", err)
		}
	}

	followTargets := make([]uuid.UUID, 0, len(authorIDs))
	seenFollowTargets := make(map[uuid.UUID]struct{}, len(authorIDs))
	for _, contentID := range contentIDs {
		row, ok := contentByID[contentID]
		if !ok {
			continue
		}
		if entity.Visibility(row.visibility) != entity.VisibilityFollowersOnly {
			continue
		}
		if viewerID == uuid.Nil || viewerID == row.authorID {
			continue
		}
		if _, seen := seenFollowTargets[row.authorID]; seen {
			continue
		}
		followTargets = append(followTargets, row.authorID)
		seenFollowTargets[row.authorID] = struct{}{}
	}

	followedSet = map[uuid.UUID]bool{}
	if len(followTargets) > 0 {
		rows, err = tx.Query(ctx, `
			SELECT following_id
			FROM user_follows
			WHERE follower_id = $1
			  AND following_id = ANY($2)
		`, viewerID, followTargets)
		if err != nil {
			return nil, fmt.Errorf("chat: nested content follow batch query failed: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var followingID uuid.UUID
			if err := rows.Scan(&followingID); err != nil {
				return nil, fmt.Errorf("chat: nested content follow batch scan failed: %w", err)
			}
			followedSet[followingID] = true
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("chat: nested content follow batch iteration failed: %w", err)
		}
	}

	result := make(map[uuid.UUID]*chatApp.NestedResourceIndicator, len(targetsByContentID))
	for targetContentID, sourceIDs := range targetsByContentID {
		row, ok := contentByID[targetContentID]
		if !ok {
			continue
		}
		authorRow, ok := authorRows[row.authorID]
		if !ok {
			continue
		}

		lifecycle := viewercontext.CoarsenLifecycle(authorRow.accountStatus, authorRow.deletedAt.Valid)
		if !contentCanResolveToLive(viewerID, row, lifecycle, blockedSet, followedSet) {
			continue
		}

		for _, sourceContentID := range sourceIDs {
			indicator := &chatApp.NestedResourceIndicator{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeContent,
				ResourceID:   row.contentID,
			}
			result[sourceContentID] = indicator
		}
	}

	return result, nil
}

func loadContentResourceOccurrences(
	ctx context.Context,
	tx db.Tx,
	contentIDs []uuid.UUID,
) (map[uuid.UUID]contentResourceOccurrenceRow, error) {
	if len(contentIDs) == 0 {
		return map[uuid.UUID]contentResourceOccurrenceRow{}, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT
			content_id,
			profile_source_id,
			content_source_id,
			for_sale_source_id,
			auction_source_id
		FROM content_resource_occurrences
		WHERE content_id = ANY($1)
	`, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: content occurrence batch query failed: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]contentResourceOccurrenceRow, len(contentIDs))
	for rows.Next() {
		var row contentResourceOccurrenceRow
		if err := rows.Scan(
			&row.contentID,
			&row.profileSourceID,
			&row.contentSourceID,
			&row.forSaleSourceID,
			&row.auctionSourceID,
		); err != nil {
			return nil, fmt.Errorf("chat: content occurrence batch scan failed: %w", err)
		}
		result[row.contentID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: content occurrence batch iteration failed: %w", err)
	}

	return result, nil
}

func (r contentResourceOccurrenceRow) nestedTarget() (nestedShareReferenceTarget, bool, error) {
	var (
		targetType entity.ShareTargetType
		targetID   uuid.UUID
		found      bool
	)

	setTarget := func(kind entity.ShareTargetType, value sql.NullString) error {
		if !value.Valid {
			return nil
		}
		parsedID, err := uuid.Parse(strings.TrimSpace(value.String))
		if err != nil || parsedID == uuid.Nil {
			if err != nil {
				return fmt.Errorf("chat: invalid content occurrence target id: %w", err)
			}
			return fmt.Errorf("chat: invalid content occurrence target id: nil uuid")
		}
		if found {
			return fmt.Errorf("chat: content occurrence row has multiple source ids for %s", r.contentID)
		}
		targetType = kind
		targetID = parsedID
		found = true
		return nil
	}

	if err := setTarget(entity.ShareTargetTypeProfile, r.profileSourceID); err != nil {
		return nestedShareReferenceTarget{}, false, err
	}
	if err := setTarget(entity.ShareTargetTypeContent, r.contentSourceID); err != nil {
		return nestedShareReferenceTarget{}, false, err
	}
	if err := setTarget(entity.ShareTargetTypeForSale, r.forSaleSourceID); err != nil {
		return nestedShareReferenceTarget{}, false, err
	}
	if err := setTarget(entity.ShareTargetTypeAuction, r.auctionSourceID); err != nil {
		return nestedShareReferenceTarget{}, false, err
	}

	if !found {
		return nestedShareReferenceTarget{}, false, nil
	}

	return nestedShareReferenceTarget{
		targetType: targetType,
		targetID:   targetID,
	}, true, nil
}

func (r *contentProjectionBatchResolver) resolveNestedProfileIndicators(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetsByProfileID map[uuid.UUID][]uuid.UUID,
) (map[uuid.UUID]*chatApp.NestedResourceIndicator, error) {
	if len(targetsByProfileID) == 0 {
		return map[uuid.UUID]*chatApp.NestedResourceIndicator{}, nil
	}

	profileIDs := make([]uuid.UUID, 0, len(targetsByProfileID))
	for profileID := range targetsByProfileID {
		profileIDs = append(profileIDs, profileID)
	}

	type profileTargetRow struct {
		userID        uuid.UUID
		accountStatus string
		deletedAt     sql.NullTime
	}

	byID := make(map[uuid.UUID]profileTargetRow, len(profileIDs))
	rows, err := tx.Query(ctx, `
		SELECT
			u.id,
			u.account_status,
			u.deleted_at
		FROM users u
		JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = ANY($1)
	`, profileIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: nested profile source batch query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row profileTargetRow
		if err := rows.Scan(&row.userID, &row.accountStatus, &row.deletedAt); err != nil {
			return nil, fmt.Errorf("chat: nested profile source batch scan failed: %w", err)
		}
		byID[row.userID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: nested profile source batch iteration failed: %w", err)
	}

	blockedSet := map[uuid.UUID]bool{}
	if viewerID != uuid.Nil {
		blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, profileIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested profile block batch query failed: %w", err)
		}
	}

	result := make(map[uuid.UUID]*chatApp.NestedResourceIndicator, len(targetsByProfileID))
	for profileID, sourceIDs := range targetsByProfileID {
		row, ok := byID[profileID]
		if !ok {
			continue
		}

		lifecycle := viewercontext.CoarsenLifecycle(row.accountStatus, row.deletedAt.Valid)
		if lifecycle != viewercontext.PublicLifecycleStateActive {
			continue
		}
		if viewerID != uuid.Nil && viewerID != profileID && blockedSet[profileID] {
			continue
		}

		for _, sourceContentID := range sourceIDs {
			indicator := &chatApp.NestedResourceIndicator{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeProfile,
				ResourceID:   profileID,
			}
			result[sourceContentID] = indicator
		}
	}

	return result, nil
}

func (r *contentProjectionBatchResolver) resolveNestedForSaleIndicators(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetsBySaleID map[uuid.UUID][]uuid.UUID,
) (map[uuid.UUID]*chatApp.NestedResourceIndicator, error) {
	if len(targetsBySaleID) == 0 {
		return map[uuid.UUID]*chatApp.NestedResourceIndicator{}, nil
	}

	saleIDs := make([]uuid.UUID, 0, len(targetsBySaleID))
	for saleID := range targetsBySaleID {
		saleIDs = append(saleIDs, saleID)
	}

	type saleTargetRow struct {
		saleID      uuid.UUID
		sellerID    uuid.UUID
		status      string
		publishedAt sql.NullTime
	}

	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, status, published_at
		FROM for_sales
		WHERE id = ANY($1)
	`, saleIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: nested fixed price sale source batch query failed: %w", err)
	}
	defer rows.Close()

	byID := make(map[uuid.UUID]saleTargetRow, len(saleIDs))
	sellerIDs := make([]uuid.UUID, 0, len(saleIDs))
	seenSellerIDs := make(map[uuid.UUID]struct{}, len(saleIDs))
	for rows.Next() {
		var row saleTargetRow
		if err := rows.Scan(&row.saleID, &row.sellerID, &row.status, &row.publishedAt); err != nil {
			return nil, fmt.Errorf("chat: nested fixed price sale source batch scan failed: %w", err)
		}
		byID[row.saleID] = row
		if row.sellerID == uuid.Nil {
			continue
		}
		if _, seen := seenSellerIDs[row.sellerID]; !seen {
			sellerIDs = append(sellerIDs, row.sellerID)
			seenSellerIDs[row.sellerID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: nested fixed price sale source batch iteration failed: %w", err)
	}

	blockedSet := map[uuid.UUID]bool{}
	if viewerID != uuid.Nil && len(sellerIDs) > 0 {
		blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested fixed price sale block batch query failed: %w", err)
		}
	}

	sellerInfos := map[uuid.UUID]sellerdisplay.Info{}
	if len(sellerIDs) > 0 {
		sellerInfos, err = sellerdisplay.FetchMany(ctx, tx, sellerIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested fixed price sale seller batch query failed: %w", err)
		}
	}

	result := make(map[uuid.UUID]*chatApp.NestedResourceIndicator, len(targetsBySaleID))
	for saleID, sourceIDs := range targetsBySaleID {
		row, ok := byID[saleID]
		if !ok {
			continue
		}
		sellerInfo := sellerInfos[row.sellerID]
		var publishedAt *time.Time
		if row.publishedAt.Valid {
			publishedAt = &row.publishedAt.Time
		}
		visibility := fpsEntity.DeriveVisibility(fpsEntity.ForSaleStatus(row.status), publishedAt)
		if !commerceshared.EvaluateForSaleViewAccess(commerceshared.ForSaleViewAccessInput{
			ViewerID:   viewerID,
			SellerID:   row.sellerID,
			Status:     row.status,
			Visibility: string(visibility),
			Blocked:    viewerID != uuid.Nil && viewerID != row.sellerID && blockedSet[row.sellerID],
			Seller: commerceshared.SellerAccessSnapshot{
				AccountStatus:      sellerInfo.AccountStatus,
				IsDeleted:          sellerInfo.IsDeleted,
				SubscriptionStatus: sellerInfo.SubscriptionStatus,
			},
		}) {
			continue
		}

		for _, sourceContentID := range sourceIDs {
			indicator := &chatApp.NestedResourceIndicator{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeForSale,
				ResourceID:   saleID,
			}
			result[sourceContentID] = indicator
		}
	}

	return result, nil
}

func (r *contentProjectionBatchResolver) resolveNestedAuctionIndicators(
	ctx context.Context,
	tx db.Tx,
	viewerID uuid.UUID,
	targetsByAuctionID map[uuid.UUID][]uuid.UUID,
) (map[uuid.UUID]*chatApp.NestedResourceIndicator, error) {
	if len(targetsByAuctionID) == 0 {
		return map[uuid.UUID]*chatApp.NestedResourceIndicator{}, nil
	}

	auctionIDs := make([]uuid.UUID, 0, len(targetsByAuctionID))
	for auctionID := range targetsByAuctionID {
		auctionIDs = append(auctionIDs, auctionID)
	}

	type auctionTargetRow struct {
		auctionID uuid.UUID
		sellerID  uuid.UUID
		status    string
	}

	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, status
		FROM auctions
		WHERE id = ANY($1)
	`, auctionIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: nested auction source batch query failed: %w", err)
	}
	defer rows.Close()

	byID := make(map[uuid.UUID]auctionTargetRow, len(auctionIDs))
	sellerIDs := make([]uuid.UUID, 0, len(auctionIDs))
	seenSellerIDs := make(map[uuid.UUID]struct{}, len(auctionIDs))
	for rows.Next() {
		var row auctionTargetRow
		if err := rows.Scan(&row.auctionID, &row.sellerID, &row.status); err != nil {
			return nil, fmt.Errorf("chat: nested auction source batch scan failed: %w", err)
		}
		byID[row.auctionID] = row
		if row.sellerID == uuid.Nil {
			continue
		}
		if _, seen := seenSellerIDs[row.sellerID]; !seen {
			sellerIDs = append(sellerIDs, row.sellerID)
			seenSellerIDs[row.sellerID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: nested auction source batch iteration failed: %w", err)
	}

	blockedSet := map[uuid.UUID]bool{}
	if viewerID != uuid.Nil && len(sellerIDs) > 0 {
		blockedSet, err = blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested auction block batch query failed: %w", err)
		}
	}

	sellerInfos := map[uuid.UUID]sellerdisplay.Info{}
	if len(sellerIDs) > 0 {
		sellerInfos, err = sellerdisplay.FetchMany(ctx, tx, sellerIDs)
		if err != nil {
			return nil, fmt.Errorf("chat: nested auction seller batch query failed: %w", err)
		}
	}

	result := make(map[uuid.UUID]*chatApp.NestedResourceIndicator, len(targetsByAuctionID))
	for auctionID, sourceIDs := range targetsByAuctionID {
		row, ok := byID[auctionID]
		if !ok {
			continue
		}
		sellerInfo := sellerInfos[row.sellerID]
		if !commerceshared.EvaluateAuctionViewAccess(commerceshared.AuctionViewAccessInput{
			ViewerID: viewerID,
			SellerID: row.sellerID,
			Status:   row.status,
			Blocked:  viewerID != uuid.Nil && viewerID != row.sellerID && blockedSet[row.sellerID],
			Seller: commerceshared.SellerAccessSnapshot{
				AccountStatus:      sellerInfo.AccountStatus,
				IsDeleted:          sellerInfo.IsDeleted,
				SubscriptionStatus: sellerInfo.SubscriptionStatus,
			},
		}) {
			continue
		}

		for _, sourceContentID := range sourceIDs {
			indicator := &chatApp.NestedResourceIndicator{
				ResourceType: chatEntity.ResourceOccurrenceResourceTypeAuction,
				ResourceID:   auctionID,
			}
			result[sourceContentID] = indicator
		}
	}

	return result, nil
}
