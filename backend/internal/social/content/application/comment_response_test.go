package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
)

// TestNewCommentResponse_AuthorLifecycle locks in the E3.2 lifecycle
// transport behaviour of NewCommentResponse. The handler layer hydrates
// the coarsened public lifecycle via viewercontext.CoarsenLifecycle and
// passes it through this builder; the builder must surface it on the
// embedded UserCard.Lifecycle slot while preserving the flat
// AuthorUsername / AuthorAvatarURL fields.
//
// Wire compatibility guarantees verified here:
//   - empty lifecycle string -> UserCard.Lifecycle == nil (legacy /
//     rollback-safe shape)
//   - "active" string -> UserCard.Lifecycle pointer to "active"
//   - "unavailable" string -> UserCard.Lifecycle pointer to "unavailable"
//   - "removed" string -> UserCard.Lifecycle pointer to "removed"
//   - flat author_id / author_username / author_avatar_url fields preserved
//     across all lifecycle states
//
// Coarsening itself is NOT under test here — that lives in
// viewercontext.CoarsenLifecycle (governance/viewercontext) and has its
// own canonical test suite.
func TestNewCommentResponse_AuthorLifecycle(t *testing.T) {
	authorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	commentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	contentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	avatar := "https://cdn.example/a.png"
	body := "test body"

	mkComment := func() *entity.Comment {
		return &entity.Comment{
			ID:        commentID,
			TargetID:  contentID,
			AuthorID:  authorID,
			Body:      &body,
			Type:      entity.CommentType("normal"),
			CreatedAt: time.Unix(1700000000, 0).UTC(),
		}
	}

	tests := []struct {
		name          string
		lifecycleIn   string
		wantLifecycle *string // nil means UserCard.Lifecycle must be nil
	}{
		{"empty -> nil (rollback-safe)", "", nil},
		{"active", "active", ptr("active")},
		{"unavailable", "unavailable", ptr("unavailable")},
		{"removed", "removed", ptr("removed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewCommentResponse(
				mkComment(),
				nil, // listing preview not relevant here
				"alice", &avatar,
				tt.lifecycleIn,
			)

			// Flat fields preserved across every lifecycle state.
			if resp.AuthorID != authorID {
				t.Fatalf("AuthorID drift: got %s want %s", resp.AuthorID, authorID)
			}
			if resp.AuthorUsername != "alice" {
				t.Fatalf("AuthorUsername drift: got %q want %q", resp.AuthorUsername, "alice")
			}
			if resp.AuthorAvatarURL == nil || *resp.AuthorAvatarURL != avatar {
				t.Fatalf("AuthorAvatarURL drift: got %v want %q", resp.AuthorAvatarURL, avatar)
			}

			// Canonical UserCard always present.
			if resp.Author == nil {
				t.Fatalf("Author UserCard must always be populated")
			}
			if resp.Author.ID != authorID {
				t.Fatalf("Author.ID drift: got %s want %s", resp.Author.ID, authorID)
			}
			if resp.Author.Username != "alice" {
				t.Fatalf("Author.Username drift: got %q want %q", resp.Author.Username, "alice")
			}

			// Lifecycle slot matches the input coarsening exactly.
			switch {
			case tt.wantLifecycle == nil:
				if resp.Author.Lifecycle != nil {
					t.Fatalf("expected nil Lifecycle, got %v", *resp.Author.Lifecycle)
				}
			default:
				if resp.Author.Lifecycle == nil {
					t.Fatalf("expected Lifecycle=%q, got nil", *tt.wantLifecycle)
				}
				if *resp.Author.Lifecycle != *tt.wantLifecycle {
					t.Fatalf("Lifecycle drift: got %q want %q",
						*resp.Author.Lifecycle, *tt.wantLifecycle)
				}
			}
		})
	}
}

func ptr(s string) *string { return &s }


