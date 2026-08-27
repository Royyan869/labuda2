package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// ContentRepositoryImpl handles content persistence using pgx-based DB layer.
type ContentRepositoryImpl struct{}

// NewContentRepository creates a new ContentRepository.
func NewContentRepository() *ContentRepositoryImpl {
	return &ContentRepositoryImpl{}
}

// Create persists a new content within a transaction.
func (r *ContentRepositoryImpl) Create(
	ctx context.Context,
	tx interface{},
	content *entity.Content,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO contents (
			id, author_id, status, caption,
			city, province,
			is_hidden,
			original_author_id,
			visibility,
			created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		content.ID,
		content.AuthorID,
		string(content.Status),
		content.Caption,
		content.City,
		content.Province,
		content.IsHidden,
		content.OriginalAuthorID,
		string(content.Visibility.Normalize()),
		content.CreatedAt,
		content.UpdatedAt,
		content.DeletedAt,
	)

	if err != nil {
		return fmt.Errorf("create content failed: %w", err)
	}

	return nil
}

// CreateMedia persists media attachments within a transaction.
func (r *ContentRepositoryImpl) CreateMedia(
	ctx context.Context,
	tx interface{},
	media []*entity.ContentMedia,
) error {
	if len(media) == 0 {
		return nil
	}

	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	for _, m := range media {
		_, err := dbTx.Exec(ctx, `
			INSERT INTO content_media (id, content_id, media_url, media_type, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			m.ID,
			m.ContentID,
			m.MediaURL,
			string(m.MediaType),
			m.Position,
			m.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("create content media failed: %w", err)
		}
	}

	return nil
}

// GetByID retrieves content without locking (for read-only operations).
func (r *ContentRepositoryImpl) GetByID(
	ctx context.Context,
	tx interface{},
	id uuid.UUID,
) (*entity.Content, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var authorID uuid.UUID
	var status string
	var caption *string
	var city, province *string
	var isHidden bool
	var createdAt, updatedAt time.Time
	var deletedAt *time.Time
	var originalAuthorIDPtr *uuid.UUID
	var visibility string

	err := dbTx.QueryRow(ctx, `
		SELECT id, author_id, status, caption,
		       city, province,
		       is_hidden,
		       original_author_id, visibility,
		       created_at, updated_at, deleted_at
		FROM contents
		WHERE id = $1
	`, id).Scan(
		&id, &authorID, &status, &caption,
		&city, &province,
		&isHidden,
		&originalAuthorIDPtr, &visibility,
		&createdAt, &updatedAt, &deletedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("content not found: %s", id)
		}
		return nil, fmt.Errorf("get content failed: %w", err)
	}

	content := &entity.Content{
		ID:               id,
		AuthorID:         authorID,
		Status:           entity.Status(status),
		Caption:          caption,
		City:             city,
		Province:         province,
		IsHidden:         isHidden,
		OriginalAuthorID: originalAuthorIDPtr,
		Visibility:       entity.Visibility(visibility).Normalize(),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		DeletedAt:        deletedAt,
	}

	// Fetch tags in the same transaction (fail-open: missing tags are non-fatal).
	if tags, err := r.GetTagsByContentID(ctx, tx, content.ID); err == nil {
		content.Tags = tags
	}

	return content, nil
}

// GetForUpdate retrieves content with FOR UPDATE lock.
func (r *ContentRepositoryImpl) GetForUpdate(
	ctx context.Context,
	tx interface{},
	id uuid.UUID,
) (*entity.Content, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var authorID uuid.UUID
	var status string
	var caption *string
	var city, province *string
	var isHidden bool
	var createdAt, updatedAt time.Time
	var deletedAt *time.Time
	var originalAuthorIDPtr *uuid.UUID
	var visibility string

	err := dbTx.QueryRow(ctx, `
		SELECT id, author_id, status, caption,
		       city, province,
		       is_hidden,
		       original_author_id, visibility,
		       created_at, updated_at, deleted_at
		FROM contents
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&id, &authorID, &status, &caption,
		&city, &province,
		&isHidden,
		&originalAuthorIDPtr, &visibility,
		&createdAt, &updatedAt, &deletedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("content not found: %s", id)
		}
		return nil, fmt.Errorf("get content for update failed: %w", err)
	}

	return &entity.Content{
		ID:               id,
		AuthorID:         authorID,
		Status:           entity.Status(status),
		Caption:          caption,
		City:             city,
		Province:         province,
		IsHidden:         isHidden,
		OriginalAuthorID: originalAuthorIDPtr,
		Visibility:       entity.Visibility(visibility).Normalize(),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		DeletedAt:        deletedAt,
	}, nil
}

// Update persists content changes within a transaction.
func (r *ContentRepositoryImpl) Update(
	ctx context.Context,
	tx interface{},
	content *entity.Content,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	_, err := dbTx.Exec(ctx, `
		UPDATE contents
		SET caption = $2, city = $3, province = $4, status = $5, is_hidden = $6,
		    original_author_id = $7, visibility = $8,
		    updated_at = $9, deleted_at = $10
		WHERE id = $1
	`,
		content.ID,
		content.Caption,
		content.City,
		content.Province,
		string(content.Status),
		content.IsHidden,
		content.OriginalAuthorID,
		string(content.Visibility.Normalize()),
		content.UpdatedAt,
		content.DeletedAt,
	)

	if err != nil {
		return fmt.Errorf("update content failed: %w", err)
	}

	return nil
}

// ListByAuthor retrieves content by author ID with cursor-based pagination.
func (r *ContentRepositoryImpl) ListByAuthor(
	ctx context.Context,
	tx interface{},
	authorID uuid.UUID,
	limit int,
	cursor string,
) ([]*entity.Content, string, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, "", fmt.Errorf("invalid transaction type")
	}

	// F1-B1 (2026-06-14): JOIN users to exclude content from suspended/banned/deleted authors.
	// Profile feed is a public discovery surface; accounts with non-active status or deleted_at
	// set must not expose their content to other viewers.
	// Column aliases (c.) are required to avoid ambiguity with users columns (id, created_at, etc.).
	query := `
		SELECT c.id, c.author_id, c.status, c.caption,
		       c.city, c.province,
		       c.is_hidden,
		       c.original_author_id, c.visibility,
		       c.created_at, c.updated_at, c.deleted_at
		FROM contents c
		JOIN users u ON u.id = c.author_id
		WHERE c.author_id = $1 AND c.deleted_at IS NULL AND c.status = 'active' AND c.is_hidden = false
		  AND u.account_status = 'active' AND u.deleted_at IS NULL
		  AND NOT (
		    c.original_author_id IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM content_resource_occurrences occ
		      WHERE occ.content_id = c.id
		        AND occ.operation = 'share_to_feed'
		        AND occ.content_source_id IS NOT NULL
		    )
		    AND (SELECT occ.content_source_id FROM content_resource_occurrences occ WHERE occ.content_id = c.id) IS NOT NULL
		    AND EXISTS (
		      SELECT 1 FROM contents orig
		      LEFT JOIN users orig_u ON orig_u.id = orig.author_id
		      WHERE orig.id = (SELECT occ.content_source_id FROM content_resource_occurrences occ WHERE occ.content_id = c.id)
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
	`
	args := []interface{}{authorID}
	argIdx := 2

	// Cursor-based pagination
	if cursor != "" {
		query += fmt.Sprintf(" AND c.created_at < $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += " ORDER BY c.created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit+1) // Fetch one extra to determine if there's a next page

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list by author failed: %w", err)
	}
	defer rows.Close()

	var contents []*entity.Content
	var nextCursor string
	fetched := 0

	for rows.Next() {
		var id uuid.UUID
		var status string
		var caption *string
		var city, province *string
		var isHidden bool
		var createdAt, updatedAt time.Time
		var deletedAt *time.Time
		var originalAuthorIDPtr *uuid.UUID
		var visibility string

		err := rows.Scan(
			&id, &authorID, &status, &caption,
			&city, &province,
			&isHidden,
			&originalAuthorIDPtr, &visibility,
			&createdAt, &updatedAt, &deletedAt,
		)
		if err != nil {
			return nil, "", fmt.Errorf("scan content failed: %w", err)
		}

		// If we have an extra row, use it to set the next cursor
		if fetched == limit {
			nextCursor = createdAt.Format(time.RFC3339Nano)
			break
		}

		contents = append(contents, &entity.Content{
			ID:               id,
			AuthorID:         authorID,
			Status:           entity.Status(status),
			Caption:          caption,
			City:             city,
			Province:         province,
			IsHidden:         isHidden,
			OriginalAuthorID: originalAuthorIDPtr,
			Visibility:       entity.Visibility(visibility).Normalize(),
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
			DeletedAt:        deletedAt,
		})
		fetched++
	}

	return contents, nextCursor, nil
}

// GetMedia retrieves all media for a content.
func (r *ContentRepositoryImpl) GetMedia(
	ctx context.Context,
	tx interface{},
	contentID uuid.UUID,
) ([]*entity.ContentMedia, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT id, content_id, media_url, media_type, position, created_at
		FROM content_media
		WHERE content_id = $1
		ORDER BY position
	`

	rows, err := dbTx.Query(ctx, query, contentID)
	if err != nil {
		return nil, fmt.Errorf("get media failed: %w", err)
	}
	defer rows.Close()

	var media []*entity.ContentMedia
	for rows.Next() {
		var id, contentID uuid.UUID
		var mediaURL string
		var mediaType string
		var position int
		var createdAt time.Time

		err := rows.Scan(&id, &contentID, &mediaURL, &mediaType, &position, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan media failed: %w", err)
		}

		media = append(media, &entity.ContentMedia{
			ID:        id,
			ContentID: contentID,
			MediaURL:  mediaURL,
			MediaType: entity.MediaType(mediaType),
			Position:  position,
			CreatedAt: createdAt,
		})
	}

	return media, nil
}

// CreateResourceOccurrence persists the canonical occurrence row for content.
func (r *ContentRepositoryImpl) CreateResourceOccurrence(
	ctx context.Context,
	tx interface{},
	occurrence *entity.ContentResourceOccurrence,
) error {
	if occurrence == nil {
		return fmt.Errorf("occurrence is required")
	}
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	var profileSourceID, contentSourceID, fpsSourceID, auctionSourceID *uuid.UUID
	switch {
	case occurrence.ProfileSourceID != nil:
		profileSourceID = occurrence.ProfileSourceID
	case occurrence.ContentSourceID != nil:
		contentSourceID = occurrence.ContentSourceID
	case occurrence.ForSaleSourceID != nil:
		fpsSourceID = occurrence.ForSaleSourceID
	case occurrence.AuctionSourceID != nil:
		auctionSourceID = occurrence.AuctionSourceID
	default:
		return fmt.Errorf("occurrence source is required")
	}

	_, err := dbTx.Exec(ctx, `
		INSERT INTO content_resource_occurrences (
			content_id, actor_id, operation,
			profile_source_id, content_source_id, for_sale_source_id, auction_source_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		occurrence.ContentID,
		occurrence.ActorID,
		string(occurrence.Operation),
		profileSourceID,
		contentSourceID,
		fpsSourceID,
		auctionSourceID,
		occurrence.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateContentResourceOccurrence
		}
		return fmt.Errorf("create content resource occurrence failed: %w", err)
	}

	return nil
}

// GetResourceOccurrenceByContentID retrieves the canonical occurrence row.
func (r *ContentRepositoryImpl) GetResourceOccurrenceByContentID(
	ctx context.Context,
	tx interface{},
	contentID uuid.UUID,
) (*entity.ContentResourceOccurrence, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var (
		actorID         uuid.UUID
		operation       string
		profileSourceID *uuid.UUID
		contentSourceID *uuid.UUID
		fpsSourceID     *uuid.UUID
		auctionSourceID *uuid.UUID
		createdAt       time.Time
	)

	err := dbTx.QueryRow(ctx, `
		SELECT content_id, actor_id, operation,
		       profile_source_id, content_source_id,
		       for_sale_source_id, auction_source_id,
		       created_at
		FROM content_resource_occurrences
		WHERE content_id = $1
	`, contentID).Scan(
		&contentID, &actorID, &operation,
		&profileSourceID, &contentSourceID,
		&fpsSourceID, &auctionSourceID,
		&createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get content resource occurrence failed: %w", err)
	}

	return &entity.ContentResourceOccurrence{
		ContentID:              contentID,
		ActorID:                actorID,
		Operation:              entity.ContentResourceOccurrenceOperation(operation),
		ProfileSourceID:        profileSourceID,
		ContentSourceID:        contentSourceID,
		ForSaleSourceID: fpsSourceID,
		AuctionSourceID:        auctionSourceID,
		CreatedAt:              createdAt,
	}, nil
}

// GetTagsByContentID retrieves hashtags for a content item from content_hashtags.
// Returns an empty (non-nil) slice when no tags exist.
func (r *ContentRepositoryImpl) GetTagsByContentID(
	ctx context.Context,
	tx interface{},
	contentID uuid.UUID,
) ([]string, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return []string{}, fmt.Errorf("invalid transaction type")
	}

	rows, err := dbTx.Query(ctx, `
		SELECT hashtag
		FROM content_hashtags
		WHERE content_id = $1
		ORDER BY hashtag ASC
	`, contentID)
	if err != nil {
		return []string{}, fmt.Errorf("get tags failed: %w", err)
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return []string{}, fmt.Errorf("scan tag failed: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return []string{}, fmt.Errorf("tags rows error: %w", err)
	}
	return tags, nil
}

// InsertTags persists hashtags for a content item within a transaction.
// Duplicate (content_id, hashtag) rows are silently ignored.
// No-op when tags is empty.
func (r *ContentRepositoryImpl) InsertTags(
	ctx context.Context,
	tx interface{},
	contentID uuid.UUID,
	tags []string,
) error {
	if len(tags) == 0 {
		return nil
	}
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	for _, tag := range tags {
		if tag == "" {
			continue
		}
		_, err := dbTx.Exec(ctx, `
			INSERT INTO content_hashtags (content_id, hashtag)
			VALUES ($1, $2)
			ON CONFLICT (content_id, hashtag) DO NOTHING
		`, contentID, tag)
		if err != nil {
			return fmt.Errorf("insert tag %q failed: %w", tag, err)
		}
	}
	return nil
}
