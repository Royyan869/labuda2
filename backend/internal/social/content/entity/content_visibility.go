package entity

// Visibility represents the public visibility of content.
type Visibility string

const (
	// VisibilityPublic is visible to all viewers.
	VisibilityPublic Visibility = "public"
	// VisibilityFollowersOnly is visible to followers and the author.
	VisibilityFollowersOnly Visibility = "followers_only"
	// VisibilityPrivate is visible only to the author.
	VisibilityPrivate Visibility = "private"
)

// IsValid reports whether the visibility is canonical.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityPublic, VisibilityFollowersOnly, VisibilityPrivate:
		return true
	default:
		return false
	}
}

// Normalize returns a canonical visibility, defaulting to public.
func (v Visibility) Normalize() Visibility {
	if v.IsValid() {
		return v
	}
	return VisibilityPublic
}
