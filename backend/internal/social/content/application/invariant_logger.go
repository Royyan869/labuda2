// Package application provides content business logic services.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// InvariantType identifies the type of invariant that was violated.
type InvariantType string

const (
	// CommerceReferenceOnPost: Someone tried to add a commerce reference to a post (only requests)
	CommerceReferenceOnPost InvariantType = "commerce_reference_on_post"
	// ForSaleOwnershipMismatch: Someone tried to reference a forSale they don't own
	ForSaleOwnershipMismatch InvariantType = "for_sale_ownership_mismatch"
	// DuplicateLikeAttempt: Someone tried to like something they already liked
	DuplicateLikeAttempt InvariantType = "duplicate_like_attempt"
	// InvalidCommentType: An invalid comment type was used
	InvalidCommentType InvariantType = "invalid_comment_type"
	// ContentNotFoundInvariant: Operation on non-existent content
	ContentNotFoundInvariant InvariantType = "content_not_found_invariant"
)

// InvariantViolation represents a logged invariant violation for monitoring.
type InvariantViolation struct {
	Type         InvariantType
	UserID       uuid.UUID
	ResourceID   uuid.UUID
	ResourceType string // "content", "comment", "for_sale", etc.
	Details      map[string]interface{}
	Timestamp    time.Time
}

// InvariantLogger defines the interface for logging invariant violations.
// This allows for nil-safe logging (if logger is nil, no error occurs).
type InvariantLogger interface {
	LogViolation(ctx context.Context, violation InvariantViolation)
}

// zapInvariantLogger implements InvariantLogger using zap.
type zapInvariantLogger struct {
	logger *zap.Logger
}

// NewZapInvariantLogger creates a new invariant logger using zap.
// If logger is nil, returns a no-op logger that safely does nothing.
func NewZapInvariantLogger(logger *zap.Logger) InvariantLogger {
	if logger == nil {
		return &noopInvariantLogger{}
	}
	return &zapInvariantLogger{logger: logger}
}

// LogViolation logs an invariant violation with structured fields.
func (l *zapInvariantLogger) LogViolation(_ context.Context, violation InvariantViolation) {
	// Create fields from details
	fields := make([]zap.Field, 0, len(violation.Details)+5)
	fields = append(fields,
		zap.String("invariant_type", string(violation.Type)),
		zap.String("user_id", violation.UserID.String()),
		zap.String("resource_id", violation.ResourceID.String()),
		zap.String("resource_type", violation.ResourceType),
		zap.Time("timestamp", violation.Timestamp),
	)

	// Add details as fields
	for k, v := range violation.Details {
		fields = append(fields, zap.Any(k, v))
	}

	// Log as warn level (not error, since we're rejecting the invalid operation)
	l.logger.Warn("INVARIANT VIOLATION REJECTED", fields...)
}

// noopInvariantLogger is a no-op implementation for when no logger is provided.
type noopInvariantLogger struct{}

func (l *noopInvariantLogger) LogViolation(_ context.Context, violation InvariantViolation) {
	// No-op - safely does nothing
}

// LogInvariantViolation is a convenience function that safely logs an invariant violation.
// If the logger is nil, it does nothing.
func LogInvariantViolation(ctx context.Context, logger InvariantLogger, invType InvariantType, userID, resourceID uuid.UUID, resourceType string, details map[string]interface{}) {
	if logger == nil {
		return
	}
	violation := InvariantViolation{
		Type:         invType,
		UserID:       userID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Details:      details,
		Timestamp:    time.Now(),
	}
	logger.LogViolation(ctx, violation)
}

// CommerceReferenceOnPostViolation logs an attempt to add a commerce reference to a post.
func CommerceReferenceOnPostViolation(ctx context.Context, logger InvariantLogger, userID, contentID uuid.UUID, resourceID uuid.UUID) {
	LogInvariantViolation(ctx, logger, CommerceReferenceOnPost, userID, contentID, "content", map[string]interface{}{
		"for_sale_id": resourceID.String(),
		"attempt":     "add_commerce_reference",
	})
}

// ForSaleOwnershipMismatchViolation logs an attempt to reference a forSale you don't own.
func ForSaleOwnershipMismatchViolation(ctx context.Context, logger InvariantLogger, userID, contentID, forSaleID, forSaleOwnerID uuid.UUID) {
	LogInvariantViolation(ctx, logger, ForSaleOwnershipMismatch, userID, contentID, "comment", map[string]interface{}{
		"for_sale_id":       forSaleID.String(),
		"for_sale_owner_id": forSaleOwnerID.String(),
		"attempt":           "reference_forSale",
	})
}

// InvalidCommentTypeViolation logs an attempt to create comment with invalid type.
func InvalidCommentTypeViolation(ctx context.Context, logger InvariantLogger, userID, targetID uuid.UUID, requestedType string) {
	LogInvariantViolation(ctx, logger, InvalidCommentType, userID, targetID, "comment", map[string]interface{}{
		"requested_type": requestedType,
		"attempt":        "create_comment",
	})
}

// ContentNotFoundViolation logs an operation on non-existent content.
func ContentNotFoundViolation(ctx context.Context, logger InvariantLogger, userID, contentID uuid.UUID, operation string) {
	LogInvariantViolation(ctx, logger, ContentNotFoundInvariant, userID, contentID, "content", map[string]interface{}{
		"operation": operation,
	})
}

// String returns a descriptive string about the violation for error messages.
func (i InvariantType) String() string {
	switch i {
	case CommerceReferenceOnPost:
		return "commerce references only allowed on requests"
	case ForSaleOwnershipMismatch:
		return "can only reference own forSales"
	case DuplicateLikeAttempt:
		return "already liked"
	case InvalidCommentType:
		return "invalid comment type"
	case ContentNotFoundInvariant:
		return "content not found"
	default:
		return "unknown invariant violation"
	}
}

// Error returns an error with the invariant violation message.
func (i InvariantType) Error() string {
	return fmt.Sprintf("invariant violation: %s", i.String())
}

// ViolationError wraps an invariant type with additional context.
type ViolationError struct {
	Type    InvariantType
	UserID  uuid.UUID
	Context string
}

func (e *ViolationError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %s", e.Type.String(), e.Context)
	}
	return e.Type.String()
}

// NewViolationError creates a new violation error.
func NewViolationError(invType InvariantType, userID uuid.UUID, context string) *ViolationError {
	return &ViolationError{
		Type:    invType,
		UserID:  userID,
		Context: context,
	}
}
