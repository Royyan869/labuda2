package repository

import (
	"errors"

	"github.com/labuda/backend/internal/incentive/coins/entity"
	"github.com/labuda/backend/pkg/db"
)

// IsReservationDuplicate reports whether err represents a duplicate reservation
// attempt for the same payment.
func IsReservationDuplicate(err error) bool {
	if err == nil {
		return false
	}

	var conflict *entity.ErrReservationConflict
	if errors.As(err, &conflict) {
		return true
	}

	return db.IsUniqueViolation(err)
}
