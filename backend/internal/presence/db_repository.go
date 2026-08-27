package presence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

type DBRepository struct {
	db *db.DB
}

func NewDBRepository(database *db.DB) *DBRepository {
	return &DBRepository{db: database}
}

func (r *DBRepository) GetLastSeen(ctx context.Context, tx db.Tx, userID uuid.UUID) (*time.Time, error) {
	row := tx.QueryRow(ctx, `
		SELECT last_seen_at
		FROM user_presence
		WHERE user_id = $1
	`, userID)

	var seen sql.NullTime
	if err := row.Scan(&seen); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get last seen: %w", err)
	}
	if !seen.Valid {
		return nil, nil
	}
	t := seen.Time.UTC()
	return &t, nil
}

func (r *DBRepository) GetLastSeenBatch(ctx context.Context, tx db.Tx, userIDs []uuid.UUID) (map[uuid.UUID]*time.Time, error) {
	result := make(map[uuid.UUID]*time.Time, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT user_id, last_seen_at
		FROM user_presence
		WHERE user_id = ANY($1)
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("get last seen batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		var seen sql.NullTime
		if err := rows.Scan(&userID, &seen); err != nil {
			return nil, fmt.Errorf("scan last seen batch: %w", err)
		}
		if seen.Valid {
			t := seen.Time.UTC()
			result[userID] = &t
		} else {
			result[userID] = nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate last seen batch: %w", err)
	}

	for _, userID := range userIDs {
		if _, ok := result[userID]; !ok {
			result[userID] = nil
		}
	}
	return result, nil
}

func (r *DBRepository) UpsertLastSeen(ctx context.Context, tx db.Tx, userID uuid.UUID, occurredAt time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_presence (user_id, last_seen_at, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE
		SET last_seen_at = CASE
			WHEN user_presence.last_seen_at IS NULL THEN EXCLUDED.last_seen_at
			WHEN EXCLUDED.last_seen_at > user_presence.last_seen_at THEN EXCLUDED.last_seen_at
			ELSE user_presence.last_seen_at
		END,
		updated_at = CASE
			WHEN user_presence.last_seen_at IS NULL OR EXCLUDED.last_seen_at > user_presence.last_seen_at THEN now()
			ELSE user_presence.updated_at
		END
	`, userID, occurredAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert last seen: %w", err)
	}
	return nil
}
