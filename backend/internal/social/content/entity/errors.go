package entity

import "errors"

var (
	// ErrResourceNotOwned is returned when the caller does not own the target
	// commerce resource for a commerce-reference comment.
	ErrResourceNotOwned = errors.New("resource not owned")

	// ErrIdempotencyConflict is returned when an idempotency key is replayed
	// with a different actor or payload fingerprint.
	ErrIdempotencyConflict = errors.New("idempotency conflict")
)
