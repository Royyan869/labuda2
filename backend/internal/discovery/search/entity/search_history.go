package entity

import (
	"time"

	"github.com/google/uuid"
)

// SearchHistory represents a user's search query history.
// Stores only the last 20 searches per user.
type SearchHistory struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Query     string
	CreatedAt time.Time
}

// MaxSearchHistory is the maximum number of searches stored per user.
const MaxSearchHistory = 20

// NewSearchHistory creates a new search history entry.
func NewSearchHistory(userID uuid.UUID, query string) *SearchHistory {
	return &SearchHistory{
		ID:        uuid.New(),
		UserID:    userID,
		Query:     query,
		CreatedAt: time.Now(),
	}
}


