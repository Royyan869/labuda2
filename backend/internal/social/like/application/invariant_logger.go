// Package application provides like business logic services.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// InvariantType identifies the type of invariant that was violated.
type InvariantType string

const (
	// DuplicateLikeAttempt: Someone tried to like something they already liked
	DuplicateLikeAttempt InvariantType = "duplicate_like_attempt"
	// LikeOnDeletedContent: Someone tried to like deleted content
	LikeOnDeletedContent InvariantType = "like_on_deleted_content"
)

// InvariantViolation represents a logged invariant violation for monitoring.
type InvariantViolation struct {
	Type       InvariantType
	UserID     uuid.UUID
	TargetID   uuid.UUID
	TargetType string
	Details    map[string]interface{}
	Timestamp  time.Time
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
		zap.String("target_id", violation.TargetID.String()),
		zap.String("target_type", violation.TargetType),
		zap.Time("timestamp", violation.Timestamp),
	)

	// Add details as fields
	for k, v := range violation.Details {
		fields = append(fields, zap.Any(k, v))
	}

	// Log as warn level (not error, since we're rejecting the invalid operation)
	l.logger.Warn("LIKE INVARIANT VIOLATION REJECTED", fields...)
}

// noopInvariantLogger is a no-op implementation for when no logger is provided.
type noopInvariantLogger struct{}

func (l *noopInvariantLogger) LogViolation(_ context.Context, violation InvariantViolation) {
	// No-op - safely does nothing
}

// LogInvariantViolation is a convenience function that safely logs an invariant violation.
// If the logger is nil, it does nothing.
func LogInvariantViolation(ctx context.Context, logger InvariantLogger, invType InvariantType, userID, targetID uuid.UUID, targetType string, details map[string]interface{}) {
	if logger == nil {
		return
	}
	violation := InvariantViolation{
		Type:       invType,
		UserID:     userID,
		TargetID:   targetID,
		TargetType: targetType,
		Details:    details,
		Timestamp:  time.Now(),
	}
	logger.LogViolation(ctx, violation)
}

// LikeOnDeletedContentViolation logs an attempt to like deleted content.
func LikeOnDeletedContentViolation(ctx context.Context, logger InvariantLogger, userID, contentID uuid.UUID) {
	LogInvariantViolation(ctx, logger, LikeOnDeletedContent, userID, contentID, "content", map[string]interface{}{
		"attempt": "like_deleted_content",
	})
}


