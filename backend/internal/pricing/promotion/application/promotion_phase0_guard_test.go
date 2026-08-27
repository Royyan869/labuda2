package application_test

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
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type phase5PromotionChecker struct {
	ownershipByTarget map[uuid.UUID]uuid.UUID
	gates             map[uuid.UUID]phase5ExternalProductGate
}

type phase5ExternalProductGate struct {
	operable bool
	reason   string
}

func (c phase5PromotionChecker) CheckOperability(
	ctx context.Context,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) (bool, string, error) {
	if targetType != entity.TargetTypeExternalProduct || targetID == nil {
		return true, "", nil
	}
	gate, ok := c.gates[*targetID]
	if !ok {
		return false, "external_product_not_found", nil
	}
	return gate.operable, gate.reason, nil
}

func (c phase5PromotionChecker) ValidateOwnership(
	ctx context.Context,
	userID uuid.UUID,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) error {
	if targetType != entity.TargetTypeExternalProduct || targetID == nil {
		return nil
	}
	ownerID, ok := c.ownershipByTarget[*targetID]
	if !ok {
		return fmt.Errorf("external product not found")
	}
	if ownerID != userID {
		return fmt.Errorf("user does not own this external product")
	}
	return nil
}

func (c phase5PromotionChecker) CheckUserEligibility(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	return true, "", nil
}

type phase5PromotionTx struct {
	now time.Time

	ownership *entity.PromotionOwnership
	pkg       *entity.PromotionPackage

	instances            map[uuid.UUID]*entity.PromotionInstance
	activeInstanceByOwn  map[uuid.UUID]*entity.PromotionInstance
	activeInstanceByTarg map[string]*entity.PromotionInstance
}

var _ db.Tx = (*phase5PromotionTx)(nil)

func newPhase5PromotionTx(now time.Time) *phase5PromotionTx {
	return &phase5PromotionTx{
		now:                  now,
		instances:            make(map[uuid.UUID]*entity.PromotionInstance),
		activeInstanceByOwn:  make(map[uuid.UUID]*entity.PromotionInstance),
		activeInstanceByTarg: make(map[string]*entity.PromotionInstance),
	}
}

func (t *phase5PromotionTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "insert into promotion_instances"):
		instance := phase5PromotionInstanceFromInsertArgs(args)
		t.instances[instance.ID] = instance
		if instance.Status == entity.InstanceStatusActive {
			t.activeInstanceByOwn[instance.OwnershipID] = instance
			t.activeInstanceByTarg[phase5TargetKey(instance.TargetType, instance.TargetID)] = instance
		}
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(normalized, "update promotion_instances"):
		instanceID := args[0].(uuid.UUID)
		existing := t.instances[instanceID]
		if existing == nil {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		existing.Status = entity.InstanceStatus(args[1].(string))
		existing.ActivatedAt = clonePhase5TimePtr(args[2])
		existing.StoppedAt = clonePhase5TimePtr(args[3])
		existing.StopReason = clonePhase5StringPtr(args[4])
		existing.PausedAt = clonePhase5TimePtr(args[5])
		existing.TotalPausedDuration = args[6].(int)
		existing.Finalized = args[7].(bool)
		existing.FinalizedAt = clonePhase5TimePtr(args[8])
		existing.FinalizedSeconds = args[9].(int)
		existing.UpdatedAt = args[10].(time.Time)
		if existing.Status == entity.InstanceStatusActive {
			t.activeInstanceByOwn[existing.OwnershipID] = existing
			t.activeInstanceByTarg[phase5TargetKey(existing.TargetType, existing.TargetID)] = existing
		} else {
			delete(t.activeInstanceByOwn, existing.OwnershipID)
			delete(t.activeInstanceByTarg, phase5TargetKey(existing.TargetType, existing.TargetID))
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(normalized, "update promotion_ownerships") &&
		strings.Contains(normalized, "consumed_duration_hours = consumed_duration_hours +"):
		if t.ownership == nil || len(args) < 2 {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		hours := args[1].(int)
		t.ownership.ConsumedDurationHours += hours
		if t.ownership.ConsumedDurationHours > t.ownership.TotalDurationHours {
			t.ownership.ConsumedDurationHours = t.ownership.TotalDurationHours
		}
		if t.ownership.ConsumedDurationHours >= t.ownership.TotalDurationHours {
			t.ownership.Status = entity.OwnershipStatusConsumed
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
}

func (t *phase5PromotionTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &phase5PromotionRows{}, nil
}

func (t *phase5PromotionTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "select now()"):
		return &phase5PromotionRow{values: []any{t.now}}
	case strings.Contains(normalized, "select consumed_duration_hours, total_duration_hours"):
		if t.ownership == nil {
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
		return &phase5PromotionRow{values: []any{t.ownership.ConsumedDurationHours, t.ownership.TotalDurationHours}}
	case strings.Contains(normalized, "from promotion_ownerships"):
		if t.ownership == nil {
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
		return &phase5PromotionRow{values: phase5PromotionOwnershipValues(t.ownership)}
	case strings.Contains(normalized, "from promotion_packages"):
		if t.pkg == nil {
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
		return &phase5PromotionRow{values: phase5PromotionPackageValues(t.pkg)}
	case strings.Contains(normalized, "select exists(") && strings.Contains(normalized, "from promotion_instances"):
		targetType, _ := args[0].(string)
		targetID, _ := args[1].(uuid.UUID)
		_, ok := t.activeInstanceByTarg[targetType+":"+targetID.String()]
		return &phase5PromotionRow{values: []any{ok}}
	case strings.Contains(normalized, "from promotion_instances"):
		if strings.Contains(normalized, "target_type = $1") && strings.Contains(normalized, "target_id = $2") {
			targetType, _ := args[0].(string)
			targetID, _ := args[1].(uuid.UUID)
			if inst, ok := t.activeInstanceByTarg[targetType+":"+targetID.String()]; ok {
				return &phase5PromotionRow{values: phase5PromotionInstanceValues(inst)}
			}
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "ownership_id = $1") {
			ownershipID, _ := args[0].(uuid.UUID)
			if inst, ok := t.activeInstanceByOwn[ownershipID]; ok {
				return &phase5PromotionRow{values: phase5PromotionInstanceValues(inst)}
			}
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "id = $1") {
			instanceID, _ := args[0].(uuid.UUID)
			if inst, ok := t.instances[instanceID]; ok {
				return &phase5PromotionRow{values: phase5PromotionInstanceValues(inst)}
			}
			return &phase5PromotionRow{err: pgx.ErrNoRows}
		}
	}

	return &phase5PromotionRow{err: fmt.Errorf("unexpected query: %s", sql)}
}

func (t *phase5PromotionTx) Commit(context.Context) error   { return nil }
func (t *phase5PromotionTx) Rollback(context.Context) error { return nil }

type phase5PromotionRow struct {
	values []any
	err    error
}

func (r *phase5PromotionRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignPhase5Value(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type phase5PromotionRows struct{}

func (r *phase5PromotionRows) Next() bool                                   { return false }
func (r *phase5PromotionRows) Scan(dest ...any) error                       { return errors.New("no rows") }
func (r *phase5PromotionRows) Err() error                                   { return nil }
func (r *phase5PromotionRows) Close()                                       {}
func (r *phase5PromotionRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *phase5PromotionRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *phase5PromotionRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *phase5PromotionRows) RawValues() [][]byte                          { return nil }
func (r *phase5PromotionRows) Values() ([]any, error)                       { return nil, nil }
func (r *phase5PromotionRows) Conn() *pgx.Conn                              { return nil }

func assignPhase5Value(dest any, value any) error {
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

func phase5PromotionOwnershipValues(ownership *entity.PromotionOwnership) []any {
	return []any{
		ownership.ID,
		ownership.UserID,
		ownership.PackageID,
		string(ownership.Status),
		ownership.PurchasedAt,
		ownership.ExpiresAt,
		ownership.TotalDurationHours,
		ownership.ConsumedDurationHours,
		ownership.SourceBillingID, // nullable; added alongside migration 000191
		ownership.CreatedAt,
		ownership.UpdatedAt,
	}
}

func phase5PromotionPackageValues(pkg *entity.PromotionPackage) []any {
	allowedTypes := make([]string, len(pkg.AllowedTargetTypes))
	for i, tt := range pkg.AllowedTargetTypes {
		allowedTypes[i] = string(tt)
	}
	return []any{
		pkg.ID,
		pkg.Name,
		pkg.TotalDurationHours,
		pkg.ValidityWindowHours,
		pkg.PriceAmount,
		allowedTypes,
		pkg.IsActive,
		pkg.CreatedAt,
	}
}

func phase5PromotionInstanceValues(instance *entity.PromotionInstance) []any {
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

func phase5PromotionInstanceFromInsertArgs(args []any) *entity.PromotionInstance {
	return &entity.PromotionInstance{
		ID:                  args[0].(uuid.UUID),
		OwnershipID:         args[1].(uuid.UUID),
		UserID:              args[2].(uuid.UUID),
		TargetType:          entity.TargetType(args[3].(string)),
		TargetID:            clonePhase5UUIDPtr(args[4]),
		Status:              entity.InstanceStatus(args[5].(string)),
		ActivatedAt:         clonePhase5TimePtr(args[6]),
		StoppedAt:           clonePhase5TimePtr(args[7]),
		StopReason:          clonePhase5StringPtr(args[8]),
		PausedAt:            clonePhase5TimePtr(args[9]),
		TotalPausedDuration: args[10].(int),
		Finalized:           args[11].(bool),
		FinalizedAt:         clonePhase5TimePtr(args[12]),
		FinalizedSeconds:    args[13].(int),
		CreatedAt:           args[14].(time.Time),
		UpdatedAt:           args[15].(time.Time),
	}
}

func clonePhase5StringPtr(value any) *string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case *string:
		if v == nil {
			return nil
		}
		out := *v
		return &out
	case string:
		out := v
		return &out
	default:
		return nil
	}
}

func clonePhase5TimePtr(value any) *time.Time {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case *time.Time:
		if v == nil {
			return nil
		}
		out := *v
		return &out
	case time.Time:
		out := v
		return &out
	default:
		return nil
	}
}

func clonePhase5UUIDPtr(value any) *uuid.UUID {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case *uuid.UUID:
		if v == nil {
			return nil
		}
		out := *v
		return &out
	case uuid.UUID:
		out := v
		return &out
	default:
		return nil
	}
}

func phase5TargetKey(targetType entity.TargetType, targetID *uuid.UUID) string {
	if targetID == nil {
		return string(targetType) + ":"
	}
	return string(targetType) + ":" + targetID.String()
}

func newPhase5Ownership(t *testing.T, userID uuid.UUID, pkgID uuid.UUID) *entity.PromotionOwnership {
	t.Helper()
	ownership, err := entity.NewPromotionOwnership(userID, pkgID, 24, 72, time.Now())
	require.NoError(t, err)
	ownership.Status = entity.OwnershipStatusAvailable
	return ownership
}

func newPhase5Package(t *testing.T) *entity.PromotionPackage {
	t.Helper()
	pkg, err := entity.NewPromotionPackage(
		"Starter",
		24,
		72,
		1500,
		[]entity.TargetType{entity.TargetTypeForSale, entity.TargetTypeAuction, entity.TargetTypeExternalProduct},
	)
	require.NoError(t, err)
	pkg.IsActive = true
	pkg.CreatedAt = time.Now()
	return pkg
}

func newPhase5Service() *application.PromotionService {
	return application.NewPromotionService(phase5PromotionChecker{
		ownershipByTarget: make(map[uuid.UUID]uuid.UUID),
		gates:             make(map[uuid.UUID]phase5ExternalProductGate),
	})
}

func TestPromotionService_ActivatePromotion_ExternalProductGateMatrix(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()
	ownershipID := uuid.New()
	pkg := newPhase5Package(t)
	ownership := newPhase5Ownership(t, userID, pkg.ID)
	ownership.ID = ownershipID
	tx := newPhase5PromotionTx(now)
	tx.ownership = ownership
	tx.pkg = pkg

	externalProductID := uuid.New()
	checker := phase5PromotionChecker{
		ownershipByTarget: map[uuid.UUID]uuid.UUID{
			externalProductID: userID,
		},
		gates: map[uuid.UUID]phase5ExternalProductGate{
			externalProductID: {operable: false, reason: "external_product_not_approved"},
		},
	}
	service := application.NewPromotionService(checker)

	cases := []struct {
		name       string
		gate       phase5ExternalProductGate
		wantOK     bool
		wantReason string
	}{
		{name: "draft", gate: phase5ExternalProductGate{operable: false, reason: "external_product_not_approved"}, wantOK: false, wantReason: "external_product_not_approved"},
		{name: "pending_review", gate: phase5ExternalProductGate{operable: false, reason: "external_product_not_approved"}, wantOK: false, wantReason: "external_product_not_approved"},
		{name: "rejected", gate: phase5ExternalProductGate{operable: false, reason: "external_product_not_approved"}, wantOK: false, wantReason: "external_product_not_approved"},
		{name: "hidden", gate: phase5ExternalProductGate{operable: false, reason: "external_product_not_approved"}, wantOK: false, wantReason: "external_product_not_approved"},
		{name: "approved_without_media", gate: phase5ExternalProductGate{operable: false, reason: "external_product_missing_media"}, wantOK: false, wantReason: "external_product_missing_media"},
		{name: "approved_with_media", gate: phase5ExternalProductGate{operable: true, reason: ""}, wantOK: true},
		{name: "owner_ineligible", gate: phase5ExternalProductGate{operable: false, reason: "seller_account_inactive"}, wantOK: false, wantReason: "seller_account_inactive"},
		{name: "invalid_deleted", gate: phase5ExternalProductGate{operable: false, reason: "external_product_not_found"}, wantOK: false, wantReason: "external_product_not_found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checker.gates[externalProductID] = tc.gate
			service = application.NewPromotionService(checker)

			result, err := service.ActivatePromotion(context.Background(), tx, application.ActivatePromotionInput{
				OwnershipID: ownershipID,
				UserID:      userID,
				TargetType:  entity.TargetTypeExternalProduct,
				TargetID:    &externalProductID,
			})

			if !tc.wantOK {
				require.Error(t, err)
				var targetErr *application.TargetNotOperableError
				require.ErrorAs(t, err, &targetErr)
				assert.Equal(t, tc.wantReason, targetErr.Reason)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Instance)
			assert.Equal(t, entity.TargetTypeExternalProduct, result.Instance.TargetType)
			assert.Equal(t, externalProductID, *result.Instance.TargetID)
			assert.Equal(t, entity.InstanceStatusActive, result.Instance.Status)
			require.NotNil(t, result.Instance.ActivatedAt)
		})
	}
}

func TestPromotionService_ActivatePromotion_TargetTypesUnchangedForForSaleAndAuction(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()
	pkg := newPhase5Package(t)
	ownership := newPhase5Ownership(t, userID, pkg.ID)
	tx := newPhase5PromotionTx(now)
	tx.ownership = ownership
	tx.pkg = pkg
	service := application.NewPromotionService(phase5PromotionChecker{
		ownershipByTarget: make(map[uuid.UUID]uuid.UUID),
		gates:             make(map[uuid.UUID]phase5ExternalProductGate),
	})

	for _, targetType := range []entity.TargetType{entity.TargetTypeForSale, entity.TargetTypeAuction} {
		t.Run(string(targetType), func(t *testing.T) {
			targetID := uuid.New()
			result, err := service.ActivatePromotion(context.Background(), tx, application.ActivatePromotionInput{
				OwnershipID: ownership.ID,
				UserID:      userID,
				TargetType:  targetType,
				TargetID:    &targetID,
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, targetType, result.Instance.TargetType)
			assert.Equal(t, targetID, *result.Instance.TargetID)
		})
	}
}

func TestPromotionService_IsTargetPromotedInTx_ExternalProductGateMatrix(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	targetID := uuid.New()
	ownerID := uuid.New()
	tx := newPhase5PromotionTx(now)
	instance := &entity.PromotionInstance{
		ID:          uuid.New(),
		OwnershipID: uuid.New(),
		UserID:      ownerID,
		TargetType:  entity.TargetTypeExternalProduct,
		TargetID:    &targetID,
		Status:      entity.InstanceStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	tx.instances[instance.ID] = instance
	tx.activeInstanceByTarg[phase5TargetKey(entity.TargetTypeExternalProduct, &targetID)] = instance

	checker := phase5PromotionChecker{
		ownershipByTarget: map[uuid.UUID]uuid.UUID{targetID: ownerID},
		gates: map[uuid.UUID]phase5ExternalProductGate{
			targetID: {operable: true, reason: ""},
		},
	}
	service := application.NewPromotionService(checker)

	ok, err := service.IsTargetPromotedInTx(context.Background(), tx, entity.TargetTypeExternalProduct, targetID)
	require.NoError(t, err)
	assert.True(t, ok)

	checker.gates[targetID] = phase5ExternalProductGate{operable: false, reason: "external_product_not_approved"}
	service = application.NewPromotionService(checker)
	ok, err = service.IsTargetPromotedInTx(context.Background(), tx, entity.TargetTypeExternalProduct, targetID)
	require.NoError(t, err)
	assert.False(t, ok)

	delete(tx.activeInstanceByTarg, phase5TargetKey(entity.TargetTypeExternalProduct, &targetID))
	service = application.NewPromotionService(checker)
	ok, err = service.IsTargetPromotedInTx(context.Background(), tx, entity.TargetTypeExternalProduct, targetID)
	require.NoError(t, err)
	assert.False(t, ok)
}
