package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/user/delivery/http/dto"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
)

func TestProjectPublicProfileIdentity(t *testing.T) {
	avatar := "https://cdn.example.com/avatar.jpg"

	tests := []struct {
		name          string
		info          *userEntity.UserPublicInfo
		lifecycle     string
		wantVisible   bool
		wantUsername  string
		wantAvatarURL *string
		wantLifecycle string
		wantSynthetic bool
	}{
		{
			name: "active canonical username and avatar",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "alice",
				AvatarURL: &avatar,
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "alice",
			wantAvatarURL: &avatar,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateActive,
			),
		},
		{
			name: "active canonical username with null avatar",
			info: &userEntity.UserPublicInfo{
				UserID:   uuid.New(),
				Username: "alice",
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "alice",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateActive,
			),
		},
		{
			name: "active blank username becomes unavailable",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "",
				AvatarURL: &avatar,
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateUnavailable,
			),
			wantSynthetic: false,
		},
		{
			name: "active whitespace username becomes unavailable",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "   ",
				AvatarURL: &avatar,
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateUnavailable,
			),
		},
		{
			name: "missing profile row follows unavailable contract",
			info: &userEntity.UserPublicInfo{
				UserID:   uuid.New(),
				Username: "",
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateUnavailable,
			),
		},
		{
			name: "suspended user redacts identity",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "alice",
				AvatarURL: &avatar,
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateUnavailable),
			wantVisible:   true,
			wantUsername:  "",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateUnavailable,
			),
		},
		{
			name: "banned user redacts identity",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "alice",
				AvatarURL: &avatar,
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateUnavailable),
			wantVisible:   true,
			wantUsername:  "",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateUnavailable,
			),
		},
		{
			name: "removed user does not fabricate a card",
			info: &userEntity.UserPublicInfo{
				UserID:    uuid.New(),
				Username:  "alice",
				AvatarURL: &avatar,
			},
			lifecycle:   string(viewercontext.PublicLifecycleStateRemoved),
			wantVisible: false,
		},
		{
			name: "genuine stored user_deadbeef is preserved verbatim",
			info: &userEntity.UserPublicInfo{
				UserID:   uuid.New(),
				Username: "user_deadbeef",
			},
			lifecycle:     string(viewercontext.PublicLifecycleStateActive),
			wantVisible:   true,
			wantUsername:  "user_deadbeef",
			wantAvatarURL: nil,
			wantLifecycle: string(
				viewercontext.PublicLifecycleStateActive,
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			card, ok := projectPublicProfileIdentity(tc.info, tc.lifecycle)
			if ok != tc.wantVisible {
				t.Fatalf("visible=%v, want %v", ok, tc.wantVisible)
			}
			if !ok {
				return
			}
			if card.Username != tc.wantUsername {
				t.Fatalf("username=%q, want %q", card.Username, tc.wantUsername)
			}
			if tc.wantAvatarURL == nil {
				if card.AvatarURL != nil {
					t.Fatalf("avatar_url=%v, want nil", card.AvatarURL)
				}
			} else if card.AvatarURL == nil || *card.AvatarURL != *tc.wantAvatarURL {
				t.Fatalf("avatar_url=%v, want %v", card.AvatarURL, tc.wantAvatarURL)
			}
			if card.Lifecycle == nil {
				t.Fatal("lifecycle must be populated")
			}
			if got := *card.Lifecycle; got != tc.wantLifecycle {
				t.Fatalf("lifecycle=%q, want %q", got, tc.wantLifecycle)
			}
			if tc.wantSynthetic && strings.HasPrefix(card.Username, "user_") {
				t.Fatalf("synthetic username leaked: %q", card.Username)
			}
		})
	}
}

func TestProjectPublicProfileIdentity_FailsClosedOnNilInput(t *testing.T) {
	card, ok := projectPublicProfileIdentity(nil, string(viewercontext.PublicLifecycleStateActive))
	if ok {
		t.Fatalf("nil publicInfo must fail closed, got card=%+v", card)
	}

	card, ok = projectPublicProfileIdentity(
		&userEntity.UserPublicInfo{UserID: uuid.Nil},
		string(viewercontext.PublicLifecycleStateActive),
	)
	if ok {
		t.Fatalf("nil user id must fail closed, got card=%+v", card)
	}
}

func TestGetPublicProfile_ReturnsNotFoundForMissingUser(t *testing.T) {
	svc := NewUserProfileService(
		&fakeUserRepo{},
		&noopSellerRepo{},
		&noopSubRepo{},
		nil,
		&fakeFirebase{},
		&fakeDB{},
	)

	_, err := svc.GetPublicProfile(
		context.Background(),
		uuid.New(),
		false,
	)
	if err == nil {
		t.Fatalf("expected an error for missing user, got nil")
	}
}

func TestPublicProfileResponseWireShape_CoherentAndSyntheticFree(t *testing.T) {
	t.Run("active canonical identity serializes coherently", func(t *testing.T) {
		info := &userEntity.UserPublicInfo{
			UserID:    uuid.New(),
			Username:  "user_deadbeef",
			AvatarURL: strPtr("https://cdn.example.com/avatar.jpg"),
		}
		card, ok := projectPublicProfileIdentity(
			info,
			string(viewercontext.PublicLifecycleStateActive),
		)
		if !ok {
			t.Fatal("expected active canonical card")
		}

		resp := dto.PublicUserResponse{
			UserID:    info.UserID,
			Username:  card.Username,
			AvatarURL: card.AvatarURL,
			CreatedAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
			Identity:  &card,
		}

		payload := mustMarshalJSONMap(t, resp)
		identity := payload["identity"].(map[string]any)

		if got := payload["username"].(string); got != "user_deadbeef" {
			t.Fatalf("top-level username=%q, want %q", got, "user_deadbeef")
		}
		if got := identity["username"].(string); got != "user_deadbeef" {
			t.Fatalf("identity username=%q, want %q", got, "user_deadbeef")
		}
		if got := identity["lifecycle"].(string); got != string(viewercontext.PublicLifecycleStateActive) {
			t.Fatalf("identity lifecycle=%q, want active", got)
		}
		if got := payload["avatar_url"].(string); got != "https://cdn.example.com/avatar.jpg" {
			t.Fatalf("top-level avatar_url=%q, want canonical avatar", got)
		}
		if got := identity["avatar_url"].(string); got != "https://cdn.example.com/avatar.jpg" {
			t.Fatalf("identity avatar_url=%q, want canonical avatar", got)
		}
	})

	t.Run("blank username serializes as unavailable without synthetic identity", func(t *testing.T) {
		info := &userEntity.UserPublicInfo{
			UserID:   uuid.New(),
			Username: "",
		}
		card, ok := projectPublicProfileIdentity(
			info,
			string(viewercontext.PublicLifecycleStateActive),
		)
		if !ok {
			t.Fatal("expected unavailable public card")
		}

		resp := dto.PublicUserResponse{
			UserID:    info.UserID,
			Username:  card.Username,
			CreatedAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
			Identity:  &card,
		}

		payload := mustMarshalJSONMap(t, resp)
		identity := payload["identity"].(map[string]any)

		if got := payload["username"].(string); got != "" {
			t.Fatalf("top-level username=%q, want empty", got)
		}
		if got := identity["username"].(string); got != "" {
			t.Fatalf("identity username=%q, want empty", got)
		}
		if got := identity["lifecycle"].(string); got != string(viewercontext.PublicLifecycleStateUnavailable) {
			t.Fatalf("identity lifecycle=%q, want unavailable", got)
		}
		if _, ok := payload["avatar_url"]; ok {
			t.Fatalf("blank username response must not emit avatar_url")
		}
		if strings.Contains(string(mustMarshalJSON(t, payload)), "user_") {
			t.Fatalf("blank username response must not invent a synthetic identity")
		}
	})
}

func mustMarshalJSONMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw := mustMarshalJSON(t, v)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return out
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return raw
}
