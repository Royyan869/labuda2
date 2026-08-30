package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contentapp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// mentionTrackingRepo records calls to InsertMentionedUsers for assertion.
type mentionTrackingRepo struct {
	content          *contententity.Content
	mentionedUserIDs []uuid.UUID
}

func (r *mentionTrackingRepo) Create(ctx context.Context, tx interface{}, content *contententity.Content) error {
	r.content = content
	return nil
}
func (r *mentionTrackingRepo) CreateMedia(ctx context.Context, tx interface{}, media []*contententity.ContentMedia) error {
	return nil
}
func (r *mentionTrackingRepo) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
	return r.content, nil
}
func (r *mentionTrackingRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
	return r.content, nil
}
func (r *mentionTrackingRepo) Update(ctx context.Context, tx interface{}, content *contententity.Content) error {
	r.content = content
	return nil
}
func (r *mentionTrackingRepo) ListByAuthor(ctx context.Context, tx interface{}, authorID uuid.UUID, viewerID uuid.UUID, limit int, cursor string) ([]*contententity.Content, string, error) {
	return nil, "", nil
}
func (r *mentionTrackingRepo) GetMedia(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]*contententity.ContentMedia, error) {
	return nil, nil
}
func (r *mentionTrackingRepo) GetTagsByContentID(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]string, error) {
	return nil, nil
}
func (r *mentionTrackingRepo) InsertTags(ctx context.Context, tx interface{}, contentID uuid.UUID, tags []string) error {
	return nil
}
func (r *mentionTrackingRepo) InsertMentionedUsers(ctx context.Context, tx interface{}, contentID uuid.UUID, userIDs []uuid.UUID) error {
	r.mentionedUserIDs = append(r.mentionedUserIDs, userIDs...)
	return nil
}
func (r *mentionTrackingRepo) GetMentionedUserIDs(ctx context.Context, tx interface{}, contentID uuid.UUID) ([]uuid.UUID, error) {
	return r.mentionedUserIDs, nil
}

// mentionFailingRepo simulates InsertMentionedUsers failure.
type mentionFailingRepo struct {
	mentionTrackingRepo
}

func (r *mentionFailingRepo) InsertMentionedUsers(ctx context.Context, tx interface{}, contentID uuid.UUID, userIDs []uuid.UUID) error {
	return fmt.Errorf("simulated mention persistence failure")
}

type mentionAccountChecker struct{}

func (mentionAccountChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error { return nil }
func (mentionAccountChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}
func (mentionAccountChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

func TestCreateContent_NoMention_PassesNilSlice(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)
	require.Nil(t, repo.mentionedUserIDs, "no mention call expected when args not provided")
}

func TestCreateContent_WithMention_PassesToRepo(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	userID := uuid.New()
	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil, []uuid.UUID{userID},
	)
	require.NoError(t, err)
	require.Len(t, repo.mentionedUserIDs, 1)
	require.Equal(t, userID, repo.mentionedUserIDs[0])
}

func TestCreateContent_MultipleMentions_AllPassed(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil, ids,
	)
	require.NoError(t, err)
	require.Len(t, repo.mentionedUserIDs, 3)
}

func TestCreateContent_NilMention_Skipped(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	// No mention args at all — tags only
	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, []string{"tag1"}, nil,
	)
	require.NoError(t, err)
	// InsertMentionedUsers should NOT have been called (nil slice)
	require.Nil(t, repo.mentionedUserIDs)
}

func TestCreateContent_DuplicateMentionIDs_AllPassedToRepo(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	dupID := uuid.New()
	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil, []uuid.UUID{dupID, dupID, dupID},
	)
	require.NoError(t, err)
	// All three IDs passed — ON CONFLICT DO NOTHING in repo handles dedup at DB level.
	require.Len(t, repo.mentionedUserIDs, 3)
}

func TestCreateContent_NilUUID_Skipped(t *testing.T) {
	repo := &mentionTrackingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil,
		[]uuid.UUID{uuid.Nil, uuid.New(), uuid.Nil},
	)
	require.NoError(t, err)
	// Only 1 valid ID (nil UUIDs filtered)
	require.Len(t, repo.mentionedUserIDs, 1)
}

func TestCreateContent_MentionFails_ContentCreationFails(t *testing.T) {
	repo := &mentionFailingRepo{}
	svc := contentapp.NewContentService(repo, nil, nil, mentionAccountChecker{}, nil)

	// Strict semantics: mention persistence failure causes content creation to fail.
	_, err := svc.CreateContent(context.Background(), nil, uuid.New(), "hello",
		contententity.VisibilityPublic, nil, nil, nil, nil, []uuid.UUID{uuid.New()},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mention persistence failed")
}
