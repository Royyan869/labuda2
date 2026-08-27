package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/interaction/notification/entity"
)

// dbQuerier is the minimal interface satisfied by both *pgxpool.Pool and pgx.Tx,
// used as the pool-level fallback executor when no transaction is active.
type dbQuerier interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
}

// FCMTokenRepository handles FCM token persistence.
// pool is used as the fallback executor when tx is nil (e.g., async push paths).
// If pool is also nil, operations that require a query executor return a controlled error
// instead of panicking on a nil interface type-assertion.
type FCMTokenRepository struct {
	pool dbQuerier
}

// NewFCMTokenRepository creates a new FCMTokenRepository.
// pool is the connection-pool fallback used when callers pass tx=nil.
// Passing pool=nil is safe but means nil-tx calls to GetActiveTokensByUser
// will return an error (no panic) instead of executing a query.
func NewFCMTokenRepository(pool dbQuerier) *FCMTokenRepository {
	return &FCMTokenRepository{pool: pool}
}

// Insert inserts a new FCM token.
// If a token with the same user_id and device_id exists, it is deactivated first.
func (r *FCMTokenRepository) Insert(ctx context.Context, tx interface{}, token *entity.FCMToken) error {
	// If device_id is provided, deactivate existing tokens for this device
	if token.DeviceID != nil && *token.DeviceID != "" {
		deactivateQuery := `
			UPDATE fcm_tokens
			SET is_active = false, updated_at = NOW()
			WHERE user_id = $1 AND device_id = $2 AND is_active = true
		`
		_, err := tx.(interface {
			Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
		}).
			Exec(ctx, deactivateQuery, token.UserID, *token.DeviceID)
		if err != nil {
			return fmt.Errorf("deactivate existing tokens failed: %w", err)
		}
	}

	query := `
		INSERT INTO fcm_tokens (id, user_id, token, platform, device_id, device_name, app_version, is_active, last_used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			token = EXCLUDED.token,
			platform = EXCLUDED.platform,
			device_name = EXCLUDED.device_name,
			app_version = EXCLUDED.app_version,
			is_active = true,
			last_used_at = EXCLUDED.last_used_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query,
			token.ID, token.UserID, token.Token, token.Platform,
			token.DeviceID, token.DeviceName, token.AppVersion,
			token.IsActive, token.LastUsedAt, token.CreatedAt, token.UpdatedAt,
		)

	if err != nil {
		return fmt.Errorf("insert fcm token failed: %w", err)
	}

	return nil
}

// GetActiveTokensByUser retrieves all active FCM tokens for a user.
// When tx is nil (e.g., async push path), the repository falls back to the
// configured pool. If both tx and pool are nil a controlled error is returned
// instead of panicking on a nil interface type-assertion.
func (r *FCMTokenRepository) GetActiveTokensByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*entity.FCMToken, error) {
	query := `
		SELECT id, user_id, token, platform, device_id, device_name, app_version, is_active, last_used_at, created_at, updated_at
		FROM fcm_tokens
		WHERE user_id = $1 AND is_active = true
		ORDER BY last_used_at DESC
	`

	var querier interface {
		Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	}

	if tx != nil {
		q, ok := tx.(interface {
			Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
		})
		if !ok {
			return nil, fmt.Errorf("fcm token query: tx does not implement Query interface")
		}
		querier = q
	} else if r.pool != nil {
		querier = r.pool
	} else {
		return nil, fmt.Errorf("fcm token query: no transaction and no pool configured")
	}

	rows, err := querier.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query fcm tokens failed: %w", err)
	}
	defer rows.Close()

	tokens, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*entity.FCMToken, error) {
		var t entity.FCMToken
		err := row.Scan(
			&t.ID, &t.UserID, &t.Token, &t.Platform,
			&t.DeviceID, &t.DeviceName, &t.AppVersion,
			&t.IsActive, &t.LastUsedAt, &t.CreatedAt, &t.UpdatedAt,
		)
		return &t, err
	})

	if err != nil {
		return nil, fmt.Errorf("scan fcm tokens failed: %w", err)
	}

	return tokens, nil
}

// DeactivateByToken deactivates a specific token by token string.
// Used when a token is invalid or the user logs out.
// When tx is nil (e.g., async cleanup after push), the pool fallback is used.
func (r *FCMTokenRepository) DeactivateByToken(ctx context.Context, tx interface{}, tokenString string) error {
	query := `
		UPDATE fcm_tokens
		SET is_active = false, updated_at = NOW()
		WHERE token = $1
	`

	var execer interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}

	if tx != nil {
		e, ok := tx.(interface {
			Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
		})
		if !ok {
			return fmt.Errorf("fcm token deactivate: tx does not implement Exec interface")
		}
		execer = e
	} else if r.pool != nil {
		execer = r.pool
	} else {
		return fmt.Errorf("fcm token deactivate: no transaction and no pool configured")
	}

	result, err := execer.Exec(ctx, query, tokenString)
	if err != nil {
		return fmt.Errorf("deactivate fcm token failed: %w", err)
	}

	// It's OK if no rows were affected - token might not exist
	_ = result.RowsAffected()

	return nil
}

// DeactivateByUserAndDevice deactivates all tokens for a specific user and device.
func (r *FCMTokenRepository) DeactivateByUserAndDevice(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) error {
	query := `
		UPDATE fcm_tokens
		SET is_active = false, updated_at = NOW()
		WHERE user_id = $1 AND device_id = $2
	`

	_, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, userID, deviceID)
	if err != nil {
		return fmt.Errorf("deactivate fcm tokens by user and device failed: %w", err)
	}

	return nil
}

// DeactivateByUserAndDeviceCount deactivates all tokens for a specific user and
// device and returns the number of rows updated.
func (r *FCMTokenRepository) DeactivateByUserAndDeviceCount(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) (int64, error) {
	query := `
		UPDATE fcm_tokens
		SET is_active = false, updated_at = NOW()
		WHERE user_id = $1 AND device_id = $2
	`

	result, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, userID, deviceID)
	if err != nil {
		return 0, fmt.Errorf("deactivate fcm tokens by user and device failed: %w", err)
	}
	return result.RowsAffected(), nil
}

// DeactivateAllByUser deactivates all active tokens for a specific user and returns the number of rows updated.
func (r *FCMTokenRepository) DeactivateAllByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (int64, error) {
	query := `
		UPDATE fcm_tokens
		SET is_active = false, updated_at = NOW()
		WHERE user_id = $1 AND is_active = true
	`

	result, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("deactivate all fcm tokens for user failed: %w", err)
	}

	return result.RowsAffected(), nil
}

// UpdateLastUsedAt updates the last_used_at timestamp for a token.
func (r *FCMTokenRepository) UpdateLastUsedAt(ctx context.Context, tx interface{}, tokenID uuid.UUID) error {
	query := `
		UPDATE fcm_tokens
		SET last_used_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := tx.(interface {
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	}).
		Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("update fcm token last_used_at failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &entity.ErrFCMTokenNotFound{TokenID: tokenID}
	}

	return nil
}


