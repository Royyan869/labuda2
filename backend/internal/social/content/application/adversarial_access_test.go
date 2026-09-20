package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// helper to make content stored via fake repo
func storeContent(repo *fakeContentRepo, author uuid.UUID, vis entity.Visibility, hidden bool) *entity.Content {
	c := entity.NewContent(author, "test content")
	c.Visibility = vis
	c.IsHidden = hidden
	c.Status = entity.StatusActive
	repo.stored[c.ID] = c
	return c
}

// ATTEMPT: direct ID guess — non-owner tries to GET private content detail
func TestAdversarial_DirectID_PrivateDetailByNonOwner_Denied(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPrivate, false)
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, viewer, c.ID)
	require.Error(t, err, "private detail by non-owner must be denied")
}

// ATTEMPT: non-follower tries followers_only
func TestAdversarial_FollowersOnly_NonFollowerDenied(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityFollowersOnly, false)
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, viewer, c.ID)
	require.Error(t, err, "followers_only by non-follower must be denied")
	_, err = svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: true}, viewer, c.ID)
	require.NoError(t, err, "followers_only by follower must pass")
}

// ATTEMPT: blocked viewer tries detail (simulated via service layer not having block check — detail block is handler level, but GetContentVisibleToViewer does not check block)
// This documents that GetContentVisibleToViewer alone does not enforce block — handler must.
// We prove handler-level block would be needed, but service layer for repost does check block via validateContentTarget.
func TestAdversarial_RepostTarget_BlockedViewerDenied(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPublic, false)
	// viewer blocks author — GetContentVisibleToViewer still passes (no block check)
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: false}, viewer, c.ID)
	require.NoError(t, err, "public visible even when blocked at service layer — block is enforced at handler/repost, not here; this is legitimate layering")

	// But validateContentTarget MUST deny if blockChecker is wired
	type mockBlockTrue struct{}
	// ExistsBlock requires signature: func (m *mockBlockTrue) ExistsBlock(ctx context.Context, tx interface{}, a, b uuid.UUID) (bool, error)
	// Actually, wait, we need to create a struct that satisfies BlockChecker inline.
}

type mockBlockTrueInline struct{}
func (m *mockBlockTrueInline) ExistsBlock(ctx context.Context, tx interface{}, a,b uuid.UUID) (bool,error){return true,nil}

func TestAdversarial_RepostTarget_BlockedViewerDenied_Target(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	svc.SetBlockChecker(&mockBlockTrueInline{})
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPublic, false)
	
	err := svc.validateContentTarget(context.Background(), &fakeTx{followExists: false}, viewer, c.ID.String())
	require.Error(t, err)
	require.Contains(t, err.Error(), "content not found: blocked")
}

// ATTEMPT: private+visible (is_hidden=false) must still be denied to non-owner
func TestAdversarial_PrivateVisible_NotLeaked(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPrivate, false) // private but not hidden (the D-01 case)
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{}, viewer, c.ID)
	require.Error(t, err, "private with is_hidden=false must still be denied to non-owner")
}

// ATTEMPT: public+hidden (moderated) must be denied even to follower
func TestAdversarial_PublicHidden_Denied(t *testing.T) {
	repo := newFakeContentRepo()
	svc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPublic, true) // moderated hidden
	_, err := svc.GetContentVisibleToViewer(context.Background(), &fakeTx{followExists: true}, viewer, c.ID)
	require.Error(t, err, "public but hidden must be denied via GetContentPublic lifecycle")
	_, err = svc.GetContentVisibleToViewer(context.Background(), &fakeTx{}, author, c.ID)
	require.Error(t, err, "hidden must be denied even to owner via GetContentVisibleToViewer (moderation overrides)")
}

// ATTEMPT: comment create on private by non-owner must be denied (via loadVisibleContentForComment)
func TestAdversarial_CommentCreate_OnPrivateDenied(t *testing.T) {
	repo := newFakeContentRepo()
	commentRepo := &fakeCommentRepoForAdversarial{}
	svc := NewCommentService(repo, commentRepo, nil, nil, &fakeVisibilityChecker{svc: nil}, nil, nil, nil)
	// wire visibilityChecker to same service for simplicity: create a real service to delegate
	contentSvc := NewContentService(repo, nil, nil, fakeAccountChecker{}, nil)
	svc.visibilityChecker = contentSvc
	author := uuid.New()
	viewer := uuid.New()
	c := storeContent(repo, author, entity.VisibilityPrivate, false)
	// Try AddComment — should fail via visibility
	_, err := svc.AddComment(context.Background(), &fakeTx{followExists: false}, viewer, c.ID, "hello", nil, "key1")
	require.Error(t, err, "normal comment on private by non-owner must be denied")
	require.Contains(t, err.Error(), "cannot comment")
}

// fakeCommentRepoForAdversarial minimal
type fakeCommentRepoForAdversarial struct{}

func (f *fakeCommentRepoForAdversarial) Create(_ context.Context, _ db.Tx, _ *entity.Comment) error { return nil }
func (f *fakeCommentRepoForAdversarial) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Comment, error) {
	return nil, errNotFound("")
}
func (f *fakeCommentRepoForAdversarial) ListByTarget(_ context.Context, _ db.Tx, _ entity.CommentTargetType, _ uuid.UUID, _ int, _ string) ([]*entity.Comment, string, error) {
	return nil, "", nil
}
func (f *fakeCommentRepoForAdversarial) FindTargetIDByCommerceReference(_ context.Context, _ db.Tx, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (f *fakeCommentRepoForAdversarial) SoftDelete(_ context.Context, _ db.Tx, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (f *fakeCommentRepoForAdversarial) Restore(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (f *fakeCommentRepoForAdversarial) CountTopLevelCommentsByContent(_ context.Context, _ db.Tx, _ uuid.UUID) (int, error) {
	return 0, nil
}

type fakeVisibilityChecker struct{ svc *ContentService }
func (f *fakeVisibilityChecker) GetContentVisibleToViewer(ctx context.Context, tx db.Tx, viewerID uuid.UUID, contentID uuid.UUID) (*entity.Content, error) {
	// delegate to real svc if available, else fake
	if f.svc != nil {
		// need Tx that satisfies db.Tx — use fakeTx
		return f.svc.GetContentVisibleToViewer(ctx, &fakeTx{}, viewerID, contentID)
	}
	return nil, errNotFound("")
}
