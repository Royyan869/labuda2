package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/repository"
	"github.com/labuda/backend/pkg/db"
)

// Service performs canonical first-admin bootstrap atomically.
type Service struct {
	db             *db.DB
	capabilityRepo capabilityRepo.CapabilityRepository
	auditLogger    audit.AdminAuditLogger
	bootstrapSvc   *capability.BootstrapService
}

// NewService creates a bootstrap service.
func NewService(database *db.DB, capRepo capabilityRepo.CapabilityRepository, auditLogger audit.AdminAuditLogger) *Service {
	return &Service{
		db:             database,
		capabilityRepo: capRepo,
		auditLogger:    auditLogger,
		bootstrapSvc:   capability.NewBootstrapService(capRepo),
	}
}

// Result summarizes bootstrap outcome.
type Result struct {
	TargetID       uuid.UUID
	Email          string
	RoleChanged    bool
	PreviousRole   string
	NewRole        string
	Created        int
	SkippedExisting int
	Invalid        int
	Errors         []capability.BootstrapError
}

// BootstrapByID is the primary canonical entry — exactly one existing human user by UUID.
func (s *Service) BootstrapByID(ctx context.Context, targetID uuid.UUID) (*Result, error) {
	if targetID == uuid.Nil {
		return nil, fmt.Errorf("target user_id must not be nil")
	}
	if audit.IsSystemCaller(targetID) {
		return nil, fmt.Errorf("SystemCallerID is not a valid human bootstrap target")
	}
	return s.bootstrapInTx(ctx, func(tx db.Tx) (uuid.UUID, error) {
		return targetID, nil
	})
}

// BootstrapByEmail resolves canonical email (normalized) to exactly one human user.
func (s *Service) BootstrapByEmail(ctx context.Context, email string) (*Result, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil, fmt.Errorf("--email must not be empty")
	}
	normalized := strings.ToLower(trimmed)
	return s.bootstrapInTx(ctx, func(tx db.Tx) (uuid.UUID, error) {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(btrim(email)) = $1 AND deleted_at IS NULL`, normalized).Scan(&id)
		if err != nil {
			return uuid.Nil, fmt.Errorf("email lookup failed for %q: %w", email, err)
		}
		if audit.IsSystemCaller(id) {
			return uuid.Nil, fmt.Errorf("SystemCallerID is not a valid human bootstrap target (resolved from email)")
		}
		var cnt int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(btrim(email)) = $1 AND deleted_at IS NULL`, normalized).Scan(&cnt); err == nil && cnt != 1 {
			return uuid.Nil, fmt.Errorf("email %q resolves to %d users, expected exactly 1", email, cnt)
		}
		return id, nil
	})
}

// DryRunByID validates without committing.
func (s *Service) DryRunByID(ctx context.Context, targetID uuid.UUID) (*Result, error) {
	if targetID == uuid.Nil {
		return nil, fmt.Errorf("target user_id must not be nil")
	}
	if audit.IsSystemCaller(targetID) {
		return nil, fmt.Errorf("SystemCallerID is not a valid human bootstrap target")
	}
	return s.dryRunInTx(ctx, targetID)
}

// DryRunByEmail validates without committing.
func (s *Service) DryRunByEmail(ctx context.Context, email string) (*Result, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil, fmt.Errorf("--email must not be empty")
	}
	normalized := strings.ToLower(trimmed)
	var id uuid.UUID
	// resolve outside tx first, then dry-run
	err := s.db.Pool().QueryRow(ctx, `SELECT id FROM users WHERE lower(btrim(email)) = $1 AND deleted_at IS NULL`, normalized).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("email lookup failed for %q: %w", email, err)
	}
	if audit.IsSystemCaller(id) {
		return nil, fmt.Errorf("SystemCallerID is not a valid human bootstrap target (resolved from email)")
	}
	return s.dryRunInTx(ctx, id)
}

func (s *Service) dryRunInTx(ctx context.Context, targetID uuid.UUID) (*Result, error) {
	var roleVal, statusVal, emailVal string
	var deletedAtVal *time.Time
	var verifiedAtVal *time.Time
	err := s.db.Pool().QueryRow(ctx, `
		SELECT role, account_status::text, deleted_at, email_verified_at, email
		FROM users WHERE id=$1
	`, targetID).Scan(&roleVal, &statusVal, &deletedAtVal, &verifiedAtVal, &emailVal)
	if err != nil {
		return nil, fmt.Errorf("dry-run query failed: %w", err)
	}
	if deletedAtVal != nil {
		return nil, fmt.Errorf("target user %s is deleted", targetID)
	}
	if statusVal != "active" {
		return nil, fmt.Errorf("target account_status=%q, expected 'active'", statusVal)
	}
	if verifiedAtVal == nil {
		return nil, fmt.Errorf("target has no verified email")
	}
	// Bootstrap grants the ENTIRE canonical universe (derived full access), not
	// a hand-picked minimum set. There is no second "bootstrap preset" authority.
	universalCaps := capability.AllCapabilityStrings()
	var skipped, created int
	for _, c := range universalCaps {
		has, err := s.capabilityRepo.HasCapability(ctx, nil, targetID, c)
		if err != nil {
			return nil, err
		}
		if has {
			skipped++
		} else {
			created++
		}
	}
	roleChanged := roleVal != capabilityEntity.AdminRole
	return &Result{
		TargetID:        targetID,
		Email:           emailVal,
		RoleChanged:     roleChanged,
		PreviousRole:    roleVal,
		NewRole:         capabilityEntity.AdminRole,
		Created:         created,
		SkippedExisting: skipped,
	}, nil
}

func (s *Service) bootstrapInTx(ctx context.Context, resolve func(db.Tx) (uuid.UUID, error)) (*Result, error) {
	var out *Result
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		targetID, err := resolve(tx)
		if err != nil {
			return err
		}
		var roleVal, statusVal, emailVal string
		var deletedAtVal *time.Time
		var verifiedAtVal *time.Time
		err = tx.QueryRow(ctx, `
			SELECT role, account_status::text, deleted_at, email_verified_at, email
			FROM users
			WHERE id = $1
			FOR UPDATE
		`, targetID).Scan(&roleVal, &statusVal, &deletedAtVal, &verifiedAtVal, &emailVal)
		if err != nil {
			return fmt.Errorf("failed to query target user %s: %w", targetID, err)
		}
		if deletedAtVal != nil {
			return fmt.Errorf("target user %s is deleted (deleted_at IS NOT NULL)", targetID)
		}
		if statusVal != "active" {
			return fmt.Errorf("target user %s account_status=%q, expected 'active'", targetID, statusVal)
		}
		if verifiedAtVal == nil {
			return fmt.Errorf("target user %s has no verified email (email_verified_at IS NULL)", targetID)
		}

		// Track role change.
		prevRole := roleVal
		roleChanged := false
		if roleVal != capabilityEntity.AdminRole {
			// Update role within same tx.
			tag, execErr := tx.Exec(ctx, `UPDATE users SET role='admin', updated_at=NOW() WHERE id=$1`, targetID)
			if execErr != nil {
				return fmt.Errorf("failed to promote role: %w", execErr)
			}
			if tag.RowsAffected() == 0 {
				return fmt.Errorf("role update affected 0 rows for %s", targetID)
			}
			// Audit must be in same tx. Use operator-agnostic audit? Operator is out-of-band; use targetID as actor? No, must not fake.
			// We log with actor=targetID only for trace? Better to log with actor=targetID is self-promotion which SetRole blocks. For bootstrap we use LogTx with actor=targetID but skip self-escalation guard?
			// Bootstrap is operator action, not user self-action. We use audit.IsSystemCaller check skip? We pass targetID as actor but audit logger skips SystemCaller only. Self-promotion audit is still meaningful as bootstrap record.
			// To avoid violating SetRole self-escalation semantics, we write direct audit row without reusing SetRole guard — bootstrap is out-of-band operator, not in-app role assignment.
			// Use a distinct bootstrap audit action if available, falling back to role_changed.
			if s.auditLogger != nil {
				// LogTx expects actorID; we use targetID with bootstrap metadata. This is traceable but not impersonating system.
				// If audit logger is nil (tests), skip.
				if err := s.auditLogger.LogTx(ctx, tx, targetID, audit.ActionRoleChanged, audit.TargetTypeUser, targetID, map[string]interface{}{
					"old_role": prevRole,
					"new_role": capabilityEntity.AdminRole,
					"source":   "bootstrap-admin",
				}); err != nil {
					return fmt.Errorf("audit log failed: %w", err)
				}
			}
			roleChanged = true
		}

		// Assign the ENTIRE canonical capability universe atomically in the same
		// tx, so bootstrap-admin yields derived full access (role=admin AND
		// coverage of capability.AllCapabilities()). The universe is derived, so
		// a capability added later is picked up automatically — there is no
		// hardcoded bootstrap list to keep in sync.
		// grantedBy = nil => operator-level system grant, does NOT use SystemCallerID.
		bsResult, err := s.bootstrapSvc.AssignInitialCapabilities(ctx, tx, targetID, capability.AllCapabilityStrings(), nil)
		if err != nil {
			return fmt.Errorf("AssignInitialCapabilities failed: %w", err)
		}
		if len(bsResult.Errors) > 0 {
			// Per spec: if ANY required capability/audit fails → rollback.
			// BootstrapService reports errors per-capability; any error must abort tx.
			return fmt.Errorf("capability bootstrap errors: %+v", bsResult.Errors)
		}
		if bsResult.Invalid > 0 {
			return fmt.Errorf("canonical capability universe contains %d invalid capabilities", bsResult.Invalid)
		}

		out = &Result{
			TargetID:        targetID,
			Email:           emailVal,
			RoleChanged:     roleChanged,
			PreviousRole:    prevRole,
			NewRole:         capabilityEntity.AdminRole,
			Created:         bsResult.Created,
			SkippedExisting: bsResult.SkippedExisting,
			Invalid:         bsResult.Invalid,
			Errors:          bsResult.Errors,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
