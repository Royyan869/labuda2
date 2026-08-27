package http

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
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type feedExternalProductQueryPool struct {
	products map[uuid.UUID]*promoentity.ExternalProduct
	users    map[uuid.UUID]feedExternalProductUserRow
}

type feedExternalProductUserRow struct {
	Username      string
	FarmName      string
	AccountStatus string
	IsDeleted     bool
}

var _ promotionQueryPool = (*feedExternalProductQueryPool)(nil)

func (p *feedExternalProductQueryPool) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "from external_products"):
		ids, _ := args[0].([]uuid.UUID)
		rows := make([][]any, 0, len(ids))
		for _, id := range ids {
			product := p.products[id]
			if product == nil || product.DeletedAt != nil || product.ReviewStatus != promoentity.ExternalProductReviewStatusApproved {
				continue
			}
			rows = append(rows, []any{
				product.ID,
				product.OwnerUserID,
				product.Title,
				product.Description,
				product.NormalizedExternalURL,
				"https://cdn.example.com/" + product.ID.String() + ".jpg",
				string(promoentity.ExternalProductMediaTypeImage),
				ptrString("https://cdn.example.com/" + product.ID.String() + "-thumb.jpg"),
			})
		}
		return &feedExternalProductRows{rows: rows}, nil
	case strings.Contains(normalized, "from users"):
		userIDs, _ := args[0].([]uuid.UUID)
		rows := make([][]any, 0, len(userIDs))
		for _, userID := range userIDs {
			user := p.users[userID]
			rows = append(rows, []any{
				userID,
				user.Username,
				user.FarmName,
				user.AccountStatus,
				user.IsDeleted,
			})
		}
		return &feedExternalProductRows{rows: rows}, nil
	default:
		return &feedExternalProductRows{}, nil
	}
}

func (p *feedExternalProductQueryPool) QueryRow(context.Context, string, ...any) pgx.Row {
	return &feedExternalProductRow{err: pgx.ErrNoRows}
}

type feedExternalProductRow struct {
	values []any
	err    error
}

func (r *feedExternalProductRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignFeedExternalProductValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type feedExternalProductRows struct {
	rows [][]any
	idx  int
}

func (r *feedExternalProductRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *feedExternalProductRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("no current row")
	}
	row := r.rows[r.idx-1]
	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(row), len(dest))
	}
	for i, value := range row {
		if err := assignFeedExternalProductValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

func (r *feedExternalProductRows) Err() error                                   { return nil }
func (r *feedExternalProductRows) Close()                                       {}
func (r *feedExternalProductRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *feedExternalProductRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *feedExternalProductRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *feedExternalProductRows) RawValues() [][]byte                          { return nil }
func (r *feedExternalProductRows) Values() ([]any, error)                       { return nil, nil }
func (r *feedExternalProductRows) Conn() *pgx.Conn                              { return nil }

func ptrString(v string) *string {
	return &v
}

func assignFeedExternalProductValue(dest any, value any) error {
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

func TestFeedHydratePromotedItems_IncludesApprovedExternalProduct(t *testing.T) {
	now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	targetID := uuid.New()
	ownerID := uuid.New()
	product := &promoentity.ExternalProduct{
		ID:                    targetID,
		OwnerUserID:           ownerID,
		Title:                 "Fresh Fish",
		Description:           ptrString("Line caught and chilled"),
		ExternalURL:           "https://example.com/fish",
		NormalizedExternalURL: "https://example.com/fish",
		ReviewStatus:          promoentity.ExternalProductReviewStatusApproved,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	inj := &FeedPromotionInjector{
		db: &feedExternalProductQueryPool{
			products: map[uuid.UUID]*promoentity.ExternalProduct{targetID: product},
			users: map[uuid.UUID]feedExternalProductUserRow{
				ownerID: {
					Username:      "alice",
					FarmName:      "Alice Farm",
					AccountStatus: "active",
					IsDeleted:     false,
				},
			},
		},
	}

	items, err := inj.hydratePromotedItems(context.Background(), []*promoentity.PromotionInstance{
		{
			ID:         uuid.New(),
			UserID:     ownerID,
			TargetType: promoentity.TargetTypeExternalProduct,
			TargetID:   &targetID,
			Status:     promoentity.InstanceStatusActive,
		},
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "promoted_external", items[0].Response["type"])
	assert.Equal(t, targetID.String(), items[0].Response["target_id"])
	assert.Equal(t, "Fresh Fish", items[0].Response["title"])
	assert.Equal(t, "https://example.com/fish", items[0].Response["external_url"])
	assert.Equal(t, true, items[0].Response["promoted"])
}

func TestFeedHydratePromotedItems_ExcludesPendingExternalProduct(t *testing.T) {
	targetID := uuid.New()
	ownerID := uuid.New()
	inj := &FeedPromotionInjector{
		db: &feedExternalProductQueryPool{
			products: map[uuid.UUID]*promoentity.ExternalProduct{
				targetID: {
					ID:                    targetID,
					OwnerUserID:           ownerID,
					Title:                 "Fresh Fish",
					NormalizedExternalURL: "https://example.com/fish",
					ReviewStatus:          promoentity.ExternalProductReviewStatusPendingReview,
				},
			},
			users: map[uuid.UUID]feedExternalProductUserRow{
				ownerID: {
					Username:      "alice",
					FarmName:      "Alice Farm",
					AccountStatus: "active",
					IsDeleted:     false,
				},
			},
		},
	}

	items, err := inj.hydratePromotedItems(context.Background(), []*promoentity.PromotionInstance{
		{
			ID:         uuid.New(),
			UserID:     ownerID,
			TargetType: promoentity.TargetTypeExternalProduct,
			TargetID:   &targetID,
			Status:     promoentity.InstanceStatusActive,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, items)
}


