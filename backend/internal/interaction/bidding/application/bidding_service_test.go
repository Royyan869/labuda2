package application

// Focused proof for the canonical My Bids view (GET /api/v1/bidding).
//
// Business truth (Owner-final):
//   - My Bids = auctions with an OPEN bidding process for the user:
//     active + waiting_settlement. Ended/cancelled/lapsed/scheduled hidden.
//   - "Bid saya" (YourLastBid) = user's LATEST bid by time, not highest.
//   - Wire contract is snake_case; envelope is {items, active_count}.
//
// These tests use a fake db.Tx (no database) and prove service-level
// behavior. Real-SQL behavior (ORDER BY execution, FK retention) is proven
// by the runtime probe against a live database, not by these tests.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/hishumi/backend/internal/commerce/auction/entity"
	"github.com/hishumi/backend/pkg/db"
)

// ── fixtures ────────────────────────────────────────────────────────────────

type auctionFixture struct {
	id         uuid.UUID
	title      string
	status     entity.Status
	endAt      time.Time
	currentBid *int64
	winner     *uuid.UUID
}

type bidFixture struct {
	amount    int64
	createdAt time.Time
}

// ── fake pgx.Row ────────────────────────────────────────────────────────────

type errRow struct{ msg string }

func (r errRow) Scan(...any) error { return &fakeErr{s: r.msg} }

type fakeErr struct{ s string }

func (e *fakeErr) Error() string { return e.s }

type bidRow struct {
	id        uuid.UUID
	auctionID uuid.UUID
	bidderID  uuid.UUID
	stub      *bidFixture
}

func (r *bidRow) Scan(dest ...any) error {
	if len(dest) != 6 {
		panic("bidRow: expected 6 dest columns")
	}
	*dest[0].(*uuid.UUID) = r.id
	*dest[1].(*uuid.UUID) = r.auctionID
	*dest[2].(*uuid.UUID) = r.bidderID
	*dest[3].(*int64) = r.stub.amount
	*dest[4].(*string) = "test-key"
	*dest[5].(*time.Time) = r.stub.createdAt
	return nil
}

var _ pgx.Row = (*bidRow)(nil)

// auctionRow fakes one joinedAuctionColumns row. Column order must match
// scanJoinedAuction in auction_repository.go; any drift fails loudly here
// instead of silently proving the wrong thing.
type auctionRow struct{ stub *auctionFixture }

func (r *auctionRow) Scan(dest ...any) error {
	if len(dest) != 33 {
		panic("auctionRow: expected 33 dest columns (joinedAuctionColumns drift?)")
	}
	now := time.Now().UTC()
	seller := uuid.New()
	product := uuid.New()
	start := r.stub.endAt.Add(-24 * time.Hour)

	*dest[0].(*uuid.UUID) = r.stub.id
	*dest[1].(*uuid.UUID) = seller
	*dest[2].(*uuid.UUID) = product
	*dest[3].(**uuid.UUID) = nil
	*dest[4].(*int64) = 50000
	*dest[5].(*int64) = 10000
	*dest[6].(**int64) = nil
	*dest[7].(*time.Time) = start
	*dest[8].(*time.Time) = r.stub.endAt
	*dest[9].(**int64) = r.stub.currentBid
	*dest[10].(**uuid.UUID) = r.stub.winner
	*dest[11].(**time.Time) = nil
	*dest[12].(*bool) = false
	*dest[13].(*bool) = false
	*dest[14].(*string) = string(r.stub.status)
	*dest[15].(*time.Time) = now
	*dest[16].(*time.Time) = now
	*dest[17].(*int64) = 0
	*dest[18].(*uuid.UUID) = product
	*dest[19].(*uuid.UUID) = seller
	*dest[20].(*string) = r.stub.title
	*dest[21].(*string) = "desc"
	*dest[22].(*json.RawMessage) = nil
	*dest[23].(*string) = ""
	*dest[24].(**int) = nil
	*dest[25].(**int) = nil
	*dest[26].(**string) = nil
	*dest[27].(**string) = nil
	*dest[28].(**string) = nil
	*dest[29].(*[]string) = nil
	*dest[30].(*string) = ""
	*dest[31].(*time.Time) = now
	*dest[32].(*time.Time) = now
	return nil
}

var _ pgx.Row = (*auctionRow)(nil)

// ── fake pgx.Rows for ListAuctionIDsByBidder ────────────────────────────────

type idRows struct {
	ids []uuid.UUID
	pos int
}

func (r *idRows) Close()                                       {}
func (r *idRows) Err() error                                   { return nil }
func (r *idRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *idRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *idRows) Next() bool                                   { r.pos++; return r.pos < len(r.ids) }
func (r *idRows) Scan(dest ...any) error {
	*dest[0].(*uuid.UUID) = r.ids[r.pos]
	return nil
}
func (r *idRows) Values() ([]any, error) { return []any{r.ids[r.pos]}, nil }
func (r *idRows) RawValues() [][]byte    { return nil }
func (r *idRows) Conn() *pgx.Conn        { return nil }

var _ pgx.Rows = (*idRows)(nil)

// ── fake db.Tx ──────────────────────────────────────────────────────────────

type myBidsFakeTx struct {
	ids      []uuid.UUID
	auctions map[uuid.UUID]*auctionFixture
	bids     map[uuid.UUID]*bidFixture

	lastBidSQL  string
	lastBidArgs []any
	listSQL     string
	listArg     any
}

func (t *myBidsFakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *myBidsFakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.listSQL = sql
	if len(args) > 0 {
		t.listArg = args[0]
	}
	return &idRows{ids: t.ids, pos: -1}, nil
}

func (t *myBidsFakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "FROM auction_bids") {
		t.lastBidSQL = sql
		t.lastBidArgs = args
		auctionID := args[1].(uuid.UUID)
		bidderID := args[0].(uuid.UUID)
		b, ok := t.bids[auctionID]
		if !ok {
			return errRow{msg: "no rows in result set"}
		}
		return &bidRow{id: uuid.New(), auctionID: auctionID, bidderID: bidderID, stub: b}
	}
	auctionID := args[0].(uuid.UUID)
	a, ok := t.auctions[auctionID]
	if !ok {
		return errRow{msg: "auction not found"}
	}
	return &auctionRow{stub: a}
}

func (t *myBidsFakeTx) Commit(context.Context) error   { return nil }
func (t *myBidsFakeTx) Rollback(context.Context) error { return nil }

var _ db.Tx = (*myBidsFakeTx)(nil)

// ── helpers ─────────────────────────────────────────────────────────────────

func int64ptr(v int64) *int64 { return &v }

func newServiceUnderTest(tx *myBidsFakeTx) (*BiddingService, db.Tx) {
	return NewBiddingService(), tx
}

// ── tests ───────────────────────────────────────────────────────────────────

// The wire contract is snake_case with exactly these fields; the envelope
// carries items + active_count only (won/lost purged — no history tab).
func TestBiddingItem_JSONContractSnakeCase(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	item := BiddingItem{
		AuctionID:   uuid.New(),
		Title:       "Kohaku 30cm",
		YourLastBid: 120000,
		CurrentBid:  150000,
		Status:      "leading",
		EndAt:       now,
		UpdatedAt:   now,
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal BiddingItem: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{
		"auction_id", "title", "your_last_bid", "current_bid",
		"status", "end_at", "updated_at",
	} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing snake_case key %q in %s", k, raw)
		}
	}
	if len(m) != 7 {
		t.Errorf("expected exactly 7 keys, got %d: %s", len(m), raw)
	}

	res := BiddingResult{Items: []BiddingItem{item}, ActiveCount: 1}
	rawRes, _ := json.Marshal(res)
	var rm map[string]any
	_ = json.Unmarshal(rawRes, &rm)
	if _, ok := rm["items"]; !ok {
		t.Error("envelope missing items")
	}
	if _, ok := rm["active_count"]; !ok {
		t.Error("envelope missing active_count")
	}
	for _, k := range []string{"won_count", "lost_count", "WonCount", "LostCount"} {
		if _, ok := rm[k]; ok {
			t.Errorf("dead envelope key %q must be purged: %s", k, rawRes)
		}
	}
}

// Core business truth: open auctions visible with latest-by-time bid;
// ended/cancelled/lapsed/scheduled hidden.
func TestGetUserBidding_VisibilityAndLatestBid(t *testing.T) {
	ctx := context.Background()
	user := uuid.New()
	other := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	activeLead := &auctionFixture{id: uuid.New(), title: "Active Lead",
		status: entity.StatusActive, endAt: now.Add(2 * time.Hour),
		currentBid: int64ptr(120000), winner: &user}
	activeOutbid := &auctionFixture{id: uuid.New(), title: "Active Outbid",
		status: entity.StatusActive, endAt: now.Add(1 * time.Hour),
		currentBid: int64ptr(200000), winner: &other}
	waiting := &auctionFixture{id: uuid.New(), title: "Waiting Claim",
		status: entity.StatusWaitingSettlement, endAt: now.Add(-1 * time.Hour),
		currentBid: int64ptr(300000), winner: &user}
	ended := &auctionFixture{id: uuid.New(), title: "Ended",
		status: entity.StatusEnded, endAt: now.Add(-2 * time.Hour),
		currentBid: int64ptr(300000), winner: &user}
	cancelled := &auctionFixture{id: uuid.New(), title: "Cancelled",
		status: entity.StatusCancelled, endAt: now.Add(3 * time.Hour)}
	lapsed := &auctionFixture{id: uuid.New(), title: "Lapsed",
		status: entity.StatusLapsed, endAt: now.Add(-3 * time.Hour)}
	scheduled := &auctionFixture{id: uuid.New(), title: "Scheduled",
		status: entity.StatusScheduled, endAt: now.Add(5 * time.Hour)}

	tx := &myBidsFakeTx{
		ids: []uuid.UUID{
			activeLead.id, activeOutbid.id, waiting.id,
			ended.id, cancelled.id, lapsed.id, scheduled.id,
		},
		auctions: map[uuid.UUID]*auctionFixture{
			activeLead.id: activeLead, activeOutbid.id: activeOutbid,
			waiting.id: waiting, ended.id: ended, cancelled.id: cancelled,
			lapsed.id: lapsed, scheduled.id: scheduled,
		},
		// Latest-by-time fixtures (e.g. 100k → 150k → 120k ⇒ 120k).
		bids: map[uuid.UUID]*bidFixture{
			activeLead.id:   {amount: 120000, createdAt: now.Add(-10 * time.Minute)},
			activeOutbid.id: {amount: 180000, createdAt: now.Add(-20 * time.Minute)},
			waiting.id:      {amount: 300000, createdAt: now.Add(-30 * time.Minute)},
			ended.id:        {amount: 300000, createdAt: now.Add(-40 * time.Minute)},
			cancelled.id:    {amount: 50000, createdAt: now.Add(-50 * time.Minute)},
			lapsed.id:       {amount: 60000, createdAt: now.Add(-60 * time.Minute)},
			scheduled.id:    {amount: 70000, createdAt: now.Add(-70 * time.Minute)},
		},
	}

	svc, _ := newServiceUnderTest(tx)
	res, err := svc.GetUserBidding(ctx, tx, user)
	if err != nil {
		t.Fatalf("GetUserBidding: %v", err)
	}

	if len(res.Items) != 3 {
		t.Fatalf("expected 3 visible items (2 active + waiting), got %d", len(res.Items))
	}
	if res.ActiveCount != 3 {
		t.Errorf("ActiveCount = %d, want 3", res.ActiveCount)
	}

	byID := map[uuid.UUID]BiddingItem{}
	for _, it := range res.Items {
		byID[it.AuctionID] = it
	}
	for _, hidden := range []uuid.UUID{ended.id, cancelled.id, lapsed.id, scheduled.id} {
		if _, ok := byID[hidden]; ok {
			t.Errorf("non-open auction %s must be hidden from My Bids", hidden)
		}
	}

	lead := byID[activeLead.id]
	if lead.Status != "leading" {
		t.Errorf("active+winner status = %q, want leading", lead.Status)
	}
	if lead.YourLastBid != 120000 {
		t.Errorf("YourLastBid = %d, want latest-by-time 120000", lead.YourLastBid)
	}
	if lead.Title != "Active Lead" || lead.CurrentBid != 120000 {
		t.Errorf("unexpected item mapping: %+v", lead)
	}
	if lead.EndAt.Unix() != activeLead.endAt.Unix() {
		t.Error("EndAt must be canonical auction.end_at")
	}

	if got := byID[activeOutbid.id].Status; got != "outbid" {
		t.Errorf("active+non-winner status = %q, want outbid", got)
	}
	if got := byID[waiting.id].Status; got != "waiting_claim" {
		t.Errorf("waiting_settlement+winner status = %q, want waiting_claim", got)
	}

	// Ordering: open items by EndAt ASC (soonest first). The waiting-claim
	// item already passed end_at so it sorts first — deterministic.
	if res.Items[0].AuctionID != waiting.id ||
		res.Items[1].AuctionID != activeOutbid.id ||
		res.Items[2].AuctionID != activeLead.id {
		t.Errorf("ordering must be EndAt ASC, got %v %v %v",
			res.Items[0].Title, res.Items[1].Title, res.Items[2].Title)
	}
}

// The latest-bid query must order by time, never by amount.
func TestGetUserLastBidForAuction_OrdersByTimeDesc(t *testing.T) {
	ctx := context.Background()
	user, auction := uuid.New(), uuid.New()
	tx := &myBidsFakeTx{
		bids: map[uuid.UUID]*bidFixture{
			auction: {amount: 120000, createdAt: time.Now().UTC()},
		},
	}
	repo := NewBiddingService().bidRepo
	got, err := repo.GetUserLastBidForAuction(ctx, tx, user, auction)
	if err != nil || got == nil {
		t.Fatalf("GetUserLastBidForAuction: %v %v", got, err)
	}
	sql := tx.lastBidSQL
	if !strings.Contains(sql, "ORDER BY created_at DESC") {
		t.Errorf("bid selection must ORDER BY created_at DESC, got:\n%s", sql)
	}
	if strings.Contains(sql, "ORDER BY amount") {
		t.Errorf("bid selection must not order by amount:\n%s", sql)
	}
	if len(tx.lastBidArgs) != 2 || tx.lastBidArgs[0] != user || tx.lastBidArgs[1] != auction {
		t.Errorf("bidder/auction scoping broken: %v", tx.lastBidArgs)
	}
}

// Bidder isolation is enforced by the query scope.
func TestGetUserBidding_BidderIsolation(t *testing.T) {
	ctx := context.Background()
	user := uuid.New()
	tx := &myBidsFakeTx{}
	svc, _ := newServiceUnderTest(tx)
	res, err := svc.GetUserBidding(ctx, tx, user)
	if err != nil {
		t.Fatalf("GetUserBidding: %v", err)
	}
	if len(res.Items) != 0 || res.ActiveCount != 0 {
		t.Errorf("empty bidder must yield valid empty result, got %+v", res)
	}
	if !strings.Contains(tx.listSQL, "WHERE bidder_id = $1") {
		t.Errorf("auction listing must scope by bidder_id:\n%s", tx.listSQL)
	}
	if tx.listArg != user {
		t.Errorf("bidder scope arg = %v, want %v", tx.listArg, user)
	}
}

// Auctions that vanished (deleted) are skipped, never fabricated.
func TestGetUserBidding_SkipsUnavailableAuction(t *testing.T) {
	ctx := context.Background()
	user := uuid.New()
	ghost := uuid.New()
	tx := &myBidsFakeTx{
		ids:      []uuid.UUID{ghost},
		auctions: map[uuid.UUID]*auctionFixture{},
		bids:     map[uuid.UUID]*bidFixture{},
	}
	svc, _ := newServiceUnderTest(tx)
	res, err := svc.GetUserBidding(ctx, tx, user)
	if err != nil {
		t.Fatalf("GetUserBidding: %v", err)
	}
	if len(res.Items) != 0 {
		t.Errorf("unavailable auction must not produce a fabricated item: %+v", res.Items)
	}
}
