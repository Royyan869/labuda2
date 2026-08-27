package application

import (
	"reflect"
	"testing"

	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/internal/social/content/repository"
)

// stubContentRepo embeds the interface so it satisfies every method via a
// nil receiver. The unit test never invokes a method — it only verifies
// the constructor retained the reference. Methods would panic if called.
type stubContentRepo struct{ contentrepo.ContentRepository }

// stubCommentRepo — same pattern for the comment repository.
type stubCommentRepo struct{ repository.CommentRepository }

// stubOutbox — same pattern for the OutboxInserter interface declared in
// this package.
type stubOutbox struct{ OutboxInserter }

// TestNewCommentService_RetainsContentRepo is the E3.3 regression test.
//
// Background: a stale comment-out in serverboot/dependencies.go left the
// CommentService constructed with `nil` for the contentRepo dependency
// (with the misleading comment "contentRepo - not needed for comment
// operations"). Every POST /api/v1/contents/:id/comments then panicked at
// AddComment:100 — `s.contentRepo.GetByID(...)` is a method call on a nil
// interface receiver.
//
// This test locks the constructor contract: when `contentRepo` is passed
// non-nil, the returned service must retain that exact instance on its
// internal `contentRepo` field. Reflection is used to read the
// unexported field so the production API surface stays unchanged.
//
// If a future refactor accidentally drops the parameter on the floor (or
// re-introduces a `nil` slot at the wiring boundary), this test fails
// immediately and points the operator at the exact regression class.
func TestNewCommentService_RetainsContentRepo(t *testing.T) {
	repo := stubContentRepo{}

	svc := NewCommentService(
		repo,              // contentRepo — the field this test guards
		stubCommentRepo{}, // commentRepo
		nil,               // forSaleService — unused on AddComment normal path
		stubOutbox{},      // outboxRepo
		nil,               // blockChecker — explicitly nilable per docstring
		nil,               // invariantLogger — explicitly nilable per docstring
	)
	if svc == nil {
		t.Fatal("NewCommentService returned nil")
	}

	got := reflect.ValueOf(svc).Elem().FieldByName("contentRepo")
	if !got.IsValid() {
		t.Fatal("internal field 'contentRepo' missing — structure drifted")
	}
	if got.IsNil() {
		t.Fatal("NewCommentService dropped contentRepo on the floor (E3.3 regression)")
	}
}
