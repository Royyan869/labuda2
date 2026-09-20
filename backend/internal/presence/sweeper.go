package presence

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Sweeper drives the existing Redis lease expiry authority.
//
// It is NOT the state authority — it only invokes the canonical
// ClaimDueUsers → SweepUser → PublishChanged chain on the existing
// Redis Lua implementation. No new key, no new expiry algorithm.
type Sweeper struct {
	service  *Service
	log      *zap.Logger
	interval time.Duration
	batch    int

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewSweeper creates a sweeper using canonical interval/batch.
func NewSweeper(service *Service, log *zap.Logger) *Sweeper {
	if log == nil {
		log = zap.NewNop()
	}
	return &Sweeper{
		service:  service,
		log:      log,
		interval: PresenceWorkerInterval,
		batch:    PresenceMaxClaimBatch,
		stopCh:   make(chan struct{}),
	}
}

// NewSweeperWithConfig allows tests to override interval/batch.
func NewSweeperWithConfig(service *Service, log *zap.Logger, interval time.Duration, batch int) *Sweeper {
	if log == nil {
		log = zap.NewNop()
	}
	if interval <= 0 {
		interval = PresenceWorkerInterval
	}
	if batch <= 0 {
		batch = PresenceMaxClaimBatch
	}
	return &Sweeper{
		service:  service,
		log:      log,
		interval: interval,
		batch:    batch,
		stopCh:   make(chan struct{}),
	}
}

func (s *Sweeper) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	if s.service == nil {
		s.log.Warn("Presence sweeper disabled: nil service")
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.wg.Add(1)
	go s.run()
	s.log.Info("Presence sweeper started",
		zap.Duration("interval", s.interval),
		zap.Int("batch", s.batch),
	)
}

func (s *Sweeper) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	s.mu.Unlock()
	s.wg.Wait()
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	s.log.Info("Presence sweeper stopped")
}

func (s *Sweeper) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Sweeper) run() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sweepOnce()
		}
	}
}

func (s *Sweeper) sweepOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dueUsers, err := s.service.ClaimDueUsers(ctx, s.batch)
	if err != nil {
		s.log.Warn("Presence sweeper ClaimDueUsers failed", zap.Error(err))
		return
	}
	if len(dueUsers) == 0 {
		return
	}
	for _, du := range dueUsers {
		// Use background for each user to avoid one failure blocking others
		uCtx, uCancel := context.WithTimeout(context.Background(), 3*time.Second)
		res, err := s.service.SweepUser(uCtx, du.UserID, du.DueAt)
		uCancel()
		if err != nil {
			s.log.Warn("Presence sweeper SweepUser failed",
				zap.String("user_id", du.UserID.String()),
				zap.Error(err),
			)
			continue
		}
		if res == nil || !res.Transitioned {
			continue
		}
		// Canonical transition: one presence.changed + one durable last_seen (if offline)
		hCtx, hCancel := context.WithTimeout(context.Background(), 4*time.Second)
		if hErr := s.service.HandleOfflineTransition(hCtx, res); hErr != nil {
			s.log.Warn("Presence sweeper HandleOfflineTransition failed",
				zap.String("user_id", du.UserID.String()),
				zap.Error(hErr),
			)
		}
		hCancel()
	}
}

// SweepOnce is exported for tests to trigger a single sweep cycle synchronously.
func (s *Sweeper) SweepOnce(ctx context.Context) {
	dueUsers, err := s.service.ClaimDueUsers(ctx, s.batch)
	if err != nil {
		return
	}
	for _, du := range dueUsers {
		res, err := s.service.SweepUser(ctx, du.UserID, du.DueAt)
		if err != nil || res == nil || !res.Transitioned {
			continue
		}
		_ = s.service.HandleOfflineTransition(ctx, res)
	}
}

var _ interface {
	Start()
	Stop()
	IsRunning() bool
} = (*Sweeper)(nil)
