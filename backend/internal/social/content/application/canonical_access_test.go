package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/social/content/entity"
)

// fakeContentRepo captures created/updated content for moderation independence tests.
type fakeContentRepo struct {
	stored map[uuid.UUID]*entity.Content
}

func newFakeContentRepo() *fakeContentRepo {
	return &fakeContentRepo{stored: map[uuid.UUID]*entity.Content{}}
}
func (r *fakeContentRepo) Create(_ context.Context, _ interface{}, c *entity.Content) error {
	r.stored[c.ID] = c
	return nil
}
func (r *fakeContentRepo) GetByID(_ context.Context, _ interface{}, id uuid.UUID) (*entity.Content, error) {
	if c, ok := r.stored[id]; ok {
		return c, nil
	}
	return nil, errNotFound(id.String())
}
func (r *fakeContentRepo) GetForUpdate(_ context.Context, _ interface{}, id uuid.UUID) (*entity.Content, error) {
	return r.GetByID(context.Background(), nil, id)
}
func (r *fakeContentRepo) Update(_ context.Context, _ interface{}, c *entity.Content) error {
	r.stored[c.ID] = c
	return nil
}
func (r *fakeContentRepo) ListByAuthor(_ context.Context, _ interface{}, _ uuid.UUID, _ uuid.UUID, _ int, _ string) ([]*entity.Content, string, error) {
	return nil, "", nil
}
func (r *fakeContentRepo) GetMedia(_ context.Context, _ interface{}, _ uuid.UUID) ([]*entity.ContentMedia, error) {
	return nil, nil
}
func (r *fakeContentRepo) CreateResourceOccurrence(_ context.Context, _ interface{}, _ *entity.ContentResourceOccurrence) error {
	return nil
}
func (r *fakeContentRepo) GetResourceOccurrenceByContentID(_ context.Context, _ interface{}, _ uuid.UUID) (*entity.ContentResourceOccurrence, error) {
	return nil, errNotFound("")
}
func (r *fakeContentRepo) GetTagsByContentID(_ context.Context, _ interface{}, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (r *fakeContentRepo) InsertTags(_ context.Context, _ interface{}, _ uuid.UUID, _ []string) error {
	return nil
}
func (r *fakeContentRepo) InsertMentionedUsers(_ context.Context, _ interface{}, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (r *fakeContentRepo) GetMentionedUserIDs(_ context.Context, _ interface{}, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (r *fakeContentRepo) CreateMedia(_ context.Context, _ interface{}, _ []*entity.ContentMedia) error {
	return nil
}
func errNotFound(s string) error { return &fakeNotFound{s} }
type fakeNotFound struct{ s string }
func (e *fakeNotFound) Error() string { return "content not found: " + e.s }

type fakeAccountChecker struct{}
func (fakeAccountChecker) EnsureActive(_ context.Context, _ uuid.UUID) error { return nil }
func (fakeAccountChecker) GetStatus(_ context.Context, _ uuid.UUID) (string, error) { return "active", nil }
func (fakeAccountChecker) IsBanned(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil }

// fakeTx for follower visibility checks.
type fakeTx struct {
	followExists bool
}
func (f *fakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if containsStr(sql, "account_status") {
		return &fakeAuthorRow{}
	}
	if containsStr(sql, "user_follows") {
		return &fakeExistsRow{exists: f.followExists}
	}
	if containsStr(sql, "author_id") && containsStr(sql, "contents") {
		// SELECT author_id FROM contents WHERE id=$1 — not used in these tests (repo path), but fallback
		return &fakeExistsRow{exists: false}
	}
	return &fakeExistsRow{exists: f.followExists}
}
func containsStr(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (f *fakeTx) Commit(_ context.Context) error   { return nil }
func (f *fakeTx) Rollback(_ context.Context) error { return nil }
type fakeAuthorRow struct{}
func (r *fakeAuthorRow) Scan(dest ...any) error {
	if len(dest) >= 2 {
		if s, ok := dest[0].(*string); ok {
			*s = "active"
		}
		if b, ok := dest[1].(*bool); ok {
			*b = false
		}
	}
	return nil
}
type fakeExistsRow struct{ exists bool }
func (r *fakeExistsRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if b, ok := dest[0].(*bool); ok {
			*b = r.exists
			return nil
		}
	}
	return nil
}

func TestVisibility_PrivateDoesNotSetHiddenOnCreateWithOccurrence(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	// also need likeRepo nil is ok for create
	caller := uuid.New()
	// create via CreateContentWithResourceOccurrence with private visibility and no occurrence (nil) — but we test the sync path via CreateContentWithResourceOccurrence which previously set IsHidden=true
	// Use the non-idempotent path directly (bypasses validator)
	content, err := svc.CreateContentWithResourceOccurrence(context.Background(), &fakeTx{}, caller, "hello private", entity.VisibilityPrivate, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.VisibilityPrivate, content.Visibility)
	require.False(t, content.IsHidden, "private visibility must NOT set is_hidden — moderation authority independent")
}

func TestVisibility_PrivateDoesNotSetHiddenOnUpdate(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	c := entity.NewContent(author, "initial public")
	c.Visibility = entity.VisibilityPublic
	c.IsHidden = false
	repo.stored[c.ID] = c
	vis := string(entity.VisibilityPrivate)
	err := svc.UpdateCaptionAndVisibility(context.Background(), &fakeTx{}, author, c.ID, nil, &vis)
	require.NoError(t, err)
	updated, _ := repo.GetByID(context.Background(), nil, c.ID)
	require.Equal(t, entity.VisibilityPrivate, updated.Visibility)
	require.False(t, updated.IsHidden, "UpdateCaptionAndVisibility must NOT sync is_hidden with private")
}

func TestVisibility_PublicDoesNotUnhideModeratedContent(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	c := entity.NewContent(author, "moderated")
	c.Visibility = entity.VisibilityPublic
	c.IsHidden = true // moderated hidden
	repo.stored[c.ID] = c
	vis := string(entity.VisibilityPublic)
	err := svc.UpdateCaptionAndVisibility(context.Background(), &fakeTx{}, author, c.ID, nil, &vis)
	require.NoError(t, err)
	updated, _ := repo.GetByID(context.Background(), nil, c.ID)
	require.True(t, updated.IsHidden, "public visibility must NOT clear is_hidden — moderation independent")
	require.Equal(t, entity.VisibilityPublic, updated.Visibility)
}

func TestVisibility_HideUnhideDoesNotChangeVisibility(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	c := entity.NewContent(author, "private content")
	c.Visibility = entity.VisibilityPrivate
	c.IsHidden = false
	repo.stored[c.ID] = c
	// Hide via service (moderation)
	err := svc.HideContent(context.Background(), &fakeTx{}, author, c.ID)
	require.NoError(t, err)
	hidden, _ := repo.GetByID(context.Background(), nil, c.ID)
	require.True(t, hidden.IsHidden)
	require.Equal(t, entity.VisibilityPrivate, hidden.Visibility, "Hide must not change visibility")
	// Unhide
	err = svc.UnhideContent(context.Background(), &fakeTx{}, author, c.ID)
	require.NoError(t, err)
	unhidden, _ := repo.GetByID(context.Background(), nil, c.ID)
	require.False(t, unhidden.IsHidden)
	require.Equal(t, entity.VisibilityPrivate, unhidden.Visibility)
}

func TestGetContentVisibleToViewer_PrivateOnlyOwner(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	other := uuid.New()
	c := entity.NewContent(author, "private")
	c.Visibility = entity.VisibilityPrivate
	c.Status = entity.StatusActive
	c.IsHidden = false
	repo.stored[c.ID] = c
	// owner sees
	got, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, author, c.ID)
	require.NoError(t, err)
	require.Equal(t, c.ID, got.ID)
	// non-owner (even follower) cannot see private
	_, err = svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: true}, other, c.ID)
	require.Error(t, err, "non-owner must not see private")
}

func TestGetContentVisibleToViewer_FollowersOnlyRequiresFollow(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	follower := uuid.New()
	nonFollower := uuid.New()
	c := entity.NewContent(author, "followers only")
	c.Visibility = entity.VisibilityFollowersOnly
	c.Status = entity.StatusActive
	repo.stored[c.ID] = c
	// follower sees
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: true}, follower, c.ID)
	require.NoError(t, err)
	// non-follower denied
	_, err = svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, nonFollower, c.ID)
	require.Error(t, err)
	// anonymous denied
	_, err = svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, uuid.Nil, c.ID)
	require.Error(t, err)
}
