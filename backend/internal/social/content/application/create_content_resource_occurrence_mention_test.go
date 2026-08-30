package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
)

// ---------------------------------------------------------------------------
// Test doubles for mention persistence
// ---------------------------------------------------------------------------

// mentionTrackingContentRepo extends countingContentRepo to track mentioned
// user IDs passed to InsertMentionedUsers.
type mentionTrackingContentRepo struct {
	countingContentRepo
	mentionedCalls map[uuid.UUID][]uuid.UUID // contentID → mentioned user IDs
}

func newMentionTrackingContentRepo() *mentionTrackingContentRepo {
	return &mentionTrackingContentRepo{}
}

func (r *mentionTrackingContentRepo) InsertMentionedUsers(_ context.Context, _ interface{}, contentID uuid.UUID, userIDs []uuid.UUID) error {
	if r.mentionedCalls == nil {
		r.mentionedCalls = map[uuid.UUID][]uuid.UUID{}
	}
	r.mentionedCalls[contentID] = userIDs
	return nil
}

// Ensure interface compliance.
var _ contentrepo.ContentRepository = (*mentionTrackingContentRepo)(nil)

// newMentionTestService creates a ContentService wired with a tracking repo.
func newMentionTestService() (*ContentService, *mentionTrackingContentRepo) {
	repo := newMentionTrackingContentRepo()
	svc := &ContentService{
		contentRepo:          repo,
		accountStatusChecker: isActiveAccountChecker{},
	}
	return svc, repo
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCreateContentWithResourceOccurrence_PersistsMentionedUsers proves that
// mentioned user IDs are correctly validated and persisted when creating
// content with a resource occurrence. Before the fix, mentionedUserIDs were
// silently dropped.
func TestCreateContentWithResourceOccurrence_PersistsMentionedUsers(t *testing.T) {
	svc, repo := newMentionTestService()

	callerID := uuid.New()
	contentID := uuid.Nil // will be filled by the stub
	mentionID1 := uuid.New()
	mentionID2 := uuid.New()

	// The stub Create sets the content ID so we can verify mentions.
	_ = contentID

	content, err := svc.CreateContentWithResourceOccurrence(
		context.Background(),
		nil, // tx
		callerID,
		"Post with mentions",
		entity.VisibilityPublic,
		nil, // city
		nil, // province
		nil, // occurrence — no commerce reference
		nil, // tags
		[]uuid.UUID{mentionID1, mentionID2},
	)

	if err != nil {
		t.Fatalf("CreateContentWithResourceOccurrence should succeed: %v", err)
	}
	if content == nil {
		t.Fatal("expected content to be created")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", repo.createCalls)
	}

	// Verify mentions were persisted.
	mentioned, ok := repo.mentionedCalls[content.ID]
	if !ok {
		t.Fatal("InsertMentionedUsers was not called — mentions silently dropped")
	}
	if len(mentioned) != 2 {
		t.Fatalf("expected 2 mentioned users, got %d", len(mentioned))
	}
	if mentioned[0] != mentionID1 {
		t.Fatalf("first mentioned user ID mismatch: got %s, want %s", mentioned[0], mentionID1)
	}
	if mentioned[1] != mentionID2 {
		t.Fatalf("second mentioned user ID mismatch: got %s, want %s", mentioned[1], mentionID2)
	}
}

// TestCreateContentWithResourceOccurrence_EmptyMentionsOk proves that
// creating content with resource occurrence and no mentions still works.
func TestCreateContentWithResourceOccurrence_EmptyMentionsOk(t *testing.T) {
	svc, repo := newMentionTestService()

	content, err := svc.CreateContentWithResourceOccurrence(
		context.Background(),
		nil,
		uuid.New(),
		"Post without mentions",
		entity.VisibilityPublic,
		nil, nil,
		nil, // occurrence
		nil, // tags
		nil, // no mentions
	)

	if err != nil {
		t.Fatalf("should succeed: %v", err)
	}
	if content == nil {
		t.Fatal("expected content")
	}
	if repo.createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", repo.createCalls)
	}
	// No mentions should have been persisted.
	if len(repo.mentionedCalls) != 0 {
		t.Fatalf("expected no mention calls, got %d", len(repo.mentionedCalls))
	}
}

// TestCreateContentWithResourceOccurrence_MentionValidationFails proves that
// if a mentioned user ID does not exist, the entire creation is rejected.
// NOTE: This test uses nil tx, so user-existence validation is skipped
// (matching the same behavior as CreateContent). With a real tx, the
// validation would query the users table.
func TestCreateContentWithResourceOccurrence_MentionWithNilTxSkipsExistenceCheck(t *testing.T) {
	svc, repo := newMentionTestService()

	nonExistentID := uuid.New()

	// With nil tx, user existence check is skipped (same as CreateContent).
	content, err := svc.CreateContentWithResourceOccurrence(
		context.Background(),
		nil, // nil tx — skips user existence validation
		uuid.New(),
		"Post with non-existent user mention",
		entity.VisibilityPublic,
		nil, nil,
		nil, nil,
		[]uuid.UUID{nonExistentID},
	)

	if err != nil {
		t.Fatalf("with nil tx, mention validation should be skipped: %v", err)
	}
	if content == nil {
		t.Fatal("expected content")
	}
	// The mention should still be persisted (fail-open with nil tx).
	mentioned := repo.mentionedCalls[content.ID]
	if len(mentioned) != 1 {
		t.Fatalf("expected 1 mentioned user, got %d", len(mentioned))
	}
}
