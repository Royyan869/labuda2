package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	withdrawrepo "github.com/hishumi/backend/internal/finance/infrastructure/repository"
	"github.com/hishumi/backend/internal/platform/capability"
	capabilityEntity "github.com/hishumi/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeWhitelistAuditRepo is an in-memory keyset repository used to prove the
// whitelist audit handler contract without a database.
type fakeWhitelistAuditRepo struct {
	records       []withdrawrepo.WhitelistAuditRecord
	page          bool // true when ListBySeller was used
	receivedLimit int
	receivedCur   *withdrawrepo.WhitelistAuditCursor
}

func (f *fakeWhitelistAuditRepo) Append(context.Context, withdrawrepo.WhitelistAuditRecord) error {
	return nil
}

func (f *fakeWhitelistAuditRepo) pageAfter(cursor *withdrawrepo.WhitelistAuditCursor, limit int) []withdrawrepo.WhitelistAuditRecord {
	out := make([]withdrawrepo.WhitelistAuditRecord, 0, limit)
	for _, rec := range f.records {
		if cursor != nil {
			// Keyset: strictly older than (created_at, id).
			if rec.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if rec.CreatedAt.Equal(cursor.CreatedAt) && rec.ID.String() >= cursor.ID.String() {
				continue
			}
		}
		out = append(out, rec)
		if len(out) == limit {
			break
		}
	}
	return out
}

func (f *fakeWhitelistAuditRepo) List(_ context.Context, limit int, cursor *withdrawrepo.WhitelistAuditCursor) ([]withdrawrepo.WhitelistAuditRecord, error) {
	f.receivedLimit = limit
	f.receivedCur = cursor
	return f.pageAfter(cursor, limit), nil
}

func (f *fakeWhitelistAuditRepo) ListBySeller(_ context.Context, sellerID uuid.UUID, limit int, cursor *withdrawrepo.WhitelistAuditCursor) ([]withdrawrepo.WhitelistAuditRecord, error) {
	f.page = true
	f.receivedLimit = limit
	f.receivedCur = cursor
	filtered := make([]withdrawrepo.WhitelistAuditRecord, 0, len(f.records))
	for _, rec := range f.records {
		if rec.SellerID != nil && *rec.SellerID == sellerID {
			filtered = append(filtered, rec)
		}
	}
	f.records = filtered
	return f.pageAfter(cursor, limit), nil
}

func newWhitelistAuditRouter(repo withdrawrepo.WhitelistAuditRepository, withCap bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		caps := []string{}
		if withCap {
			caps = []string{"finance.withdraw.read"}
		}
		actor := &capabilityEntity.Actor{ID: uuid.New(), Role: "admin", Capabilities: caps}
		c.Request = c.Request.WithContext(capability.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	handler := &AdminPayoutHandler{whitelistAuditRepo: repo, log: zap.NewNop()}
	router.GET("/admin/payouts/whitelist/audit", handler.ListWhitelistAudit)
	return router
}

func whitelistRow(t *testing.T, createdAt time.Time) withdrawrepo.WhitelistAuditRecord {
	t.Helper()
	return withdrawrepo.WhitelistAuditRecord{
		ID:        uuid.New(),
		Action:    "SELLER_ADDED",
		ActorID:   "admin",
		Reason:    "test",
		Source:    "config",
		CreatedAt: createdAt,
	}
}

func decodeWhitelistBody(t *testing.T, body []byte) (rows []map[string]interface{}, hasMore bool, nextCursor *string) {
	t.Helper()
	var parsed struct {
		AuditLog   []map[string]interface{} `json:"audit_log"`
		HasMore    bool                     `json:"has_more"`
		NextCursor *string                  `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.AuditLog, parsed.HasMore, parsed.NextCursor
}

func TestListWhitelistAudit_KeysetContract(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	recs := []withdrawrepo.WhitelistAuditRecord{
		whitelistRow(t, base),                     // newest
		whitelistRow(t, base.Add(-time.Hour)),     // middle
		whitelistRow(t, base.Add(-2*time.Hour)),   // oldest
	}

	t.Run("first page returns limit rows with cursor and has_more", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{records: recs}
		router := newWhitelistAuditRouter(repo, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit?limit=2", nil)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		rows, hasMore, nextCursor := decodeWhitelistBody(t, w.Body.Bytes())
		assert.Len(t, rows, 2)
		assert.True(t, hasMore)
		require.NotNil(t, nextCursor)
		// Handler fetched limit+1 to decide has_more without COUNT.
		assert.Equal(t, 3, repo.receivedLimit)
		assert.Nil(t, repo.receivedCur)
	})

	t.Run("continuation page advances keyset and terminates", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{records: recs}
		router := newWhitelistAuditRouter(repo, true)

		first := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit?limit=2", nil)
		router.ServeHTTP(first, req)
		_, _, nextCursor := decodeWhitelistBody(t, first.Body.Bytes())
		require.NotNil(t, nextCursor)

		second := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit?limit=2&cursor="+*nextCursor, nil)
		router.ServeHTTP(second, req2)

		require.Equal(t, http.StatusOK, second.Code)
		rows, hasMore, next := decodeWhitelistBody(t, second.Body.Bytes())
		assert.Len(t, rows, 1)
		assert.False(t, hasMore)
		assert.Nil(t, next)
		require.NotNil(t, repo.receivedCur)
		assert.Equal(t, recs[1].ID, repo.receivedCur.ID)
	})

	t.Run("no records yields empty page and no cursor", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{}
		router := newWhitelistAuditRouter(repo, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit", nil)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		rows, hasMore, next := decodeWhitelistBody(t, w.Body.Bytes())
		assert.Empty(t, rows)
		assert.False(t, hasMore)
		assert.Nil(t, next)
	})

	t.Run("seller filter routes to ListBySeller", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{records: recs}
		router := newWhitelistAuditRouter(repo, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit?seller_id="+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, repo.page)
	})

	t.Run("malformed cursor is rejected", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{records: recs}
		router := newWhitelistAuditRouter(repo, true)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit?cursor=not-base64!!", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing capability is forbidden", func(t *testing.T) {
		repo := &fakeWhitelistAuditRepo{records: recs}
		router := newWhitelistAuditRouter(repo, false)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/payouts/whitelist/audit", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestDecodeWhitelistAuditCursorRejectsGarbage(t *testing.T) {
	raw := base64.RawURLEncoding.EncodeToString([]byte("not-a-timestamp|not-a-uuid"))
	_, err := decodeWhitelistAuditCursor(raw)
	require.Error(t, err)
}
