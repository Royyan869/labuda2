package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type analyticsRepoFixture struct {
	impressionsTotal   int
	clicksTotal        int
	feedImpressions    int
	feedClicks         int
	searchImpressions  int
	searchClicks       int
	exploreImpressions int
	exploreClicks      int
}

type analyticsRepoTx struct {
	fixture   analyticsRepoFixture
	lastQuery string
	lastArgs  []any
}

var _ db.Tx = (*analyticsRepoTx)(nil)

func (t *analyticsRepoTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 0"), nil
}

func (t *analyticsRepoTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.lastQuery = sql
	t.lastArgs = append([]any{}, args...)

	normalized := strings.ToLower(sql)
	if strings.Contains(normalized, "from promotion_events") {
		return &analyticsRepoRow{values: []any{
			t.fixture.impressionsTotal,
			t.fixture.clicksTotal,
			t.fixture.feedImpressions,
			t.fixture.feedClicks,
			t.fixture.searchImpressions,
			t.fixture.searchClicks,
			t.fixture.exploreImpressions,
			t.fixture.exploreClicks,
		}}
	}

	return &analyticsRepoRow{err: fmt.Errorf("unexpected query: %s", sql)}
}

func (t *analyticsRepoTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &analyticsRepoRows{}, nil
}

func (t *analyticsRepoTx) Commit(context.Context) error   { return nil }
func (t *analyticsRepoTx) Rollback(context.Context) error { return nil }

type analyticsRepoRow struct {
	values []any
	err    error
}

func (r *analyticsRepoRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignAnalyticsRepoValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type analyticsRepoRows struct{}

func (r *analyticsRepoRows) Next() bool                                   { return false }
func (r *analyticsRepoRows) Err() error                                   { return nil }
func (r *analyticsRepoRows) Close()                                       {}
func (r *analyticsRepoRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *analyticsRepoRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *analyticsRepoRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *analyticsRepoRows) RawValues() [][]byte                          { return nil }
func (r *analyticsRepoRows) Values() ([]any, error)                       { return nil, nil }
func (r *analyticsRepoRows) Scan(dest ...any) error                       { return errors.New("no rows") }
func (r *analyticsRepoRows) Conn() *pgx.Conn                              { return nil }

func assignAnalyticsRepoValue(dest any, value any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer, got %T", dest)
	}

	target := rv.Elem()
	vv := reflect.ValueOf(value)
	if vv.Type().AssignableTo(target.Type()) {
		target.Set(vv)
		return nil
	}
	if vv.Type().ConvertibleTo(target.Type()) {
		target.Set(vv.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("unsupported scan conversion from %T to %T", value, dest)
}

func TestGetCampaignAnalytics_ZeroEvents(t *testing.T) {
	repo := NewPromotionEventRepository()
	tx := &analyticsRepoTx{}
	instanceID := uuid.New()

	summary, err := repo.GetCampaignAnalytics(context.Background(), tx, instanceID, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, instanceID, summary.InstanceID)
	assert.Equal(t, 0, summary.ImpressionsTotal)
	assert.Equal(t, 0, summary.ClicksTotal)
	assert.Equal(t, 0.0, summary.CTR)
	assert.Equal(t, 0, summary.FeedImpressions)
	assert.Equal(t, 0, summary.FeedClicks)
	assert.Equal(t, 0, summary.SearchImpressions)
	assert.Equal(t, 0, summary.SearchClicks)
	assert.Equal(t, 0, summary.ExploreImpressions)
	assert.Equal(t, 0, summary.ExploreClicks)
	assert.Contains(t, strings.ToLower(tx.lastQuery), "from promotion_events")
	assert.Contains(t, strings.ToLower(tx.lastQuery), "count(*) filter")
}

func TestGetCampaignAnalytics_AggregatesCountsAndBreakdown(t *testing.T) {
	repo := NewPromotionEventRepository()
	tx := &analyticsRepoTx{
		fixture: analyticsRepoFixture{
			impressionsTotal:   12,
			clicksTotal:        3,
			feedImpressions:    7,
			feedClicks:         2,
			searchImpressions:  4,
			searchClicks:       1,
			exploreImpressions: 1,
			exploreClicks:      0,
		},
	}
	instanceID := uuid.New()
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	summary, err := repo.GetCampaignAnalytics(context.Background(), tx, instanceID, &from, &to)

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 12, summary.ImpressionsTotal)
	assert.Equal(t, 3, summary.ClicksTotal)
	assert.InDelta(t, 0.25, summary.CTR, 0.0001)
	assert.Equal(t, 7, summary.FeedImpressions)
	assert.Equal(t, 2, summary.FeedClicks)
	assert.Equal(t, 4, summary.SearchImpressions)
	assert.Equal(t, 1, summary.SearchClicks)
	assert.Equal(t, 1, summary.ExploreImpressions)
	assert.Equal(t, 0, summary.ExploreClicks)
	require.NotNil(t, summary.WindowFrom)
	require.NotNil(t, summary.WindowTo)
	assert.True(t, summary.WindowFrom.Equal(from))
	assert.True(t, summary.WindowTo.Equal(to))
	assert.Len(t, tx.lastArgs, 3)
	assert.Equal(t, instanceID, tx.lastArgs[0])
}
