package dto

// ShareReferenceRequest represents a share reference in the request.
// This shared DTO is used by both content and comment handlers.
type ShareReferenceRequest struct {
	TargetType string               `json:"targetType" binding:"required,oneof=content for_sale auction profile"`
	TargetID   string               `json:"targetId" binding:"required"`
	Preview    *SharePreviewRequest `json:"preview"`
}

// SharePreviewRequest represents preview data in the request.
type SharePreviewRequest struct {
	Title       string `json:"title"`
	ImageURL    string `json:"imageUrl"`
	IsAvailable *bool  `json:"isAvailable"`
	IsSold      *bool  `json:"isSold"`
	IsClosed    *bool  `json:"isClosed"`
	IsDeleted   *bool  `json:"isDeleted"`
}
