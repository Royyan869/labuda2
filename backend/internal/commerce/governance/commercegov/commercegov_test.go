package commercegov

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
)

// stubTx is a minimal db.Tx that fails on any SQL use (the repository is
// faked in these tests, so no SQL is executed).
type stubTx struct{}

func (stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (stubTx) QueryRow(context.Context, string, ...any) pgx.Row         { return nil }
func (stubTx) Commit(context.Context) error                              { return nil }
func (stubTx) Rollback(context.Context) error                            { return nil }

var _ db.Tx = stubTx{}

// fakeRepo is an in-memory commercegov.Repository.
type fakeRepo struct {
	violations  []*Violation
	restriction *Restriction
}

func (f *fakeRepo) InsertViolation(_ context.Context, _ db.Tx, v *Violation) error {
	f.violations = append(f.violations, v)
	return nil
}

func (f *fakeRepo) GetRestrictionForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*Restriction, error) {
	return f.restriction, nil
}

func (f *fakeRepo) UpsertRestriction(_ context.Context, _ db.Tx, r *Restriction) error {
	f.restriction = r
	return nil
}

var _ Repository = (*fakeRepo)(nil)

func recordOnce(t *testing.T, repo Repository, userID uuid.UUID) *Restriction {
	t.Helper()
	_, res, err := RecordViolationAndRestrict(context.Background(), stubTx{}, repo, RecordInput{
		UserID:        userID,
		ViolationType: ViolationBuyerShippingTimeout,
		SourceType:    SourceTypeAuction,
		SourceID:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}
	return res
}

func TestRestrictionStacking_FirstViolation_7Days(t *testing.T) {
	repo := &fakeRepo{}
	userID := uuid.New()
	before := time.Now()

	res := recordOnce(t, repo, userID)

	if res.ViolationCount != 1 {
		t.Fatalf("violation_count = %d, want 1", res.ViolationCount)
	}
	wantMin := before.Add(7 * 24 * time.Hour)
	if res.RestrictedUntil.Before(wantMin) {
		t.Fatalf("restricted_until = %v, want >= %v (7d from now)", res.RestrictedUntil, wantMin)
	}
	if len(repo.violations) != 1 {
		t.Fatalf("violations = %d, want 1", len(repo.violations))
	}
}

func TestRestrictionStacking_SecondViolation_15Days(t *testing.T) {
	repo := &fakeRepo{}
	userID := uuid.New()

	first := recordOnce(t, repo, userID)
	second := recordOnce(t, repo, userID)

	if second.ViolationCount != 2 {
		t.Fatalf("violation_count = %d, want 2", second.ViolationCount)
	}
	// EXTEND semantics: second deadline = first deadline + 15d.
	want := first.RestrictedUntil.Add(15 * 24 * time.Hour)
	if !second.RestrictedUntil.Equal(want) {
		t.Fatalf("restricted_until = %v, want %v (EXTEND from current + 15d)", second.RestrictedUntil, want)
	}
}

func TestRestrictionStacking_ThirdViolation_30Days(t *testing.T) {
	repo := &fakeRepo{}
	userID := uuid.New()

	first := recordOnce(t, repo, userID)
	second := recordOnce(t, repo, userID)
	third := recordOnce(t, repo, userID)

	if first.ViolationCount != 1 || second.ViolationCount != 2 || third.ViolationCount != 3 {
		t.Fatalf("counts = %d,%d,%d, want 1,2,3", first.ViolationCount, second.ViolationCount, third.ViolationCount)
	}
	// EXTEND: 2nd = 1st + 15d; 3rd = 2nd + 30d.
	wantSecond := first.RestrictedUntil.Add(15 * 24 * time.Hour)
	if !second.RestrictedUntil.Equal(wantSecond) {
		t.Fatalf("2nd restricted_until = %v, want %v", second.RestrictedUntil, wantSecond)
	}
	wantThird := second.RestrictedUntil.Add(30 * 24 * time.Hour)
	if !third.RestrictedUntil.Equal(wantThird) {
		t.Fatalf("3rd restricted_until = %v, want %v (EXTEND + 30d)", third.RestrictedUntil, wantThird)
	}
}

func TestRestriction_FromExpiredRestriction_StartsFresh(t *testing.T) {
	repo := &fakeRepo{}
	userID := uuid.New()

	// Existing restriction already expired: stacking must start from now
	// (not extend the past deadline), but the count still increments.
	repo.restriction = &Restriction{
		ID:              uuid.New(),
		UserID:          userID,
		ViolationCount:  1,
		RestrictedUntil: time.Now().Add(-24 * time.Hour),
		LastViolationID: uuid.New(),
	}

	before := time.Now()
	res := recordOnce(t, repo, userID)

	if res.ViolationCount != 2 {
		t.Fatalf("violation_count = %d, want 2", res.ViolationCount)
	}
	wantMin := before.Add(15 * 24 * time.Hour)
	if res.RestrictedUntil.Before(wantMin) {
		t.Fatalf("restricted_until = %v, want >= %v (fresh 15d from now)", res.RestrictedUntil, wantMin)
	}
}

func TestIsUserRestricted(t *testing.T) {
	ctx := context.Background()

	t.Run("no restriction row -> not restricted", func(t *testing.T) {
		repo := &fakeRepo{}
		ok, until, err := IsUserRestricted(ctx, stubTx{}, repo, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		if ok || until != nil {
			t.Fatalf("expected unrestricted, got ok=%v until=%v", ok, until)
		}
	})

	t.Run("active restriction -> restricted", func(t *testing.T) {
		repo := &fakeRepo{restriction: &Restriction{
			ID:              uuid.New(),
			UserID:          uuid.New(),
			ViolationCount:  1,
			RestrictedUntil: time.Now().Add(7 * 24 * time.Hour),
			LastViolationID: uuid.New(),
		}}
		ok, until, err := IsUserRestricted(ctx, stubTx{}, repo, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		if !ok || until == nil {
			t.Fatalf("expected restricted, got ok=%v until=%v", ok, until)
		}
	})
}

func TestViolationType_CanonicalTaxonomy(t *testing.T) {
	cases := map[ViolationType]bool{
		ViolationBuyerShippingTimeout:  true,
		ViolationSellerShippingDefault: true,
		ViolationBuyerBNR:              true,
		ViolationType("made_up"):       false,
	}
	for v, want := range cases {
		if got := v.IsValid(); got != want {
			t.Errorf("IsValid(%q) = %v, want %v", v, got, want)
		}
	}
}
