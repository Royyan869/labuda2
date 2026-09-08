//go:build integration

package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	financeapp "github.com/labuda/backend/internal/finance/application"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	configrepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	contractapp "github.com/labuda/backend/internal/pricing/promotion/contract/application"
	contractentity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepoImpl "github.com/labuda/backend/internal/pricing/promotion/contract/infrastructure/repository"
	deliveryapp "github.com/labuda/backend/internal/pricing/promotion/delivery/application"
	deliveryRepoImpl "github.com/labuda/backend/internal/pricing/promotion/delivery/infrastructure/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestCanonicalDeliveryGate_Disabled_IssueTicketFails(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgRepo := configrepo.NewPlatformConfigRepository()
	cfgSvc := configapp.NewConfigService(cfgRepo)
	deliveryRepo := deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool()))

	contractSvc := contractapp.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		allowAllGate{},
		deliveryRepo,
	)

	deliverySvc := deliveryapp.NewDeliveryService(
		db.NewFromPool(tdb.Pool()),
		contractRepoImpl.NewContractRepository(),
		deliveryRepo,
		financeSvc,
		cfgSvc,
		&stubEligibility{preflightOperable: true, txBoundaryOperable: true},
	)

	// Explicitly disable delivery
	_, err = tdb.Pool().Exec(ctx, `UPDATE platform_configs SET value_text = 'disabled' WHERE key = 'promotion_delivery_enabled'`)
	require.NoError(t, err)

	sellerID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, sellerID, "fb-test-"+sellerID.String()[:8], sellerID.String()+"@test.local")
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return financeSvc.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), sellerID, 500000)
	})
	require.NoError(t, err)

	targetID := uuid.New()
	viewerID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, viewerID, "fb-viewer-"+viewerID.String()[:8], viewerID.String()+"@test.local")
	require.NoError(t, err)

	// Create contract
	contract, err := contractSvc.Create(ctx, contractapp.CreatePromotionInput{
		SellerID:     sellerID,
		Kind:         contractentity.KindInternal,
		BudgetRupiah: 100_000,
		DurationDays: 1,
	})
	require.NoError(t, err)

	_, err = deliverySvc.IssueTicket(ctx, deliveryapp.IssueTicketInput{
		ContractID: contract.ID,
		TargetType: promoentity.TargetTypeForSale,
		TargetID:   targetID,
		ViewerID:   viewerID,
	})
	require.ErrorIs(t, err, deliveryapp.ErrDeliveryDisabled)

	// Assert no ticket created
	var count int
	err = tdb.Pool().QueryRow(ctx, "SELECT count(*) FROM promotion_delivery_tickets").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestCanonicalDeliveryGate_Disabled_QualifyTicketFails(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	bootstrap := financeapp.NewSystemAccountBootstrapFromPgx(db.NewFromPool(tdb.Pool()))
	_, err := bootstrap.EnsureSystemAccounts(ctx)
	require.NoError(t, err)

	financeSvc := financeapp.NewFinanceService()
	cfgRepo := configrepo.NewPlatformConfigRepository()
	cfgSvc := configapp.NewConfigService(cfgRepo)
	deliveryRepo := deliveryRepoImpl.NewDeliveryRepository(db.NewFromPool(tdb.Pool()))

	contractSvc := contractapp.NewPromotionContractService(
		db.NewFromPool(tdb.Pool()),
		financeSvc,
		cfgSvc,
		allowAllGate{},
		deliveryRepo,
	)

	deliverySvc := deliveryapp.NewDeliveryService(
		db.NewFromPool(tdb.Pool()),
		contractRepoImpl.NewContractRepository(),
		deliveryRepo,
		financeSvc,
		cfgSvc,
		&stubEligibility{preflightOperable: true, txBoundaryOperable: true},
	)

	sellerID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, sellerID, "fb-test-"+sellerID.String()[:8], sellerID.String()+"@test.local")
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return financeSvc.RecordPromoteBalanceFunding(ctx, tx, uuid.New(), sellerID, 500000)
	})
	require.NoError(t, err)

	targetID := uuid.New()
	viewerID := uuid.New()
	_, err = tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, viewerID, "fb-viewer-"+viewerID.String()[:8], viewerID.String()+"@test.local")
	require.NoError(t, err)

	// Ensure delivery is enabled so we can issue a ticket (in-flight ticket)
	_, err = tdb.Pool().Exec(ctx, `UPDATE platform_configs SET value_text = 'enabled' WHERE key = 'promotion_delivery_enabled'`)
	require.NoError(t, err)

	// Create contract
	contract, err := contractSvc.Create(ctx, contractapp.CreatePromotionInput{
		SellerID:     sellerID,
		Kind:         contractentity.KindInternal,
		BudgetRupiah: 100_000,
		DurationDays: 1,
	})
	require.NoError(t, err)

	ticket, err := deliverySvc.IssueTicket(ctx, deliveryapp.IssueTicketInput{
		ContractID: contract.ID,
		TargetType: promoentity.TargetTypeForSale,
		TargetID:   targetID,
		ViewerID:   viewerID,
	})
	require.NoError(t, err)

	// Now DISABLE delivery
	_, err = tdb.Pool().Exec(ctx, `UPDATE platform_configs SET value_text = 'disabled' WHERE key = 'promotion_delivery_enabled'`)
	require.NoError(t, err)

	// Attempt qualification of the in-flight ticket
	_, err = deliverySvc.QualifyTicket(ctx, deliveryapp.QualifyTicketInput{
		TicketID: ticket.ID,
	})
	require.ErrorIs(t, err, deliveryapp.ErrDeliveryDisabled)

	// Assert no qualified impression created
	var count int
	err = tdb.Pool().QueryRow(ctx, "SELECT count(*) FROM promotion_qualified_impressions").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
