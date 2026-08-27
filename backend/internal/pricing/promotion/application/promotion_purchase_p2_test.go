package application_test

// promotion_purchase_p2_test.go — P2-A / P2-C regression lock
//
// Verifies:
//   1. PurchasePackage stores SourceBillingID on the created ownership.
//   2. A second PurchasePackage call with the same BillingID fails at the
//      repository layer (unique constraint violation), preventing duplicate ownership.
//   3. PurchasePackage with a zero BillingID leaves SourceBillingID nil (test/seed path).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// purchaseP2MockRepo records the ownership passed to CreateOwnership and
// can simulate a unique violation on a second call for the same billing ID.
type purchaseP2MockRepo struct {
	pkg              *entity.PromotionPackage
	createdOwnership *entity.PromotionOwnership
	seenBillingIDs   map[uuid.UUID]bool
}

func newPurchaseP2MockRepo(pkg *entity.PromotionPackage) *purchaseP2MockRepo {
	return &purchaseP2MockRepo{
		pkg:            pkg,
		seenBillingIDs: make(map[uuid.UUID]bool),
	}
}

func (r *purchaseP2MockRepo) GetDBTime(_ context.Context, _ db.Tx) (time.Time, error) {
	return time.Now(), nil
}
func (r *purchaseP2MockRepo) GetPackageByID(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.PromotionPackage, error) {
	if r.pkg == nil || r.pkg.ID != id {
		return nil, nil
	}
	return r.pkg, nil
}
func (r *purchaseP2MockRepo) CreateOwnership(_ context.Context, _ db.Tx, o *entity.PromotionOwnership) error {
	// Simulate the DB unique index on source_billing_id WHERE NOT NULL.
	if o.SourceBillingID != nil && *o.SourceBillingID != uuid.Nil {
		if r.seenBillingIDs[*o.SourceBillingID] {
			// Simulate pgx unique violation (code 23505).
			return &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
		}
		r.seenBillingIDs[*o.SourceBillingID] = true
	}
	r.createdOwnership = o
	return nil
}

// Remaining interface methods — panic if called (not exercised by these tests).
func (r *purchaseP2MockRepo) CreatePackage(_ context.Context, _ db.Tx, _ *entity.PromotionPackage) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListPackages(_ context.Context, _ db.Tx, _ bool) ([]*entity.PromotionPackage, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) UpdatePackage(_ context.Context, _ db.Tx, _ *entity.PromotionPackage) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListCampaignsAdmin(_ context.Context, _ db.Tx, _ promotionRepo.AdminCampaignFilter) ([]*promotionRepo.AdminCampaignRow, int, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetOwnershipByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetOwnershipForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetOwnershipWithInstances(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionOwnership, []*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) UpdateOwnership(_ context.Context, _ db.Tx, _ *entity.PromotionOwnership) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) AddConsumedDurationToOwnership(_ context.Context, _ db.Tx, _ uuid.UUID, _ int) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListOwnershipsByUser(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.OwnershipStatus) ([]*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListOwnershipsByUserPaginated(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.OwnershipStatus, _, _ int) ([]*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) FindActiveOwnershipByUserAndPackage(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListExpiredOwnerships(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionOwnership, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) CreateInstance(_ context.Context, _ db.Tx, _ *entity.PromotionInstance) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetInstanceByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetInstanceForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) UpdateInstance(_ context.Context, _ db.Tx, _ *entity.PromotionInstance) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListInstancesByUser(_ context.Context, _ db.Tx, _ uuid.UUID, _ entity.InstanceStatus) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListInstancesByOwnership(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetActiveInstanceByOwnership(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetActiveInstanceByOwnershipForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetActiveInstancesByTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetPausedInstancesByTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetActiveInstancesForDiscovery(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetAllActiveInstances(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetAllPausedInstances(_ context.Context, _ db.Tx, _ int) ([]*entity.PromotionInstance, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) HasActivePromotionForTarget(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) (bool, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetActiveInstanceByTargetForUpdate(_ context.Context, _ db.Tx, _ entity.TargetType, _ uuid.UUID) (*entity.PromotionInstance, error) {
	panic("not exercised")
}

// External product stubs
func (r *purchaseP2MockRepo) CreateDraft(_ context.Context, _ db.Tx, _ *entity.ExternalProduct) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) UpdateOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID, _ entity.ExternalProductUpdateInput) (*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) UpdateByID(_ context.Context, _ db.Tx, _ *entity.ExternalProduct) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) SubmitOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ResubmitOwned(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetOwnedByID(_ context.Context, _ db.Tx, _, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListOwned(_ context.Context, _ db.Tx, _ uuid.UUID, _ promotionRepo.ExternalProductListFilters) ([]*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListForReview(_ context.Context, _ db.Tx, _ promotionRepo.ExternalProductAdminListFilters) ([]*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ExternalProduct, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListReviewHistory(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.ExternalProductReviewHistory, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) AppendReviewHistory(_ context.Context, _ db.Tx, _ *entity.ExternalProductReviewHistory) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) AddMedia(_ context.Context, _ db.Tx, _ *entity.ExternalProductMedia) error {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) ListMedia(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*entity.ExternalProductMedia, error) {
	panic("not exercised")
}
func (r *purchaseP2MockRepo) SoftDeleteMedia(_ context.Context, _ db.Tx, _, _, _ uuid.UUID) error {
	panic("not exercised")
}

type purchaseP2Tx struct{}

func (purchaseP2Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &purchaseP2NopRow{}
}
func (purchaseP2Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (purchaseP2Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), nil
}
func (purchaseP2Tx) Commit(ctx context.Context) error   { return nil }
func (purchaseP2Tx) Rollback(ctx context.Context) error { return nil }

type purchaseP2NopRow struct{}

func (r *purchaseP2NopRow) Scan(dest ...any) error { return nil }

func makeTestPackage() *entity.PromotionPackage {
	return &entity.PromotionPackage{
		ID:                  uuid.New(),
		Name:                "Test Package",
		TotalDurationHours:  72,
		ValidityWindowHours: 336,
		PriceAmount:         5000,
		AllowedTargetTypes:  []entity.TargetType{entity.TargetTypeForSale},
		IsActive:            true,
		CreatedAt:           time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPurchasePackage_StoresSourceBillingID verifies that PurchasePackage
// sets ownership.SourceBillingID from input.BillingID (P2-C).
func TestPurchasePackage_StoresSourceBillingID(t *testing.T) {
	pkg := makeTestPackage()
	repo := newPurchaseP2MockRepo(pkg)
	svc := application.NewPromotionServiceWithRepo(repo, allowAllChecker{operable: true})

	billingID := uuid.New()
	_, err := svc.PurchasePackage(context.Background(), purchaseP2Tx{}, application.PurchasePackageInput{
		UserID:    uuid.New(),
		PackageID: pkg.ID,
		BillingID: billingID,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.createdOwnership)
	require.NotNil(t, repo.createdOwnership.SourceBillingID, "SourceBillingID must be set when BillingID provided")
	assert.Equal(t, billingID, *repo.createdOwnership.SourceBillingID)
}

// TestPurchasePackage_ZeroBillingID_LeavesSourceBillingIDNil confirms that
// test/seed paths passing BillingID=uuid.Nil result in nil SourceBillingID
// (no FK violation, no unique index conflict).
func TestPurchasePackage_ZeroBillingID_LeavesSourceBillingIDNil(t *testing.T) {
	pkg := makeTestPackage()
	repo := newPurchaseP2MockRepo(pkg)
	svc := application.NewPromotionServiceWithRepo(repo, allowAllChecker{operable: true})

	_, err := svc.PurchasePackage(context.Background(), purchaseP2Tx{}, application.PurchasePackageInput{
		UserID:    uuid.New(),
		PackageID: pkg.ID,
		BillingID: uuid.Nil, // test/seed path
	})
	require.NoError(t, err)
	require.NotNil(t, repo.createdOwnership)
	assert.Nil(t, repo.createdOwnership.SourceBillingID, "zero BillingID must not set SourceBillingID")
}

// TestPurchasePackage_DuplicateBillingID_FailsAtRepo simulates the P2-A/P2-C
// concurrent-webhook race: two PurchasePackage calls with the same BillingID.
// The first succeeds; the second is rejected by the repo's unique constraint.
// This regression-locks the DB-level guard against duplicate ownership creation.
func TestPurchasePackage_DuplicateBillingID_FailsAtRepo(t *testing.T) {
	pkg := makeTestPackage()
	repo := newPurchaseP2MockRepo(pkg)
	svc := application.NewPromotionServiceWithRepo(repo, allowAllChecker{operable: true})

	billingID := uuid.New()
	userID := uuid.New()
	input := application.PurchasePackageInput{
		UserID:    userID,
		PackageID: pkg.ID,
		BillingID: billingID,
	}

	// First call — must succeed
	_, err1 := svc.PurchasePackage(context.Background(), purchaseP2Tx{}, input)
	require.NoError(t, err1, "first purchase with billing ID must succeed")

	// Second call with identical BillingID — must fail (unique violation from repo)
	_, err2 := svc.PurchasePackage(context.Background(), purchaseP2Tx{}, input)
	require.Error(t, err2, "second purchase with same billing ID must fail at the unique constraint")
	assert.Contains(t, err2.Error(), fmt.Sprintf("%v", "23505"),
		"error must indicate a unique constraint violation")
}
