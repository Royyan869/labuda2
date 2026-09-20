//go:build integration

package application

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Real-DB proof of the canonical chat message idempotency authority:
// (sender_id, idempotency_key) UNIQUE + command_fingerprint equality.
func newChatIdempotencyIntegrationService(t *testing.T) (*Service, *db.DB, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	appDB := db.NewFromPool(tdb.Pool())

	service := NewService(
		appDB,
		chatInfraRepo.NewChatRepository(),
		socialInfraRepo.NewSocialRepository(),
		noopChatOutbox{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	return service, appDB, cleanup
}

func countMessagesByActorKey(t *testing.T, ctx context.Context, appDB *db.DB, senderID uuid.UUID, key string) int {
	t.Helper()
	var n int
	err := appDB.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM chat_messages WHERE sender_id = $1 AND idempotency_key = $2
	`, senderID, key).Scan(&n)
	require.NoError(t, err)
	return n
}

func TestChatIdempotencyAuthority_Integration(t *testing.T) {
	service, appDB, cleanup := newChatIdempotencyIntegrationService(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	senderA := insertChatAuthorityUser(t, ctx, appDB, "idem-sender-a")
	senderB := insertChatAuthorityUser(t, ctx, appDB, "idem-sender-b")
	roomAB := insertChatAuthorityRoom(t, ctx, appDB, senderA, senderB)

	const key = "shared-key"

	// ---- Case A: first submission persists exactly one message. ----
	bodyA := "command from A"
	firstA, err := service.SendMessage(ctx, roomAB, senderA, chatEntity.MessageTypeText, &bodyA, nil, key)
	require.NoError(t, err, "case A: first submission")
	require.NotNil(t, firstA)
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderA, key), "case A: one persisted row")

	// ---- Case B: same sender+key+same command replays (no second row). ----
	replayA := "command from A"
	secondA, err := service.SendMessage(ctx, roomAB, senderA, chatEntity.MessageTypeText, &replayA, nil, key)
	require.NoError(t, err, "case B: replay")
	require.Equal(t, firstA.ID, secondA.ID, "case B: replay returns the canonical message")
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderA, key), "case B: still one row")

	// ---- Case C: same sender+key, different command -> conflict, no replay. ----
	bodyADifferent := "a different command from A"
	_, err = service.SendMessage(ctx, roomAB, senderA, chatEntity.MessageTypeText, &bodyADifferent, nil, key)
	require.ErrorIs(t, err, chatRepo.ErrIdempotencyKeyConflict, "case C: fingerprint mismatch is a conflict")
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderA, key), "case C: no new row")

	// ---- Case D: different sender, same key -> independent message. ----
	bodyB := "command from B"
	firstB, err := service.SendMessage(ctx, roomAB, senderB, chatEntity.MessageTypeText, &bodyB, nil, key)
	require.NoError(t, err, "case D: different actor must be able to persist its own command")
	require.NotNil(t, firstB)
	require.NotEqual(t, firstA.ID, firstB.ID, "case D: B must not receive A's message")
	require.Equal(t, senderB, firstB.SenderID, "case D: B's message is authored by B")
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderB, key), "case D: B has its own row")
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderA, key), "case D: A's row unchanged")

	// ---- Case E: concurrent duplicate -> exactly one persisted message. ----
	senderE := insertChatAuthorityUser(t, ctx, appDB, "idem-sender-e")
	otherE := insertChatAuthorityUser(t, ctx, appDB, "idem-sender-e-other")
	roomE := insertChatAuthorityRoom(t, ctx, appDB, senderE, otherE)
	const keyE = "concurrent-key"

	const attempts = 4
	var wg sync.WaitGroup
	results := make([]*chatEntity.ChatMessage, attempts)
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := "concurrent command"
			results[idx], errs[idx] = service.SendMessage(ctx, roomE, senderE, chatEntity.MessageTypeText, &body, nil, keyE)
		}(i)
	}
	wg.Wait()

	var winnerID uuid.UUID
	for i := 0; i < attempts; i++ {
		require.NoError(t, errs[i], "case E: concurrent attempt %d", i)
		require.NotNil(t, results[i], "case E: concurrent attempt %d result", i)
		if winnerID == uuid.Nil {
			winnerID = results[i].ID
		}
		require.Equal(t, winnerID, results[i].ID, "case E: all attempts converge on one message")
	}
	require.Equal(t, 1, countMessagesByActorKey(t, ctx, appDB, senderE, keyE), "case E: exactly one persisted row")

	// Sanity: persisted fingerprint is non-empty (DB CHECK constraint).
	var fingerprint string
	require.NoError(t, appDB.Pool().QueryRow(ctx, `
		SELECT command_fingerprint FROM chat_messages WHERE sender_id = $1 AND idempotency_key = $2
	`, senderE, keyE).Scan(&fingerprint))
	require.NotEmpty(t, fingerprint)
}
