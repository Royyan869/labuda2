package application_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type allowAllChecker struct {
	operable bool
	reason   string
}

func (c allowAllChecker) CheckOperability(
	ctx context.Context,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) (bool, string, error) {
	return c.operable, c.reason, nil
}

func (allowAllChecker) ValidateOwnership(
	ctx context.Context,
	userID uuid.UUID,
	targetType entity.TargetType,
	targetID *uuid.UUID,
) error {
	return nil
}

func (c allowAllChecker) CheckUserEligibility(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	return c.operable, c.reason, nil
}

type authorityTestTx struct {
	now time.Time

	instance  *entity.PromotionInstance
	ownership *entity.PromotionOwnership

	instanceByIDCalls      int
	instanceForUpdateCalls int
	ownershipBakeCalls     int
	updateInstanceCalls    int
}

func (t *authorityTestTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "select now()"):
		return &authorityTestRow{values: []any{t.now}}
	case strings.Contains(normalized, "select consumed_duration_hours, total_duration_hours"):
		if t.ownership == nil {
			return &authorityTestRow{err: pgx.ErrNoRows}
		}
		return &authorityTestRow{values: []any{t.ownership.ConsumedDurationHours, t.ownership.TotalDurationHours}}
	case strings.Contains(normalized, "from promotion_instances"):
		if t.instance == nil {
			return &authorityTestRow{err: pgx.ErrNoRows}
		}
		if strings.Contains(normalized, "for update") {
			t.instanceForUpdateCalls++
		} else {
			t.instanceByIDCalls++
		}
		return &authorityTestRow{values: instanceRowValues(t.instance)}
	case strings.Contains(normalized, "from promotion_ownerships"):
		if t.ownership == nil {
			return &authorityTestRow{err: pgx.ErrNoRows}
		}
		return &authorityTestRow{values: ownershipRowValues(t.ownership)}
	default:
		return &authorityTestRow{err: fmt.Errorf("unexpected query: %s", sql)}
	}
}

func (t *authorityTestTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &authorityTestRows{}, nil
}

func (t *authorityTestTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	normalized := strings.ToLower(sql)

	switch {
	case strings.Contains(normalized, "update promotion_instances"):
		t.updateInstanceCalls++
		if t.instance != nil && len(args) >= 11 {
			t.instance.Status = entity.InstanceStatus(args[1].(string))
			t.instance.ActivatedAt = cloneTimePtr(args[2])
			t.instance.StoppedAt = cloneTimePtr(args[3])
			t.instance.StopReason = cloneStringPtr(args[4])
			t.instance.PausedAt = cloneTimePtr(args[5])
			t.instance.TotalPausedDuration = args[6].(int)
			t.instance.Finalized = args[7].(bool)
			t.instance.FinalizedAt = cloneTimePtr(args[8])
			t.instance.FinalizedSeconds = args[9].(int)
			t.instance.UpdatedAt = args[10].(time.Time)
		}
	case strings.Contains(normalized, "update promotion_ownerships") &&
		strings.Contains(normalized, "consumed_duration_hours = consumed_duration_hours +"):
		t.ownershipBakeCalls++
		if t.ownership != nil && len(args) >= 2 {
			hours := args[1].(int)
			t.ownership.ConsumedDurationHours += hours
			if t.ownership.ConsumedDurationHours > t.ownership.TotalDurationHours {
				t.ownership.ConsumedDurationHours = t.ownership.TotalDurationHours
			}
			if t.ownership.ConsumedDurationHours >= t.ownership.TotalDurationHours {
				t.ownership.Status = entity.OwnershipStatusConsumed
			}
		}
	}

	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (t *authorityTestTx) Commit(ctx context.Context) error   { return nil }
func (t *authorityTestTx) Rollback(ctx context.Context) error { return nil }

type authorityTestRow struct {
	values []any
	err    error
}

func (r *authorityTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return fmt.Errorf("scan argument count mismatch: have %d, want %d", len(r.values), len(dest))
	}
	for i, value := range r.values {
		if err := assignAuthorityValue(dest[i], value); err != nil {
			return err
		}
	}
	return nil
}

type authorityTestRows struct{}

func (r *authorityTestRows) Next() bool                                   { return false }
func (r *authorityTestRows) Err() error                                   { return nil }
func (r *authorityTestRows) Close()                                       {}
func (r *authorityTestRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *authorityTestRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *authorityTestRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *authorityTestRows) RawValues() [][]byte                          { return nil }
func (r *authorityTestRows) Values() ([]any, error)                       { return nil, nil }
func (r *authorityTestRows) Scan(dest ...any) error                       { return errors.New("no rows") }
func (r *authorityTestRows) Conn() *pgx.Conn                              { return nil }

func assignAuthorityValue(dest any, value any) error {
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

func cloneTimePtr(value any) *time.Time {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		copy := v
		return &copy
	case *time.Time:
		if v == nil {
			return nil
		}
		copy := *v
		return &copy
	default:
		return nil
	}
}

func cloneStringPtr(value any) *string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		copy := v
		return &copy
	case *string:
		if v == nil {
			return nil
		}
		copy := *v
		return &copy
	default:
		return nil
	}
}

func instanceRowValues(instance *entity.PromotionInstance) []any {
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

func ownershipRowValues(ownership *entity.PromotionOwnership) []any {
	return []any{
		ownership.ID,
		ownership.UserID,
		ownership.PackageID,
		string(ownership.Status),
		ownership.PurchasedAt,
		ownership.ExpiresAt,
		ownership.TotalDurationHours,
		ownership.ConsumedDurationHours,
		ownership.SourceBillingID, // added: source_billing_id (nullable)
		ownership.CreatedAt,
		ownership.UpdatedAt,
	}
}

func newAuthorityTestServiceState(t *testing.T, runDuration time.Duration) (*application.PromotionService, *authorityTestTx, *entity.PromotionInstance, *entity.PromotionOwnership) {
	t.Helper()

	instance := createActiveInstanceWithDuration(t, runDuration)
	ownership := newAuthorityTestOwnership(t, instance, 72)
	tx := &authorityTestTx{
		now:       time.Now(),
		instance:  instance,
		ownership: ownership,
	}

	service := application.NewPromotionService(allowAllChecker{operable: true})
	return service, tx, instance, ownership
}

func newAuthorityTestOwnership(t *testing.T, instance *entity.PromotionInstance, totalHours int) *entity.PromotionOwnership {
	t.Helper()

	ownership, err := entity.NewPromotionOwnership(
		instance.UserID,
		uuid.New(),
		totalHours,
		336,
		time.Now(),
	)
	require.NoError(t, err)
	ownership.ID = instance.OwnershipID
	return ownership
}

func TestPromotionService_ApplyOperabilityRecommendation_Pause(t *testing.T) {
	service, tx, instance, _ := newAuthorityTestServiceState(t, 5*time.Second)

	recommendation := application.OperabilityRecommendation{
		Action:      application.OperabilityRecommendationPause,
		InstanceID:  instance.ID,
		OwnershipID: instance.OwnershipID,
		UserID:      instance.UserID,
		TargetType:  instance.TargetType,
		TargetID:    instance.TargetID,
	}

	err := service.ApplyOperabilityRecommendation(context.Background(), tx, recommendation)
	require.NoError(t, err)

	assert.Equal(t, 1, tx.updateInstanceCalls)
	assert.Equal(t, entity.InstanceStatusPaused, instance.Status)
	assert.False(t, instance.Finalized)
	assert.Equal(t, 0, tx.ownershipBakeCalls)
}

func TestPromotionService_ApplyOperabilityRecommendation_Resume(t *testing.T) {
	service, tx, instance, ownership := newAuthorityTestServiceState(t, 5*time.Second)

	pauseTime := time.Now().Add(-3 * time.Second)
	require.NoError(t, instance.Pause(pauseTime))
	tx.now = pauseTime.Add(5 * time.Second)
	ownership.Status = entity.OwnershipStatusAvailable
	ownership.ExpiresAt = tx.now.Add(24 * time.Hour)

	recommendation := application.OperabilityRecommendation{
		Action:      application.OperabilityRecommendationResume,
		InstanceID:  instance.ID,
		OwnershipID: instance.OwnershipID,
		UserID:      instance.UserID,
		TargetType:  instance.TargetType,
		TargetID:    instance.TargetID,
	}

	err := service.ApplyOperabilityRecommendation(context.Background(), tx, recommendation)
	require.NoError(t, err)

	assert.Equal(t, entity.InstanceStatusActive, instance.Status)
	assert.Nil(t, instance.PausedAt)
	assert.Greater(t, instance.TotalPausedDuration, 0)
	assert.Equal(t, 1, tx.updateInstanceCalls)
	assert.Equal(t, 0, tx.ownershipBakeCalls)
}

func TestPromotionService_ApplyOperabilityRecommendation_StopUsesInstanceForUpdate(t *testing.T) {
	service, tx, instance, _ := newAuthorityTestServiceState(t, 5*time.Second)

	recommendation := application.OperabilityRecommendation{
		Action:      application.OperabilityRecommendationStop,
		Reason:      string(entity.StopReasonUserCancelled),
		InstanceID:  instance.ID,
		OwnershipID: instance.OwnershipID,
		UserID:      instance.UserID,
		TargetType:  instance.TargetType,
		TargetID:    instance.TargetID,
	}

	err := service.ApplyOperabilityRecommendation(context.Background(), tx, recommendation)
	require.NoError(t, err)

	assert.Equal(t, 1, tx.instanceForUpdateCalls)
	assert.Equal(t, 1, tx.ownershipBakeCalls)
	assert.True(t, instance.Finalized)
	assert.Equal(t, entity.InstanceStatusCancelled, instance.Status)
}

func TestPromotionService_ApplyOperabilityRecommendation_DoubleFinalizationPrevented(t *testing.T) {
	service, tx, instance, _ := newAuthorityTestServiceState(t, 5*time.Second)

	recommendation := application.OperabilityRecommendation{
		Action:      application.OperabilityRecommendationStop,
		Reason:      string(entity.StopReasonUserCancelled),
		InstanceID:  instance.ID,
		OwnershipID: instance.OwnershipID,
		UserID:      instance.UserID,
		TargetType:  instance.TargetType,
		TargetID:    instance.TargetID,
	}

	err := service.ApplyOperabilityRecommendation(context.Background(), tx, recommendation)
	require.NoError(t, err)

	err = service.ApplyOperabilityRecommendation(context.Background(), tx, recommendation)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already stopped")
	assert.Equal(t, 2, tx.instanceForUpdateCalls)
	assert.Equal(t, 1, tx.ownershipBakeCalls)
	assert.True(t, instance.Finalized)
}

func TestOperabilityCheckerSweepIsReadOnlyAtBoundary(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	checkerPath := filepath.Join(filepath.Dir(testFile), "operability_checker.go")
	src, err := os.ReadFile(checkerPath)
	require.NoError(t, err)
	body := string(src)

	assert.Contains(t, body, "func (c *OperabilityCheckerImpl) SweepInactivePromotions")
	assert.Contains(t, body, "func (c *OperabilityCheckerImpl) SweepPausedPromotions")
	assert.NotContains(t, body, "AddConsumedDurationToOwnership(")
	assert.NotContains(t, body, "UpdateInstance(")
	assert.NotContains(t, body, "instance.Pause(")
	assert.NotContains(t, body, "instance.Resume(")
	assert.NotContains(t, body, "instance.Stop(")
}
