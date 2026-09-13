package application

// content_update_owner_authority_test.go — service-layer proof of the
// canonical content-update authorization (ADMIN-AUTHORITY follow-up scope).
//
// Locked owner decision: the HTTP handler's membership-only admin branch was
// dead code (the service always rejected non-owners with ErrOwnerRequired,
// surfacing a 500) and no existing capability represents "edit another
// user's content". The branch was removed: content update is OWNER-ONLY at
// every layer. There is no PATH B (admin + capability) for this endpoint;
// privileged content-state changes belong to the governance moderation
// pipeline (Case → Decision → Enforcement), which never calls this service.
//
// Because UpdateCaptionAndVisibility has no role/capability parameter at
// all, an admin member — with or without any capability — is denied exactly
// like a normal user. These tests pin that: ownership is the only input.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// updateAuthorityStubRepo implements ContentRepository by embedding the
// interface (nil-receiver panics on unused methods) and overriding only the
// methods UpdateCaptionAndVisibility touches: GetForUpdate and Update.
type updateAuthorityStubRepo struct {
	contentrepo.ContentRepository
	content      *contententity.Content
	updateCalls  int
	returnUpdate error
}

func (s *updateAuthorityStubRepo) GetForUpdate(ctx context.Context, tx interface{}, id uuid.UUID) (*contententity.Content, error) {
	if s.content == nil {
		return nil, errors.New("stub: content not set")
	}
	return s.content, nil
}

func (s *updateAuthorityStubRepo) Update(ctx context.Context, tx interface{}, content *contententity.Content) error {
	s.updateCalls++
	s.content = content
	return s.returnUpdate
}

func newUpdateAuthorityService(repo *updateAuthorityStubRepo) *ContentService {
	return &ContentService{
		contentRepo: repo,
	}
}

func TestUpdateCaptionAndVisibility_Owner_Allowed(t *testing.T) {
	ownerID := uuid.New()
	repo := &updateAuthorityStubRepo{content: &contententity.Content{
		ID:         uuid.New(),
		AuthorID:   ownerID,
		Visibility: contententity.VisibilityPublic,
	}}
	svc := newUpdateAuthorityService(repo)

	caption := "new caption"
	err := svc.UpdateCaptionAndVisibility(context.Background(), db.Tx(nil), ownerID, repo.content.ID, &caption, nil)

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls, "owner update must reach the repository")
	require.Equal(t, "new caption", *repo.content.Caption)
}

// TestUpdateCaptionAndVisibility_NonOwner_Denied pins CASE 2–5 of the
// matrix at the service layer: the caller is rejected with
// ErrOwnerRequired no matter their role or capability, because the method
// receives no role/capability inputs — ownership is the only authority.
func TestUpdateCaptionAndVisibility_NonOwner_Denied(t *testing.T) {
	ownerID := uuid.New()
	repo := &updateAuthorityStubRepo{content: &contententity.Content{
		ID:         uuid.New(),
		AuthorID:   ownerID,
		Visibility: contententity.VisibilityPublic,
	}}
	svc := newUpdateAuthorityService(repo)

	// The four denied actors from the required matrix. Role and capability
	// are not inputs of this method; what matters is none of them is the
	// owner, so each must be denied.
	nonOwners := []struct {
		name   string
		caller uuid.UUID
	}{
		{name: "CASE2 normal_user_no_capability", caller: uuid.New()},
		{name: "CASE3 admin_member_no_capability", caller: uuid.New()},
		{name: "CASE4 admin_member_with_capability", caller: uuid.New()},
		{name: "CASE5 normal_user_with_capability", caller: uuid.New()},
	}

	for _, tc := range nonOwners {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			caption := "intruder caption"
			err := svc.UpdateCaptionAndVisibility(context.Background(), db.Tx(nil), tc.caller, repo.content.ID, &caption, nil)
			require.ErrorIs(t, err, auth.ErrOwnerRequired, "non-owner must be denied regardless of role/capability")
			require.Equal(t, 0, repo.updateCalls, "denied update must never reach the repository")
		})
	}
}

// TestUpdateCaptionAndVisibility_InvalidCaller guards the auth.ValidateCaller
// precondition: a nil caller UUID is rejected before any ownership check.
func TestUpdateCaptionAndVisibility_InvalidCaller(t *testing.T) {
	repo := &updateAuthorityStubRepo{content: &contententity.Content{
		ID:         uuid.New(),
		AuthorID:   uuid.New(),
		Visibility: contententity.VisibilityPublic,
	}}
	svc := newUpdateAuthorityService(repo)

	caption := "x"
	err := svc.UpdateCaptionAndVisibility(context.Background(), db.Tx(nil), uuid.Nil, repo.content.ID, &caption, nil)
	require.ErrorIs(t, err, auth.ErrInvalidCaller)
}
