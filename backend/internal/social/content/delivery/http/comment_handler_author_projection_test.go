package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/social/content/entity"
)

type fakeAuthorRow struct {
	userID       uuid.UUID
	username     string
	avatarURL    *string
	accountState string
	isDeleted    bool
	scanErr      error
}

func (r fakeAuthorRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) != 5 {
		return errors.New("unexpected scan arity")
	}

	if v, ok := dest[0].(*uuid.UUID); ok {
		*v = r.userID
	} else {
		return errors.New("dest[0] type mismatch")
	}
	if v, ok := dest[1].(*string); ok {
		*v = r.username
	} else {
		return errors.New("dest[1] type mismatch")
	}
	if v, ok := dest[2].(**string); ok {
		*v = r.avatarURL
	} else {
		return errors.New("dest[2] type mismatch")
	}
	if v, ok := dest[3].(*string); ok {
		*v = r.accountState
	} else {
		return errors.New("dest[3] type mismatch")
	}
	if v, ok := dest[4].(*bool); ok {
		*v = r.isDeleted
	} else {
		return errors.New("dest[4] type mismatch")
	}

	return nil
}

type fakeCommentTx struct {
	row pgx.Row
}

func (t *fakeCommentTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (t *fakeCommentTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (t *fakeCommentTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return t.row
}

func (t *fakeCommentTx) Commit(context.Context) error {
	return nil
}

func (t *fakeCommentTx) Rollback(context.Context) error {
	return nil
}

func TestCommentHandlerBuildCanonicalCommentResponse_ProjectsAuthorAndReplyMetadata(t *testing.T) {
	h := &CommentHandler{}
	commentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	authorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	parentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	body := "reply body"

	comment := &entity.Comment{
		ID:        commentID,
		TargetID:  contentID,
		AuthorID:  authorID,
		Body:      &body,
		
		ParentID:  &parentID,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}

	resp, err := h.buildCanonicalCommentResponse(
		context.Background(),
		&fakeCommentTx{row: fakeAuthorRow{
			userID:       authorID,
			username:     "",
			avatarURL:    nil,
			accountState: "active",
			isDeleted:    false,
		}},
		comment,
	)
	if err != nil {
		t.Fatalf("buildCanonicalCommentResponse returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("buildCanonicalCommentResponse returned nil response")
	}
	if resp.ParentID == nil || *resp.ParentID != parentID {
		t.Fatalf("ParentID = %v, want %s", resp.ParentID, parentID)
	}
	if resp.Author == nil {
		t.Fatal("Author card = nil, want hydrated card")
	}
	if resp.Author.Username != "" {
		t.Fatalf("Author.Username = %q, want empty string without anonymous fallback", resp.Author.Username)
	}
	if resp.AuthorAvatarURL != nil {
		t.Fatalf("AuthorAvatarURL = %v, want nil", *resp.AuthorAvatarURL)
	}
	if resp.Author.Lifecycle == nil || *resp.Author.Lifecycle != "active" {
		t.Fatalf("Author.Lifecycle = %v, want active", resp.Author.Lifecycle)
	}
}

func TestCommentHandlerBuildCanonicalCommentResponse_RedactsUnavailableAuthor(t *testing.T) {
	h := &CommentHandler{}
	commentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	authorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	avatar := "https://cdn.example/avatar.png"

	comment := &entity.Comment{
		ID:        commentID,
		TargetID:  contentID,
		AuthorID:  authorID,
		
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}

	resp, err := h.buildCanonicalCommentResponse(
		context.Background(),
		&fakeCommentTx{row: fakeAuthorRow{
			userID:       authorID,
			username:     "alice",
			avatarURL:    &avatar,
			accountState: "suspended",
			isDeleted:    false,
		}},
		comment,
	)
	if err != nil {
		t.Fatalf("buildCanonicalCommentResponse returned error: %v", err)
	}
	if resp.Author == nil {
		t.Fatal("Author card = nil, want hydrated card")
	}
	if resp.Author.Username != "" {
		t.Fatalf("Author.Username = %q, want redacted empty string", resp.Author.Username)
	}
	if resp.AuthorAvatarURL != nil {
		t.Fatalf("AuthorAvatarURL = %v, want nil", *resp.AuthorAvatarURL)
	}
	if resp.Author.Lifecycle == nil || *resp.Author.Lifecycle != "unavailable" {
		t.Fatalf("Author.Lifecycle = %v, want unavailable", resp.Author.Lifecycle)
	}
}

func TestCommentHandlerBuildCanonicalCommentResponse_FailsClosedOnHydrationError(t *testing.T) {
	h := &CommentHandler{}
	comment := &entity.Comment{
		ID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		TargetID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		AuthorID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		
	}

	resp, err := h.buildCanonicalCommentResponse(
		context.Background(),
		&fakeCommentTx{row: fakeAuthorRow{scanErr: errors.New("boom")}},
		comment,
	)
	if err == nil {
		t.Fatal("buildCanonicalCommentResponse succeeded; want hydration error")
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil on hydration failure", resp)
	}
}
