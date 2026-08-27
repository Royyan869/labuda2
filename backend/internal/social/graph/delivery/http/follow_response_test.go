package http

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// C2E2A — FollowUserCardResponse DTO tests
// =============================================================================

func TestFollowUserCardResponse_JSONKeys(t *testing.T) {
	card := FollowUserCardResponse{
		ID:             uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Username:       "alice",
		AvatarURL:      ptr("https://cdn.example.com/a.jpg"),
		Lifecycle:      "active",
		FollowersCount: 42,
		FollowingCount: 7,
	}

	data, err := json.Marshal(card)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	t.Run("exact keys", func(t *testing.T) {
		expectedKeys := []string{"id", "username", "avatar_url", "lifecycle", "followers_count", "following_count"}
		for _, k := range expectedKeys {
			_, ok := raw[k]
			assert.True(t, ok, "missing key %q", k)
		}
		assert.Len(t, raw, len(expectedKeys), "no extra keys")
	})

	t.Run("values", func(t *testing.T) {
		assert.Equal(t, "11111111-1111-1111-1111-111111111111", raw["id"])
		assert.Equal(t, "alice", raw["username"])
		assert.Equal(t, "https://cdn.example.com/a.jpg", raw["avatar_url"])
		assert.Equal(t, "active", raw["lifecycle"])
		assert.Equal(t, float64(42), raw["followers_count"])
		assert.Equal(t, float64(7), raw["following_count"])
	})

	t.Run("no user_ in keys or values", func(t *testing.T) {
		compact, _ := json.Marshal(card)
		assert.NotContains(t, string(compact), "user_")
		assert.NotContains(t, string(compact), "Pascal")
	})
}

func TestFollowUserCardResponse_ZeroValueJSON(t *testing.T) {
	card := FollowUserCardResponse{
		ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Lifecycle: "active",
	}
	data, err := json.Marshal(card)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"username":""`)
	assert.Contains(t, string(data), `"avatar_url":null`)
	assert.Contains(t, string(data), `"lifecycle":"active"`)
	assert.Contains(t, string(data), `"followers_count":0`)
	assert.Contains(t, string(data), `"following_count":0`)
	assert.NotContains(t, string(data), "user_")
}

// =============================================================================
// C2E2A — sanitizeFollowCard tests
// =============================================================================

func TestSanitizeFollowCard_ActiveWithProfile(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	card := sanitizeFollowCard(
		id, "alice", ptr("https://cdn/a.jpg"),
		string(viewercontext.PublicLifecycleStateActive), 42, 7,
	)

	assert.Equal(t, id, card.ID)
	assert.Equal(t, "alice", card.Username)
	assert.Equal(t, "https://cdn/a.jpg", *card.AvatarURL)
	assert.Equal(t, "active", card.Lifecycle)
	assert.Equal(t, 42, card.FollowersCount)
	assert.Equal(t, 7, card.FollowingCount)
	assert.NotContains(t, card.Username, "user_")
}

func TestSanitizeFollowCard_ActiveWithoutProfile(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	card := sanitizeFollowCard(
		id, "", nil,
		string(viewercontext.PublicLifecycleStateActive), 0, 0,
	)

	assert.Equal(t, id, card.ID)
	assert.Empty(t, card.Username, "empty username for active without profile")
	assert.Nil(t, card.AvatarURL)
	assert.Equal(t, "active", card.Lifecycle)
	assert.NotContains(t, card.Username, "user_")
}

func TestSanitizeFollowCard_Suspended(t *testing.T) {
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	// Deliberately store non-empty username and avatar to prove redaction.
	card := sanitizeFollowCard(
		id, "banned_user", ptr("https://cdn/b.jpg"),
		string(viewercontext.PublicLifecycleStateUnavailable), 100, 50,
	)

	assert.Equal(t, id, card.ID)
	assert.Empty(t, card.Username, "suspended username must be redacted")
	assert.Nil(t, card.AvatarURL, "suspended avatar must be redacted")
	assert.Equal(t, "unavailable", card.Lifecycle)
	assert.Equal(t, 100, card.FollowersCount, "counts preserved")
	assert.Equal(t, 50, card.FollowingCount, "counts preserved")
}

func TestSanitizeFollowCard_Banned(t *testing.T) {
	// Banned has same public result as suspended.
	id := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	card := sanitizeFollowCard(
		id, "troll", ptr("https://cdn/c.jpg"),
		string(viewercontext.PublicLifecycleStateUnavailable), 5, 3,
	)

	assert.Empty(t, card.Username, "banned username must be redacted")
	assert.Nil(t, card.AvatarURL, "banned avatar must be redacted")
	assert.Equal(t, "unavailable", card.Lifecycle)
}

func TestSanitizeFollowCard_SoftDeleted(t *testing.T) {
	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	// Deliberately store non-empty username and avatar to prove redaction.
	card := sanitizeFollowCard(
		id, "deleted_user", ptr("https://cdn/d.jpg"),
		string(viewercontext.PublicLifecycleStateRemoved), 10, 2,
	)

	assert.Equal(t, id, card.ID)
	assert.Empty(t, card.Username, "removed username must be redacted")
	assert.Nil(t, card.AvatarURL, "removed avatar must be redacted")
	assert.Equal(t, "removed", card.Lifecycle)
}

func TestSanitizeFollowCard_NeverSynthesizesIdentity(t *testing.T) {
	// Every lifecycle variant must keep ID stable and never create user_<UUID>.
	ids := []uuid.UUID{
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
	}

	lifecycles := []string{"active", "unavailable", "removed"}
	for _, lc := range lifecycles {
		for _, id := range ids {
			card := sanitizeFollowCard(id, "realname", ptr("https://cdn/x.jpg"), lc, 0, 0)

			assert.Equal(t, id, card.ID, "ID must be stable for %s/%s", lc, id)
			assert.NotContains(t, card.Username, "user_",
				"must never synthesize user_ for %s/%s", lc, id)
			assert.NotContains(t, card.Username, "UUID",
				"must never contain UUID for %s/%s", lc, id)
		}
	}
}

func TestSanitizeFollowCard_NullLifecycle(t *testing.T) {
	// Even a zero-value lifecycle string (which should never happen in
	// production via CoarsenLifecycle) must not leak identity.
	id := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	card := sanitizeFollowCard(
		id, "leaky", ptr("https://cdn/leak.jpg"), "", 0, 0,
	)

	// Falls into the "default" case → redacted.
	assert.Empty(t, card.Username)
	assert.Nil(t, card.AvatarURL)
}

// =============================================================================
// Negative contracts — JSON does NOT contain forbidden fields
// =============================================================================

func TestFollowUserCardResponse_NoForbiddenFields(t *testing.T) {
	card := sanitizeFollowCard(
		uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		"normal_user", ptr("https://cdn/n.jpg"),
		string(viewercontext.PublicLifecycleStateActive), 0, 0,
	)

	data, err := json.Marshal(card)
	require.NoError(t, err)
	s := string(data)

	forbidden := []string{
		"user_",
		"email",
		"phone",
		"full_name",
		"legal_name",
		"store_name",
		"farm_name",
		"firebase_uid",
		"account_status",
		"deleted_at",
	}
	for _, f := range forbidden {
		assert.NotContains(t, s, f, "must not contain %q", f)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func ptr(s string) *string { return &s }
