package dto

import "github.com/google/uuid"

// ResourceOccurrenceRequest is the canonical content attachment request.
// Preview/snapshot data is intentionally absent.
type ResourceOccurrenceRequest struct {
	Operation    string    `json:"operation"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	Preview      any       `json:"preview,omitempty"`
}
