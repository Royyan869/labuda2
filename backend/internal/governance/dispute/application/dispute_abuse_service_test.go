package application

// =============================================================================
// DisputeAbuseService tests — FIX-1 real query activation
//
// Verifies that CheckUserBeforeDispute enforces frequency and counterparty
// thresholds using real repository data (no placeholder zeros).
// =============================================================================

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	disputeEntity "github.com/labuda/backend/internal/governance/dispute/entity"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/pkg/db"
)

// =============================================================================
// mock repository
// =============================================================================

// mockAbuseRepo is a minimal stub satisfying DisputeRepository for abuse tests.
// Only GetCallerDisputeCount and GetCallerDisputeCountAgainstParty are exercised;
// all other methods panic to catch unexpected calls.
type mockAbuseRepo struct {
	// callerCounts: callerID.String() → count returned for any time window
	callerCounts map[string]int
	// counterpartyCounts: "callerID/partyID" → count returned for any time window
	counterpartyCounts map[string]int
	// forceErr: if non-nil, all queries return this error
	forceErr error
}

var _ disputeRepo.DisputeRepository = (*mockAbuseRepo)(nil)

func newMockAbuseRepo() *mockAbuseRepo {
	return &mockAbuseRepo{
		callerCounts:       make(map[string]int),
		counterpartyCounts: make(map[string]int),
	}
}

func (m *mockAbuseRepo) setCallerCount(callerID uuid.UUID, count int) {
	m.callerCounts[callerID.String()] = count
}

func (m *mockAbuseRepo) setCounterpartyCount(callerID, partyID uuid.UUID, count int) {
	m.counterpartyCounts[callerID.String()+"/"+partyID.String()] = count
}

func (m *mockAbuseRepo) GetCallerDisputeCount(_ context.Context, _ db.Tx, callerID uuid.UUID, _ time.Time) (int, error) {
	if m.forceErr != nil {
		return 0, m.forceErr
	}
	return m.callerCounts[callerID.String()], nil
}

func (m *mockAbuseRepo) GetCallerDisputeCountAgainstParty(_ context.Context, _ db.Tx, callerID uuid.UUID, partyID uuid.UUID, _ time.Time) (int, error) {
	if m.forceErr != nil {
		return 0, m.forceErr
	}
	return m.counterpartyCounts[callerID.String()+"/"+partyID.String()], nil
}

// Unused interface methods — panic to detect unexpected calls.
func (m *mockAbuseRepo) Create(_ context.Context, _ db.Tx, _ *disputeEntity.Dispute) error {
	panic("unexpected call to Create")
}
func (m *mockAbuseRepo) GetByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	panic("unexpected call to GetByOrderID")
}
func (m *mockAbuseRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	panic("unexpected call to GetForUpdate")
}
func (m *mockAbuseRepo) Update(_ context.Context, _ db.Tx, _ *disputeEntity.Dispute) error {
	panic("unexpected call to Update")
}
func (m *mockAbuseRepo) CreateMedia(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) error {
	panic("unexpected call to CreateMedia")
}
func (m *mockAbuseRepo) ListMedia(_ context.Context, _ db.Tx, _ uuid.UUID) ([]string, error) {
	panic("unexpected call to ListMedia")
}
func (m *mockAbuseRepo) ListAll(_ context.Context, _ db.Tx, _ disputeRepo.DisputeListFilters) ([]*disputeEntity.Dispute, int64, error) {
	panic("unexpected call to ListAll")
}
func (m *mockAbuseRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*disputeEntity.Dispute, error) {
	panic("unexpected call to GetByID")
}
func (m *mockAbuseRepo) FindOverdueCandidates(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	panic("unexpected call to FindOverdueCandidates")
}
func (m *mockAbuseRepo) FindTimeoutCandidates(_ context.Context, _ db.Tx, _ int) ([]uuid.UUID, error) {
	panic("unexpected call to FindTimeoutCandidates")
}

// newAbuseServiceWithMock returns a DisputeAbuseService backed by the given mock.
func newAbuseServiceWithMock(mock disputeRepo.DisputeRepository) *DisputeAbuseService {
	return &DisputeAbuseService{disputeRepo: mock}
}

// =============================================================================
// CheckUserBeforeDispute tests
// =============================================================================

func TestCheckUserBeforeDispute_NoDisputes_Allowed(t *testing.T) {
	mock := newMockAbuseRepo()
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckUserBeforeDispute_BelowFrequencyThreshold_Allowed(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	mock.setCallerCount(userID, MaxDisputesPer30Days-1)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, uuid.New())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckUserBeforeDispute_AtFrequencyThreshold_Blocked(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	mock.setCallerCount(userID, MaxDisputesPer30Days)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, uuid.New())
	if err == nil {
		t.Fatal("expected ErrHighDisputeFrequency, got nil")
	}
	var freqErr *ErrHighDisputeFrequency
	if !errors.As(err, &freqErr) {
		t.Fatalf("expected ErrHighDisputeFrequency, got %T: %v", err, err)
	}
	if freqErr.Count != MaxDisputesPer30Days {
		t.Errorf("expected count=%d, got %d", MaxDisputesPer30Days, freqErr.Count)
	}
}

func TestCheckUserBeforeDispute_AboveFrequencyThreshold_Blocked(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	mock.setCallerCount(userID, MaxDisputesPer30Days+3)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, uuid.New())
	var freqErr *ErrHighDisputeFrequency
	if !errors.As(err, &freqErr) {
		t.Fatalf("expected ErrHighDisputeFrequency, got %T: %v", err, err)
	}
}

func TestCheckUserBeforeDispute_BelowCounterpartyThreshold_Allowed(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	counterpartyID := uuid.New()
	mock.setCallerCount(userID, 2)
	mock.setCounterpartyCount(userID, counterpartyID, MaxDisputesWithSameCounterparty-1)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, counterpartyID)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckUserBeforeDispute_AtCounterpartyThreshold_Blocked(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	counterpartyID := uuid.New()
	mock.setCallerCount(userID, 2) // under frequency
	mock.setCounterpartyCount(userID, counterpartyID, MaxDisputesWithSameCounterparty)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, counterpartyID)
	if err == nil {
		t.Fatal("expected ErrRepeatedCounterpartyAbuse, got nil")
	}
	var cpErr *ErrRepeatedCounterpartyAbuse
	if !errors.As(err, &cpErr) {
		t.Fatalf("expected ErrRepeatedCounterpartyAbuse, got %T: %v", err, err)
	}
	if cpErr.Counterparty != counterpartyID {
		t.Errorf("expected counterparty %v, got %v", counterpartyID, cpErr.Counterparty)
	}
}

func TestCheckUserBeforeDispute_NilCounterparty_SkipsCounterpartyCheck(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	mock.setCallerCount(userID, 1)
	// No counterparty counts configured; passing uuid.Nil should skip the check.
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, uuid.Nil)
	if err != nil {
		t.Fatalf("expected nil (counterparty skipped for uuid.Nil), got %v", err)
	}
}

func TestCheckUserBeforeDispute_FrequencyBlocksBeforeCounterparty(t *testing.T) {
	// Frequency check runs first; counterparty error should not be returned.
	mock := newMockAbuseRepo()
	userID := uuid.New()
	counterpartyID := uuid.New()
	mock.setCallerCount(userID, MaxDisputesPer30Days+1)
	mock.setCounterpartyCount(userID, counterpartyID, MaxDisputesWithSameCounterparty+1)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, counterpartyID)
	var freqErr *ErrHighDisputeFrequency
	if !errors.As(err, &freqErr) {
		t.Fatalf("expected ErrHighDisputeFrequency to take priority, got %T: %v", err, err)
	}
}

func TestCheckUserBeforeDispute_QueryError_FailsClosed(t *testing.T) {
	mock := newMockAbuseRepo()
	mock.forceErr = errors.New("db: connection refused")
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error (fail-closed on DB error), got nil")
	}
}

func TestCheckUserBeforeDispute_BothBelowThresholds_Allowed(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	counterpartyID := uuid.New()
	mock.setCallerCount(userID, MaxDisputesPer30Days-1)
	mock.setCounterpartyCount(userID, counterpartyID, MaxDisputesWithSameCounterparty-1)
	svc := newAbuseServiceWithMock(mock)

	err := svc.CheckUserBeforeDispute(context.Background(), nil, userID, counterpartyID)
	if err != nil {
		t.Fatalf("expected nil (both below threshold), got %v", err)
	}
}

// =============================================================================
// GetUserDisputeStats tests
// =============================================================================

func TestGetUserDisputeStats_ZeroDisputes(t *testing.T) {
	mock := newMockAbuseRepo()
	svc := newAbuseServiceWithMock(mock)

	stats, err := svc.GetUserDisputeStats(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalDisputes != 0 || stats.DisputesLast30Days != 0 || stats.DisputesLast90Days != 0 {
		t.Errorf("expected all zero counts, got %+v", stats)
	}
}

func TestGetUserDisputeStats_ReturnsCounts(t *testing.T) {
	mock := newMockAbuseRepo()
	userID := uuid.New()
	// mock returns same count for all time windows for simplicity
	mock.setCallerCount(userID, 7)
	svc := newAbuseServiceWithMock(mock)

	stats, err := svc.GetUserDisputeStats(context.Background(), nil, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.DisputesLast30Days != 7 {
		t.Errorf("expected DisputesLast30Days=7, got %d", stats.DisputesLast30Days)
	}
	if stats.DisputesLast90Days != 7 {
		t.Errorf("expected DisputesLast90Days=7, got %d", stats.DisputesLast90Days)
	}
	if stats.TotalDisputes != 7 {
		t.Errorf("expected TotalDisputes=7, got %d", stats.TotalDisputes)
	}
}

func TestGetUserDisputeStats_RateAlwaysZero_Deferred(t *testing.T) {
	// Rate check is explicitly deferred; must always return 0.0.
	mock := newMockAbuseRepo()
	userID := uuid.New()
	mock.setCallerCount(userID, 10)
	svc := newAbuseServiceWithMock(mock)

	stats, err := svc.GetUserDisputeStats(context.Background(), nil, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.DisputeRate != 0.0 {
		t.Errorf("DisputeRate should be 0.0 (deferred), got %f", stats.DisputeRate)
	}
}

func TestGetUserDisputeStats_QueryError_ReturnsError(t *testing.T) {
	mock := newMockAbuseRepo()
	mock.forceErr = errors.New("db: timeout")
	svc := newAbuseServiceWithMock(mock)

	_, err := svc.GetUserDisputeStats(context.Background(), nil, uuid.New())
	if err == nil {
		t.Fatal("expected error from query failure, got nil")
	}
}


