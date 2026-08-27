package application

// Idempotency contract tests for auction bid placement.
//
// These tests verify three layers of the bidder-scoped idempotency guarantee:
//   1. Service entry guard: empty idempotency key is rejected immediately.
//   2. Same-bidder replay: if a bid already exists for (auction, bidder, key),
//      the service returns it without re-inserting (idempotent 201).
//   3. Schema contract: migration 000214 enforces the constraint at DB level on
//      (auction_id, bidder_id, idempotency_key), preventing cross-bidder collisions.
//
// Cross-bidder entity-level independence is covered in entity/auction_bid_test.go
// (TestCrossBidderSameKeyAreIndependent).

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ── fake pgx.Row implementations ─────────────────────────────────────────────

// bidRowNotFound simulates "no rows in result set" from QueryRow.Scan.
type bidRowNotFound struct{}

func (bidRowNotFound) Scan(dest ...any) error {
	return fmt.Errorf("no rows in result set")
}

var _ pgx.Row = bidRowNotFound{}

// bidRowFound simulates a QueryRow result that populates (id, amount, created_at)
// — the three columns scanned by GetByAuctionAndIdempotencyKey.
type bidRowFound struct {
	id        uuid.UUID
	amount    int64
	createdAt time.Time
}

func (r *bidRowFound) Scan(dest ...any) error {
	if len(dest) < 3 {
		return fmt.Errorf("expected 3 dest columns, got %d", len(dest))
	}
	*dest[0].(*uuid.UUID) = r.id
	*dest[1].(*int64) = r.amount
	*dest[2].(*time.Time) = r.createdAt
	return nil
}

var _ pgx.Row = (*bidRowFound)(nil)

// ── spy tx ────────────────────────────────────────────────────────────────────

// idempotencySpyTx intercepts QueryRow calls and returns a controlled pgx.Row.
// All other db.Tx methods delegate to fakeTx (no-op / panic-safe for this test).
type idempotencySpyTx struct {
	row pgx.Row
}

func (t *idempotencySpyTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return t.row
}

func (t *idempotencySpyTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *idempotencySpyTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *idempotencySpyTx) Commit(_ context.Context) error   { return nil }
func (t *idempotencySpyTx) Rollback(_ context.Context) error { return nil }

var _ db.Tx = (*idempotencySpyTx)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func newMinimalAuctionService() *AuctionService {
	return &AuctionService{
		accountStatus: noopAccountStatusChecker{},
		auctionRepo:   &auctionRepo.AuctionRepository{},
		bidRepo:       &auctionRepo.AuctionBidRepository{},
		outboxRepo:    &outboxRepo.OutboxRepository{},
		configService: &platformconfigApp.ConfigService{},
		roleChecker:   noopRoleChecker{},
		log:           zap.NewNop(),
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestPlaceBid_RequiresIdempotencyKey verifies the service rejects an empty
// idempotency key before touching the DB. This is the first guard at line 553
// of auction_service.go.
func TestPlaceBid_RequiresIdempotencyKey(t *testing.T) {
	svc := newMinimalAuctionService()

	_, err := svc.PlaceBid(context.Background(), &idempotencySpyTx{row: bidRowNotFound{}}, PlaceBidInput{
		AuctionID:      uuid.New(),
		BidderID:       uuid.New(),
		Amount:         10_000,
		IdempotencyKey: "",
	})

	if err == nil {
		t.Fatal("expected error for empty idempotency key, got nil")
	}
	if err.Error() != "idempotency key is required" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// TestPlaceBid_SameBidderReplay_ReturnsExistingBid verifies that when the
// repository lookup finds an existing bid for (auction, bidder, key), the
// service returns it immediately without attempting a second INSERT.
//
// This is the idempotent replay path at lines 581–591 of auction_service.go.
// The spy tx makes QueryRow return a populated row, simulating the DB finding
// bidder A's previous bid.
func TestPlaceBid_SameBidderReplay_ReturnsExistingBid(t *testing.T) {
	svc := newMinimalAuctionService()

	auctionID := uuid.New()
	bidderID := uuid.New()
	existingBidID := uuid.New()
	existingAmount := int64(15_000)
	existingCreatedAt := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	idempotencyKey := "client-retry-key-xyz"

	spyTx := &idempotencySpyTx{
		row: &bidRowFound{
			id:        existingBidID,
			amount:    existingAmount,
			createdAt: existingCreatedAt,
		},
	}

	got, err := svc.PlaceBid(context.Background(), spyTx, PlaceBidInput{
		AuctionID:      auctionID,
		BidderID:       bidderID,
		Amount:         existingAmount,
		IdempotencyKey: idempotencyKey,
	})

	if err != nil {
		t.Fatalf("replay should not return error; got: %v", err)
	}
	if got == nil {
		t.Fatal("replay must return the existing bid, got nil")
	}
	if got.ID != existingBidID {
		t.Fatalf("replay bid ID: want %s, got %s", existingBidID, got.ID)
	}
	if got.Amount != existingAmount {
		t.Fatalf("replay bid amount: want %d, got %d", existingAmount, got.Amount)
	}
	if got.AuctionID != auctionID {
		t.Fatalf("replay bid AuctionID: want %s, got %s", auctionID, got.AuctionID)
	}
	if got.BidderID != bidderID {
		t.Fatalf("replay bid BidderID: want %s, got %s", bidderID, got.BidderID)
	}
	if got.IdempotencyKey != idempotencyKey {
		t.Fatalf("replay bid IdempotencyKey: want %s, got %s", idempotencyKey, got.IdempotencyKey)
	}
}

// TestAuctionBidIdempotency_SchemaIsBidderScoped reads the canonical schema and
// asserts the UNIQUE constraint on auction_bids includes bidder_id.
//
// This test is the schema contract guard: it fails if the canonical schema is
// edited to drop bidder_id from the constraint, which would reintroduce the P1
// cross-bidder collision originally fixed in migration 000214.
func TestAuctionBidIdempotency_SchemaIsBidderScoped(t *testing.T) {
	canonicalPath := "../../../../migrations/000001_canonical_schema.up.sql"
	content, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("canonical schema not found at %s: %v", canonicalPath, err)
	}

	sql := string(content)

	// The constraint must reference all three columns.
	for _, col := range []string{"auction_id", "bidder_id", "idempotency_key"} {
		if !strings.Contains(sql, col) {
			t.Errorf("canonical schema auction_bids UNIQUE constraint must include %q but it is absent", col)
		}
	}

	// The bidder-scoped constraint name must be present.
	const wantConstraint = "auction_bids_auction_id_bidder_id_idempotency_key_key"
	if !strings.Contains(sql, wantConstraint) {
		t.Errorf("canonical schema must define constraint %q, not found in SQL", wantConstraint)
	}
}
