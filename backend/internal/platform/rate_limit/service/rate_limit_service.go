// Package service provides rate limiting functionality.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

var (
	// ErrRateLimitExceeded is returned when the rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	// ErrInvalidWindowDuration is returned when the window duration is invalid.
	ErrInvalidWindowDuration = errors.New("window duration must be positive")
	// ErrInvalidMaxRequests is returned when max requests is invalid.
	ErrInvalidMaxRequests = errors.New("max requests must be positive")
)

// ActionType defines the type of action being rate limited.
type ActionType string

const (
	// Order actions
	ActionOrderCreate      ActionType = "order.create"
	ActionOrderCancel      ActionType = "order.cancel"
	ActionOrderComplete    ActionType = "order.complete"
	ActionOrderShip        ActionType = "order.ship"
	ActionOrderRefund      ActionType = "order.refund"
	ActionOrderDispute     ActionType = "order.dispute"

	// Payment actions
	ActionPaymentCreate    ActionType = "payment.create"
	ActionPaymentVerify    ActionType = "payment.verify"

	// General actions
	ActionLoginAttempt     ActionType = "auth.login_attempt"
	ActionPasswordReset    ActionType = "auth.password_reset"
	ActionContactSupport   ActionType = "support.contact"
)

// Result represents the result of a rate limit check.
type Result struct {
	Allowed     bool          `json:"allowed"`
	Reason      string        `json:"reason,omitempty"`
	Current     int           `json:"current_count,omitempty"`
	Max         int           `json:"max_requests,omitempty"`
	Remaining   int           `json:"remaining,omitempty"`
	RetryAfter  int           `json:"retry_after,omitempty"` // Seconds until retry
	BlockedUntil *time.Time   `json:"blocked_until,omitempty"`
}

// Config holds rate limit configuration for different action types.
type Config struct {
	WindowMinutes int
	MaxRequests   int
	BlockDuration time.Duration // How long to block after limit exceeded
}

// Default configurations for different action types
var DefaultConfigs = map[ActionType]Config{
	// Order actions - more restrictive for financial operations
	ActionOrderCreate:   {WindowMinutes: 60, MaxRequests: 10, BlockDuration: 1 * time.Hour},
	ActionOrderCancel:   {WindowMinutes: 60, MaxRequests: 20, BlockDuration: 30 * time.Minute},
	ActionOrderComplete: {WindowMinutes: 60, MaxRequests: 50, BlockDuration: 15 * time.Minute},
	ActionOrderRefund:   {WindowMinutes: 60, MaxRequests: 5, BlockDuration: 2 * time.Hour},
	ActionOrderDispute:  {WindowMinutes: 1440, MaxRequests: 3, BlockDuration: 24 * time.Hour}, // 3 per day

	// Payment actions - very restrictive
	ActionPaymentCreate: {WindowMinutes: 60, MaxRequests: 5, BlockDuration: 2 * time.Hour},
	ActionPaymentVerify: {WindowMinutes: 5, MaxRequests: 10, BlockDuration: 30 * time.Minute},

	// Auth actions - moderate restrictions
	ActionLoginAttempt:  {WindowMinutes: 15, MaxRequests: 10, BlockDuration: 30 * time.Minute},
	ActionPasswordReset: {WindowMinutes: 60, MaxRequests: 3, BlockDuration: 1 * time.Hour},

	// Support actions
	ActionContactSupport: {WindowMinutes: 60, MaxRequests: 5, BlockDuration: 1 * time.Hour},
}

// Service handles rate limiting operations.
//
// DESIGN PRINCIPLES:
// - Per-user per-action limits
// - Sliding window approach
// - Automatic blocking when limit exceeded
// - Configurable limits per action type
type Service struct {
	db     *db.DB
	config map[ActionType]Config
}

// NewService creates a new RateLimitService with default configurations.
func NewService(database *db.DB) *Service {
	return &Service{
		db:     database,
		config: DefaultConfigs,
	}
}

// NewServiceWithConfig creates a new RateLimitService with custom configurations.
func NewServiceWithConfig(database *db.DB, config map[ActionType]Config) *Service {
	// Merge with defaults
	merged := make(map[ActionType]Config)
	for k, v := range DefaultConfigs {
		merged[k] = v
	}
	for k, v := range config {
		merged[k] = v
	}

	return &Service{
		db:     database,
		config: merged,
	}
}

// SetConfig sets the configuration for a specific action type.
func (s *Service) SetConfig(actionType ActionType, config Config) {
	s.config[actionType] = config
}

// Check checks if the user is allowed to perform the action.
// Returns a Result indicating if the action is allowed.
func (s *Service) Check(ctx context.Context, userID uuid.UUID, actionType ActionType) (*Result, error) {
	config, ok := s.config[actionType]
	if !ok {
		// Use default config if not found
		config = Config{WindowMinutes: 60, MaxRequests: 100, BlockDuration: 1 * time.Hour}
	}

	return s.checkWithConfig(ctx, userID, string(actionType), config)
}

// CheckCustom checks with custom window and max requests.
func (s *Service) CheckCustom(
	ctx context.Context,
	userID uuid.UUID,
	actionType ActionType,
	windowMinutes int,
	maxRequests int,
) (*Result, error) {
	if windowMinutes <= 0 {
		return nil, ErrInvalidWindowDuration
	}
	if maxRequests <= 0 {
		return nil, ErrInvalidMaxRequests
	}

	config := Config{
		WindowMinutes: windowMinutes,
		MaxRequests:   maxRequests,
		BlockDuration: 1 * time.Hour,
	}

	return s.checkWithConfig(ctx, userID, string(actionType), config)
}

// checkWithConfig performs the actual rate limit check with the given configuration.
func (s *Service) checkWithConfig(
	ctx context.Context,
	userID uuid.UUID,
	actionType string,
	config Config,
) (*Result, error) {
	// Use the database function for atomic checking
	query := `
		SELECT check_rate_limit($1, $2, $3, $4) AS result
	`

	var resultJSON []byte
	err := s.db.Pool().QueryRow(ctx, query, userID, actionType, config.WindowMinutes, config.MaxRequests).Scan(&resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	var result Result
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return nil, fmt.Errorf("failed to parse rate limit result: %w", err)
	}

	return &result, nil
}

// Record records a rate limit hit (increments counter).
// Most of the time, Check() is sufficient as it auto-increments.
// This is useful for recording hits after-the-fact.
func (s *Service) Record(ctx context.Context, userID uuid.UUID, actionType ActionType) error {
	config, ok := s.config[actionType]
	if !ok {
		config = Config{WindowMinutes: 60, MaxRequests: 100, BlockDuration: 1 * time.Hour}
	}

	// Calculate window start
	windowStart := time.Now().Truncate(time.Duration(config.WindowMinutes) * time.Minute)

	query := `
		INSERT INTO rate_limits (
			user_id, action_type, window_start, request_count,
			window_duration_minutes, max_requests
		) VALUES ($1, $2, $3, 1, $4, $5)
		ON CONFLICT (user_id, action_type, window_start)
		DO UPDATE SET request_count = rate_limits.request_count + 1, updated_at = NOW()
	`

	_, err := s.db.Pool().Exec(ctx, query, userID, string(actionType), windowStart, config.WindowMinutes, config.MaxRequests)
	if err != nil {
		return fmt.Errorf("failed to record rate limit hit: %w", err)
	}

	return nil
}

// Reset resets the rate limit counter for a user and action.
// Useful for admin intervention or testing.
func (s *Service) Reset(ctx context.Context, userID uuid.UUID, actionType ActionType) error {
	query := `
		DELETE FROM rate_limits
		WHERE user_id = $1 AND action_type = $2
	`

	_, err := s.db.Pool().Exec(ctx, query, userID, string(actionType))
	if err != nil {
		return fmt.Errorf("failed to reset rate limit: %w", err)
	}

	return nil
}

// ResetAll resets all rate limits for a user.
// Useful for admin intervention.
func (s *Service) ResetAll(ctx context.Context, userID uuid.UUID) error {
	query := `
		DELETE FROM rate_limits WHERE user_id = $1
	`

	_, err := s.db.Pool().Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to reset all rate limits: %w", err)
	}

	return nil
}

// GetStatus returns the current rate limit status for a user and action.
// Does NOT increment the counter.
func (s *Service) GetStatus(ctx context.Context, userID uuid.UUID, actionType ActionType) (*Result, error) {
	query := `
		SELECT
			COALESCE(SUM(request_count), 0) as current_count,
			MAX(max_requests) as max_requests,
			MAX(blocked_until) as blocked_until
		FROM rate_limits
		WHERE user_id = $1 AND action_type = $2
		  AND window_start >= NOW() - (window_duration_minutes || ' minutes')::interval
	`

	var currentCount int
	var maxRequests int
	var blockedUntil *time.Time

	err := s.db.Pool().QueryRow(ctx, query, userID, string(actionType)).Scan(
		&currentCount,
		&maxRequests,
		&blockedUntil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit status: %w", err)
	}

	result := &Result{
		Current:       currentCount,
		Max:           maxRequests,
		Remaining:     maxRequests - currentCount,
		Allowed:       currentCount < maxRequests && (blockedUntil == nil || blockedUntil.Before(time.Now())),
		BlockedUntil:  blockedUntil,
	}

	if !result.Allowed {
		result.Reason = "rate_limit_exceeded"
		if blockedUntil != nil && blockedUntil.After(time.Now()) {
			result.RetryAfter = int(time.Until(*blockedUntil).Seconds())
		}
	}

	return result, nil
}

// Cleanup removes old rate limit records.
// Records older than the specified age are deleted.
func (s *Service) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	query := `
		DELETE FROM rate_limits
		WHERE window_start < NOW() - ($1::interval)
	`

	result, err := s.db.Pool().Exec(ctx, query, fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup rate limit records: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// IsAllowed is a convenience method that returns true if the action is allowed.
// Use this for simple checks where you don't need the detailed result.
func (s *Service) IsAllowed(ctx context.Context, userID uuid.UUID, actionType ActionType) bool {
	result, err := s.Check(ctx, userID, actionType)
	if err != nil {
		return false
	}
	return result.Allowed
}

// Middleware returns a middleware function that checks rate limits before processing.
// This can be used in HTTP handlers.
func (s *Service) Middleware(actionType ActionType) func(ctx context.Context, userID uuid.UUID) error {
	return func(ctx context.Context, userID uuid.UUID) error {
		result, err := s.Check(ctx, userID, actionType)
		if err != nil {
			return err
		}
		if !result.Allowed {
			return fmt.Errorf("%w: %s", ErrRateLimitExceeded, result.Reason)
		}
		return nil
	}
}


