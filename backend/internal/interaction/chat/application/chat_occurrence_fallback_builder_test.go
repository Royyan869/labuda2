package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

// CONTENT FALLBACK SNAPSHOT CONTRACT
//
// The persisted `fallback_snapshot` for a Content occurrence carries DISPLAY
// identity only: caption excerpt + author identity. Content media is never
// transported here, because a persisted `content_media.media_url` is a storage
// reference that only the canonical Content read authority (mediaresolve, via
// the chat content projection resolver) may project into a readable URL. A raw
// persisted reference must never leak into an unrelated display snapshot.

// capturedQueryRowTx captures the SQL actually issued by the fallback builder and
// returns a canned row, so the snapshot contract can be proven without a database.
type capturedQueryRowTx struct {
	db.Tx // embedded nil interface: the Content fallback builder only calls QueryRow
	sql  string
	row  pgx.Row
}

func (t *capturedQueryRowTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	t.sql = sql
	return t.row
}

// contentFallbackRow fakes the single row the Content fallback builder reads:
// caption, author username, author avatar url — in SELECT order.
type contentFallbackRow struct {
	caption        *string
	authorUsername string
	authorAvatar   *string
}

func (r *contentFallbackRow) Scan(dest ...any) error {
	if len(dest) != 3 {
		return fmt.Errorf("content fallback row scan expects 3 destinations; got %d", len(dest))
	}
	captionDest, ok := dest[0].(**string)
	if !ok {
		return fmt.Errorf("content fallback scan[0] is %T; want **string", dest[0])
	}
	usernameDest, ok := dest[1].(*string)
	if !ok {
		return fmt.Errorf("content fallback scan[1] is %T; want *string", dest[1])
	}
	avatarDest, ok := dest[2].(**string)
	if !ok {
		return fmt.Errorf("content fallback scan[2] is %T; want **string", dest[2])
	}

	*captionDest = r.caption
	*usernameDest = r.authorUsername
	*avatarDest = r.authorAvatar
	return nil
}

func buildContentFallbackSnapshot(
	t *testing.T,
	caption *string,
	authorUsername string,
	authorAvatar *string,
) (map[string]json.RawMessage, string) {
	t.Helper()

	tx := &capturedQueryRowTx{
		row: &contentFallbackRow{
			caption:        caption,
			authorUsername: authorUsername,
			authorAvatar:   authorAvatar,
		},
	}

	raw, err := (&defaultContentFallbackBuilder{}).BuildContentFallback(
		context.Background(),
		tx,
		uuid.New(),
	)
	require.NoError(t, err)

	var snapshot map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	return snapshot, tx.sql
}

func TestBuildContentFallbackSnapshot_CarriesDisplayIdentityOnly(t *testing.T) {
	caption := "konten media bersama"
	avatar := "https://cdn.example.test/author.png"

	snapshot, sql := buildContentFallbackSnapshot(t, &caption, "author_user", &avatar)

	require.Len(t, snapshot, 3, "the Content fallback snapshot contract is exactly: %v", snapshot)
	require.Contains(t, snapshot, "caption_excerpt")
	require.Contains(t, snapshot, "author_username")
	require.Contains(t, snapshot, "author_avatar_url")

	// NEGATIVE: no media field may be transported, under any name.
	for key := range snapshot {
		require.NotContains(t, key, "media",
			"the Content fallback snapshot must not transport media references")
	}

	// NEGATIVE: the builder must not read `content_media` at all — the only
	// authority allowed to project a persisted content media reference is the
	// canonical Content read path (mediaresolve).
	require.NotContains(t, sql, "content_media",
		"the Content fallback builder must not read the content media table")
}

func TestBuildContentFallbackSnapshot_TruncatesCaptionExcerpt(t *testing.T) {
	longCaption := strings.Repeat("a", 205)

	snapshot, _ := buildContentFallbackSnapshot(t, &longCaption, "author_user", nil)

	var excerpt string
	require.NoError(t, json.Unmarshal(snapshot["caption_excerpt"], &excerpt))
	require.Equal(t, strings.Repeat("a", 200)+"...", excerpt)
	require.JSONEq(t, `null`, string(snapshot["author_avatar_url"]),
		"an absent author avatar stays an explicit null, never a media reference")
}

// RESIDUE GUARD: the obsolete raw `first_media_url` producer and its content
// media subquery were purged from this builder. Neither may return — a raw
// persisted reference in the snapshot would be an unresolved media read.
func TestContentFallbackBuilder_NoObsoleteMediaResidue(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	sourcePath := filepath.Join(filepath.Dir(thisFile), "chat_occurrence_fallback_builder.go")
	source, err := os.ReadFile(sourcePath)
	require.NoError(t, err)

	body := string(source)

	// Prose comments may name the removed authority (doctrine notes live next to
	// the code); the assertion targets the executable producer surfaces.
	obsoleteSurfaces := map[string]string{
		"first_media_url":      "the obsolete first_media_url snapshot field must not be reintroduced",
		"FirstMediaURL":        "the obsolete first_media_url snapshot field must not be reintroduced",
		"SELECT cm.media_url":  "the Content fallback builder must not select a persisted content media reference",
		"FROM content_media":   "the Content fallback builder must not read the content media table",
		"contentFallbackMedia": "no Content fallback media surface may be reintroduced",
	}
	for surface, reason := range obsoleteSurfaces {
		require.NotContains(t, body, surface, reason)
	}
}
