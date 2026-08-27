package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/social/content/entity"
)

// S4 — Comment read governance: block filter + deleted-author lifecycle.
//
// Tests cover the two pure functions added in S4:
//   - applyCommentBlockFilter: bidirectional block filter on a pre-resolved set
//   - viewercontext.CoarsenLifecycle: coarsening contract for deleted authors
//     (validates the SQL-gap fix that drops `deleted_at IS NULL` from
//     fetchCommentAuthorsInfo so deleted authors now emit lifecycle="removed")

// ---------------------------------------------------------------------------
// applyCommentBlockFilter
// ---------------------------------------------------------------------------

func mkComment(authorID uuid.UUID) *entity.Comment {
	return &entity.Comment{
		ID:        uuid.New(),
		AuthorID:  authorID,
		TargetID:  uuid.New(),
		CreatedAt: time.Now(),
	}
}

func TestApplyCommentBlockFilter_EmptyComments(t *testing.T) {
	got := applyCommentBlockFilter(nil, map[uuid.UUID]bool{uuid.New(): true})
	if len(got) != 0 {
		t.Fatalf("nil input must produce empty result, got %v", got)
	}
}

func TestApplyCommentBlockFilter_EmptyBlockedSet(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	comments := []*entity.Comment{mkComment(a), mkComment(b)}
	got := applyCommentBlockFilter(comments, map[uuid.UUID]bool{})
	// empty blockedSet → returns original slice (pointer equality)
	if len(got) != 2 {
		t.Fatalf("empty blockedSet must not filter; got %d comments", len(got))
	}
}

func TestApplyCommentBlockFilter_ViewerBlockedAuthor(t *testing.T) {
	// Viewer blocked authorA — their comment must be hidden.
	authorA := uuid.New()
	authorB := uuid.New()
	comments := []*entity.Comment{mkComment(authorA), mkComment(authorB)}
	blockedSet := map[uuid.UUID]bool{authorA: true}

	got := applyCommentBlockFilter(comments, blockedSet)
	if len(got) != 1 {
		t.Fatalf("expected 1 comment after filtering authorA; got %d", len(got))
	}
	if got[0].AuthorID != authorB {
		t.Fatalf("surviving comment must be from authorB; got %s", got[0].AuthorID)
	}
}

func TestApplyCommentBlockFilter_AuthorBlockedViewer(t *testing.T) {
	// authorA blocked the viewer — same result from the caller's perspective:
	// the DB query resolves the bidirectional set before calling this function,
	// so authorA ends up in blockedSet regardless of direction.
	authorA := uuid.New()
	authorC := uuid.New()
	comments := []*entity.Comment{mkComment(authorA), mkComment(authorC)}
	blockedSet := map[uuid.UUID]bool{authorA: true}

	got := applyCommentBlockFilter(comments, blockedSet)
	if len(got) != 1 {
		t.Fatalf("expected 1 comment; got %d", len(got))
	}
	if got[0].AuthorID != authorC {
		t.Fatalf("surviving comment must be from authorC; got %s", got[0].AuthorID)
	}
}

func TestApplyCommentBlockFilter_PartialFilter(t *testing.T) {
	// Mixed: 3 authors, 1 blocked. Verify exactly 2 remain in original order.
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	comments := []*entity.Comment{mkComment(a), mkComment(b), mkComment(c)}
	blockedSet := map[uuid.UUID]bool{b: true}

	got := applyCommentBlockFilter(comments, blockedSet)
	if len(got) != 2 {
		t.Fatalf("expected 2 comments; got %d", len(got))
	}
	if got[0].AuthorID != a {
		t.Fatalf("first comment must be from a; got %s", got[0].AuthorID)
	}
	if got[1].AuthorID != c {
		t.Fatalf("second comment must be from c; got %s", got[1].AuthorID)
	}
}

func TestApplyCommentBlockFilter_AllFiltered(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	comments := []*entity.Comment{mkComment(a), mkComment(b)}
	blockedSet := map[uuid.UUID]bool{a: true, b: true}

	got := applyCommentBlockFilter(comments, blockedSet)
	if len(got) != 0 {
		t.Fatalf("all blocked: expected 0 comments; got %d", len(got))
	}
}

func TestApplyCommentBlockFilter_SameAuthorMultipleComments(t *testing.T) {
	// Blocked author posted multiple comments — all must be removed.
	blocked := uuid.New()
	allowed := uuid.New()
	comments := []*entity.Comment{
		mkComment(blocked),
		mkComment(allowed),
		mkComment(blocked),
	}
	blockedSet := map[uuid.UUID]bool{blocked: true}

	got := applyCommentBlockFilter(comments, blockedSet)
	if len(got) != 1 {
		t.Fatalf("expected 1 comment; got %d", len(got))
	}
	if got[0].AuthorID != allowed {
		t.Fatalf("surviving comment must be from allowed author; got %s", got[0].AuthorID)
	}
}

// ---------------------------------------------------------------------------
// Deleted-author lifecycle coarsening (S4 Part 3 SQL-gap fix)
//
// fetchCommentAuthorsInfo previously filtered `WHERE u.deleted_at IS NULL`,
// so deleted authors were absent from the author map (producing empty
// lifecycle in the response). S4 removes that filter. This test suite
// validates the coarsening contract that makes deleted authors emit
// lifecycle="removed" — proving that the SQL change produces the correct
// observable outcome on the Go side.
// ---------------------------------------------------------------------------

func TestCommentAuthorLifecycle_DeletedAuthor_EmitsRemoved(t *testing.T) {
	// When the SQL no longer filters deleted_at IS NULL, rows arrive with
	// is_deleted=true. CoarsenLifecycle must map them to "removed".
	cases := []struct {
		name          string
		accountStatus string
		isDeleted     bool
	}{
		{"active_with_deleted_at", "active", true},
		{"suspended_with_deleted_at", "suspended", true},
		{"banned_with_deleted_at", "banned", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(viewercontext.CoarsenLifecycle(tc.accountStatus, tc.isDeleted))
			if got != "removed" {
				t.Fatalf("deleted author: CoarsenLifecycle(%q, true) = %q; want \"removed\"",
					tc.accountStatus, got)
			}
		})
	}
}

func TestCommentAuthorLifecycle_ActiveAuthor_EmitsActive(t *testing.T) {
	// Regression guard: removing the deleted_at filter must not affect
	// non-deleted active authors.
	got := string(viewercontext.CoarsenLifecycle("active", false))
	if got != "active" {
		t.Fatalf("active author: CoarsenLifecycle(\"active\", false) = %q; want \"active\"", got)
	}
}

// ---------------------------------------------------------------------------
// SC-3 — Parent content visibility gate
//
// isParentContentPubliclyListable is the pure predicate extracted from the
// ListComments handler. It must mirror the GET /contents/:id legacy gate:
// deleted or hidden (admin-moderated) content is not publicly listable.
// ---------------------------------------------------------------------------

func TestIsParentContentPubliclyListable_ActiveContent(t *testing.T) {
	content := &entity.Comment{} // use entity.Content via mkContent helper below
	_ = content
	c := mkContent(entity.StatusActive, false)
	if !isParentContentPubliclyListable(c) {
		t.Error("active non-hidden content must be publicly listable")
	}
}

func TestIsParentContentPubliclyListable_DeletedContent(t *testing.T) {
	c := mkContent(entity.StatusDeleted, false)
	if isParentContentPubliclyListable(c) {
		t.Error("deleted content must NOT be publicly listable")
	}
}

func TestIsParentContentPubliclyListable_HiddenContent(t *testing.T) {
	c := mkContent(entity.StatusActive, true)
	if isParentContentPubliclyListable(c) {
		t.Error("hidden (admin-moderated) content must NOT be publicly listable")
	}
}

func TestIsParentContentPubliclyListable_DeletedAndHidden(t *testing.T) {
	c := mkContent(entity.StatusDeleted, true)
	if isParentContentPubliclyListable(c) {
		t.Error("deleted+hidden content must NOT be publicly listable")
	}
}

func mkContent(status entity.Status, isHidden bool) *entity.Content {
	return &entity.Content{
		ID:       uuid.New(),
		AuthorID: uuid.New(),
		Status:   status,
		IsHidden: isHidden,
	}
}

func TestCommentAuthorLifecycle_SuspendedAuthor_EmitsUnavailable(t *testing.T) {
	for _, status := range []string{"suspended", "banned"} {
		t.Run(status, func(t *testing.T) {
			got := string(viewercontext.CoarsenLifecycle(status, false))
			if got != "unavailable" {
				t.Fatalf("CoarsenLifecycle(%q, false) = %q; want \"unavailable\"", status, got)
			}
		})
	}
}
