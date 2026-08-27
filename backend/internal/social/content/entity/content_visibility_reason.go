package entity

// VisibilityReason represents the reason for a visibility decision.
type VisibilityReason string

const (
	// VisibilityReasonAllowedContent visible to the viewer
	VisibilityReasonAllowed VisibilityReason = "allowed"
	// VisibilityReasonAuthor content authored by viewer
	VisibilityReasonAuthor VisibilityReason = "author"
	// VisibilityReasonPrivate content is private
	VisibilityReasonPrivate VisibilityReason = "private"
	// VisibilityReasonFollowersOnlyViewerNotFollower viewer does not follow author
	VisibilityReasonFollowersOnlyNotFollower VisibilityReason = "followers_only_not_follower"
	// VisibilityReasonBlocked viewer is blocked by author
	VisibilityReasonBlocked VisibilityReason = "blocked"
	// VisibilityReasonAuthorLifecycle author account is not active
	VisibilityReasonAuthorLifecycle VisibilityReason = "author_lifecycle"
	// VisibilityReasonContentDeleted content is deleted or hidden
	VisibilityReasonContentDeleted VisibilityReason = "content_deleted"
	// VisibilityReasonContentLifecycle repost target content is deleted or hidden
	VisibilityReasonContentLifecycle VisibilityReason = "content_lifecycle"
	// VisibilityReasonTargetAuthorLifecycle repost target author is not active
	VisibilityReasonTargetAuthorLifecycle VisibilityReason = "target_author_lifecycle"
	// VisibilityReasonTargetBlocked repost target author has blocked viewer
	VisibilityReasonTargetBlocked VisibilityReason = "target_blocked"
)
