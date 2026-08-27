// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Dispute domain manages dispute state and resolution.
// All financial operations are delegated to WalletService.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/dispute/infrastructure/repository"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// TASK 3: ABUSE MONITORING
// =============================================================================
// This service detects dispute abuse patterns and flags suspicious users.
//
// ABUSE PATTERNS DETECTED:
// 1. High dispute frequency: caller opens >= 5 disputes in 30 days
// 2. Repeated counterparty: caller opens >= 3 disputes against same party in 30 days
//
// DEFERRED:
// 3. Dispute rate (disputes / total orders): requires cross-domain order count.
//    Deferred until order domain exposes a caller-scoped order count query.
// 4. Same reason code abuse: deferred — needs operational baseline first.
//
// THRESHOLDS:
// - Max disputes per 30 days: 5
// - Max disputes with same counterparty in 30 days: 3
//
// FAIL MODE:
// If the DB query fails, CheckUserBeforeDispute returns an error and dispute
// creation is blocked (fail-closed). This is intentional: a degraded abuse
// check is more dangerous than a temporarily blocked dispute.
// =============================================================================

const (
	// MaxDisputesPer30Days is the maximum allowed disputes per user in 30 days
	MaxDisputesPer30Days = 5
	// MaxDisputesWithSameCounterparty is the maximum allowed disputes with same counterparty in 30 days
	MaxDisputesWithSameCounterparty = 3
	// HighDisputeRateThreshold is the dispute rate (disputes/orders) considered suspicious.
	// Rate check is deferred (requires cross-domain order count) but threshold is kept for future use.
	HighDisputeRateThreshold = 0.5 // 50%
)

// AbuseRiskLevel represents the risk level of a user
type AbuseRiskLevel string

const (
	AbuseRiskLevelLow      AbuseRiskLevel = "low"
	AbuseRiskLevelMedium   AbuseRiskLevel = "medium"
	AbuseRiskLevelHigh     AbuseRiskLevel = "high"
	AbuseRiskLevelCritical AbuseRiskLevel = "critical"
)

// DisputeAbuseService handles dispute abuse detection and monitoring.
type DisputeAbuseService struct {
	disputeRepo disputeRepo.DisputeRepository
}

// NewDisputeAbuseService creates a new DisputeAbuseService.
func NewDisputeAbuseService() *DisputeAbuseService {
	return &DisputeAbuseService{
		disputeRepo: repository.NewDisputeRepository(),
	}
}

// UserDisputeStats contains dispute statistics for a user.
type UserDisputeStats struct {
	UserID             uuid.UUID
	TotalDisputes      int
	DisputesLast30Days int
	DisputesLast90Days int
	// DisputeRate is deferred — requires cross-domain order count query.
	// Set to 0.0 until activated. Rate check in CheckUserBeforeDispute is bypassed.
	DisputeRate float64
	RiskLevel   AbuseRiskLevel
}

// AbuseFlags contains detected abuse flags for a user.
type AbuseFlags struct {
	UserID               uuid.UUID
	HighDisputeFrequency bool
	RepeatedCounterparty bool
	HighDisputeRate      bool // always false (rate check deferred)
	SameReasonAbuse      bool // always false (deferred)
	RiskLevel            AbuseRiskLevel
	FlaggedAt            time.Time
	Details              []string
}

// CheckUserBeforeDispute checks if a user should be allowed to open a dispute.
// counterpartyID is the other party in the order (buyer checks against seller, seller checks against buyer).
// Returns an error if the user has abusive patterns.
// Fail-closed on DB error: a degraded check blocks dispute creation.
func (s *DisputeAbuseService) CheckUserBeforeDispute(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	counterpartyID uuid.UUID,
) error {
	now := time.Now()
	window30 := now.Add(-30 * 24 * time.Hour)

	// Check 1: high dispute frequency in last 30 days.
	count30, err := s.disputeRepo.GetCallerDisputeCount(ctx, tx, userID, window30)
	if err != nil {
		return fmt.Errorf("abuse check: frequency query failed: %w", err)
	}
	if count30 >= MaxDisputesPer30Days {
		return &ErrHighDisputeFrequency{
			UserID:    userID,
			Count:     count30,
			Threshold: MaxDisputesPer30Days,
		}
	}

	// Check 2: repeated disputes against the same counterparty in last 30 days.
	if counterpartyID != uuid.Nil {
		counterpartyCount, err := s.disputeRepo.GetCallerDisputeCountAgainstParty(ctx, tx, userID, counterpartyID, window30)
		if err != nil {
			return fmt.Errorf("abuse check: counterparty query failed: %w", err)
		}
		if counterpartyCount >= MaxDisputesWithSameCounterparty {
			return &ErrRepeatedCounterpartyAbuse{
				UserID:       userID,
				Counterparty: counterpartyID,
				Count:        counterpartyCount,
			}
		}
	}

	return nil
}

// GetUserDisputeStats retrieves dispute statistics for a user.
// DisputeRate is always 0.0 (rate check deferred; requires cross-domain order count).
func (s *DisputeAbuseService) GetUserDisputeStats(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*UserDisputeStats, error) {
	now := time.Now()
	window30 := now.Add(-30 * 24 * time.Hour)
	window90 := now.Add(-90 * 24 * time.Hour)

	last30, err := s.disputeRepo.GetCallerDisputeCount(ctx, tx, userID, window30)
	if err != nil {
		return nil, fmt.Errorf("GetUserDisputeStats: last30 query: %w", err)
	}

	last90, err := s.disputeRepo.GetCallerDisputeCount(ctx, tx, userID, window90)
	if err != nil {
		return nil, fmt.Errorf("GetUserDisputeStats: last90 query: %w", err)
	}

	// Total = all time; use a very old epoch as "since" lower bound.
	allTime, err := s.disputeRepo.GetCallerDisputeCount(ctx, tx, userID, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("GetUserDisputeStats: allTime query: %w", err)
	}

	return &UserDisputeStats{
		UserID:             userID,
		TotalDisputes:      allTime,
		DisputesLast30Days: last30,
		DisputesLast90Days: last90,
		DisputeRate:        0.0, // deferred: requires cross-domain order count
		RiskLevel:          AbuseRiskLevelLow,
	}, nil
}

// DetectAbusePatterns analyzes a user's dispute patterns and flags suspicious activity.
// counterpartyID may be uuid.Nil if not available (counterparty check will be skipped).
func (s *DisputeAbuseService) DetectAbusePatterns(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	counterpartyID uuid.UUID,
) (*AbuseFlags, error) {
	flags := &AbuseFlags{
		UserID:    userID,
		RiskLevel: AbuseRiskLevelLow,
		FlaggedAt: time.Now(),
		Details:   []string{},
	}

	stats, err := s.GetUserDisputeStats(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user dispute stats: %w", err)
	}

	// Check 1: High dispute frequency
	if stats.DisputesLast30Days >= MaxDisputesPer30Days {
		flags.HighDisputeFrequency = true
		flags.Details = append(flags.Details,
			fmt.Sprintf("User has %d disputes in last 30 days (threshold: %d)",
				stats.DisputesLast30Days, MaxDisputesPer30Days))
		flags.RiskLevel = AbuseRiskLevelHigh
	}

	// Check 2: Repeated disputes against same counterparty
	if counterpartyID != uuid.Nil {
		window30 := time.Now().Add(-30 * 24 * time.Hour)
		counterpartyCount, err := s.disputeRepo.GetCallerDisputeCountAgainstParty(ctx, tx, userID, counterpartyID, window30)
		if err != nil {
			return nil, fmt.Errorf("failed to get counterparty dispute count: %w", err)
		}
		if counterpartyCount >= MaxDisputesWithSameCounterparty {
			flags.RepeatedCounterparty = true
			flags.Details = append(flags.Details,
				fmt.Sprintf("User has %d disputes against counterparty %s in last 30 days (threshold: %d)",
					counterpartyCount, counterpartyID, MaxDisputesWithSameCounterparty))
			if flags.RiskLevel == AbuseRiskLevelLow {
				flags.RiskLevel = AbuseRiskLevelMedium
			}
		}
	}

	// Check 3: High dispute rate — deferred (requires cross-domain order count).
	// Check 4: Same reason code abuse — deferred (needs operational baseline).

	// Escalate to critical if multiple flags
	flagCount := 0
	if flags.HighDisputeFrequency {
		flagCount++
	}
	if flags.RepeatedCounterparty {
		flagCount++
	}

	if flagCount >= 2 {
		flags.RiskLevel = AbuseRiskLevelCritical
	} else if flagCount == 1 && flags.RiskLevel == AbuseRiskLevelLow {
		flags.RiskLevel = AbuseRiskLevelMedium
	}

	return flags, nil
}

// =============================================================================
// ERROR TYPES
// =============================================================================

// ErrHighDisputeFrequency is returned when a user has too many disputes.
type ErrHighDisputeFrequency struct {
	UserID    uuid.UUID
	Count     int
	Threshold int
}

func (e *ErrHighDisputeFrequency) Error() string {
	return fmt.Sprintf("user %s has exceeded dispute frequency limit: %d disputes in last 30 days (threshold: %d)",
		e.UserID, e.Count, e.Threshold)
}

// ErrHighDisputeRate is returned when a user has too high a dispute rate.
// Currently not instantiated (rate check deferred).
type ErrHighDisputeRate struct {
	UserID uuid.UUID
	Rate   float64
}

func (e *ErrHighDisputeRate) Error() string {
	return fmt.Sprintf("user %s has suspiciously high dispute rate: %.1f%%",
		e.UserID, e.Rate*100)
}

// ErrRepeatedCounterpartyAbuse is returned when a user repeatedly disputes the same counterparty.
type ErrRepeatedCounterpartyAbuse struct {
	UserID       uuid.UUID
	Counterparty uuid.UUID
	Count        int
}

func (e *ErrRepeatedCounterpartyAbuse) Error() string {
	return fmt.Sprintf("user %s has repeatedly disputed counterparty %s: %d times in last 30 days",
		e.UserID, e.Counterparty, e.Count)
}


