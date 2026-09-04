package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commerceRestrictionFakeRepo is a minimal commercegov.Repository stub that
// returns a configurable restriction state.
type commerceRestrictionFakeRepo struct {
	restricted bool
	until      *time.Time
}

func (f *commerceRestrictionFakeRepo) InsertViolation(_ context.Context, _ db.Tx, _ *commercegov.Violation) error {
	return nil
}

func (f *commerceRestrictionFakeRepo) GetRestrictionForUpdate(_ context.Context, _ db.Tx, userID uuid.UUID) (*commercegov.Restriction, error) {
	if f.restricted {
		until := time.Now().Add(7 * 24 * time.Hour)
		if f.until != nil {
			until = *f.until
		}
		return &commercegov.Restriction{
			ID:              uuid.New(),
			UserID:          userID,
			ViolationCount:  1,
			RestrictedUntil: until,
			LastViolationID: uuid.New(),
		}, nil
	}
	return nil, nil
}

func (f *commerceRestrictionFakeRepo) UpsertRestriction(_ context.Context, _ db.Tx, _ *commercegov.Restriction) error {
	return nil
}

// TestRequireBuyerNotRestricted_RestrictedBuyer_BlocksCheckout proves that
// a restricted buyer is rejected with auth.ErrCommerceRestricted when the
// restriction check is invoked at the order creation boundary.
func TestRequireBuyerNotRestricted_RestrictedBuyer_BlocksCheckout(t *testing.T) {
	svc := &OrderCreationService{
		commerceGovRepo: &commerceRestrictionFakeRepo{restricted: true},
	}

	err := svc.requireBuyerNotRestricted(context.Background(), nil, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
		"restricted buyer should be blocked with ErrCommerceRestricted")
}

// TestRequireBuyerNotRestricted_UnrestrictedBuyer_AllowsCheckout proves that
// an unrestricted buyer passes the restriction check.
func TestRequireBuyerNotRestricted_UnrestrictedBuyer_AllowsCheckout(t *testing.T) {
	svc := &OrderCreationService{
		commerceGovRepo: &commerceRestrictionFakeRepo{restricted: false},
	}

	err := svc.requireBuyerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "unrestricted buyer should be allowed")
}

// TestRequireBuyerNotRestricted_NilRepo_FailOpen proves that when the
// commerceGovRepo is nil (not wired), the check fails open for backward
// compatibility.
func TestRequireBuyerNotRestricted_NilRepo_FailOpen(t *testing.T) {
	svc := &OrderCreationService{
		commerceGovRepo: nil,
	}

	err := svc.requireBuyerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "nil repo should fail-open")
}
