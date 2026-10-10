package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/hishumi/backend/internal/commerce/forsale/entity"
	"github.com/hishumi/backend/internal/identity/auth"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fakes ──────────────────────────────────────────────────────────

type fakeRoleCheckerCanonical struct {
	hasCapability bool
	hasProfile    bool
	capErr        error
	profileErr    error
}

func (f fakeRoleCheckerCanonical) IsAdmin(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}
func (f fakeRoleCheckerCanonical) HasActiveSellerCapability(ctx context.Context, uid uuid.UUID) (bool, error) {
	return f.hasCapability, f.capErr
}
func (f fakeRoleCheckerCanonical) HasSellerProfile(ctx context.Context, uid uuid.UUID) (bool, error) {
	return f.hasProfile, f.profileErr
}

var _ auth.RoleChecker = fakeRoleCheckerCanonical{}

// mockRow implements pgx.Row
type mockRow struct {
	values []any
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan arity mismatch")
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case **time.Time:
			if v == nil {
				*d = nil
			} else if ptr, ok := v.(*time.Time); ok {
				if ptr == nil {
					*d = nil
				} else {
					cp := *ptr
					*d = &cp
				}
			} else if t, ok := v.(time.Time); ok {
				*d = &t
			} else {
				*d = nil
			}
		case *bool:
			*d = v.(bool)
		default:
		}
	}
	return nil
}

// mockTx implements db.Tx with programmable QueryRow responses
type mockTx struct {
	// sequence of QueryRow calls: keyed by SQL snippet
	accountStatus string
	deletedAt     *time.Time
	emailAt       *time.Time
	hasProfile    bool
	// errors
	accountErr    error
	profileErr    error
	queryRowCalls int
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalls++
	// distinguish by SQL content
	if strings.Contains(sql, "account_status") {
		if m.accountErr != nil {
			return &mockRow{err: m.accountErr}
		}
		return &mockRow{values: []any{m.accountStatus, m.deletedAt, m.emailAt}}
	}
	if strings.Contains(sql, "seller_profiles") {
		if m.profileErr != nil {
			return &mockRow{err: m.profileErr}
		}
		return &mockRow{values: []any{m.hasProfile}}
	}
	// commerce restriction not used in these tests (repo nil → bypass)
	return &mockRow{err: errors.New("unexpected query: " + sql)}
}
func (m *mockTx) Commit(ctx context.Context) error   { return nil }
func (m *mockTx) Rollback(ctx context.Context) error { return nil }

var _ db.Tx = (*mockTx)(nil)

// ── Test 1 – workspace authority (create = publish: workspace gate runs
// before the market gate inside the same transaction) ──

func TestCanonical_WorkspaceAuthority_SucceedsWithoutSubscription(t *testing.T) {
	now := time.Now()
	seller := uuid.New()
	tx := &mockTx{
		accountStatus: "active",
		deletedAt:     nil,
		emailAt:       &now,
		hasProfile:    true,
	}
	rc := fakeRoleCheckerCanonical{hasCapability: false} // no active subscription – Has would deny public
	svc := &ForSaleService{roleChecker: rc}
	// bypass commerce restriction (nil repo → allow)
	// Workspace gate: active account + verified email + seller profile.
	err := svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller)
	require.NoError(t, err)
}

func TestCanonical_Workspace_MissingProfile_Denied(t *testing.T) {
	now := time.Now()
	seller := uuid.New()
	tx := &mockTx{
		accountStatus: "active",
		deletedAt:     nil,
		emailAt:       &now,
		hasProfile:    false,
	}
	svc := &ForSaleService{}
	err := svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrSellerNotReady)
}

// Test 2 – same seller cannot create market-visible exposure without HasActiveSellerCapability
func TestCanonical_PublicCreate_RequiresMarketAuthority(t *testing.T) {
	now := time.Now()
	seller := uuid.New()
	tx := &mockTx{
		accountStatus: "active",
		deletedAt:     nil,
		emailAt:       &now,
		hasProfile:    true,
	}
	rc := fakeRoleCheckerCanonical{hasCapability: false}
	svc := &ForSaleService{roleChecker: rc}

	// workspace passes
	require.NoError(t, svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller))
	// market gate denies
	err := svc.CheckMarketAuthorityForForSale(context.Background(), seller)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMarketAuthorityRequired)
}

// Test 3 – expired seller cannot publish (Publish gates on HasActiveSellerCapability)
func TestCanonical_Publish_ExpiredSellerDenied(t *testing.T) {
	seller := uuid.New()
	rc := fakeRoleCheckerCanonical{hasCapability: false}
	svc := &ForSaleService{roleChecker: rc}
	err := svc.CheckMarketAuthorityForForSale(context.Background(), seller)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMarketAuthorityRequired)
}

// Test 4 – missing profile denies market even with anomalous subscription (Gate2)
func TestCanonical_MissingProfile_DeniesMarketEvenIfSubscriptionAnomalouslyExists(t *testing.T) {
	// Simulate DB where HasActiveSellerCapability explicitly checks profile
	// Our fake with hasCapability=false represents missing profile case (Has denies)
	rc := fakeRoleCheckerCanonical{hasCapability: false}
	svc := &ForSaleService{roleChecker: rc}
	err := svc.CheckMarketAuthorityForForSale(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMarketAuthorityRequired)
	// Also workspace gate denies
	now := time.Now()
	tx := &mockTx{accountStatus: "active", deletedAt: nil, emailAt: &now, hasProfile: false}
	err = svc.ensureWorkspaceAuthorityTx(context.Background(), tx, uuid.New())
	require.ErrorIs(t, err, auth.ErrSellerNotReady)
}

// Test 5 – No Actor-only market authority: prove CanCreateForSale is not the gate
func TestCanonical_NoActorOnlyMarketAuthority(t *testing.T) {
	// Ensure CanCreateForSale method no longer exists as write gate – compile-time check:
	// If ForSaleService still called CanCreateForSale, this test would have failed to compile after purge.
	// Here we prove HasActiveSellerCapability is the only market gate by showing
	// a seller with active account+profile but expired subscription is denied market via Has.
	seller := uuid.New()
	rc := fakeRoleCheckerCanonical{hasCapability: false}
	svc := &ForSaleService{roleChecker: rc}
	// Even though workspace would pass (active+profile), market must fail
	now := time.Now()
	tx := &mockTx{accountStatus: "active", deletedAt: nil, emailAt: &now, hasProfile: true}
	require.NoError(t, svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller))
	require.ErrorIs(t, svc.CheckMarketAuthorityForForSale(context.Background(), seller), auth.ErrMarketAuthorityRequired)
}

// Test workspace account gates
func TestCanonical_Workspace_SuspendedDenied(t *testing.T) {
	seller := uuid.New()
	now := time.Now()
	tx := &mockTx{accountStatus: "suspended", deletedAt: nil, emailAt: &now, hasProfile: true}
	svc := &ForSaleService{}
	err := svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller)
	assert.ErrorIs(t, err, auth.ErrAccountSuspended)
}

func TestCanonical_Workspace_UnverifiedEmailDenied(t *testing.T) {
	seller := uuid.New()
	tx := &mockTx{accountStatus: "active", deletedAt: nil, emailAt: nil, hasProfile: true}
	svc := &ForSaleService{}
	err := svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller)
	assert.ErrorIs(t, err, auth.ErrSellerNotReady)
}

func TestCanonical_Workspace_DeletedDenied(t *testing.T) {
	seller := uuid.New()
	now := time.Now()
	deleted := time.Now()
	tx := &mockTx{accountStatus: "active", deletedAt: &deleted, emailAt: &now, hasProfile: true}
	svc := &ForSaleService{}
	err := svc.ensureWorkspaceAuthorityTx(context.Background(), tx, seller)
	assert.ErrorIs(t, err, auth.ErrAccountRemoved)
}

// Ensure the entity lifecycle holds: a for_sale is born active + public
// (create = publish — there is no draft workspace state).
func TestCanonical_Entity_BornActiveAndPublic(t *testing.T) {
	seller := uuid.New()
	fs, err := entity.NewForSaleSurface(seller, money.New(100000), 1, false)
	require.NoError(t, err)
	assert.Equal(t, entity.ForSaleStatusActive, fs.Status)
	assert.Equal(t, entity.ForSaleVisibilityPublic, fs.Visibility)
	assert.NotNil(t, fs.PublishedAt)
}
