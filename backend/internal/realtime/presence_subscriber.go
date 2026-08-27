package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	presencepkg "github.com/labuda/backend/internal/presence"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PresenceSubscriber fans out Redis presence.changed events to watcher connections.
type PresenceSubscriber struct {
	service *presencepkg.Service
	hub     *Hub
	log     *zap.Logger

	mu                sync.Mutex
	running           bool
	stopCh            chan struct{}
	wg                sync.WaitGroup
	pubsub            *goredis.PubSub
	deliveredVersions map[uuid.UUID]int64
}

func NewPresenceSubscriber(service *presencepkg.Service, hub *Hub, log *zap.Logger) *PresenceSubscriber {
	if log == nil {
		log = zap.NewNop()
	}
	return &PresenceSubscriber{
		service:           service,
		hub:               hub,
		log:               log,
		stopCh:            make(chan struct{}),
		deliveredVersions: make(map[uuid.UUID]int64),
	}
}

func (s *PresenceSubscriber) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.service == nil || s.hub == nil {
		s.log.Warn("Presence subscriber disabled: missing dependencies")
		s.mu.Unlock()
		return
	}
	s.stopCh = make(chan struct{})
	s.running = true
	s.deliveredVersions = make(map[uuid.UUID]int64)
	s.wg.Add(1)
	go s.run()
	s.mu.Unlock()

	s.log.Info("Presence subscriber started")
}

func (s *PresenceSubscriber) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	if s.pubsub != nil {
		_ = s.pubsub.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.pubsub = nil
	s.deliveredVersions = make(map[uuid.UUID]int64)
	s.mu.Unlock()

	s.log.Info("Presence subscriber stopped")
}

func (s *PresenceSubscriber) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *PresenceSubscriber) run() {
	defer s.wg.Done()

	backoff := presencepkg.PresenceRetryBackoff
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		pubsub, err := s.service.SubscribeEvents(context.Background())
		if err != nil {
			s.log.Warn("Presence subscriber connect failed", zap.Error(err))
			if !s.waitOrStop(backoff) {
				return
			}
			backoff = nextPresenceBackoff(backoff)
			continue
		}

		s.mu.Lock()
		s.pubsub = pubsub
		s.mu.Unlock()

		ch := pubsub.Channel()

	connectedLoop:
		for {
			select {
			case <-s.stopCh:
				_ = pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					_ = pubsub.Close()
					break connectedLoop
				}
				s.handleMessage(context.Background(), msg.Payload)
			}
		}

		s.mu.Lock()
		if s.pubsub == pubsub {
			s.pubsub = nil
		}
		s.mu.Unlock()

		if !s.waitOrStop(backoff) {
			return
		}
		backoff = nextPresenceBackoff(backoff)
	}
}

func (s *PresenceSubscriber) handleMessage(ctx context.Context, payload string) {
	var event presencepkg.Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		s.log.Debug("Ignoring malformed presence event", zap.Error(err))
		return
	}
	if event.Type != "presence.changed" {
		return
	}
	if event.State.UserID == uuid.Nil {
		return
	}
	if !s.shouldDeliver(event.State.UserID, event.State.Version) {
		return
	}

	conns := s.hub.GetUserConnections(event.State.UserID)
	if len(conns) == 0 {
		return
	}

	viewerIDs := make([]uuid.UUID, 0, len(conns))
	seen := make(map[uuid.UUID]struct{}, len(conns))
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		if _, ok := seen[conn.UserID]; ok {
			continue
		}
		seen[conn.UserID] = struct{}{}
		viewerIDs = append(viewerIDs, conn.UserID)
	}
	if len(viewerIDs) == 0 {
		return
	}

	visibleStates, err := s.service.ResolveVisibleStatesForTarget(ctx, event.State.UserID, viewerIDs, event.State)
	if err != nil {
		s.log.Debug("Failed to resolve visible presence state",
			zap.String("user_id", event.State.UserID.String()),
			zap.Error(err),
		)
		return
	}

	for _, conn := range conns {
		if conn == nil {
			continue
		}
		state := visibleStates[conn.UserID]
		state.UserID = event.State.UserID
		if conn.UserID == event.State.UserID {
			state = event.State
		}
		payload := marshalPresenceChanged(state)
		select {
		case conn.Send <- payload:
		default:
			s.log.Warn("Slow client during presence fanout, disconnecting",
				zap.String("connection_id", conn.ID),
				zap.String("user_id", conn.UserID.String()),
			)
			conn.Close()
		}
	}
}

func marshalPresenceChanged(state presencepkg.State) []byte {
	payload, err := json.Marshal(presencepkg.Event{
		Type:  "presence.changed",
		State: state,
	})
	if err != nil {
		return nil
	}
	return payload
}

func (s *PresenceSubscriber) shouldDeliver(userID uuid.UUID, version int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.deliveredVersions[userID]
	if ok && version <= current {
		return false
	}
	s.deliveredVersions[userID] = version
	return true
}

func (s *PresenceSubscriber) waitOrStop(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-s.stopCh:
		return false
	case <-timer.C:
		return true
	}
}

func nextPresenceBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return presencepkg.PresenceRetryBackoff
	}
	next := current * 2
	max := 30 * time.Second
	if next > max {
		return max
	}
	return next
}

// Ensure the subscriber compiles against the same lifecycle shape as other workers.
var _ interface {
	Start()
	Stop()
	IsRunning() bool
} = (*PresenceSubscriber)(nil)
