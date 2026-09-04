//go:build integration

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestCreateContent_CanonicalResourceOccurrenceBindingAndNegativeContracts(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())
	handler := newVisibilityHTTPHandlerFromPool(appDB)

	actorID := seedVisibilityHTTPUser(t, ctx, appDB, "active")
	targetID := seedVisibilityHTTPUser(t, ctx, appDB, "active")

	gin.SetMode(gin.TestMode)

	t.Run("CW13 canonical request contains identity + operation only", func(t *testing.T) {
		body := `{
			"caption":"canonical share",
			"resource_occurrence":{
				"operation":"share_to_feed",
				"resource_type":"profile",
				"resource_id":"` + targetID.String() + `"
			}
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "canonical-resource-occurrence")
		c.Request = req
		c.Set("userID", actorID)

		handler.CreateContent(c)

		require.Equal(t, http.StatusCreated, w.Code)

		var created struct {
			Data struct {
				ID uuid.UUID `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

		var hasShareReferenceColumn bool
		require.NoError(t, tdb.Pool().QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'contents'
				  AND column_name = 'share_reference'
			)
		`).Scan(&hasShareReferenceColumn))
		require.False(t, hasShareReferenceColumn)

		var storedActorID uuid.UUID
		require.NoError(t, tdb.Pool().QueryRow(ctx, `
			SELECT actor_id
			FROM content_resource_occurrences
			WHERE content_id = $1
		`, created.Data.ID).Scan(&storedActorID))
		require.Equal(t, actorID, storedActorID)
	})

	t.Run("preview field inside resource_occurrence is rejected", func(t *testing.T) {
		body := `{
			"caption":"canonical share",
			"resource_occurrence":{
				"operation":"share_to_feed",
				"resource_type":"profile",
				"resource_id":"` + targetID.String() + `",
				"preview":{"title":"forbidden"}
			}
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "canonical-resource-occurrence-preview")
		c.Request = req
		c.Set("userID", actorID)

		handler.CreateContent(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Invalid request")
	})

	t.Run("CW14 dual authority rejected", func(t *testing.T) {
		body := `{
			"caption":"canonical share",
			"share_reference":{
				"targetType":"profile",
				"targetId":"` + targetID.String() + `",
				"preview":{"title":"legacy"}
			},
			"resource_occurrence":{
				"operation":"share_to_feed",
				"resource_type":"profile",
				"resource_id":"` + targetID.String() + `"
			}
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "canonical-resource-occurrence-dual")
		c.Request = req
		c.Set("userID", actorID)

		handler.CreateContent(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		// share_reference is legacy and rejected by bindStrictContentJSON before
		// any mutual-exclusion check. The canonical dual-authority rejection is
		// implicit: the legacy field is blocked as unsupported.
		require.Contains(t, w.Body.String(), "not supported")
	})

	t.Run("CW20 update content cannot mutate canonical occurrence", func(t *testing.T) {
		var contentID uuid.UUID
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			content, err := handler.contentService.CreateContent(
				ctx,
				tx,
				actorID,
				"update target",
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

		body := `{
			"caption":"updated",
			"resource_occurrence":{
				"operation":"share_to_feed",
				"resource_type":"profile",
				"resource_id":"` + targetID.String() + `"
			}
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "update-with-resource-occurrence")
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
		c.Set("userID", actorID)

		handler.UpdateContent(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Invalid request")
	})

	t.Run("CW20 update content rejects all canonical mutation attempts", func(t *testing.T) {
		var contentID uuid.UUID
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			content, err := handler.contentService.CreateContent(
				ctx,
				tx,
				actorID,
				"update target matrix",
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

		cases := []struct {
			name string
			body string
		}{
			{
				name: "attach",
				body: `{"caption":"updated","resource_occurrence":{"operation":"share_to_feed","resource_type":"profile","resource_id":"` + targetID.String() + `"}}`,
			},
			{
				name: "replace",
				body: `{"caption":"updated","resource_occurrence":{"operation":"share_to_feed","resource_type":"content","resource_id":"` + uuid.NewString() + `"}}`,
			},
			{
				name: "clear",
				body: `{"caption":"updated","resource_occurrence":null}`,
			},
			{
				name: "change_operation",
				body: `{"caption":"updated","resource_occurrence":{"operation":"direct_commerce_insert_content","resource_type":"profile","resource_id":"` + targetID.String() + `"}}`,
			},
			{
				name: "change_resource_id",
				body: `{"caption":"updated","resource_occurrence":{"operation":"share_to_feed","resource_type":"profile","resource_id":"` + uuid.NewString() + `"}}`,
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				req := httptest.NewRequest(http.MethodPut, "/api/v1/contents/"+contentID.String(), strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", "update-with-resource-occurrence-"+tc.name)
				c.Request = req
				c.Params = gin.Params{{Key: "id", Value: contentID.String()}}
				c.Set("userID", actorID)

				handler.UpdateContent(c)

				require.Equal(t, http.StatusBadRequest, w.Code)
				require.Contains(t, w.Body.String(), "Invalid request")
			})
		}
	})
}
