package application

import (
	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
)

// MessagePageResult carries a page of chat messages plus their associated
// resource occurrences keyed by message_id.
//
// This is the INTERNAL read carrier for the ListMessages service method.
// It is NOT a public HTTP contract — messageToResponse remains unchanged.
//
// ResourceOccurrencesByMessageID is populated by a single batch query
// (GetResourceOccurrencesByMessageIDs) after ListMessagesByRoom succeeds.
// An empty map means no messages in the page have occurrences.
type MessagePageResult struct {
	Messages                       []*chatEntity.ChatMessage
	ResourceOccurrencesByMessageID map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence
}
