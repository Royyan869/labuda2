package entity

import "testing"

func ptr(s string) *string { return &s }

func TestSocialMediaToMap_PresenceOnly(t *testing.T) {
	sm := &SocialMedia{
		InstagramHandle: ptr("labuda_farm"),
		FacebookHandle:  ptr(""),
		TiktokHandle:    nil,
		TwitterHandle:   ptr("labuda"),
	}

	m := sm.ToMap()

	if got, ok := m["instagram_handle"]; !ok || got != "labuda_farm" {
		t.Fatalf("instagram_handle = %v; want labuda_farm", got)
	}
	if got, ok := m["twitter_handle"]; !ok || got != "labuda" {
		t.Fatalf("twitter_handle = %v; want labuda", got)
	}
	if _, ok := m["facebook_handle"]; ok {
		t.Fatalf("empty facebook_handle must be omitted (presence = visible)")
	}
	if _, ok := m["tiktok_handle"]; ok {
		t.Fatalf("nil tiktok_handle must be omitted")
	}
}

func TestSocialMediaToMap_Cleared(t *testing.T) {
	sm := &SocialMedia{
		InstagramHandle: nil,
		FacebookHandle:  ptr(""),
		TiktokHandle:    ptr(""),
		TwitterHandle:   nil,
	}

	m := sm.ToMap()
	if len(m) != 0 {
		t.Fatalf("cleared social media must map to an empty object; got %v", m)
	}
	if sm.HasAny() {
		t.Fatalf("HasAny must be false when every handle is empty")
	}
}

func TestSocialMediaToMap_Nil(t *testing.T) {
	var sm *SocialMedia
	if sm.ToMap() != nil {
		t.Fatalf("nil SocialMedia must map to nil")
	}
	if sm.HasAny() {
		t.Fatalf("nil SocialMedia must have no handles")
	}
}

func TestSocialMediaHasAny(t *testing.T) {
	sm := &SocialMedia{InstagramHandle: ptr("x")}
	if !sm.HasAny() {
		t.Fatalf("HasAny must be true when one handle is present")
	}
}
