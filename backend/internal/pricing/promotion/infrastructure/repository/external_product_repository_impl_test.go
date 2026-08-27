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
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externalProductRepoTx struct {
	now     time.Time
	product *entity.ExternalProduct
	media   []*entity.ExternalProductMedia

	execSQLs  []string
	querySQLs []string
	lastExec  string
	lastQuery string
}

var _ db.Tx = (*externalProductRepoTx)(nil)

func (t *externalProductRepoTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.lastExec = sql
	t.execSQLs = append(t.execSQLs, sql)
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (t *externalProductRepoTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.lastQuery = sql
	t.querySQLs = append(t.querySQLs, sql)

	normalized := strings.ToLower(sql)
	switch {
	case strings.Contains(normalized, "select now()"):
		return &externalProductRepoRow{values: []any{t.now}}
	case strings.Contains(normalized, "from external_products"):
		if t.product == nil {
			return &externalProductRepoRow{err: pgx.ErrNoRows}
		}
		return &externalProductRepoRow{values: externalProductRowValues(t.product)}
	default:
		return &externalProductRepoRow{err: fmt.Errorf("unexpected query: %s", sql)}
	}
}

func (t *externalProductRepoTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.lastQuery = sql
	t.querySQLs = append(t.querySQLs, sql)
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "from external_products"):
		if t.product == nil {
			return &externalProductRepoRows{}, nil
		}
		return &externalProductRepoRows{rows: [][]any{externalProductRowValues(t.product)}}, nil
	case strings.Contains(normalized, "from external_product_media"):
		rows := make([][]any, 0, len(t.media))
		for _, item := range t.media {
			rows = append(rows, externalProductMediaRowValues(item))
		}
		return &externalProductRepoRows{rows: rows}, nil
	default:
		return &externalProductRepoRows{}, nil
	}
}

func (t *externalProductRepoTx) Commit(context.Context) error   { return nil }
func (t *externalProductRepoTx) Rollback(context.Context) error { return nil }

type externalProductRepoRow struct {
	values []any
	err    error
}

func (r *externalProductRepoRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignExternalProductRepoValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type externalProductRepoRows struct {
	rows [][]any
	idx  int
}

func (r *externalProductRepoRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *externalProductRepoRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("no current row")
	}
	row := r.rows[r.idx-1]
	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(row), len(dest))
	}
	for i, value := range row {
		if err := assignExternalProductRepoValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

func (r *externalProductRepoRows) Err() error                                   { return nil }
func (r *externalProductRepoRows) Close()                                       {}
func (r *externalProductRepoRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *externalProductRepoRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *externalProductRepoRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *externalProductRepoRows) RawValues() [][]byte                          { return nil }
func (r *externalProductRepoRows) Values() ([]any, error)                       { return nil, nil }
func (r *externalProductRepoRows) Conn() *pgx.Conn                              { return nil }

func assignExternalProductRepoValue(dest any, value any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer, got %T", dest)
	}

	target := rv.Elem()
	if value == nil {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}

	vv := reflect.ValueOf(value)
	if vv.Type().AssignableTo(target.Type()) {
		target.Set(vv)
		return nil
	}

	if target.Kind() == reflect.Ptr {
		elemType := target.Type().Elem()
		if vv.Type().AssignableTo(elemType) {
			ptr := reflect.New(elemType)
			ptr.Elem().Set(vv)
			target.Set(ptr)
			return nil
		}
		if vv.Type().ConvertibleTo(elemType) {
			ptr := reflect.New(elemType)
			ptr.Elem().Set(vv.Convert(elemType))
			target.Set(ptr)
			return nil
		}
	}

	if vv.Type().ConvertibleTo(target.Type()) {
		target.Set(vv.Convert(target.Type()))
		return nil
	}

	return fmt.Errorf("unsupported scan conversion from %T to %T", value, dest)
}

func externalProductRowValues(product *entity.ExternalProduct) []any {
	return []any{
		product.ID,
		product.OwnerUserID,
		product.Title,
		product.Description,
		product.ExternalURL,
		product.NormalizedExternalURL,
		string(product.ReviewStatus),
		product.RejectionReason,
		product.UnsafeURLFlag,
		product.SubmittedAt,
		product.ApprovedAt,
		product.RejectedAt,
		product.HiddenAt,
		product.LastReviewedBy,
		product.CreatedAt,
		product.UpdatedAt,
		product.DeletedAt,
	}
}

func externalProductMediaRowValues(media *entity.ExternalProductMedia) []any {
	return []any{
		media.ID,
		media.ExternalProductID,
		string(media.MediaType),
		media.StorageKey,
		media.URL,
		media.ThumbnailURL,
		media.SortOrder,
		media.Metadata,
		media.CreatedAt,
		media.DeletedAt,
	}
}

func newTestExternalProduct(now time.Time) *entity.ExternalProduct {
	ownerID := uuid.New()
	product, err := entity.NewExternalProductDraft(ownerID, "Product", nil, "https://example.com/product", now)
	if err != nil {
		panic(err)
	}
	return product
}

func TestExternalProductRepository_CreateDraft(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()
	product, err := entity.NewExternalProductDraft(uuid.New(), "Fish", nil, "https://example.com/fish", now)
	require.NoError(t, err)
	tx := &externalProductRepoTx{now: now}

	err = repo.CreateDraft(context.Background(), tx, product)
	require.NoError(t, err)
	require.NotEmpty(t, tx.execSQLs)
	assert.Contains(t, strings.ToLower(tx.execSQLs[0]), "insert into external_products")
	assert.Equal(t, entity.ExternalProductReviewStatusDraft, product.ReviewStatus)
}

func TestExternalProductRepository_SubmitAndResubmit(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()

	ownerID := uuid.New()
	product := newTestExternalProduct(now)
	product.OwnerUserID = ownerID

	tx := &externalProductRepoTx{now: now, product: product}
	submitted, err := repo.SubmitOwned(context.Background(), tx, ownerID, product.ID)
	require.NoError(t, err)
	require.NotNil(t, submitted)
	assert.Equal(t, entity.ExternalProductReviewStatusPendingReview, submitted.ReviewStatus)
	assert.True(t, containsSQL(tx.execSQLs, "external_product_review_history"))

	submitted.ReviewStatus = entity.ExternalProductReviewStatusRejected
	tx = &externalProductRepoTx{now: now, product: submitted}
	resubmitted, err := repo.ResubmitOwned(context.Background(), tx, ownerID, submitted.ID)
	require.NoError(t, err)
	require.NotNil(t, resubmitted)
	assert.Equal(t, entity.ExternalProductReviewStatusPendingReview, resubmitted.ReviewStatus)
	assert.True(t, containsSQL(tx.execSQLs, "external_product_review_history"))
}

func TestExternalProductRepository_UpdateApprovedReturnsPendingReview(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()

	ownerID := uuid.New()
	product := newTestExternalProduct(now)
	product.OwnerUserID = ownerID
	product.ReviewStatus = entity.ExternalProductReviewStatusApproved
	tx := &externalProductRepoTx{now: now, product: product}

	updated, err := repo.UpdateOwned(context.Background(), tx, ownerID, product.ID, entity.ExternalProductUpdateInput{
		Title: ptrRepoString("Updated Title"),
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, entity.ExternalProductReviewStatusPendingReview, updated.ReviewStatus)
	assert.True(t, containsSQL(tx.execSQLs, "external_product_review_history"))
}

func TestExternalProductRepository_GetAndListOwnedExcludeDeleted(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()

	ownerID := uuid.New()
	product := newTestExternalProduct(now)
	product.OwnerUserID = ownerID
	tx := &externalProductRepoTx{now: now, product: product}

	got, err := repo.GetOwnedByID(context.Background(), tx, ownerID, product.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, product.ID, got.ID)

	listed, err := repo.ListOwned(context.Background(), tx, ownerID, ExternalProductListFilters{Limit: 10})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, product.ID, listed[0].ID)
}

func TestExternalProductRepository_AddAndListMedia(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()

	productID := uuid.New()
	media, err := entity.NewExternalProductMedia(productID, entity.ExternalProductMediaTypeImage, "s3://bucket/key", "https://cdn.example.com/key.jpg", nil, 0, nil, now)
	require.NoError(t, err)
	tx := &externalProductRepoTx{now: now, media: []*entity.ExternalProductMedia{media}}

	require.NoError(t, repo.AddMedia(context.Background(), tx, media))
	assert.True(t, containsSQL(tx.execSQLs, "external_product_media"))

	listed, err := repo.ListMedia(context.Background(), tx, productID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, media.ID, listed[0].ID)
}

func TestExternalProductRepository_SoftDeleteMedia(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	repo := NewPromotionRepository()

	ownerID := uuid.New()
	product := newTestExternalProduct(now)
	product.OwnerUserID = ownerID
	media, err := entity.NewExternalProductMedia(product.ID, entity.ExternalProductMediaTypeImage, "s3://bucket/key", "https://cdn.example.com/key.jpg", nil, 0, nil, now)
	require.NoError(t, err)
	tx := &externalProductRepoTx{now: now, product: product, media: []*entity.ExternalProductMedia{media}}

	require.NoError(t, repo.SoftDeleteMedia(context.Background(), tx, ownerID, product.ID, media.ID))
	assert.True(t, containsSQL(tx.execSQLs, "update external_product_media"))
}

func containsSQL(sqls []string, needle string) bool {
	for _, sql := range sqls {
		if strings.Contains(strings.ToLower(sql), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func ptrRepoString(v string) *string {
	return &v
}
