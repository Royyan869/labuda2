//go:build integration

// REAL POSTGRESQL PROOF — PAYOUT WHITELIST AUDIT KEYSET CURSOR.
//
// Proves the whitelist audit endpoint's keyset pagination end-to-end against a
// live PostgreSQL schema (via the canonical pkg/testdb infrastructure):
// deterministic (created_at DESC, id DESC) ordering, same-timestamp tie-break,
// cursor continuation with no duplicates and no missing rows, seller-filter
// boundary isolation, final-page has_more=false, and the truthful HTTP
// contract (audit_log/limit/has_more/next_cursor; no offset/page/fake count).
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	financerepo "github.com/hishumi/backend/internal/finance/infrastructure/repository"
	"github.com/hishumi/backend/internal/platform/capability"
	capabilityEntity "github.com/hishumi/backend/internal/platform/capability/entity"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
)

type auditHTTPResponse struct {
	AuditLog []struct {
		ID        uuid.UUID `json:"id"`
		SellerID  *uuid.UUID `json:"seller_id"`
		Action    string    `json:"action"`
		Reason    string    `json:"reason"`
		CreatedAt string    `json:"created_at"`
	} `json:"audit_log"`
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
}

func newWhitelistIntegrationRouter(repo financerepo.WhitelistAuditRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		actor := &capabilityEntity.Actor{ID: uuid.New(), Role: "admin", Capabilities: []string{"finance.withdraw.read"}}
		c.Request = c.Request.WithContext(capability.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	h := &AdminPayoutHandler{whitelistAuditRepo: repo, log: zap.NewNop()}
	router.GET("/admin/payouts/whitelist/audit", h.ListWhitelistAudit)
	return router
}

// resetWhitelistAudit isolates this test from any rows left by a previously
// failed run (testdb only truncates on success).
func resetWhitelistAudit(t *testing.T, ctx context.Context, tdb *testdb.TestDB) {
	t.Helper()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM payout_whitelist_audit_logs")
		return err
	}))
}

func getAuditPage(t *testing.T, router *gin.Engine, query string) (auditHTTPResponse, map[string]json.RawMessage) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/payouts/whitelist/audit"+query, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp auditHTTPResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	return resp, raw
}

func TestWhitelistAudit_RealDB_KeysetCursor(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	resetWhitelistAudit(t, ctx, tdb)
	repo := financerepo.NewWhitelistAuditRepository(db.NewFromPool(tdb.Pool()))
	router := newWhitelistIntegrationRouter(repo)

	sellerA := uuid.New()
	sellerB := uuid.New()
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	seed := []struct {
		reason    string
		seller    *uuid.UUID
		createdAt time.Time
	}{
		{"r1", &sellerA, base},                     // same timestamp group
		{"r2", &sellerA, base},                     // same timestamp group
		{"r3", &sellerA, base},                     // same timestamp group
		{"r4", &sellerA, base.Add(1 * time.Minute)},  // newest
		{"r5", &sellerA, base.Add(-1 * time.Minute)}, // older
		{"r6", &sellerB, base.Add(-2 * time.Minute)}, // oldest, different seller
	}
	for _, s := range seed {
		require.NoError(t, repo.Append(ctx, financerepo.WhitelistAuditRecord{
			SellerID:  s.seller,
			Action:    "SELLER_ADDED",
			ActorID:   "admin",
			Reason:    s.reason,
			Source:    "config",
			CreatedAt: s.createdAt,
		}))
	}

	// The authoritative global keyset order is the full one-query listing
	// (created_at DESC, id DESC); pagination must reproduce it exactly.
	all, err := repo.List(ctx, 100, nil)
	require.NoError(t, err)
	require.Len(t, all, len(seed))
	expected := all

	byReason := make(map[string]financerepo.WhitelistAuditRecord, len(all))
	for _, r := range all {
		byReason[r.Reason] = r
	}
	// The three same-timestamp records must be a genuine tie and must be
	// ordered by id DESC in the authoritative listing.
	require.True(t, byReason["r1"].CreatedAt.Equal(byReason["r2"].CreatedAt))
	require.True(t, byReason["r2"].CreatedAt.Equal(byReason["r3"].CreatedAt))
	var tieIDs []uuid.UUID
	for _, r := range all {
		if r.Reason == "r1" || r.Reason == "r2" || r.Reason == "r3" {
			tieIDs = append(tieIDs, r.ID)
		}
	}
	require.Len(t, tieIDs, 3)
	for i := 0; i+1 < len(tieIDs); i++ {
		require.Equal(t, 1, bytes.Compare(tieIDs[i][:], tieIDs[i+1][:]), "same-timestamp rows must be id DESC: %v", tieIDs)
	}

	expectedIDs := func(recs []financerepo.WhitelistAuditRecord) []uuid.UUID {
		ids := make([]uuid.UUID, len(recs))
		for i, r := range recs {
			ids[i] = r.ID
		}
		return ids
	}
	gotIDs := func(resp auditHTTPResponse) []uuid.UUID {
		ids := make([]uuid.UUID, len(resp.AuditLog))
		for i, r := range resp.AuditLog {
			ids[i] = r.ID
		}
		return ids
	}

	t.Run("A/B first page is the newest records in the canonical order", func(t *testing.T) {
		resp, raw := getAuditPage(t, router, "?limit=2")
		require.Equal(t, 2, resp.Limit)
		assert.True(t, resp.HasMore)
		require.NotNil(t, resp.NextCursor)
		assert.Equal(t, expectedIDs(expected[:2]), gotIDs(resp))

		// Same-timestamp tie-break: the second row of page 1 is the HIGHEST id
		// of the T-group (ordering is id DESC for an equal created_at).
		assert.Equal(t, byReason["r4"].ID, resp.AuditLog[0].ID)
		assert.Equal(t, tieIDs[0], resp.AuditLog[1].ID)

		// HTTP contract: no offset/page/fake count.
		_, hasOffset := raw["offset"]
		_, hasCount := raw["count"]
		_, hasPage := raw["page"]
		assert.False(t, hasOffset, "offset residue")
		assert.False(t, hasCount, "fake count residue")
		assert.False(t, hasPage, "page residue")
		assert.Len(t, raw, 4, "exactly audit_log/limit/has_more/next_cursor")
	})

	t.Run("C/D/E cursor continuation is lossless and terminates", func(t *testing.T) {
		var collected []uuid.UUID
		cursor := ""
		for i := 0; i < 10; i++ {
			q := "?limit=2"
			if cursor != "" {
				q += "&cursor=" + cursor
			}
			resp, _ := getAuditPage(t, router, q)
			collected = append(collected, gotIDs(resp)...)
			if !resp.HasMore {
				assert.Nil(t, resp.NextCursor, "terminal page has a null next_cursor")
				break
			}
			require.NotNil(t, resp.NextCursor)
			cursor = *resp.NextCursor
		}

		// No duplicates, no missing, exact global order.
		assert.Equal(t, expectedIDs(expected), collected)
		assert.Equal(t, len(expected), len(collected))
	})
}

func TestWhitelistAudit_RealDB_SellerFilterBoundary(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := financerepo.NewWhitelistAuditRepository(db.NewFromPool(tdb.Pool()))
	router := newWhitelistIntegrationRouter(repo)

	sellerA := uuid.New()
	sellerB := uuid.New()
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		require.NoError(t, repo.Append(ctx, financerepo.WhitelistAuditRecord{
			SellerID: &sellerA, Action: "SELLER_ADDED", ActorID: "admin",
			Reason: "a" + string(rune('0'+i)), Source: "config",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}))
	}
	require.NoError(t, repo.Append(ctx, financerepo.WhitelistAuditRecord{
		SellerID: &sellerB, Action: "SELLER_ADDED", ActorID: "admin",
		Reason: "b0", Source: "config", CreatedAt: base.Add(10 * time.Minute),
	}))

	resp, _ := getAuditPage(t, router, "?seller_id="+sellerA.String()+"&limit=10")
	require.False(t, resp.HasMore)
	for _, row := range resp.AuditLog {
		require.NotNil(t, row.SellerID)
		assert.Equal(t, sellerA, *row.SellerID, "cursor must not cross into another seller")
		assert.NotEqual(t, "b0", row.Reason)
	}
	assert.Len(t, resp.AuditLog, 4)

	// Paginating within the seller filter remains inside the seller boundary.
	resp2, _ := getAuditPage(t, router, "?seller_id="+sellerB.String()+"&limit=1")
	require.Len(t, resp2.AuditLog, 1)
	assert.Equal(t, "b0", resp2.AuditLog[0].Reason)
	assert.False(t, resp2.HasMore)
	assert.Nil(t, resp2.NextCursor)
}
