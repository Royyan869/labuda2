//go:build integration

package http

// PASS 9 closure — runtime proof of the canonical Content idempotency contract
// split (owner decision, locked):
//
//	PUT  /api/v1/contents/:id  → NO application-level idempotency, NO Idempotency-Key
//	POST /api/v1/contents      → Idempotency-Key REQUIRED, idempotency enforced
//
// This is the executable counterpart to the static removal of the obsolete PUT
// header requirement in content_handler.go (UpdateContent). It fails if a PUT
// key requirement, a key-derived replay, or a key-derived conflict is ever
// reintroduced, or if POST /contents loses its mandatory key.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// putUpdate drives handler.UpdateContent directly. key == "" omits the
// Idempotency-Key header entirely (the keyless canonical path).
func putUpdate(t *testing.T, handler *ContentHandler, contentID, userID uuid.UUID, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
	c.Set("userID", userID)
	handler.UpdateContent(c)
	return w
}

func TestContentIdempotencyContract_UpdateKeyless_CreateRequiresKey(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)
	userID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	var contentID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		content, _, err := handler.contentService.CreateContentIdempotent(
			ctx,
			tx,
			userID,
			uuid.NewString(),
			"original caption",
			contententity.VisibilityPublic,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		contentID = content.ID
		return nil
	}))

	readCaption := func() string {
		var caption string
		require.NoError(t, tdb.Pool().QueryRow(ctx,
			`SELECT COALESCE(caption, '') FROM contents WHERE id = $1`, contentID,
		).Scan(&caption))
		return caption
	}

	// PUT requires no Idempotency-Key: a keyless update is accepted and persisted.
	t.Run("PUT without Idempotency-Key succeeds", func(t *testing.T) {
		w := putUpdate(t, handler, contentID, userID, "", `{"caption":"updated without key"}`)
		require.Equal(t, http.StatusOK, w.Code,
			"PUT must succeed without Idempotency-Key (no update idempotency contract)")
		require.Equal(t, "updated without key", readCaption())
	})

	// A supplied key is inert: no consumption, no replay, no conflict.
	t.Run("PUT ignores a supplied Idempotency-Key (no replay/conflict)", func(t *testing.T) {
		w := putUpdate(t, handler, contentID, userID, "update-key-1", `{"caption":"updated with key"}`)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "updated with key", readCaption())

		// Same key replayed with a DIFFERENT payload — must NOT 409, unlike create.
		w = putUpdate(t, handler, contentID, userID, "update-key-1", `{"caption":"replayed same key"}`)
		require.Equal(t, http.StatusOK, w.Code,
			"reusing an Idempotency-Key on PUT must not raise an idempotency conflict")
		require.Equal(t, "replayed same key", readCaption())
	})

	// POST /contents keeps its mandatory key (protected create contract).
	t.Run("POST create without Idempotency-Key is rejected", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contents",
			strings.NewReader(`{"caption":"create without key"}`))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Set("userID", userID)

		handler.CreateContent(c)

		require.Equal(t, http.StatusBadRequest, w.Code,
			"POST /contents must still require Idempotency-Key")
		require.Contains(t, w.Body.String(), "Idempotency-Key header required")
	})
}
