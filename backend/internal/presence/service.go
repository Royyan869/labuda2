package presence

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/platform/events"
	"github.com/labuda/backend/pkg/db"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Service struct {
	db       *db.DB
	redis    *RedisRepository
	presence LastSeenWriter
	outbox   OutboxInserter
	log      *zap.Logger
}

type subjectSnapshot struct {
	SubjectFacts
	Blocklisted bool
	RedisState  State
}

func NewService(database *db.DB, redisRepo *RedisRepository, presenceRepo LastSeenWriter, log *zap.Logger, outbox ...OutboxInserter) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	var inserter OutboxInserter
	if len(outbox) > 0 {
		inserter = outbox[0]
	}
	return &Service{db: database, redis: redisRepo, presence: presenceRepo, outbox: inserter, log: log}
}

func (s *Service) ResumeLease(ctx context.Context, userID uuid.UUID, connectionID string) (*LeaseResult, error) {
	return s.redis.ResumeLease(ctx, userID, connectionID, time.Now().UTC())
}

func (s *Service) LeaveLease(ctx context.Context, userID uuid.UUID, connectionID string) (*LeaseResult, error) {
	return s.redis.LeaveLease(ctx, userID, connectionID, time.Now().UTC())
}

func (s *Service) BumpVisibilityVersion(ctx context.Context, userID uuid.UUID) (*LeaseResult, error) {
	return s.redis.BumpVersion(ctx, userID)
}

func (s *Service) PublishChanged(ctx context.Context, state State) error {
	return s.redis.Publish(ctx, Event{
		Type:  "presence.changed",
		State: state,
	})
}

func (s *Service) SubscribeEvents(ctx context.Context) (*goredis.PubSub, error) {
	return s.redis.Subscribe(ctx)
}

func (s *Service) ClaimDueUsers(ctx context.Context, limit int) ([]ClaimedDueUser, error) {
	return s.redis.ClaimDueUsers(ctx, limit, time.Now().UTC())
}

func (s *Service) SweepUser(ctx context.Context, userID uuid.UUID, dueAt time.Time) (*LeaseResult, error) {
	return s.redis.SweepLease(ctx, userID, dueAt.UTC(), time.Now().UTC())
}

func (s *Service) PersistLastSeen(ctx context.Context, userID uuid.UUID, occurredAt time.Time, version int64) error {
	occurredAt = occurredAt.UTC()
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.presence.UpsertLastSeen(ctx, tx, userID, occurredAt)
	}); err == nil {
		return nil
	} else if s.outbox == nil {
		return err
	}

	payload := LastSeenRecordPayload{
		UserID:     userID,
		LastSeenAt: occurredAt.Format(time.RFC3339),
		Version:    version,
	}
	idempotencyKey := fmt.Sprintf("%s.%d", userID.String(), version)
	if outboxErr := s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.outbox.InsertTx(ctx, tx, events.EventUserPresenceLastSeenRecord, payload, idempotencyKey)
	}); outboxErr != nil {
		s.log.Warn("presence last_seen retry enqueue failed",
			zap.String("user_id", userID.String()),
			zap.Int64("version", version),
			zap.Error(outboxErr),
		)
		return fmt.Errorf("persist last seen failed and enqueue retry failed: %w", outboxErr)
	}
	s.log.Warn("presence last_seen persisted via outbox retry",
		zap.String("user_id", userID.String()),
		zap.Int64("version", version),
	)
	return nil
}

func (s *Service) BuildSnapshot(ctx context.Context, viewerID uuid.UUID, targetIDs []uuid.UUID) ([]State, error) {
	targetIDs = dedupeAndSort(targetIDs)
	if len(targetIDs) == 0 {
		return []State{}, nil
	}

	subjects, err := s.loadSubjects(ctx, targetIDs)
	if err != nil {
		return nil, err
	}

	redisStates, err := s.redis.GetStatesBatch(ctx, targetIDs)
	if err != nil {
		return nil, err
	}

	blocked, err := s.blockedSet(ctx, viewerID, targetIDs)
	if err != nil {
		return nil, err
	}

	states := make([]State, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		subject := subjects[targetID]
		subject.RedisState = redisStates[targetID]
		subject.Blocklisted = blocked[targetID]
		states = append(states, s.visibleStateForViewer(viewerID, targetID, subject))
	}
	return states, nil
}

func (s *Service) ResolveVisibleStatesForTarget(ctx context.Context, targetID uuid.UUID, viewerIDs []uuid.UUID, canonical State) (map[uuid.UUID]State, error) {
	viewerIDs = dedupeAndSort(viewerIDs)
	if len(viewerIDs) == 0 {
		return map[uuid.UUID]State{}, nil
	}

	subjects, err := s.loadSubjects(ctx, []uuid.UUID{targetID})
	if err != nil {
		return nil, err
	}
	subject := subjects[targetID]
	subject.RedisState = canonical

	blocked, err := s.blockedSet(ctx, targetID, viewerIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]State, len(viewerIDs))
	for _, viewerID := range viewerIDs {
		subject.Blocklisted = blocked[viewerID]
		result[viewerID] = s.visibleStateForViewer(viewerID, targetID, subject)
	}
	return result, nil
}

func (s *Service) loadSubjects(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]subjectSnapshot, error) {
	result := make(map[uuid.UUID]subjectSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	rows, err := s.db.Pool().Query(ctx, `
		SELECT
			u.id,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted,
			COALESCE(p.privacy->>'show_activity_status', 'true') AS show_activity_status,
			up.last_seen_at
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		LEFT JOIN user_presence up ON up.user_id = u.id
		WHERE u.id = ANY($1)
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("presence load subjects: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var accountStatus string
		var isDeleted bool
		var showActivity string
		var lastSeen sql.NullTime
		if err := rows.Scan(&id, &accountStatus, &isDeleted, &showActivity, &lastSeen); err != nil {
			return nil, fmt.Errorf("presence load subjects scan: %w", err)
		}
		result[id] = subjectSnapshot{
			SubjectFacts: SubjectFacts{
				UserID:             id,
				AccountStatus:      accountStatus,
				Deleted:            isDeleted,
				ShowActivityStatus: strings.EqualFold(showActivity, "true"),
				LastSeenAt:         nullTimePtr(lastSeen),
			},
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("presence load subjects rows: %w", err)
	}

	for _, id := range userIDs {
		if _, ok := result[id]; !ok {
			result[id] = subjectSnapshot{
				SubjectFacts: SubjectFacts{UserID: id, ShowActivityStatus: true},
			}
		}
	}
	return result, nil
}

func (s *Service) blockedSet(ctx context.Context, viewerID uuid.UUID, targetIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if viewerID == uuid.Nil || len(targetIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	result := make(map[uuid.UUID]bool, len(targetIDs))
	rows, err := s.db.Pool().Query(ctx, `
		SELECT DISTINCT
			CASE
				WHEN blocker_id = $1 THEN blocked_id
				ELSE blocker_id
			END AS blocked_author
		FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = ANY($2))
		   OR (blocker_id = ANY($2) AND blocked_id = $1)
	`, viewerID, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("presence blocked set: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("presence blocked set scan: %w", err)
		}
		result[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("presence blocked set rows: %w", err)
	}
	return result, nil
}

func (s *Service) visibleStateForViewer(viewerID, targetID uuid.UUID, subject subjectSnapshot) State {
	redisState := subject.RedisState
	if redisState.UserID == uuid.Nil {
		redisState.UserID = targetID
	}
	if viewerID == targetID {
		if redisState.Version == 0 {
			redisState.Version = subject.Version
		}
		if redisState.LastSeenAt == nil {
			redisState.LastSeenAt = subject.LastSeenAt
		}
		return redisState
	}

	lifecycle := viewercontext.CoarsenLifecycle(subject.AccountStatus, subject.Deleted)
	if lifecycle != viewercontext.PublicLifecycleStateActive || subject.Blocklisted || !subject.ShowActivityStatus {
		return State{
			UserID:     targetID,
			IsOnline:   false,
			LastSeenAt: nil,
			Version:    redisState.Version,
		}
	}

	if redisState.Version == 0 {
		redisState.Version = subject.Version
	}
	if redisState.LastSeenAt == nil {
		redisState.LastSeenAt = subject.LastSeenAt
	}
	redisState.UserID = targetID
	return redisState
}

func dedupeAndSort(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}
