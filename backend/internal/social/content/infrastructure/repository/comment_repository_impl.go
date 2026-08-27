package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// CommentRepositoryImpl handles comment persistence using pgx-based DB layer.
type CommentRepositoryImpl struct{}

// NewCommentRepository creates a new CommentRepository.
func NewCommentRepository() *CommentRepositoryImpl {
	return &CommentRepositoryImpl{}
}

// Create persists a new comment within a transaction.
func (r *CommentRepositoryImpl) Create(
	ctx context.Context,
	tx db.Tx,
	comment *entity.Comment,
) error {
	if comment == nil {
		return fmt.Errorf("comment is required")
	}

	var query string
	var args []any
	if comment.ParentID != nil && *comment.ParentID != uuid.Nil {
		query = `
			INSERT INTO comments (
				id, author_id, body, target_id, target_type,
				parent_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		args = []any{
			comment.ID, comment.AuthorID, comment.Body,
			comment.TargetID, comment.TargetType,
			comment.ParentID, comment.CreatedAt, comment.UpdatedAt,
		}
	} else {
		query = `
			INSERT INTO comments (
				id, author_id, body, target_id, target_type,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		args = []any{
			comment.ID, comment.AuthorID, comment.Body,
			comment.TargetID, comment.TargetType,
			comment.CreatedAt, comment.UpdatedAt,
		}
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("create comment failed: %w", err)
	}

	if comment.Reference != nil {
		if err := r.insertCommentCommerceReference(ctx, tx, comment.ID, comment.Reference); err != nil {
			return err
		}
	}

	return nil
}

// GetByID retrieves a comment by ID (without lock).
func (r *CommentRepositoryImpl) GetByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Comment, error) {
	return r.loadComment(ctx, tx, `
		SELECT c.id, c.author_id, c.body, c.target_id, c.target_type,
		       c.parent_id, c.created_at, c.updated_at, c.deleted_at,
		       ccr.for_sale_id, ccr.auction_id
		FROM comments c
		LEFT JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
		WHERE c.id = $1
	`, id)
}

// ListByTarget retrieves comments for any target type with cursor-based pagination.
func (r *CommentRepositoryImpl) ListByTarget(
	ctx context.Context,
	tx db.Tx,
	targetType entity.CommentTargetType,
	targetID uuid.UUID,
	limit int,
	cursor string,
) ([]*entity.Comment, string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var query string
	var args []any
	if targetType == entity.TargetContent {
		query = `
			SELECT c.id, c.author_id, c.body, c.target_id, c.target_type,
			       c.parent_id, c.created_at, c.updated_at, c.deleted_at,
			       ccr.for_sale_id, ccr.auction_id
			FROM comments c
			INNER JOIN contents cnt ON c.target_id = cnt.id
			JOIN users u ON u.id = c.author_id
			LEFT JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
			WHERE c.target_type = $1 AND c.target_id = $2
			  AND c.deleted_at IS NULL
			  AND cnt.status != 'deleted'
			  AND u.account_status = 'active' AND u.deleted_at IS NULL
		`
		args = []any{targetType, targetID}
	} else {
		query = `
			SELECT c.id, c.author_id, c.body, c.target_id, c.target_type,
			       c.parent_id, c.created_at, c.updated_at, c.deleted_at,
			       ccr.for_sale_id, ccr.auction_id
			FROM comments c
			JOIN users u ON u.id = c.author_id
			LEFT JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
			WHERE c.target_type = $1 AND c.target_id = $2 AND c.deleted_at IS NULL
			  AND u.account_status = 'active' AND u.deleted_at IS NULL
		`
		args = []any{targetType, targetID}
	}

	if cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339Nano, cursor)
		if err == nil {
			query += ` AND c.created_at > $` + fmt.Sprint(len(args)+1)
			args = append(args, cursorTime)
		}
	}

	query += ` ORDER BY c.created_at ASC LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit+1)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list comments by target failed: %w", err)
	}
	defer rows.Close()

	var comments []*entity.Comment
	for rows.Next() {
		comment, err := scanCommentRow(rows)
		if err != nil {
			return nil, "", err
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate comments failed: %w", err)
	}

	var nextCursor string
	if len(comments) > limit {
		comments = comments[:limit]
		nextCursor = comments[limit-1].CreatedAt.Format(time.RFC3339Nano)
	}

	return comments, nextCursor, nil
}

func (r *CommentRepositoryImpl) insertCommentCommerceReference(ctx context.Context, tx db.Tx, commentID uuid.UUID, ref *entity.ShareReference) error {
	if ref == nil {
		return nil
	}

	var forSaleID, auctionID *uuid.UUID
	switch ref.TargetType {
	case entity.ShareTargetTypeForSale:
		id, err := uuid.Parse(ref.TargetID)
		if err != nil {
			return fmt.Errorf("invalid fixed-price sale reference target id: %w", err)
		}
		forSaleID = &id
	case entity.ShareTargetTypeAuction:
		id, err := uuid.Parse(ref.TargetID)
		if err != nil {
			return fmt.Errorf("invalid auction reference target id: %w", err)
		}
		auctionID = &id
	default:
		return fmt.Errorf("unsupported commerce reference target type: %s", ref.TargetType)
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO comment_commerce_references (comment_id, for_sale_id, auction_id, created_at)
		VALUES ($1, $2, $3, NOW())
	`, commentID, forSaleID, auctionID)
	if err != nil {
		return fmt.Errorf("create comment commerce reference failed: %w", err)
	}
	return nil
}

func (r *CommentRepositoryImpl) loadComment(ctx context.Context, tx db.Tx, query string, id uuid.UUID) (*entity.Comment, error) {
	comment, err := scanCommentRow(tx.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("get comment failed: %w", err)
	}
	return comment, nil
}

func scanCommentRow(row interface{ Scan(...any) error }) (*entity.Comment, error) {
	var (
		comment          entity.Comment
		forSaleID *uuid.UUID
		auctionID        *uuid.UUID
	)

	if err := row.Scan(
		&comment.ID,
		&comment.AuthorID,
		&comment.Body,
		&comment.TargetID,
		&comment.TargetType,
		&comment.ParentID,
		&comment.CreatedAt,
		&comment.UpdatedAt,
		&comment.DeletedAt,
		&forSaleID,
		&auctionID,
	); err != nil {
		return nil, err
	}

	switch {
	case forSaleID != nil:
		comment.Type = entity.CommentTypeCommerceReference
		ref := entity.NewShareReference(entity.ShareTargetTypeForSale, forSaleID.String(), entity.SharePreview{})
		comment.Reference = ref
	case auctionID != nil:
		comment.Type = entity.CommentTypeCommerceReference
		ref := entity.NewShareReference(entity.ShareTargetTypeAuction, auctionID.String(), entity.SharePreview{})
		comment.Reference = ref
	default:
		comment.Type = entity.CommentTypeNormal
	}

	return &comment, nil
}
