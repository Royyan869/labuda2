package http

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/viewercontext"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	capabilityentity "github.com/labuda/backend/internal/platform/capability/entity"
)

// ============================================================================
// Test helpers
// ============================================================================

func newTestGinContext(setKeys map[string]interface{}) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	for k, v := range setKeys {
		c.Set(k, v)
	}
	return c
}

// mockTx implements db.Tx for testing. Only QueryRow is functionally
// implemented; other methods return zero values since hydrateViewerLifecycle
// only uses QueryRow.
type mockTx struct {
	row *mockRow
}

func (m *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return m.row }
func (m *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockTx) Commit(_ context.Context) error   { return nil }
func (m *mockTx) Rollback(_ context.Context) error { return nil }

// mockRow implements pgx.Row for testing.
type mockRow struct {
	accountStatus string
	deletedAt     interface{} // nil = no deleted_at; non-nil = deleted
	err           error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) >= 1 {
		if p, ok := dest[0].(*string); ok {
			*p = r.accountStatus
		}
	}
	if len(dest) >= 2 {
		if p, ok := dest[1].(*interface{}); ok {
			*p = r.deletedAt
		}
	}
	return nil
}

// activeMockTx returns a mockTx that simulates a DB row with account_status='active'
// and deleted_at=NULL.
func activeMockTx() *mockTx {
	return &mockTx{row: &mockRow{accountStatus: "active", deletedAt: nil}}
}

func sellerActor() *capabilityentity.Actor {
	status := string(capabilityentity.SellerStatusActive)
	return &capabilityentity.Actor{
		Role:          "user",
		AccountStatus: "active",
		EmailVerified: true,
		SellerStatus:  &status,
	}
}

// ============================================================================
// Existing construction tests — updated to pass tx
// ============================================================================

// TestConstructSearchContentViewerContext_Anonymous verifies that absent
// user_id in the gin context produces an explicit AnonymousViewer per
// docs/03-architecture/viewer-context-contract.md §3.1.
// Anonymous path returns before the lifecycle DB query; tx is not used.
func TestConstructSearchContentViewerContext_Anonymous(t *testing.T) {
	c := newTestGinContext(nil)

	vc := constructSearchContentViewerContext(c, nil)
	if vc == nil {
		t.Fatal("constructSearchContentViewerContext returned nil — viewer-context-contract.md §8.1 violated")
	}
	if !vc.IsAnonymous() {
		t.Error("Anonymous request did not produce AnonymousViewer")
	}
	if vc.Surface() != viewercontext.SurfacePublicDiscovery {
		t.Errorf("Surface = %q; want %q", vc.Surface(), viewercontext.SurfacePublicDiscovery)
	}
	if vc.Origin() != viewercontext.RequestOriginREST {
		t.Errorf("Origin = %q; want %q", vc.Origin(), viewercontext.RequestOriginREST)
	}
	rel := vc.Relationship()
	if !rel.IsAnonymousEmpty() {
		t.Error("AnonymousViewer relationship overlay must be hydrated-as-anonymous-empty per §3.1")
	}
	// Anonymous lifecycle is active + hydrated=true by topology definition.
	if !vc.Lifecycle().IsHydrated() {
		t.Error("AnonymousViewer lifecycle must be IsHydrated()=true (active by topology)")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("AnonymousViewer lifecycle.State = %q; want %q", vc.Lifecycle().State, viewercontext.PublicLifecycleStateActive)
	}
}

// TestConstructSearchContentViewerContext_NilUUID verifies the fallback to
// AnonymousViewer when user_id is uuid.Nil per the §8.1 nil-fallback rule.
// This path returns before the lifecycle DB query; tx is not used.
func TestConstructSearchContentViewerContext_NilUUID(t *testing.T) {
	c := newTestGinContext(map[string]interface{}{
		"user_id": uuid.Nil,
	})

	vc := constructSearchContentViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Error("uuid.Nil user_id did not fall back to AnonymousViewer")
	}
}

// TestConstructSearchContentViewerContext_WrongType verifies the fallback
// to AnonymousViewer when user_id is not a uuid.UUID.
// This path returns before the lifecycle DB query; tx is not used.
func TestConstructSearchContentViewerContext_WrongType(t *testing.T) {
	c := newTestGinContext(map[string]interface{}{
		"user_id": "not-a-uuid",
	})

	vc := constructSearchContentViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Error("user_id of wrong type did not fall back to AnonymousViewer")
	}
}

// TestConstructSearchContentViewerContext_Authenticated verifies that a
// resolvable user_id produces an AuthenticatedViewer with the canonical
// identity overlay shape per §3.2 / §4.1. The mock tx returns active status.
func TestConstructSearchContentViewerContext_Authenticated(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{
		"user_id":      userID,
		"firebase_uid": "firebase-test-uid",
	})
	c.Request = httptest.NewRequest("GET", "/search/content", nil)
	c.Request = c.Request.WithContext(capabilityctx.WithActor(c.Request.Context(), sellerActor()))

	vc := constructSearchContentViewerContext(c, activeMockTx())
	if vc.IsAnonymous() {
		t.Fatal("Authenticated request produced AnonymousViewer")
	}
	if vc.Identity().CanonicalUserID != userID {
		t.Errorf("CanonicalUserID = %v; want %v", vc.Identity().CanonicalUserID, userID)
	}
	if vc.Identity().FirebaseUID != "firebase-test-uid" {
		t.Errorf("FirebaseUID = %q; want %q", vc.Identity().FirebaseUID, "firebase-test-uid")
	}
	if !vc.Capability().IsSeller {
		t.Error("Capability.IsSeller was lost during construction")
	}
	if vc.Capability().IsAdmin {
		t.Error("Capability.IsAdmin was set despite no is_admin in gin context")
	}
	rel := vc.Relationship()
	if rel.IsHydrated() {
		t.Error("AuthenticatedViewer relationship must be unhydrated until WithRelationship attaches a resolved set")
	}
}

// TestConstructSearchContentViewerContext_AlternativeKey verifies the
// fallback to "userID" when "user_id" is absent.
func TestConstructSearchContentViewerContext_AlternativeKey(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{
		"userID": userID,
	})

	vc := constructSearchContentViewerContext(c, activeMockTx())
	if vc.IsAnonymous() {
		t.Fatal("Authenticated request via alternative key produced AnonymousViewer")
	}
	if vc.Identity().CanonicalUserID != userID {
		t.Errorf("CanonicalUserID = %v; want %v", vc.Identity().CanonicalUserID, userID)
	}
}

// TestConstructSearchContentViewerContext_Immutable verifies that the
// returned ViewerContext does not capture the gin context in a way that
// allows post-construction mutation per viewer-context-contract.md §8.3.
func TestConstructSearchContentViewerContext_Immutable(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{
		"user_id": userID,
	})

	vc := constructSearchContentViewerContext(c, activeMockTx())

	// Mutate the gin context after construction.
	c.Set("user_id", uuid.New())
	c.Set("is_admin", true)

	if vc.Identity().CanonicalUserID != userID {
		t.Error("ViewerContext mutated by post-construction gin context change — violates §8.3")
	}
	if vc.Capability().IsAdmin {
		t.Error("Capability.IsAdmin was added by post-construction gin context change — violates §8.3")
	}
}

// ============================================================================
// Lifecycle hydration tests (new per PHASE C hydration task)
// ============================================================================

// TestViewerLifecycle_ActiveViewer verifies that account_status='active' with
// no deleted_at produces lifecycle.State=active, IsHydrated()=true per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-task-design.md §4.1-§4.2.
func TestViewerLifecycle_ActiveViewer(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	tx := &mockTx{row: &mockRow{accountStatus: "active", deletedAt: nil}}

	vc := constructSearchContentViewerContext(c, tx)

	if vc.IsAnonymous() {
		t.Fatal("Active viewer produced AnonymousViewer")
	}
	if !vc.Lifecycle().IsHydrated() {
		t.Error("Active viewer lifecycle: IsHydrated()=false; want true")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("Active viewer lifecycle.State = %q; want %q", vc.Lifecycle().State, viewercontext.PublicLifecycleStateActive)
	}
}

// TestViewerLifecycle_SuspendedViewer verifies that account_status='suspended'
// produces lifecycle.State=unavailable, IsHydrated()=true per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-task-design.md §4.4.
func TestViewerLifecycle_SuspendedViewer(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	tx := &mockTx{row: &mockRow{accountStatus: "suspended", deletedAt: nil}}

	vc := constructSearchContentViewerContext(c, tx)

	if !vc.Lifecycle().IsHydrated() {
		t.Error("Suspended viewer lifecycle: IsHydrated()=false; want true")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateUnavailable {
		t.Errorf("Suspended viewer lifecycle.State = %q; want %q", vc.Lifecycle().State, viewercontext.PublicLifecycleStateUnavailable)
	}
}

// TestViewerLifecycle_BannedViewer verifies that account_status='banned'
// produces lifecycle.State=unavailable, IsHydrated()=true.
func TestViewerLifecycle_BannedViewer(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	tx := &mockTx{row: &mockRow{accountStatus: "banned", deletedAt: nil}}

	vc := constructSearchContentViewerContext(c, tx)

	if !vc.Lifecycle().IsHydrated() {
		t.Error("Banned viewer lifecycle: IsHydrated()=false; want true")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateUnavailable {
		t.Errorf("Banned viewer lifecycle.State = %q; want %q", vc.Lifecycle().State, viewercontext.PublicLifecycleStateUnavailable)
	}
}

// TestViewerLifecycle_DeletedAtPresent verifies that a non-nil deleted_at
// produces lifecycle.State=removed, IsHydrated()=true regardless of
// account_status, per docs/05-rollout/search-content-viewer-lifecycle-
// hydration-runtime-task-design.md §4.3.
func TestViewerLifecycle_DeletedAtPresent(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	// deleted_at non-nil → removed (terminal; overrides account_status).
	tx := &mockTx{row: &mockRow{accountStatus: "active", deletedAt: "2024-01-01T00:00:00Z"}}

	vc := constructSearchContentViewerContext(c, tx)

	if !vc.Lifecycle().IsHydrated() {
		t.Error("Soft-deleted viewer lifecycle: IsHydrated()=false; want true")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateRemoved {
		t.Errorf("Soft-deleted viewer lifecycle.State = %q; want %q", vc.Lifecycle().State, viewercontext.PublicLifecycleStateRemoved)
	}
}

// TestViewerLifecycle_DBError verifies that a DB query error produces
// lifecycle.IsHydrated()=false without failing the HTTP request per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-task-design.md §6.1.
func TestViewerLifecycle_DBError(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	tx := &mockTx{row: &mockRow{err: errors.New("simulated DB error")}}

	vc := constructSearchContentViewerContext(c, tx)

	if vc == nil {
		t.Fatal("constructSearchContentViewerContext returned nil on DB error — must continue without HTTP failure")
	}
	if vc.IsAnonymous() {
		t.Error("DB error must not degrade to AnonymousViewer — caller is authenticated")
	}
	if vc.Lifecycle().IsHydrated() {
		t.Error("DB error lifecycle: IsHydrated()=true; want false")
	}
}

// TestViewerLifecycle_MissingRow verifies that a missing users row
// (pgx.ErrNoRows equivalent) produces IsHydrated()=false per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-task-design.md §6.2.
func TestViewerLifecycle_MissingRow(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})
	tx := &mockTx{row: &mockRow{err: pgx.ErrNoRows}}

	vc := constructSearchContentViewerContext(c, tx)

	if vc == nil {
		t.Fatal("constructSearchContentViewerContext returned nil on missing row")
	}
	if vc.IsAnonymous() {
		t.Error("Missing row must not degrade to AnonymousViewer — caller is authenticated")
	}
	if vc.Lifecycle().IsHydrated() {
		t.Error("Missing row lifecycle: IsHydrated()=true; want false")
	}
}

// TestViewerLifecycle_AnonymousIsHydrated verifies that the AnonymousViewer
// lifecycle is always IsHydrated()=true with State=active by topology
// definition per docs/05-rollout/search-content-viewer-lifecycle-hydration-
// runtime-task-design.md §4.5 / §6.5. No DB query is issued for anonymous.
func TestViewerLifecycle_AnonymousIsHydrated(t *testing.T) {
	c := newTestGinContext(nil) // no user_id → anonymous

	// Pass nil tx — anonymous path returns before the DB query.
	vc := constructSearchContentViewerContext(c, nil)

	if !vc.IsAnonymous() {
		t.Fatal("Expected AnonymousViewer")
	}
	if !vc.Lifecycle().IsHydrated() {
		t.Error("AnonymousViewer lifecycle: IsHydrated()=false; want true (active by topology)")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("AnonymousViewer lifecycle.State = %q; want active", vc.Lifecycle().State)
	}
}

// TestViewerLifecycle_NilTx verifies that a nil tx for an authenticated
// viewer produces IsHydrated()=false without HTTP failure, consistent with
// treating nil tx as DB error per hydrateViewerLifecycle §6.1.
func TestViewerLifecycle_NilTx(t *testing.T) {
	userID := uuid.New()
	c := newTestGinContext(map[string]interface{}{"user_id": userID})

	vc := constructSearchContentViewerContext(c, nil)

	if vc == nil {
		t.Fatal("constructSearchContentViewerContext returned nil on nil tx")
	}
	if vc.IsAnonymous() {
		t.Error("Nil tx must not degrade to AnonymousViewer — caller is authenticated")
	}
	if vc.Lifecycle().IsHydrated() {
		t.Error("Nil tx lifecycle: IsHydrated()=true; want false")
	}
}
