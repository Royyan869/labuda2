package entity

import (
	"time"

	"github.com/google/uuid"
)

// Admin represents a support admin with their current workload.
type Admin struct {
	ID                uuid.UUID
	IsActive          bool
	ActiveTicketCount int
	LastAssignedAt    *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewAdmin creates a new support admin record.
func NewAdmin(adminID uuid.UUID) *Admin {
	now := time.Now()

	return &Admin{
		ID:                adminID,
		IsActive:          true,
		ActiveTicketCount: 0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// CanTakeMoreTickets returns true if the admin can be assigned more tickets.
func (a *Admin) CanTakeMoreTickets(maxConcurrent int) bool {
	if !a.IsActive {
		return false
	}
	return a.ActiveTicketCount < maxConcurrent
}

// IncrementActiveCount increments the active ticket count.
func (a *Admin) IncrementActiveCount() {
	a.ActiveTicketCount++
	a.UpdatedAt = time.Now()
	now := time.Now()
	a.LastAssignedAt = &now
}

// DecrementActiveCount decrements the active ticket count.
func (a *Admin) DecrementActiveCount() {
	if a.ActiveTicketCount > 0 {
		a.ActiveTicketCount--
	}
	a.UpdatedAt = time.Now()
}

// SetActive sets the admin's active status.
func (a *Admin) SetActive(active bool) {
	a.IsActive = active
	a.UpdatedAt = time.Now()
}


