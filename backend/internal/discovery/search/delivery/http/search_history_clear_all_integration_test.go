package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/application"
	searchrepo "github.com/labuda/backend/internal/discovery/search/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newSearchHistoryHandlerForTest(t *testing.T, tdb *testdb.TestDB) *SearchHandler {
	t.Helper()

	database := db.NewFromPool(tdb.Pool())
	svc := application.NewSearchService(searchrepo.NewSearchRepository())
	return NewSearchHandler(svc, database, zap.NewNop(), nil, nil)
}

func injectUserID(userID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	}
}

func insertSearchHistoryRow(t *testing.T, tdb *testdb.TestDB, userID uuid.UUID, query string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = 'replica'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO search_history (id, user_id, query, created_at)
			VALUES ($1, $2, $3, NOW())
		`, id, userID, query)
		return err
	})
	if err != nil {
		t.Fatalf("insert search history row: %v", err)
	}
	return id
}

func countSearchHistoryRows(t *testing.T, tdb *testdb.TestDB, userID uuid.UUID) int64 {
	t.Helper()

	var count int64
	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM search_history
			WHERE user_id = $1
		`, userID).Scan(&count)
	})
	if err != nil {
		t.Fatalf("count search history rows: %v", err)
	}
	return count
}

func countSearchHistoryRowByID(t *testing.T, tdb *testdb.TestDB, id uuid.UUID) int64 {
	t.Helper()

	var count int64
	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM search_history
			WHERE id = $1
		`, id).Scan(&count)
	})
	if err != nil {
		t.Fatalf("count search history row by id: %v", err)
	}
	return count
}

func performSearchHistoryRequest(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSearchHistoryClearAll_DeleteOne_And_AuthBehavior(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	handler := newSearchHistoryHandlerForTest(t, tdb)

	t.Run("clear_all_removes_only_authenticated_users_rows", func(t *testing.T) {
		userID := uuid.New()
		otherUserID := uuid.New()

		insertSearchHistoryRow(t, tdb, userID, "Kohaku")
		insertSearchHistoryRow(t, tdb, userID, "Sanke")
		insertSearchHistoryRow(t, tdb, otherUserID, "Showa")

		router := gin.New()
		router.DELETE("/search/history", injectUserID(userID), handler.ClearSearchHistory)

		rec := performSearchHistoryRequest(t, router, http.MethodDelete, "/search/history")
		if rec.Code != http.StatusOK {
			t.Fatalf("DELETE /search/history returned %d, want %d", rec.Code, http.StatusOK)
		}

		if got := countSearchHistoryRows(t, tdb, userID); got != 0 {
			t.Fatalf("authenticated user's search history rows = %d, want 0", got)
		}
		if got := countSearchHistoryRows(t, tdb, otherUserID); got != 1 {
			t.Fatalf("other user's search history rows = %d, want 1", got)
		}
	})

	t.Run("delete_one_still_deletes_exactly_one_row", func(t *testing.T) {
		userID := uuid.New()
		otherUserID := uuid.New()

		targetID := insertSearchHistoryRow(t, tdb, userID, "Kohaku")
		survivorID := insertSearchHistoryRow(t, tdb, userID, "Sanke")
		insertSearchHistoryRow(t, tdb, otherUserID, "Showa")

		router := gin.New()
		router.DELETE("/search/history/:id", injectUserID(userID), handler.DeleteSearchHistory)

		rec := performSearchHistoryRequest(t, router, http.MethodDelete, "/search/history/"+targetID.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("DELETE /search/history/:id returned %d, want %d", rec.Code, http.StatusOK)
		}

		if got := countSearchHistoryRowByID(t, tdb, targetID); got != 0 {
			t.Fatalf("target search history row count = %d, want 0", got)
		}
		if got := countSearchHistoryRowByID(t, tdb, survivorID); got != 1 {
			t.Fatalf("survivor search history row count = %d, want 1", got)
		}
		if got := countSearchHistoryRows(t, tdb, otherUserID); got != 1 {
			t.Fatalf("other user's search history rows = %d, want 1", got)
		}
	})

	t.Run("auth_failures_match_canonical_behavior", func(t *testing.T) {
		routerMissing := gin.New()
		routerMissing.DELETE("/search/history", handler.ClearSearchHistory)

		rec := performSearchHistoryRequest(t, routerMissing, http.MethodDelete, "/search/history")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("DELETE /search/history without userID returned %d, want %d", rec.Code, http.StatusUnauthorized)
		}

		routerMalformed := gin.New()
		routerMalformed.DELETE("/search/history", func(c *gin.Context) {
			c.Set("userID", "not-a-uuid")
			handler.ClearSearchHistory(c)
		})

		rec = performSearchHistoryRequest(t, routerMalformed, http.MethodDelete, "/search/history")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("DELETE /search/history with malformed userID returned %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}
