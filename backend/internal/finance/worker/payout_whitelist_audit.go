package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	financeRepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	"go.uber.org/zap"
)

// WhitelistChangeType describes a pilot-whitelist mutation.
type WhitelistChangeType string

const (
	WhitelistEventInitialized WhitelistChangeType = "WHITELIST_INITIALIZED"
	WhitelistEventSellerAdded WhitelistChangeType = "SELLER_ADDED"
	WhitelistEventSellerRemoved WhitelistChangeType = "SELLER_REMOVED"
)

// WhitelistAuditEntry is an immutable record of a whitelist change.
// Written once, never mutated.
type WhitelistAuditEntry struct {
	Timestamp     time.Time           `json:"timestamp"`
	EventType     WhitelistChangeType `json:"event_type"`
	SellerID      uuid.UUID           `json:"seller_id"`      // uuid.Nil for INITIALIZED event
	ActorID       string              `json:"actor_id"`       // "system:startup", "admin:<uuid>"
	Reason        string              `json:"reason,omitempty"`
	WhitelistSize int                 `json:"whitelist_size"` // size after this event
}

// WhitelistAuditLog is an append-only in-process audit log for pilot whitelist changes.
//
// COMPLIANCE: This log is the authoritative record of every whitelist mutation
// in the current process lifetime. Structured zap events are always emitted.
// When repo is non-nil (staging/production), entries are also persisted to DB;
// a DB write failure returns an error so callers can fail-closed at startup.
// When repo is nil (local dev), the in-process log and structured logs are the
// only record — callers should warn rather than crash.
type WhitelistAuditLog struct {
	mu      sync.RWMutex
	entries []WhitelistAuditEntry
	log     *zap.Logger
	repo    financeRepo.WhitelistAuditRepository // nil = dev mode
}

// NewWhitelistAuditLog creates an audit log with optional DB persistence.
// Pass repo=nil for local dev (warn-only mode).
// Pass a real repo for staging/production (fail-closed mode).
func NewWhitelistAuditLog(log *zap.Logger, repo financeRepo.WhitelistAuditRepository) *WhitelistAuditLog {
	if log == nil {
		log = zap.NewNop()
	}
	return &WhitelistAuditLog{log: log, repo: repo}
}

// Record appends an entry, emits a structured log event, and — when a DB repo
// is wired — persists the record. Returns an error only when the DB write fails
// (repo is non-nil). Callers that require fail-closed behavior must check the
// returned error and terminate startup accordingly.
func (a *WhitelistAuditLog) Record(ctx context.Context, entry WhitelistAuditEntry) error {
	a.mu.Lock()
	a.entries = append(a.entries, entry)
	total := len(a.entries)
	a.mu.Unlock()

	a.log.Info("PAYOUT_WHITELIST_AUDIT",
		zap.String("event_type", string(entry.EventType)),
		zap.String("seller_id", entry.SellerID.String()),
		zap.String("actor_id", entry.ActorID),
		zap.String("reason", entry.Reason),
		zap.Time("timestamp", entry.Timestamp),
		zap.Int("whitelist_size_after", entry.WhitelistSize),
		zap.Int("audit_log_total_entries", total),
	)

	if a.repo == nil {
		return nil
	}

	rec := financeRepo.WhitelistAuditRecord{
		Action:    string(entry.EventType),
		ActorID:   entry.ActorID,
		Reason:    entry.Reason,
		Source:    "config",
		CreatedAt: entry.Timestamp,
	}
	if entry.SellerID != uuid.Nil {
		id := entry.SellerID
		rec.SellerID = &id
	}

	if err := a.repo.Append(ctx, rec); err != nil {
		return fmt.Errorf("whitelist audit DB persistence failed (event=%s seller=%s): %w",
			entry.EventType, entry.SellerID, err)
	}
	return nil
}

// All returns a copy of all entries in append order.
func (a *WhitelistAuditLog) All() []WhitelistAuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]WhitelistAuditEntry, len(a.entries))
	copy(out, a.entries)
	return out
}

// Since returns entries recorded after t.
func (a *WhitelistAuditLog) Since(t time.Time) []WhitelistAuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []WhitelistAuditEntry
	for _, e := range a.entries {
		if e.Timestamp.After(t) {
			out = append(out, e)
		}
	}
	return out
}

// ForSeller returns all entries for a specific seller.
func (a *WhitelistAuditLog) ForSeller(sellerID uuid.UUID) []WhitelistAuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []WhitelistAuditEntry
	for _, e := range a.entries {
		if e.SellerID == sellerID {
			out = append(out, e)
		}
	}
	return out
}

// ============================================================================
// WHITELIST MANAGER
// ============================================================================

// WhitelistManager provides audited, concurrent-safe access to the pilot whitelist.
// Every add/remove is recorded in the WhitelistAuditLog and emitted as a
// structured log event so there is no silent mutation.
type WhitelistManager struct {
	mu        sync.RWMutex
	whitelist map[uuid.UUID]bool
	auditLog  *WhitelistAuditLog
}

// NewWhitelistManager creates a WhitelistManager from the startup configuration
// and records a WHITELIST_INITIALIZED event. Returns an error if DB persistence
// is wired (repo != nil) and the initial audit write fails — callers must treat
// this as a fatal startup error in non-dev environments.
func NewWhitelistManager(
	ctx context.Context,
	initial []uuid.UUID,
	actorID string,
	reason string,
	auditLog *WhitelistAuditLog,
) (*WhitelistManager, error) {
	wm := &WhitelistManager{
		whitelist: make(map[uuid.UUID]bool, len(initial)),
		auditLog:  auditLog,
	}
	for _, id := range initial {
		wm.whitelist[id] = true
	}
	if err := auditLog.Record(ctx, WhitelistAuditEntry{
		Timestamp:     time.Now(),
		EventType:     WhitelistEventInitialized,
		SellerID:      uuid.Nil,
		ActorID:       actorID,
		Reason:        reason,
		WhitelistSize: len(wm.whitelist),
	}); err != nil {
		return nil, fmt.Errorf("whitelist manager init: %w", err)
	}
	return wm, nil
}

// IsWhitelisted returns true if the seller is in the pilot whitelist.
func (wm *WhitelistManager) IsWhitelisted(sellerID uuid.UUID) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.whitelist[sellerID]
}

// Add inserts a seller and records an audit entry. Returns an error if DB
// persistence fails (repo non-nil). In local dev (repo nil) always succeeds.
func (wm *WhitelistManager) Add(ctx context.Context, sellerID uuid.UUID, actorID, reason string) error {
	wm.mu.Lock()
	wm.whitelist[sellerID] = true
	size := len(wm.whitelist)
	wm.mu.Unlock()

	return wm.auditLog.Record(ctx, WhitelistAuditEntry{
		Timestamp:     time.Now(),
		EventType:     WhitelistEventSellerAdded,
		SellerID:      sellerID,
		ActorID:       actorID,
		Reason:        reason,
		WhitelistSize: size,
	})
}

// Remove deletes a seller and records an audit entry. Returns an error if DB
// persistence fails (repo non-nil). In local dev (repo nil) always succeeds.
func (wm *WhitelistManager) Remove(ctx context.Context, sellerID uuid.UUID, actorID, reason string) error {
	wm.mu.Lock()
	delete(wm.whitelist, sellerID)
	size := len(wm.whitelist)
	wm.mu.Unlock()

	return wm.auditLog.Record(ctx, WhitelistAuditEntry{
		Timestamp:     time.Now(),
		EventType:     WhitelistEventSellerRemoved,
		SellerID:      sellerID,
		ActorID:       actorID,
		Reason:        reason,
		WhitelistSize: size,
	})
}

// Size returns the current whitelist size.
func (wm *WhitelistManager) Size() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.whitelist)
}

// AuditLog returns the associated audit log.
func (wm *WhitelistManager) AuditLog() *WhitelistAuditLog {
	return wm.auditLog
}


