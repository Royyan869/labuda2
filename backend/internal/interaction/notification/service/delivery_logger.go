package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	dbpkg "github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// DeliveryChannel represents the notification delivery channel.
type DeliveryChannel string

const (
	ChannelInApp DeliveryChannel = "in_app"
	ChannelPush  DeliveryChannel = "push"
	ChannelEmail DeliveryChannel = "email"
)

// DeliveryStatus represents the delivery status.
type DeliveryStatus string

const (
	StatusSent    DeliveryStatus = "sent"
	StatusFailed  DeliveryStatus = "failed"
	StatusSkipped DeliveryStatus = "skipped"
	StatusRetrying DeliveryStatus = "retrying"
)

// DeliveryLogEntry represents a single delivery log entry.
// Channel and Status are plain strings so entries can be constructed
// from both typed constants and the worker.DeliveryLogger interface
// which uses string parameters.
type DeliveryLogEntry struct {
	NotificationID uuid.UUID
	UserID         uuid.UUID
	Channel        string
	Status         string
	Reason         string
	Metadata       map[string]interface{}
}

// DeliveryLogger provides append-only logging for notification delivery events.
// Logging failures never affect notification delivery.
type DeliveryLogger struct {
	db  *dbpkg.DB
	log *zap.Logger

	// Async channel for non-blocking writes
	logChan chan DeliveryLogEntry

	// Buffer size for async logging
	bufferSize int
}

// NewDeliveryLogger creates a new DeliveryLogger with async logging.
func NewDeliveryLogger(db *dbpkg.DB, log *zap.Logger) *DeliveryLogger {
	if log == nil {
		log = zap.NewNop()
	}

	dl := &DeliveryLogger{
		db:         db,
		log:        log,
		bufferSize: 1000,
		logChan:    make(chan DeliveryLogEntry, 1000),
	}

	// Start background writer
	go dl.backgroundWriter()

	return dl
}

// LogDelivery records a delivery event asynchronously.
// This is non-blocking and safe to call from notification hot paths.
// channel and status are plain strings so this method satisfies the
// worker.DeliveryLogger interface (which uses string, not typed aliases).
func (dl *DeliveryLogger) LogDelivery(
	ctx context.Context,
	notificationID, userID uuid.UUID,
	channel string,
	status string,
	reason string,
	metadata map[string]interface{},
) {
	entry := DeliveryLogEntry{
		NotificationID: notificationID,
		UserID:         userID,
		Channel:        channel,
		Status:         status,
		Reason:         reason,
		Metadata:       metadata,
	}

	// Non-blocking send with timeout fallback
	select {
	case dl.logChan <- entry:
		// Successfully queued for async write
	default:
		// Channel full - log but don't block
		dl.log.Warn("Delivery log channel full, dropping log entry",
			zap.String("notification_id", notificationID.String()),
			zap.String("user_id", userID.String()),
			zap.String("channel", channel),
			zap.String("status", status),
		)
	}
}

// LogDeliverySync records a delivery event synchronously.
// Use this for critical events where you need confirmation of logging.
// Falls back to async if the sync write fails.
func (dl *DeliveryLogger) LogDeliverySync(
	ctx context.Context,
	notificationID, userID uuid.UUID,
	channel string,
	status string,
	reason string,
	metadata map[string]interface{},
) {
	err := dl.writeLog(ctx, DeliveryLogEntry{
		NotificationID: notificationID,
		UserID:         userID,
		Channel:        channel,
		Status:         status,
		Reason:         reason,
		Metadata:       metadata,
	})

	if err != nil {
		dl.log.Warn("Failed to write delivery log synchronously, retrying async",
			zap.Error(err),
		)
		// Fallback to async
		dl.LogDelivery(ctx, notificationID, userID, channel, status, reason, metadata)
	}
}

// backgroundWriter processes log entries from the channel.
// Runs in a separate goroutine and batches writes for efficiency.
func (dl *DeliveryLogger) backgroundWriter() {
	batch := make([]DeliveryLogEntry, 0, 100)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := dl.writeBatch(ctx, batch); err != nil {
			dl.log.Warn("Failed to flush delivery log batch",
				zap.Int("count", len(batch)),
				zap.Error(err),
			)
		}

		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-dl.logChan:
			if !ok {
				// Channel closed, flush remaining
				flush()
				return
			}
			batch = append(batch, entry)

			// Flush when batch is full
			if len(batch) >= 100 {
				flush()
			}

		case <-ticker.C:
			// Periodic flush
			flush()
		}
	}
}

// writeBatch writes a batch of log entries to the database.
func (dl *DeliveryLogger) writeBatch(ctx context.Context, batch []DeliveryLogEntry) error {
	if len(batch) == 0 {
		return nil
	}

	query := `
		INSERT INTO notification_delivery_log (notification_id, user_id, channel, status, reason, metadata)
		VALUES `

	// Build query with placeholders
	args := make([]interface{}, 0, len(batch)*6)
	for i, entry := range batch {
		if i > 0 {
			query += ", "
		}
		placeholder := i * 6
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
			placeholder+1, placeholder+2, placeholder+3, placeholder+4, placeholder+5, placeholder+6)

		args = append(args, entry.NotificationID, entry.UserID, entry.Channel, entry.Status, entry.Reason, entry.Metadata)
	}

	_, err := dl.db.Pool().Exec(ctx, query, args...)
	return err
}

// writeLog writes a single log entry to the database.
func (dl *DeliveryLogger) writeLog(ctx context.Context, entry DeliveryLogEntry) error {
	query := `
		INSERT INTO notification_delivery_log (notification_id, user_id, channel, status, reason, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	metadataJSON, _ := json.Marshal(entry.Metadata)

	_, err := dl.db.Pool().Exec(ctx, query,
		entry.NotificationID,
		entry.UserID,
		entry.Channel,
		entry.Status,
		entry.Reason,
		metadataJSON,
	)

	return err
}

// Close stops the background writer.
func (dl *DeliveryLogger) Close() {
	close(dl.logChan)
}

// GetDeliveryStats returns delivery statistics for a user.
func (dl *DeliveryLogger) GetDeliveryStats(ctx context.Context, userID uuid.UUID, since time.Time) (map[string]int64, error) {
	query := `
		SELECT
			channel,
			status,
			COUNT(*) as count
		FROM notification_delivery_log
		WHERE user_id = $1
			AND created_at >= $2
		GROUP BY channel, status
	`

	rows, err := dl.db.Pool().Query(ctx, query, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var channel, status string
		var count int64
		if err := rows.Scan(&channel, &status, &count); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s.%s", channel, status)
		stats[key] = count
	}

	return stats, rows.Err()
}

// GetFailedDeliveries returns recently failed delivery attempts for monitoring.
func (dl *DeliveryLogger) GetFailedDeliveries(ctx context.Context, limit int, since time.Time) ([]DeliveryLogEntry, error) {
	query := `
		SELECT notification_id, user_id, channel, status, reason, metadata
		FROM notification_delivery_log
		WHERE status = 'failed'
			AND created_at >= $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := dl.db.Pool().Query(ctx, query, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DeliveryLogEntry
	for rows.Next() {
		var entry DeliveryLogEntry
		var metadataJSON []byte
		if err := rows.Scan(
			&entry.NotificationID,
			&entry.UserID,
			&entry.Channel,
			&entry.Status,
			&entry.Reason,
			&metadataJSON,
		); err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &entry.Metadata)
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// FailedDeliveryEntry extends DeliveryLogEntry with timestamps for admin queries.
type FailedDeliveryEntry struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	UserID         uuid.UUID
	Channel        string
	Status         string
	Reason         string
	Metadata       map[string]interface{}
	CreatedAt      time.Time
}

// FailedDeliveryResult holds paginated failed delivery results.
type FailedDeliveryResult struct {
	Entries []FailedDeliveryEntry
	Total   int
}

// GetFailedDeliveriesPaginated returns failed deliveries with pagination support.
// O4: admin endpoint backing query.
func (dl *DeliveryLogger) GetFailedDeliveriesPaginated(ctx context.Context, page, pageSize int, since time.Time) (*FailedDeliveryResult, error) {
	offset := (page - 1) * pageSize

	// Count total
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM notification_delivery_log
		WHERE status = 'failed'
			AND created_at >= $1
	`
	err := dl.db.Pool().QueryRow(ctx, countQuery, since).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count failed deliveries: %w", err)
	}

	// Fetch page
	query := `
		SELECT id, notification_id, user_id, channel, status, reason, metadata, created_at
		FROM notification_delivery_log
		WHERE status = 'failed'
			AND created_at >= $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := dl.db.Pool().Query(ctx, query, since, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("query failed deliveries: %w", err)
	}
	defer rows.Close()

	var entries []FailedDeliveryEntry
	for rows.Next() {
		var e FailedDeliveryEntry
		var metadataJSON []byte
		if err := rows.Scan(
			&e.ID, &e.NotificationID, &e.UserID,
			&e.Channel, &e.Status, &e.Reason,
			&metadataJSON, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan failed delivery: %w", err)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &e.Metadata)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed deliveries: %w", err)
	}

	return &FailedDeliveryResult{
		Entries: entries,
		Total:   total,
	}, nil
}

// CleanupOldLogs removes logs older than the specified retention period.
// Returns the number of logs deleted.
func (dl *DeliveryLogger) CleanupOldLogs(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)

	query := `
		DELETE FROM notification_delivery_log
		WHERE created_at < $1
	`

	result, err := dl.db.Pool().Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	rowsAffected := result.RowsAffected()
	dl.log.Info("Cleaned up old notification delivery logs",
		zap.Int64("count", rowsAffected),
		zap.Time("cutoff", cutoff),
	)

	return rowsAffected, nil
}


