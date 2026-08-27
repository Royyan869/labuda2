package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

type strictBindJSONErrorRow struct {
	err error
}

func (r *strictBindJSONErrorRow) Scan(...any) error {
	if r.err != nil {
		return r.err
	}
	return pgx.ErrNoRows
}

func strictBindJSON(c *gin.Context, dst any) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return errors.New("missing request body")
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing data")
		}
		return err
	}

	return nil
}

func (h *CommentHandler) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if h != nil && h.blockQueryOverride != nil {
		return h.blockQueryOverride.QueryRow(ctx, sql, args...)
	}
	if h != nil && h.db != nil {
		return h.db.Pool().QueryRow(ctx, sql, args...)
	}
	return &strictBindJSONErrorRow{err: errors.New("comment handler database unavailable")}
}

func (h *CommentHandler) buildCanonicalCommentResponse(
	ctx context.Context,
	tx db.Tx,
	comment *entity.Comment,
) (*contentApp.CommentResponse, error) {
	if comment == nil {
		return nil, errors.New("comment is nil")
	}

	const query = `
		SELECT
			u.id,
			COALESCE(p.username, '') AS username,
			p.avatar_url,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1
	`

	var (
		userID        uuid.UUID
		username      string
		avatarURL     *string
		accountStatus string
		isDeleted     bool
	)
	if err := tx.QueryRow(ctx, query, comment.AuthorID).Scan(&userID, &username, &avatarURL, &accountStatus, &isDeleted); err != nil {
		return nil, err
	}

	lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
	if lifecycle != string(viewercontext.PublicLifecycleStateActive) {
		username = ""
		avatarURL = nil
	}
	authorCard := publiccard.UserCard{
		ID:       userID,
		Username: username,
	}
	if avatarURL != nil && *avatarURL != "" {
		authorCard.AvatarURL = avatarURL
	}
	if lifecycle != "" {
		authorCard.Lifecycle = &lifecycle
	}

	resp := &contentApp.CommentResponse{
		ID:              comment.ID,
		TargetID:        comment.TargetID,
		AuthorID:        comment.AuthorID,
		AuthorUsername:  username,
		AuthorAvatarURL: authorCard.AvatarURL,
		Author:          &authorCard,
		Body:            comment.Body,
		Type:            string(comment.Type),
		ParentID:        comment.ParentID,
		Reference:       comment.Reference,
		CreatedAt:       comment.CreatedAt,
		DeletedAt:       comment.DeletedAt,
	}

	return resp, nil
}
