package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/response"
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externalProductHandlerStore struct {
	now        time.Time
	products   map[uuid.UUID]*entity.ExternalProduct
	media      map[uuid.UUID]*entity.ExternalProductMedia
	promotions map[uuid.UUID]*entity.PromotionInstance
	history    []*entity.ExternalProductReviewHistory
}

type externalProductHandlerDB struct {
	store *externalProductHandlerStore
}

var _ db.Transactor = (*externalProductHandlerDB)(nil)

func (d *externalProductHandlerDB) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(&externalProductHandlerTx{store: d.store})
}

type externalProductHandlerTx struct {
	store *externalProductHandlerStore
}

var _ db.Tx = (*externalProductHandlerTx)(nil)

func (t *externalProductHandlerTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "insert into external_products"):
		product := externalProductFromInsertArgs(args)
		t.store.products[product.ID] = cloneExternalProduct(product)
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(normalized, "update external_products"):
		var product *entity.ExternalProduct
		switch len(args) {
		case 15:
			product = externalProductFromUpdateArgs(args)
			current, ok := t.store.products[product.ID]
			if !ok || current == nil || current.DeletedAt != nil || current.OwnerUserID != product.OwnerUserID {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			}
		case 14:
			product = externalProductFromAdminUpdateArgs(args)
			current, ok := t.store.products[product.ID]
			if !ok || current == nil || current.DeletedAt != nil {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			}
		default:
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		t.store.products[product.ID] = cloneExternalProduct(product)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(normalized, "insert into external_product_review_history"):
		history := externalProductHistoryFromInsertArgs(args)
		t.store.history = append(t.store.history, history)
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(normalized, "insert into external_product_media"):
		media := externalProductMediaFromInsertArgs(args)
		t.store.media[media.ID] = cloneExternalProductMedia(media)
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(normalized, "update external_product_media"):
		mediaID, _ := args[0].(uuid.UUID)
		externalProductID, _ := args[1].(uuid.UUID)
		ownerID, _ := args[2].(uuid.UUID)
		media, ok := t.store.media[mediaID]
		if !ok || media == nil || media.DeletedAt != nil || media.ExternalProductID != externalProductID {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		product, ok := t.store.products[externalProductID]
		if !ok || product == nil || product.DeletedAt != nil || product.OwnerUserID != ownerID {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		deletedAt := t.store.now
		media.DeletedAt = &deletedAt
		t.store.media[mediaID] = cloneExternalProductMedia(media)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(normalized, "insert into promotion_instances"):
		instance := promotionInstanceFromInsertArgs(args)
		if t.store.promotions == nil {
			t.store.promotions = make(map[uuid.UUID]*entity.PromotionInstance)
		}
		t.store.promotions[instance.ID] = clonePromotionInstance(instance)
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(normalized, "update promotion_instances"):
		instanceID, _ := args[0].(uuid.UUID)
		instance, ok := t.store.promotions[instanceID]
		if !ok || instance == nil {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		instance.Status = entity.InstanceStatus(args[1].(string))
		instance.ActivatedAt = cloneTimePtrArg(args[2])
		instance.StoppedAt = cloneTimePtrArg(args[3])
		instance.StopReason = cloneStringPtrArg(args[4])
		instance.PausedAt = cloneTimePtrArg(args[5])
		instance.TotalPausedDuration = args[6].(int)
		instance.Finalized = args[7].(bool)
		instance.FinalizedAt = cloneTimePtrArg(args[8])
		instance.FinalizedSeconds = args[9].(int)
		instance.UpdatedAt = args[10].(time.Time)
		t.store.promotions[instanceID] = clonePromotionInstance(instance)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
}

func (t *externalProductHandlerTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "select now()"):
		return &externalProductHandlerRow{values: []any{t.store.now}}
	case strings.Contains(normalized, "from external_products"):
		if len(args) == 0 {
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		productID, _ := args[0].(uuid.UUID)
		product, ok := t.store.products[productID]
		if !ok || product == nil || product.DeletedAt != nil {
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		if len(args) > 1 {
			ownerID, _ := args[1].(uuid.UUID)
			if ownerID != uuid.Nil && product.OwnerUserID != ownerID {
				return &externalProductHandlerRow{err: pgx.ErrNoRows}
			}
		}
		return &externalProductHandlerRow{values: externalProductRowValues(cloneExternalProduct(product))}
	case strings.Contains(normalized, "from promotion_instances"):
		if len(args) == 0 {
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "target_type = $1") && strings.Contains(normalized, "target_id = $2") {
			targetType, _ := args[0].(string)
			targetID, _ := args[1].(uuid.UUID)
			for _, inst := range t.store.promotions {
				if inst == nil || inst.Status != entity.InstanceStatusActive {
					continue
				}
				if string(inst.TargetType) == targetType && inst.TargetID != nil && *inst.TargetID == targetID {
					return &externalProductHandlerRow{values: promotionInstanceRowValues(clonePromotionInstance(inst))}
				}
			}
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "ownership_id = $1") {
			ownershipID, _ := args[0].(uuid.UUID)
			for _, inst := range t.store.promotions {
				if inst == nil || inst.Status != entity.InstanceStatusActive {
					continue
				}
				if inst.OwnershipID == ownershipID {
					return &externalProductHandlerRow{values: promotionInstanceRowValues(clonePromotionInstance(inst))}
				}
			}
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "id = $1") {
			instanceID, _ := args[0].(uuid.UUID)
			inst, ok := t.store.promotions[instanceID]
			if !ok || inst == nil {
				return &externalProductHandlerRow{err: pgx.ErrNoRows}
			}
			return &externalProductHandlerRow{values: promotionInstanceRowValues(clonePromotionInstance(inst))}
		}
		return &externalProductHandlerRow{err: pgx.ErrNoRows}
	case strings.Contains(normalized, "from external_product_media"):
		if len(args) == 0 {
			return &externalProductHandlerRow{err: pgx.ErrNoRows}
		}
		productID, _ := args[0].(uuid.UUID)
		items := make([]*entity.ExternalProductMedia, 0)
		for _, media := range t.store.media {
			if media == nil || media.ExternalProductID != productID || media.DeletedAt != nil {
				continue
			}
			items = append(items, cloneExternalProductMedia(media))
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].SortOrder == items[j].SortOrder {
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			}
			return items[i].SortOrder < items[j].SortOrder
		})
		rows := make([][]any, 0, len(items))
		for _, media := range items {
			rows = append(rows, externalProductMediaRowValues(media))
		}
		return &externalProductHandlerRows{rows: rows}
	default:
		return &externalProductHandlerRow{err: fmt.Errorf("unexpected query: %s", sql)}
	}
}

func (t *externalProductHandlerTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "from external_products"):
		if len(args) == 0 {
			return &externalProductHandlerRows{}, nil
		}
		if strings.Contains(normalized, "review_status in") {
			statuses := make(map[entity.ExternalProductReviewStatus]struct{})
			for _, arg := range args {
				switch v := arg.(type) {
				case string:
					if strings.Contains(strings.ToLower(v), "limit") || strings.Contains(strings.ToLower(v), "offset") {
						continue
					}
					statuses[entity.ExternalProductReviewStatus(v)] = struct{}{}
				}
			}
			products := make([]*entity.ExternalProduct, 0)
			for _, product := range t.store.products {
				if product == nil || product.DeletedAt != nil {
					continue
				}
				if len(statuses) > 0 {
					if _, ok := statuses[product.ReviewStatus]; !ok {
						continue
					}
				}
				products = append(products, cloneExternalProduct(product))
			}
			sort.Slice(products, func(i, j int) bool {
				if products[i].UpdatedAt.Equal(products[j].UpdatedAt) {
					return products[i].CreatedAt.After(products[j].CreatedAt)
				}
				return products[i].UpdatedAt.After(products[j].UpdatedAt)
			})
			if strings.Contains(normalized, "limit $") {
				for i := len(args) - 1; i >= 0; i-- {
					if limit, ok := args[i].(int); ok {
						if limit > 0 && len(products) > limit {
							products = products[:limit]
						}
						break
					}
				}
			}
			if strings.Contains(normalized, "offset $") {
				for i := len(args) - 1; i >= 0; i-- {
					if offset, ok := args[i].(int); ok {
						if offset > 0 && offset < len(products) {
							products = products[offset:]
						} else if offset >= len(products) {
							products = nil
						}
						break
					}
				}
			}
			rows := make([][]any, 0, len(products))
			for _, product := range products {
				rows = append(rows, externalProductRowValues(product))
			}
			return &externalProductHandlerRows{rows: rows}, nil
		}
		ownerID, _ := args[0].(uuid.UUID)
		products := make([]*entity.ExternalProduct, 0)
		for _, product := range t.store.products {
			if product == nil {
				continue
			}
			if product.OwnerUserID != ownerID {
				continue
			}
			if strings.Contains(normalized, "deleted_at is null") && product.DeletedAt != nil {
				continue
			}
			products = append(products, cloneExternalProduct(product))
		}
		sort.Slice(products, func(i, j int) bool {
			return products[i].CreatedAt.After(products[j].CreatedAt)
		})
		if strings.Contains(normalized, "limit $2") && len(args) >= 2 {
			limit, _ := args[1].(int)
			if limit > 0 && len(products) > limit {
				products = products[:limit]
			}
		}
		if strings.Contains(normalized, "offset $3") && len(args) >= 3 {
			offset, _ := args[2].(int)
			if offset > 0 && offset < len(products) {
				products = products[offset:]
			} else if offset >= len(products) {
				products = nil
			}
		}
		rows := make([][]any, 0, len(products))
		for _, product := range products {
			rows = append(rows, externalProductRowValues(product))
		}
		return &externalProductHandlerRows{rows: rows}, nil
	case strings.Contains(normalized, "from external_product_media"):
		if len(args) == 0 {
			return &externalProductHandlerRows{}, nil
		}
		productID, _ := args[0].(uuid.UUID)
		items := make([]*entity.ExternalProductMedia, 0)
		for _, media := range t.store.media {
			if media == nil || media.ExternalProductID != productID || media.DeletedAt != nil {
				continue
			}
			items = append(items, cloneExternalProductMedia(media))
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].SortOrder == items[j].SortOrder {
				return items[i].CreatedAt.Before(items[j].CreatedAt)
			}
			return items[i].SortOrder < items[j].SortOrder
		})
		rows := make([][]any, 0, len(items))
		for _, media := range items {
			rows = append(rows, externalProductMediaRowValues(media))
		}
		return &externalProductHandlerRows{rows: rows}, nil
	case strings.Contains(normalized, "from external_product_review_history"):
		if len(args) == 0 {
			return &externalProductHandlerRows{}, nil
		}
		productID, _ := args[0].(uuid.UUID)
		items := make([]*entity.ExternalProductReviewHistory, 0)
		for _, history := range t.store.history {
			if history == nil || history.ExternalProductID != productID {
				continue
			}
			items = append(items, cloneExternalProductReviewHistory(history))
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		rows := make([][]any, 0, len(items))
		for _, history := range items {
			rows = append(rows, externalProductHistoryRowValues(history))
		}
		return &externalProductHandlerRows{rows: rows}, nil
	default:
		return &externalProductHandlerRows{}, nil
	}
}

func (t *externalProductHandlerTx) Commit(context.Context) error   { return nil }
func (t *externalProductHandlerTx) Rollback(context.Context) error { return nil }

type externalProductHandlerRow struct {
	values []any
	err    error
}

func (r *externalProductHandlerRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignExternalProductHandlerValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type externalProductHandlerRows struct {
	rows [][]any
	idx  int
}

func (r *externalProductHandlerRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *externalProductHandlerRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("no current row")
	}
	row := r.rows[r.idx-1]
	if len(row) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(row), len(dest))
	}
	for i, value := range row {
		if err := assignExternalProductHandlerValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

func (r *externalProductHandlerRows) Err() error                                   { return nil }
func (r *externalProductHandlerRows) Close()                                       {}
func (r *externalProductHandlerRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *externalProductHandlerRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *externalProductHandlerRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *externalProductHandlerRows) RawValues() [][]byte                          { return nil }
func (r *externalProductHandlerRows) Values() ([]any, error)                       { return nil, nil }
func (r *externalProductHandlerRows) Conn() *pgx.Conn                              { return nil }

func assignExternalProductHandlerValue(dest any, value any) error {
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

func cloneExternalProduct(product *entity.ExternalProduct) *entity.ExternalProduct {
	if product == nil {
		return nil
	}

	clone := *product
	if product.Description != nil {
		description := *product.Description
		clone.Description = &description
	}
	if product.RejectionReason != nil {
		rejectionReason := *product.RejectionReason
		clone.RejectionReason = &rejectionReason
	}
	if product.SubmittedAt != nil {
		submittedAt := *product.SubmittedAt
		clone.SubmittedAt = &submittedAt
	}
	if product.ApprovedAt != nil {
		approvedAt := *product.ApprovedAt
		clone.ApprovedAt = &approvedAt
	}
	if product.RejectedAt != nil {
		rejectedAt := *product.RejectedAt
		clone.RejectedAt = &rejectedAt
	}
	if product.HiddenAt != nil {
		hiddenAt := *product.HiddenAt
		clone.HiddenAt = &hiddenAt
	}
	if product.LastReviewedBy != nil {
		lastReviewedBy := *product.LastReviewedBy
		clone.LastReviewedBy = &lastReviewedBy
	}
	if product.DeletedAt != nil {
		deletedAt := *product.DeletedAt
		clone.DeletedAt = &deletedAt
	}
	return &clone
}

func externalProductFromInsertArgs(args []any) *entity.ExternalProduct {
	return &entity.ExternalProduct{
		ID:                    args[0].(uuid.UUID),
		OwnerUserID:           args[1].(uuid.UUID),
		Title:                 args[2].(string),
		Description:           cloneStringPtrArg(args[3]),
		ExternalURL:           args[4].(string),
		NormalizedExternalURL: args[5].(string),
		ReviewStatus:          entity.ExternalProductReviewStatus(args[6].(string)),
		RejectionReason:       cloneStringPtrArg(args[7]),
		UnsafeURLFlag:         args[8].(bool),
		SubmittedAt:           cloneTimePtrArg(args[9]),
		ApprovedAt:            cloneTimePtrArg(args[10]),
		RejectedAt:            cloneTimePtrArg(args[11]),
		HiddenAt:              cloneTimePtrArg(args[12]),
		LastReviewedBy:        cloneUUIDPtrArg(args[13]),
		CreatedAt:             args[14].(time.Time),
		UpdatedAt:             args[15].(time.Time),
		DeletedAt:             cloneTimePtrArg(args[16]),
	}
}

func externalProductFromUpdateArgs(args []any) *entity.ExternalProduct {
	return &entity.ExternalProduct{
		ID:                    args[0].(uuid.UUID),
		Title:                 args[1].(string),
		Description:           cloneStringPtrArg(args[2]),
		ExternalURL:           args[3].(string),
		NormalizedExternalURL: args[4].(string),
		ReviewStatus:          entity.ExternalProductReviewStatus(args[5].(string)),
		RejectionReason:       cloneStringPtrArg(args[6]),
		UnsafeURLFlag:         args[7].(bool),
		SubmittedAt:           cloneTimePtrArg(args[8]),
		ApprovedAt:            cloneTimePtrArg(args[9]),
		RejectedAt:            cloneTimePtrArg(args[10]),
		HiddenAt:              cloneTimePtrArg(args[11]),
		LastReviewedBy:        cloneUUIDPtrArg(args[12]),
		UpdatedAt:             args[13].(time.Time),
		OwnerUserID:           args[14].(uuid.UUID),
	}
}

func externalProductFromAdminUpdateArgs(args []any) *entity.ExternalProduct {
	return &entity.ExternalProduct{
		ID:                    args[0].(uuid.UUID),
		Title:                 args[1].(string),
		Description:           cloneStringPtrArg(args[2]),
		ExternalURL:           args[3].(string),
		NormalizedExternalURL: args[4].(string),
		ReviewStatus:          entity.ExternalProductReviewStatus(args[5].(string)),
		RejectionReason:       cloneStringPtrArg(args[6]),
		UnsafeURLFlag:         args[7].(bool),
		SubmittedAt:           cloneTimePtrArg(args[8]),
		ApprovedAt:            cloneTimePtrArg(args[9]),
		RejectedAt:            cloneTimePtrArg(args[10]),
		HiddenAt:              cloneTimePtrArg(args[11]),
		LastReviewedBy:        cloneUUIDPtrArg(args[12]),
		UpdatedAt:             args[13].(time.Time),
	}
}

func externalProductHistoryFromInsertArgs(args []any) *entity.ExternalProductReviewHistory {
	return &entity.ExternalProductReviewHistory{
		ID:                args[0].(uuid.UUID),
		ExternalProductID: args[1].(uuid.UUID),
		ActorAdminID:      cloneUUIDPtrArg(args[2]),
		ActorUserID:       cloneUUIDPtrArg(args[3]),
		FromStatus:        cloneReviewStatusPtrArg(args[4]),
		ToStatus:          entity.ExternalProductReviewStatus(args[5].(string)),
		Reason:            cloneStringPtrArg(args[6]),
		CreatedAt:         args[7].(time.Time),
	}
}

func externalProductHistoryRowValues(history *entity.ExternalProductReviewHistory) []any {
	return []any{
		history.ID,
		history.ExternalProductID,
		history.ActorAdminID,
		history.ActorUserID,
		stringValuePtr(history.FromStatus),
		string(history.ToStatus),
		history.Reason,
		history.CreatedAt,
	}
}

func promotionInstanceFromInsertArgs(args []any) *entity.PromotionInstance {
	return &entity.PromotionInstance{
		ID:                  args[0].(uuid.UUID),
		OwnershipID:         args[1].(uuid.UUID),
		UserID:              args[2].(uuid.UUID),
		TargetType:          entity.TargetType(args[3].(string)),
		TargetID:            cloneUUIDPtrArg(args[4]),
		Status:              entity.InstanceStatus(args[5].(string)),
		ActivatedAt:         cloneTimePtrArg(args[6]),
		StoppedAt:           cloneTimePtrArg(args[7]),
		StopReason:          cloneStringPtrArg(args[8]),
		PausedAt:            cloneTimePtrArg(args[9]),
		TotalPausedDuration: args[10].(int),
		Finalized:           args[11].(bool),
		FinalizedAt:         cloneTimePtrArg(args[12]),
		FinalizedSeconds:    args[13].(int),
		CreatedAt:           args[14].(time.Time),
		UpdatedAt:           args[15].(time.Time),
	}
}

func promotionInstanceRowValues(instance *entity.PromotionInstance) []any {
	return []any{
		instance.ID,
		instance.OwnershipID,
		instance.UserID,
		string(instance.TargetType),
		instance.TargetID,
		string(instance.Status),
		instance.ActivatedAt,
		instance.StoppedAt,
		instance.StopReason,
		instance.PausedAt,
		instance.TotalPausedDuration,
		instance.Finalized,
		instance.FinalizedAt,
		instance.FinalizedSeconds,
		instance.CreatedAt,
		instance.UpdatedAt,
	}
}

func clonePromotionInstance(instance *entity.PromotionInstance) *entity.PromotionInstance {
	if instance == nil {
		return nil
	}
	clone := *instance
	if instance.TargetID != nil {
		targetID := *instance.TargetID
		clone.TargetID = &targetID
	}
	if instance.ActivatedAt != nil {
		activatedAt := *instance.ActivatedAt
		clone.ActivatedAt = &activatedAt
	}
	if instance.StoppedAt != nil {
		stoppedAt := *instance.StoppedAt
		clone.StoppedAt = &stoppedAt
	}
	if instance.StopReason != nil {
		stopReason := *instance.StopReason
		clone.StopReason = &stopReason
	}
	if instance.PausedAt != nil {
		pausedAt := *instance.PausedAt
		clone.PausedAt = &pausedAt
	}
	if instance.FinalizedAt != nil {
		finalizedAt := *instance.FinalizedAt
		clone.FinalizedAt = &finalizedAt
	}
	return &clone
}

func externalProductMediaFromInsertArgs(args []any) *entity.ExternalProductMedia {
	return &entity.ExternalProductMedia{
		ID:                args[0].(uuid.UUID),
		ExternalProductID: args[1].(uuid.UUID),
		MediaType:         entity.ExternalProductMediaType(args[2].(string)),
		StorageKey:        args[3].(string),
		URL:               args[4].(string),
		ThumbnailURL:      cloneStringPtrArg(args[5]),
		SortOrder:         args[6].(int),
		Metadata:          cloneRawMessageArg(args[7]),
		CreatedAt:         args[8].(time.Time),
		DeletedAt:         cloneTimePtrArg(args[9]),
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

func cloneExternalProductMedia(media *entity.ExternalProductMedia) *entity.ExternalProductMedia {
	if media == nil {
		return nil
	}

	clone := *media
	if media.ThumbnailURL != nil {
		thumbnailURL := *media.ThumbnailURL
		clone.ThumbnailURL = &thumbnailURL
	}
	if media.Metadata != nil {
		metadata := append(json.RawMessage(nil), media.Metadata...)
		clone.Metadata = metadata
	}
	if media.DeletedAt != nil {
		deletedAt := *media.DeletedAt
		clone.DeletedAt = &deletedAt
	}
	return &clone
}

func cloneExternalProductReviewHistory(history *entity.ExternalProductReviewHistory) *entity.ExternalProductReviewHistory {
	if history == nil {
		return nil
	}

	clone := *history
	if history.ActorAdminID != nil {
		adminID := *history.ActorAdminID
		clone.ActorAdminID = &adminID
	}
	if history.ActorUserID != nil {
		userID := *history.ActorUserID
		clone.ActorUserID = &userID
	}
	if history.FromStatus != nil {
		status := *history.FromStatus
		clone.FromStatus = &status
	}
	if history.Reason != nil {
		reason := *history.Reason
		clone.Reason = &reason
	}
	return &clone
}

func cloneStringPtrArg(v any) *string {
	if v == nil {
		return nil
	}
	s, ok := v.(*string)
	if !ok {
		return nil
	}
	if s == nil {
		return nil
	}
	out := *s
	return &out
}

func cloneTimePtrArg(v any) *time.Time {
	if v == nil {
		return nil
	}
	t, ok := v.(*time.Time)
	if !ok {
		return nil
	}
	if t == nil {
		return nil
	}
	out := *t
	return &out
}

func cloneUUIDPtrArg(v any) *uuid.UUID {
	if v == nil {
		return nil
	}
	id, ok := v.(*uuid.UUID)
	if !ok {
		return nil
	}
	if id == nil {
		return nil
	}
	out := *id
	return &out
}

func cloneRawMessageArg(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	switch b := v.(type) {
	case json.RawMessage:
		if b == nil {
			return nil
		}
		return append(json.RawMessage(nil), b...)
	case []byte:
		if b == nil {
			return nil
		}
		return append(json.RawMessage(nil), b...)
	default:
		return nil
	}
}

func cloneReviewStatusPtrArg(v any) *entity.ExternalProductReviewStatus {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case *string:
		if s == nil {
			return nil
		}
		status := entity.ExternalProductReviewStatus(*s)
		return &status
	case *entity.ExternalProductReviewStatus:
		if s == nil {
			return nil
		}
		status := *s
		return &status
	default:
		return nil
	}
}

func stringValuePtr(v *entity.ExternalProductReviewStatus) *string {
	if v == nil {
		return nil
	}
	value := v.String()
	return &value
}

// mockOperabilityChecker is sufficient for external product-only handler tests.
type mockOperabilityChecker struct{}

func (m mockOperabilityChecker) CheckOperability(context.Context, entity.TargetType, *uuid.UUID) (bool, string, error) {
	return true, "", nil
}

func (m mockOperabilityChecker) ValidateOwnership(context.Context, uuid.UUID, entity.TargetType, *uuid.UUID) error {
	return nil
}

func (m mockOperabilityChecker) CheckUserEligibility(context.Context, uuid.UUID) (bool, string, error) {
	return true, "", nil
}

func newTestExternalProductHandler(store *externalProductHandlerStore) *PromotionHandler {
	if store.products == nil {
		store.products = map[uuid.UUID]*entity.ExternalProduct{}
	}
	if store.media == nil {
		store.media = map[uuid.UUID]*entity.ExternalProductMedia{}
	}
	if store.promotions == nil {
		store.promotions = map[uuid.UUID]*entity.PromotionInstance{}
	}
	return NewPromotionHandler(
		promotionApp.NewPromotionService(mockOperabilityChecker{}),
		nil,
		nil,
		&externalProductHandlerDB{store: store},
		nil,
	)
}

func mockAccountStatusGate(statusErr error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if statusErr != nil {
			switch statusErr {
			case auth.ErrAccountSuspended:
				response.Error(c, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Your account has been suspended.")
			case auth.ErrAccountBanned:
				response.Error(c, http.StatusForbidden, "ACCOUNT_BANNED", "Your account has been banned.")
			default:
				response.InternalServerError(c, "Failed to verify account status")
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

type apiResponseEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeExternalProductResponse(t *testing.T, body *httptest.ResponseRecorder) ExternalProductResponse {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))

	var resp ExternalProductResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &resp))
	return resp
}

func decodeExternalProductListResponse(t *testing.T, body *httptest.ResponseRecorder) ExternalProductListResponse {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))

	var resp ExternalProductListResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &resp))
	return resp
}

func TestExternalProductCreate_UnauthenticatedBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &externalProductHandlerStore{now: time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC), products: map[uuid.UUID]*entity.ExternalProduct{}}
	handler := newTestExternalProductHandler(store)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := `{"title":"Fresh Fish","external_url":"https://example.com/fish"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(reqBody))

	handler.CreateExternalProduct(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExternalProductCreate_ActiveUserAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &externalProductHandlerStore{now: time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC), products: map[uuid.UUID]*entity.ExternalProduct{}}
	handler := newTestExternalProductHandler(store)

	router := gin.New()
	ownerID := uuid.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", ownerID)
		c.Next()
	})
	router.Use(mockAccountStatusGate(nil))
	router.POST("/external-products", handler.CreateExternalProduct)

	w := httptest.NewRecorder()
	reqBody := `{"title":"Fresh Fish","description":"Ocean caught","external_url":"https://example.com/fish"}`
	req := httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(reqBody))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeExternalProductResponse(t, w)
	assert.Equal(t, ownerID, resp.OwnerUserID)
	assert.Equal(t, "draft", resp.ReviewStatus)
	assert.True(t, resp.CanSubmit)
	assert.False(t, resp.CanResubmit)
	assert.False(t, resp.PublicVisible)
}

func TestExternalProductCreate_SuspendedUserBlockedByGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &externalProductHandlerStore{now: time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC), products: map[uuid.UUID]*entity.ExternalProduct{}}
	handler := newTestExternalProductHandler(store)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(mockAccountStatusGate(auth.ErrAccountSuspended))
	router.POST("/external-products", handler.CreateExternalProduct)

	w := httptest.NewRecorder()
	reqBody := `{"title":"Fresh Fish","external_url":"https://example.com/fish"}`
	req := httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(reqBody))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "ACCOUNT_SUSPENDED")
}

func TestExternalProductCreate_InvalidTitleAndURLRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &externalProductHandlerStore{now: time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC), products: map[uuid.UUID]*entity.ExternalProduct{}}
	handler := newTestExternalProductHandler(store)

	t.Run("invalid title", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uuid.New())
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(`{"title":"","external_url":"https://example.com/fish"}`))

		handler.CreateExternalProduct(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid url", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uuid.New())
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(`{"title":"Fresh Fish","external_url":"ftp://example.com"}`))

		handler.CreateExternalProduct(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestExternalProductLifecycleAndOwnershipGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	otherID := uuid.New()

	buildStore := func() *externalProductHandlerStore {
		draft, err := entity.NewExternalProductDraft(ownerID, "Fresh Fish", nil, "https://example.com/fish", now)
		require.NoError(t, err)
		draft.OwnerUserID = ownerID
		draft.ReviewStatus = entity.ExternalProductReviewStatusDraft
		draft.CreatedAt = now
		draft.UpdatedAt = now

		approved, err := entity.NewExternalProductDraft(ownerID, "Approved Fish", nil, "https://example.com/approved", now)
		require.NoError(t, err)
		approved.ReviewStatus = entity.ExternalProductReviewStatusApproved
		approved.ApprovedAt = ptrTime(now)
		approved.CreatedAt = now
		approved.UpdatedAt = now

		rejected, err := entity.NewExternalProductDraft(ownerID, "Rejected Fish", nil, "https://example.com/rejected", now)
		require.NoError(t, err)
		rejected.ReviewStatus = entity.ExternalProductReviewStatusRejected
		rejected.RejectedAt = ptrTime(now)
		rejected.CreatedAt = now
		rejected.UpdatedAt = now

		pending, err := entity.NewExternalProductDraft(ownerID, "Pending Fish", nil, "https://example.com/pending", now)
		require.NoError(t, err)
		pending.ReviewStatus = entity.ExternalProductReviewStatusPendingReview
		pending.SubmittedAt = ptrTime(now)
		pending.CreatedAt = now
		pending.UpdatedAt = now

		hidden, err := entity.NewExternalProductDraft(ownerID, "Hidden Fish", nil, "https://example.com/hidden", now)
		require.NoError(t, err)
		hidden.ReviewStatus = entity.ExternalProductReviewStatusHidden
		hidden.HiddenAt = ptrTime(now)
		hidden.CreatedAt = now
		hidden.UpdatedAt = now

		deleted, err := entity.NewExternalProductDraft(ownerID, "Deleted Fish", nil, "https://example.com/deleted", now)
		require.NoError(t, err)
		deleted.DeletedAt = ptrTime(now)
		deleted.CreatedAt = now
		deleted.UpdatedAt = now

		other, err := entity.NewExternalProductDraft(otherID, "Other Fish", nil, "https://example.com/other", now)
		require.NoError(t, err)
		other.CreatedAt = now.Add(time.Minute)
		other.UpdatedAt = now.Add(time.Minute)

		return &externalProductHandlerStore{
			now: now,
			products: map[uuid.UUID]*entity.ExternalProduct{
				draft.ID:    draft,
				approved.ID: approved,
				rejected.ID: rejected,
				pending.ID:  pending,
				hidden.ID:   hidden,
				deleted.ID:  deleted,
				other.ID:    other,
			},
			media: map[uuid.UUID]*entity.ExternalProductMedia{},
		}
	}

	t.Run("detail by owner succeeds", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		productID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+productID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: productID.String()}}

		handler.GetExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, productID, resp.ID)
		assert.Equal(t, ownerID, resp.OwnerUserID)
		assert.False(t, resp.PublicVisible)
	})

	t.Run("detail other user forbidden", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		otherProduct := pickProductID(store, otherID, entity.ExternalProductReviewStatusDraft, false)
		require.NotEqual(t, uuid.Nil, otherProduct)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+otherProduct.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: otherProduct.String()}}

		handler.GetExternalProduct(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("detail deleted product not found", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		deletedID := pickDeletedProductID(store)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+deletedID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: deletedID.String()}}

		handler.GetExternalProduct(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("update draft works", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPatch, "/external-products/"+draftID.String(), strings.NewReader(`{"title":"Updated Fish","external_url":"https://example.com/updated"}`))
		c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

		handler.UpdateExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, "Updated Fish", resp.Title)
		assert.Equal(t, "draft", resp.ReviewStatus)
	})

	t.Run("approved material edit returns pending_review", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		approvedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusApproved, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPatch, "/external-products/"+approvedID.String(), strings.NewReader(`{"title":"Approved Fish 2"}`))
		c.Params = gin.Params{{Key: "id", Value: approvedID.String()}}

		handler.UpdateExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, "pending_review", resp.ReviewStatus)
		assert.False(t, resp.CanEdit)
		assert.False(t, resp.CanSubmit)
		assert.False(t, resp.CanResubmit)
	})

	t.Run("pending_review update blocked", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPatch, "/external-products/"+pendingID.String(), strings.NewReader(`{"title":"Still Pending"}`))
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.UpdateExternalProduct(c)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("hidden cannot be made public by owner", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		hiddenID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusHidden, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPatch, "/external-products/"+hiddenID.String(), strings.NewReader(`{"title":"Try Public"}`))
		c.Params = gin.Params{{Key: "id", Value: hiddenID.String()}}

		handler.UpdateExternalProduct(c)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("submit draft returns pending_review", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+draftID.String()+"/submit", nil)
		c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

		handler.SubmitExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, "pending_review", resp.ReviewStatus)
		assert.False(t, resp.PublicVisible)
	})

	t.Run("resubmit rejected returns pending_review", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		rejectedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusRejected, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+rejectedID.String()+"/resubmit", nil)
		c.Params = gin.Params{{Key: "id", Value: rejectedID.String()}}

		handler.ResubmitExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, "pending_review", resp.ReviewStatus)
	})

	t.Run("update other user forbidden", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		otherProduct := pickProductID(store, otherID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPatch, "/external-products/"+otherProduct.String(), strings.NewReader(`{"title":"Hacked"}`))
		c.Params = gin.Params{{Key: "id", Value: otherProduct.String()}}

		handler.UpdateExternalProduct(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("submit other user forbidden", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		otherProduct := pickProductID(store, otherID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+otherProduct.String()+"/submit", nil)
		c.Params = gin.Params{{Key: "id", Value: otherProduct.String()}}

		handler.SubmitExternalProduct(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("resubmit other user forbidden", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		otherProduct := pickProductID(store, otherID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+otherProduct.String()+"/resubmit", nil)
		c.Params = gin.Params{{Key: "id", Value: otherProduct.String()}}

		handler.ResubmitExternalProduct(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("list only returns owned products", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodGet, "/my/external-products?page=1&page_size=10", nil)

		handler.ListMyExternalProducts(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductListResponse(t, w)
		require.GreaterOrEqual(t, resp.Count, 1)
		for _, item := range resp.Items {
			assert.Equal(t, ownerID, item.OwnerUserID)
		}
	})

	t.Run("attach media to draft product succeeds", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		body := `{"media_type":"image","storage_key":"external-products/draft-1.jpg","url":"https://cdn.example.com/draft-1.jpg","sort_order":1,"metadata":{"kind":"cover"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+draftID.String()+"/media", strings.NewReader(body))
		c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

		handler.AttachExternalProductMedia(c)

		require.Equal(t, http.StatusCreated, w.Code)
		resp := decodeExternalProductResponse(t, w)
		require.Len(t, resp.Media, 1)
		assert.Equal(t, "draft", resp.ReviewStatus)
		assert.Equal(t, "image", resp.Media[0].MediaType)
		assert.Equal(t, "external-products/draft-1.jpg", resp.Media[0].StorageKey)
		assert.Equal(t, 1, resp.Media[0].SortOrder)
		assert.Equal(t, `{"kind":"cover"}`, string(resp.Media[0].Metadata))
	})

	t.Run("attach media to approved product returns pending_review", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		approvedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusApproved, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		body := `{"media_type":"video","storage_key":"external-products/approved-1.mp4","url":"https://cdn.example.com/approved-1.mp4","sort_order":0}`
		c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+approvedID.String()+"/media", strings.NewReader(body))
		c.Params = gin.Params{{Key: "id", Value: approvedID.String()}}

		handler.AttachExternalProductMedia(c)

		require.Equal(t, http.StatusCreated, w.Code)
		resp := decodeExternalProductResponse(t, w)
		require.Len(t, resp.Media, 1)
		assert.Equal(t, "pending_review", resp.ReviewStatus)
		assert.False(t, resp.PublicVisible)
		assert.Equal(t, "video", resp.Media[0].MediaType)
	})

	t.Run("list media returns attached media", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		attachBody := `{"media_type":"image","storage_key":"external-products/draft-2.jpg","url":"https://cdn.example.com/draft-2.jpg","sort_order":2}`
		attachW := httptest.NewRecorder()
		attachC, _ := gin.CreateTestContext(attachW)
		attachC.Set("user_id", ownerID)
		attachC.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+draftID.String()+"/media", strings.NewReader(attachBody))
		attachC.Params = gin.Params{{Key: "id", Value: draftID.String()}}
		handler.AttachExternalProductMedia(attachC)
		require.Equal(t, http.StatusCreated, attachW.Code)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+draftID.String()+"/media", nil)
		c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

		handler.ListExternalProductMedia(c)

		require.Equal(t, http.StatusOK, w.Code)
		var listResp ExternalProductMediaListResponse
		require.NoError(t, json.Unmarshal(decodeExternalProductResponseEnvelope(t, w), &listResp))
		require.Len(t, listResp.Items, 1)
		assert.Equal(t, "image", listResp.Items[0].MediaType)
		assert.Equal(t, 1, listResp.Count)
	})

	t.Run("delete media removes it and approved product returns pending_review", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		approvedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusApproved, false)
		media, err := entity.NewExternalProductMedia(
			approvedID,
			entity.ExternalProductMediaTypeImage,
			"external-products/approved-delete.jpg",
			"https://cdn.example.com/approved-delete.jpg",
			nil,
			0,
			json.RawMessage(`{"kind":"gallery"}`),
			now,
		)
		require.NoError(t, err)
		store.media[media.ID] = media

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", ownerID)
		c.Request = httptest.NewRequest(http.MethodDelete, "/external-products/"+approvedID.String()+"/media/"+media.ID.String(), nil)
		c.Params = gin.Params{
			{Key: "id", Value: approvedID.String()},
			{Key: "media_id", Value: media.ID.String()},
		}

		handler.DeleteExternalProductMedia(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeExternalProductResponse(t, w)
		assert.Equal(t, "pending_review", resp.ReviewStatus)
		assert.Empty(t, resp.Media)
		require.Len(t, store.media, 1)
		for _, item := range store.media {
			assert.NotNil(t, item.DeletedAt)
		}

		listW := httptest.NewRecorder()
		listC, _ := gin.CreateTestContext(listW)
		listC.Set("user_id", ownerID)
		listC.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+approvedID.String()+"/media", nil)
		listC.Params = gin.Params{{Key: "id", Value: approvedID.String()}}
		handler.ListExternalProductMedia(listC)

		require.Equal(t, http.StatusOK, listW.Code)
		var listResp ExternalProductMediaListResponse
		require.NoError(t, json.Unmarshal(decodeExternalProductResponseEnvelope(t, listW), &listResp))
		assert.Empty(t, listResp.Items)
		assert.Equal(t, 0, listResp.Count)
	})

	t.Run("media ownership checks block other users", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)
		approvedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusApproved, false)
		media, err := entity.NewExternalProductMedia(
			approvedID,
			entity.ExternalProductMediaTypeImage,
			"external-products/owner-only.jpg",
			"https://cdn.example.com/owner-only.jpg",
			nil,
			0,
			nil,
			now,
		)
		require.NoError(t, err)
		store.media[media.ID] = media

		t.Run("attach forbidden", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", otherID)
			c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+draftID.String()+"/media", strings.NewReader(`{"media_type":"image","storage_key":"external-products/forbidden.jpg","url":"https://cdn.example.com/forbidden.jpg"}`))
			c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

			handler.AttachExternalProductMedia(c)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})

		t.Run("list forbidden", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", otherID)
			c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+approvedID.String()+"/media", nil)
			c.Params = gin.Params{{Key: "id", Value: approvedID.String()}}

			handler.ListExternalProductMedia(c)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})

		t.Run("delete forbidden", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", otherID)
			c.Request = httptest.NewRequest(http.MethodDelete, "/external-products/"+approvedID.String()+"/media/"+media.ID.String(), nil)
			c.Params = gin.Params{
				{Key: "id", Value: approvedID.String()},
				{Key: "media_id", Value: media.ID.String()},
			}

			handler.DeleteExternalProductMedia(c)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	})

	t.Run("invalid media payloads are rejected", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		cases := map[string]string{
			"invalid media type":  `{"media_type":"audio","storage_key":"external-products/invalid.jpg","url":"https://cdn.example.com/invalid.jpg"}`,
			"missing storage key": `{"media_type":"image","url":"https://cdn.example.com/invalid.jpg"}`,
			"missing url":         `{"media_type":"image","storage_key":"external-products/invalid.jpg"}`,
		}

		for name, body := range cases {
			t.Run(name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Set("user_id", ownerID)
				c.Request = httptest.NewRequest(http.MethodPost, "/external-products/"+draftID.String()+"/media", strings.NewReader(body))
				c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

				handler.AttachExternalProductMedia(c)

				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})
}

func TestExternalProductAdminReviewWorkflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	adminID := uuid.New()
	ownerID := uuid.New()

	buildStore := func() *externalProductHandlerStore {
		pending, err := entity.NewExternalProductDraft(ownerID, "Pending Fish", nil, "https://example.com/pending", now)
		require.NoError(t, err)
		pending.ReviewStatus = entity.ExternalProductReviewStatusPendingReview
		pending.SubmittedAt = ptrTime(now)
		pending.CreatedAt = now
		pending.UpdatedAt = now

		approved, err := entity.NewExternalProductDraft(ownerID, "Approved Fish", nil, "https://example.com/approved", now)
		require.NoError(t, err)
		approved.ReviewStatus = entity.ExternalProductReviewStatusApproved
		approved.ApprovedAt = ptrTime(now)
		approved.CreatedAt = now
		approved.UpdatedAt = now

		rejected, err := entity.NewExternalProductDraft(ownerID, "Rejected Fish", nil, "https://example.com/rejected", now)
		require.NoError(t, err)
		rejected.ReviewStatus = entity.ExternalProductReviewStatusRejected
		rejected.RejectedAt = ptrTime(now)
		rejected.CreatedAt = now
		rejected.UpdatedAt = now

		draft, err := entity.NewExternalProductDraft(ownerID, "Draft Fish", nil, "https://example.com/draft", now)
		require.NoError(t, err)
		draft.CreatedAt = now
		draft.UpdatedAt = now

		media, err := entity.NewExternalProductMedia(
			pending.ID,
			entity.ExternalProductMediaTypeImage,
			"external-products/pending-cover.jpg",
			"https://cdn.example.com/pending-cover.jpg",
			nil,
			0,
			json.RawMessage(`{"kind":"cover"}`),
			now,
		)
		require.NoError(t, err)

		history, err := entity.NewExternalProductReviewHistory(
			pending.ID,
			ptrReviewStatus(entity.ExternalProductReviewStatusDraft),
			entity.ExternalProductReviewStatusPendingReview,
			ptrString("submitted for review"),
			nil,
			&adminID,
			now,
		)
		require.NoError(t, err)

		return &externalProductHandlerStore{
			now: now,
			products: map[uuid.UUID]*entity.ExternalProduct{
				pending.ID:  pending,
				approved.ID: approved,
				rejected.ID: rejected,
				draft.ID:    draft,
			},
			media: map[uuid.UUID]*entity.ExternalProductMedia{
				media.ID: media,
			},
			history: []*entity.ExternalProductReviewHistory{history},
		}
	}

	t.Run("unauthorized admin blocked", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		targetID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+targetID.String()+"/approve", strings.NewReader(`{"note":"ok"}`))
		c.Params = gin.Params{{Key: "id", Value: targetID.String()}}

		handler.ApproveExternalProduct(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("list queue returns reviewable products", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodGet, "/admin/external-products", nil)

		handler.ListAdminExternalProducts(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductListResponse(t, w)
		require.GreaterOrEqual(t, resp.Count, 3)
		for _, item := range resp.Items {
			assert.NotEqual(t, "draft", item.ReviewStatus)
		}
	})

	t.Run("detail includes media and history", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodGet, "/admin/external-products/"+pendingID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.GetAdminExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductResponse(t, w)
		assert.Equal(t, pendingID, resp.ID)
		require.Len(t, resp.Media, 1)
		require.Len(t, resp.ReviewHistory, 1)
		assert.True(t, resp.CanApprove)
		assert.True(t, resp.CanReject)
		assert.True(t, resp.CanHide)
	})

	t.Run("approve pending appends history", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+pendingID.String()+"/approve", strings.NewReader(`{"note":"looks good"}`))
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.ApproveExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductResponse(t, w)
		assert.Equal(t, "approved", resp.ReviewStatus)
		assert.False(t, resp.CanApprove)
		require.Len(t, store.history, 2)
		assert.Equal(t, entity.ExternalProductReviewStatusApproved, store.history[len(store.history)-1].ToStatus)
	})

	t.Run("reject pending appends history", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+pendingID.String()+"/reject", strings.NewReader(`{"reason":"policy violation"}`))
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.RejectExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductResponse(t, w)
		assert.Equal(t, "rejected", resp.ReviewStatus)
		require.Len(t, store.history, 2)
		assert.Equal(t, entity.ExternalProductReviewStatusRejected, store.history[len(store.history)-1].ToStatus)
	})

	t.Run("request changes pending appends history", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+pendingID.String()+"/request-changes", strings.NewReader(`{"reason":"add more detail"}`))
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.RequestChangesExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductResponse(t, w)
		// request_changes is now a canonical status distinct from "rejected"
		assert.Equal(t, "request_changes", resp.ReviewStatus)
		require.Len(t, store.history, 2)
		assert.Equal(t, entity.ExternalProductReviewStatusRequestChanges, store.history[len(store.history)-1].ToStatus)
	})

	t.Run("hide approved appends history", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		approvedID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusApproved, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+approvedID.String()+"/hide", strings.NewReader(`{"reason":"takedown"}`))
		c.Params = gin.Params{{Key: "id", Value: approvedID.String()}}

		handler.HideExternalProduct(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductResponse(t, w)
		assert.Equal(t, "hidden", resp.ReviewStatus)
		require.Len(t, store.history, 2)
		assert.Equal(t, entity.ExternalProductReviewStatusHidden, store.history[len(store.history)-1].ToStatus)
	})

	t.Run("approve draft blocked", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		draftID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusDraft, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/external-products/"+draftID.String()+"/approve", strings.NewReader(`{"note":"not allowed"}`))
		c.Params = gin.Params{{Key: "id", Value: draftID.String()}}

		handler.ApproveExternalProduct(c)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("history endpoint returns rows", func(t *testing.T) {
		store := buildStore()
		handler := newTestExternalProductHandler(store)
		pendingID := pickProductID(store, ownerID, entity.ExternalProductReviewStatusPendingReview, false)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", adminID)
		c.Request = httptest.NewRequest(http.MethodGet, "/admin/external-products/"+pendingID.String()+"/reviews", nil)
		c.Params = gin.Params{{Key: "id", Value: pendingID.String()}}

		handler.ListAdminExternalProductReviews(c)

		require.Equal(t, http.StatusOK, w.Code)
		resp := decodeAdminExternalProductReviewHistoryListResponse(t, w)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, pendingID, resp.Items[0].ExternalProductID)
	})
}

func TestExternalProductLifecycle_PublicVisibleFalseAndActivationStillBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &externalProductHandlerStore{
		now:        time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC),
		products:   map[uuid.UUID]*entity.ExternalProduct{},
		media:      map[uuid.UUID]*entity.ExternalProductMedia{},
		promotions: map[uuid.UUID]*entity.PromotionInstance{},
	}
	handler := newTestExternalProductHandler(store)

	ownerID := uuid.New()
	product, err := entity.NewExternalProductDraft(ownerID, "Fresh Fish", nil, "https://example.com/fish", store.now)
	require.NoError(t, err)
	product.OwnerUserID = ownerID
	store.products[product.ID] = product

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", ownerID)
	c.Request = httptest.NewRequest(http.MethodPost, "/external-products", strings.NewReader(`{"title":"Fresh Fish 2","external_url":"https://example.com/fish-2"}`))

	handler.CreateExternalProduct(c)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeExternalProductResponse(t, w)
	assert.False(t, resp.PublicVisible)

	product.ReviewStatus = entity.ExternalProductReviewStatusApproved
	media, err := entity.NewExternalProductMedia(product.ID, entity.ExternalProductMediaTypeImage, "s3://bucket/fish.jpg", "https://cdn.example.com/fish.jpg", ptrString("https://cdn.example.com/fish-thumb.jpg"), 0, nil, store.now)
	require.NoError(t, err)
	store.media[media.ID] = media
	store.products[product.ID] = product
	store.promotions[uuid.New()] = &entity.PromotionInstance{
		ID:          uuid.New(),
		OwnershipID: uuid.New(),
		UserID:      ownerID,
		TargetType:  entity.TargetTypeExternalProduct,
		TargetID:    &product.ID,
		Status:      entity.InstanceStatusActive,
		CreatedAt:   store.now,
		UpdatedAt:   store.now,
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("user_id", ownerID)
	c.Request = httptest.NewRequest(http.MethodGet, "/external-products/"+product.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: product.ID.String()}}

	handler.GetExternalProduct(c)

	require.Equal(t, http.StatusOK, w.Code)
	resp = decodeExternalProductResponse(t, w)
	assert.True(t, resp.PublicVisible)
}

func pickAnyProductID(store *externalProductHandlerStore, status entity.ExternalProductReviewStatus) uuid.UUID {
	for id, product := range store.products {
		if product != nil && product.ReviewStatus == status {
			return id
		}
	}
	return uuid.Nil
}

func pickProductID(store *externalProductHandlerStore, ownerID uuid.UUID, status entity.ExternalProductReviewStatus, includeDeleted bool) uuid.UUID {
	for id, product := range store.products {
		if product == nil {
			continue
		}
		if product.OwnerUserID != ownerID {
			continue
		}
		if product.ReviewStatus != status {
			continue
		}
		if !includeDeleted && product.DeletedAt != nil {
			continue
		}
		return id
	}
	return uuid.Nil
}

func pickDeletedProductID(store *externalProductHandlerStore) uuid.UUID {
	for id, product := range store.products {
		if product != nil && product.DeletedAt != nil {
			return id
		}
	}
	return uuid.Nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func decodeExternalProductResponseEnvelope(t *testing.T, body *httptest.ResponseRecorder) []byte {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))
	return envelope.Data
}

func decodeAdminExternalProductResponse(t *testing.T, body *httptest.ResponseRecorder) AdminExternalProductResponse {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))

	var resp AdminExternalProductResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &resp))
	return resp
}

func decodeAdminExternalProductListResponse(t *testing.T, body *httptest.ResponseRecorder) AdminExternalProductListResponse {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))

	var resp AdminExternalProductListResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &resp))
	return resp
}

func decodeAdminExternalProductReviewHistoryListResponse(t *testing.T, body *httptest.ResponseRecorder) ExternalProductReviewHistoryListResponse {
	t.Helper()

	var envelope apiResponseEnvelope
	require.NoError(t, json.Unmarshal(body.Body.Bytes(), &envelope))

	var resp ExternalProductReviewHistoryListResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &resp))
	return resp
}

func ptrReviewStatus(status entity.ExternalProductReviewStatus) *entity.ExternalProductReviewStatus {
	return &status
}

func ptrString(v string) *string {
	return &v
}
