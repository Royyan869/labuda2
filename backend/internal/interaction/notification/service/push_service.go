package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	firebasepkg "github.com/labuda/backend/pkg/firebase"
	"go.uber.org/zap"
)

// PoolQuerier is the minimal interface required by PushService for FCM token
// pool-fallback queries. Satisfied by *pgxpool.Pool and by test fakes.
type PoolQuerier interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
}

// PushRetryQueue defines the interface for enqueuing failed push notifications.
type PushRetryQueue interface {
	EnqueuePushRetry(
		ctx context.Context,
		notificationID, recipientID uuid.UUID,
		fcmToken, title, body string,
		data map[string]interface{},
	) error
}

// PushService handles sending push notifications via FCM.
//
// HARDENED (SESSION 2):
// - Failed pushes are enqueued for retry (no fire-and-forget)
// - Retry queue implements exponential backoff with 24h window
type PushService struct {
	firebaseClient *firebasepkg.Client
	tokenRepo      *repository.FCMTokenRepository
	retryQueue     PushRetryQueue // Optional: if nil, no retry
	log            *zap.Logger
}

// NewPushService creates a new PushService.
// pool is used as the DB fallback when SendNotification is called with tx=nil
// (the normal async push path). Passing pool=nil is safe: token lookup will
// return a controlled error (logged as Warn) instead of panicking.
func NewPushService(firebaseClient *firebasepkg.Client, pool PoolQuerier, retryQueue PushRetryQueue, log *zap.Logger) *PushService {
	if log == nil {
		log = zap.NewNop()
	}
	return &PushService{
		firebaseClient: firebaseClient,
		tokenRepo:      repository.NewFCMTokenRepository(pool),
		retryQueue:     retryQueue,
		log:            log,
	}
}

// SendNotification sends a push notification for a given notification.
// Implements worker.PushSender interface.
// It safely handles cases where:
// - No FCM tokens exist for the user
// - All tokens are invalid
// - FCM sending fails
//
// Notification record creation is NOT affected by push failures.
//
// The notification parameter can be either:
// - *entity.Notification: Full notification entity with all fields
// - map[string]interface{}: Minimal notification info from worker (contains recipient_id, type, data)
//
// HARDENED (SESSION 2):
// - Failed pushes are enqueued for retry if retryQueue is configured
func (s *PushService) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	// Extract recipientID and type from notification
	var recipientID uuid.UUID
	var notifyType string
	var data map[string]interface{}
	var notificationID uuid.UUID

	switch n := notification.(type) {
	case *entity.Notification:
		recipientID = n.RecipientID
		notifyType = string(n.Type)
		data = n.Data
		notificationID = n.ID
	case map[string]interface{}:
		// Extract from worker's minimal notification map
		if recipientIDStr, ok := n["recipient_id"].(string); ok {
			if parsed, err := uuid.Parse(recipientIDStr); err == nil {
				recipientID = parsed
			}
		}
		notifyType, _ = n["type"].(string)
		data = n
		if idStr, ok := n["id"].(string); ok {
			if parsed, err := uuid.Parse(idStr); err == nil {
				notificationID = parsed
			}
		}
	default:
		s.log.Warn("Unknown notification type for push", zap.Any("type", fmt.Sprintf("%T", notification)))
		return nil
	}

	if recipientID == uuid.Nil {
		s.log.Warn("Invalid recipient ID for push notification")
		return nil
	}

	// Get active FCM tokens for the recipient
	tokens, err := s.tokenRepo.GetActiveTokensByUser(ctx, tx, recipientID)
	if err != nil {
		s.log.Warn("Failed to get FCM tokens for push",
			zap.String("recipient_id", recipientID.String()),
			zap.Error(err),
		)
		// FAIL-SAFE: Don't fail - notification record is still created
		return nil
	}

	if len(tokens) == 0 {
		s.log.Debug("No active FCM tokens for user, skipping push",
			zap.String("recipient_id", recipientID.String()),
		)
		return nil
	}

	// Prepare data payload for navigation
	fcmData := s.buildDataPayload(data, notifyType)

	// Collect FCM token strings
	fcmTokens := make([]string, len(tokens))
	for i, t := range tokens {
		fcmTokens[i] = t.Token
	}

	// Send push notification via Firebase
	// If Firebase client is not available (mock mode), just log
	if s.firebaseClient == nil || s.firebaseClient.MessagingClient == nil {
		s.log.Debug("Firebase messaging client not available, skipping push",
			zap.String("recipient_id", recipientID.String()),
			zap.Int("token_count", len(fcmTokens)),
		)
		return nil
	}

	// Send to all tokens
	invalidTokens, err := s.firebaseClient.SendBatchPush(ctx, fcmTokens, title, body, fcmData)
	if err != nil {
		s.log.Warn("Failed to send batch FCM push",
			zap.String("recipient_id", recipientID.String()),
			zap.Error(err),
		)

		// HARDENED: Enqueue for retry instead of fire-and-forget
		if s.retryQueue != nil {
			for _, token := range fcmTokens {
				// Skip invalid tokens
				isInvalid := false
				for _, invalid := range invalidTokens {
					if token == invalid {
						isInvalid = true
						break
					}
				}
				if isInvalid {
					continue
				}

				// Enqueue for retry
				if retryErr := s.retryQueue.EnqueuePushRetry(
					ctx,
					notificationID,
					recipientID,
					token,
					title,
					body,
					data,
				); retryErr != nil {
					s.log.Error("Failed to enqueue push retry",
						zap.String("recipient_id", recipientID.String()),
						zap.Error(retryErr),
					)
				}
			}
		}

		// Don't fail - notification record is still created
		return nil
	}

	// Clean up invalid tokens asynchronously (don't block)
	if len(invalidTokens) > 0 {
		go s.cleanupInvalidTokens(context.Background(), recipientID, invalidTokens)
	}

	s.log.Debug("Push notification sent successfully",
		zap.String("recipient_id", recipientID.String()),
		zap.Int("token_count", len(fcmTokens)),
		zap.Int("invalid_count", len(invalidTokens)),
	)

	return nil
}

// buildDataPayload builds the data payload for FCM.
// This includes navigation data and notification metadata.
func (s *PushService) buildDataPayload(data map[string]interface{}, notifyType string) map[string]string {
	fcmData := make(map[string]string)

	// Add notification type for routing
	if notifyType != "" {
		fcmData["type"] = notifyType
	}

	// Add navigation data from the notification's Data field
	if data != nil {
		for k, v := range data {
			// Convert interface{} to string
			switch val := v.(type) {
			case string:
				fcmData[k] = val
			case uuid.UUID:
				fcmData[k] = val.String()
			case int, int32, int64:
				fcmData[k] = fmt.Sprintf("%d", val)
			case float64:
				fcmData[k] = fmt.Sprintf("%.0f", val)
			case bool:
				if val {
					fcmData[k] = "true"
				} else {
					fcmData[k] = "false"
				}
			}
		}
	}

	return fcmData
}

// cleanupInvalidTokens removes invalid FCM tokens from the database.
// This runs asynchronously to avoid blocking the notification flow.
func (s *PushService) cleanupInvalidTokens(ctx context.Context, userID uuid.UUID, invalidTokens []string) {
	for _, token := range invalidTokens {
		if err := s.tokenRepo.DeactivateByToken(ctx, nil, token); err != nil {
			s.log.Warn("Failed to deactivate invalid FCM token",
				zap.String("token", token[:20]+"..."),
				zap.Error(err),
			)
		}
	}

	s.log.Info("Cleaned up invalid FCM tokens",
		zap.String("user_id", userID.String()),
		zap.Int("count", len(invalidTokens)),
	)
}


