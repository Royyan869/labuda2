//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	notificationpkg "github.com/labuda/backend/internal/interaction/notification"
	notificationhttp "github.com/labuda/backend/internal/interaction/notification/delivery/http"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	notificationrepoimpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// unreadCountFixture is one migrated test database plus an actor and a
// recipient, wired to the real notification handler and the real repository —
// the same authority stack production uses.
type unreadCountFixture struct {
	appDB     *db.DB
	repo      notificationpkg.Repository
	handler   *notificationhttp.NotificationHandler
	router    *gin.Engine
	actorID   uuid.UUID
	recipient uuid.UUID
}

func setupUnreadCountFixture(t *testing.T) *unreadCountFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	appDB := db.NewFromPool(tdb.Pool())
	handler := notificationhttp.NewNotificationHandlerWithDefaults(appDB, zap.NewNop())

	ctx := context.Background()
	actorID, recipientID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{actorID, recipientID} {
		_, err := appDB.Pool().Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			 VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())`,
			id, "fb-"+id.String(), id.String()+"@test.invalid",
		)
		require.NoError(t, err, "seed user %s", id)
	}

	return &unreadCountFixture{
		appDB:     appDB,
		repo:      notificationrepoimpl.NewNotificationRepository(),
		handler:   handler,
		router:    newUnreadCountHTTPRouter(recipientID, handler),
		actorID:   actorID,
		recipient: recipientID,
	}
}

// newUnreadCountHTTPRouter mounts exactly the routes under test, with the
// auth middleware replaced by a fixed userID — same pattern as the existing
// notification integration suites.
func newUnreadCountHTTPRouter(userID uuid.UUID, handler *notificationhttp.NotificationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	notifications := router.Group("/api/v1/notifications")
	notifications.POST("/:id/read", handler.MarkNotificationAsRead)
	notifications.POST("/read-all", handler.MarkAllAsRead)
	notifications.DELETE("/:id", handler.DeleteNotification)
	notifications.GET("/unread-count", handler.GetUnreadCount)
	return router
}

// seedNotification writes one unread notification through the canonical
// repository Insert (the only INSERT path in production).
func seedNotification(t *testing.T, f *unreadCountFixture, notifyType string) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.appDB.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		id, _, err = f.repo.Insert(context.Background(), tx,
			notificationentity.NewNotification(
				f.recipient,
				notificationentity.UserActor(f.actorID),
				notificationentity.NotificationType(notifyType),
				uuid.New(),
				map[string]interface{}{},
			),
		)
		return err
	})
	require.NoError(t, err, "seed %s", notifyType)
	require.NotEqual(t, uuid.Nil, id)
	return id
}

// performUnreadCountRequest executes the request and unwraps the standard
// response envelope, returning the `data` payload.
func performUnreadCountRequest(t *testing.T, router *gin.Engine, method, path string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	require.NotNil(t, resp.Data, "response must carry a data payload: %s", w.Body.String())
	return resp.Data
}

// requireUnreadCountField asserts the mutation response carries the canonical
// post-mutation unread_count.
func requireUnreadCountField(t *testing.T, data map[string]any, want int) {
	t.Helper()

	raw, ok := data["unread_count"]
	require.True(t, ok, "mutation response must carry unread_count, got payload: %v", data)
	count, ok := raw.(float64)
	require.True(t, ok, "unread_count must be numeric, got %T", raw)
	require.Equal(t, want, int(count), "response unread_count")
}

// requireCanonicalCount asserts the dedicated unread-count endpoint — the
// canonical authority — agrees with the expected value.
func requireCanonicalCount(t *testing.T, router *gin.Engine, want int) {
	t.Helper()

	data := performUnreadCountRequest(t, router, http.MethodGet, "/api/v1/notifications/unread-count")
	raw, ok := data["count"].(float64)
	require.True(t, ok, "unread-count endpoint must return numeric count, got payload: %v", data)
	require.Equal(t, want, int(raw), "canonical /unread-count")
}

// TestNotificationMarkRead_ReturnsPostMutationUnreadCount proves marking one
// unread notification read returns the canonical count AFTER the mutation
// (N-1), and that the dedicated unread-count endpoint agrees.
func TestNotificationMarkRead_ReturnsPostMutationUnreadCount(t *testing.T) {
	f := setupUnreadCountFixture(t)

	ids := []uuid.UUID{
		seedNotification(t, f, "user.followed"),
		seedNotification(t, f, "content.liked"),
		seedNotification(t, f, "order.created"),
	}
	requireCanonicalCount(t, f.router, 3)

	data := performUnreadCountRequest(t, f.router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", ids[0]))

	require.Equal(t, true, data["success"])
	requireUnreadCountField(t, data, 2)
	requireCanonicalCount(t, f.router, 2)
}

// TestNotificationMarkRead_AlreadyRead_IsIdempotentCountUnchanged proves
// re-marking a read notification is a successful no-op: the count does not
// change and the response still carries the canonical count.
func TestNotificationMarkRead_AlreadyRead_IsIdempotentCountUnchanged(t *testing.T) {
	f := setupUnreadCountFixture(t)

	alreadyRead := seedNotification(t, f, "user.followed")
	unread := seedNotification(t, f, "content.liked")

	first := performUnreadCountRequest(t, f.router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", alreadyRead))
	requireUnreadCountField(t, first, 1)

	// Mark the same (already-read) notification again.
	second := performUnreadCountRequest(t, f.router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", alreadyRead))
	require.Equal(t, true, second["success"])
	requireUnreadCountField(t, second, 1)
	requireCanonicalCount(t, f.router, 1)

	// The untouched unread notification is still unread.
	third := performUnreadCountRequest(t, f.router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", unread))
	requireUnreadCountField(t, third, 0)
	requireCanonicalCount(t, f.router, 0)
}

// TestNotificationMarkAllRead_ReturnsZeroUnreadCount proves mark-all returns
// the canonical post-mutation count of zero.
func TestNotificationMarkAllRead_ReturnsZeroUnreadCount(t *testing.T) {
	f := setupUnreadCountFixture(t)

	seedNotification(t, f, "user.followed")
	seedNotification(t, f, "content.liked")
	seedNotification(t, f, "order.created")
	seedNotification(t, f, "comment")
	requireCanonicalCount(t, f.router, 4)

	data := performUnreadCountRequest(t, f.router, http.MethodPost, "/api/v1/notifications/read-all")

	require.Equal(t, true, data["success"])
	requireUnreadCountField(t, data, 0)
	requireCanonicalCount(t, f.router, 0)
}

// TestNotificationDeleteUnread_ReturnsPostMutationUnreadCount proves deleting
// an unread notification returns the canonical count AFTER the deletion (N-1).
func TestNotificationDeleteUnread_ReturnsPostMutationUnreadCount(t *testing.T) {
	f := setupUnreadCountFixture(t)

	deleted := seedNotification(t, f, "user.followed")
	seedNotification(t, f, "content.liked")
	seedNotification(t, f, "order.created")
	requireCanonicalCount(t, f.router, 3)

	data := performUnreadCountRequest(t, f.router, http.MethodDelete,
		fmt.Sprintf("/api/v1/notifications/%s", deleted))

	require.Equal(t, true, data["success"])
	requireUnreadCountField(t, data, 2)
	requireCanonicalCount(t, f.router, 2)
}

// TestNotificationDeleteRead_UnreadCountUnchanged proves deleting an
// already-read notification does not change the unread count.
func TestNotificationDeleteRead_UnreadCountUnchanged(t *testing.T) {
	f := setupUnreadCountFixture(t)

	readNotification := seedNotification(t, f, "user.followed")
	seedNotification(t, f, "content.liked")

	// Mark the first one read, leaving exactly one unread.
	performUnreadCountRequest(t, f.router, http.MethodPost,
		fmt.Sprintf("/api/v1/notifications/%s/read", readNotification))
	requireCanonicalCount(t, f.router, 1)

	// Delete the already-read notification: unread count must not move.
	data := performUnreadCountRequest(t, f.router, http.MethodDelete,
		fmt.Sprintf("/api/v1/notifications/%s", readNotification))

	require.Equal(t, true, data["success"])
	requireUnreadCountField(t, data, 1)
	requireCanonicalCount(t, f.router, 1)
}
