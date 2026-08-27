//go:build integration

package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	alertrepo "github.com/labuda/backend/internal/platform/alert/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupOrderAlertIntegrationDB(t *testing.T) (*testdb.TestDB, func()) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	return tdb, cleanup
}

func insertWorkerTestUser(t *testing.T, ctx context.Context, tx db.Tx, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, role, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'active', 'user', NOW(), NOW())
	`, id, id.String(), fmt.Sprintf("%s-%s@test.invalid", label, id.String()))
	require.NoError(t, err)
	return id
}

func insertWorkerTestOrder(t *testing.T, ctx context.Context, tx db.Tx, buyerID, sellerID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	orderID := uuid.New()
	sourceID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id,
			quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, 'for_sale', $4,
		        1, 100000, 100000, 0,
		        0, 0, $5, NOW(), NOW())
	`, orderID, buyerID, sellerID, sourceID, status)
	require.NoError(t, err)
	return orderID
}

func insertWorkerTestDispute(t *testing.T, ctx context.Context, tx db.Tx, orderID, buyerID, sellerID uuid.UUID, openedAt time.Time) uuid.UUID {
	t.Helper()
	disputeID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO disputes (
			id, order_id, buyer_id, seller_id, reason, status,
			opened_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'under_review', $6, NOW(), NOW())
	`, disputeID, orderID, buyerID, sellerID, "Test dispute", openedAt)
	require.NoError(t, err)
	return disputeID
}

func TestDisputeOpenStuckRule_PostgresCompatibleAndDeterministic(t *testing.T) {
	tdb, cleanup := setupOrderAlertIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	rule := NewDisputeOpenStuckRule(nil, zap.NewNop())
	alertSvc := application.NewAlertService(db.NewFromPool(tdb.Pool()), alertrepo.NewAlertRepository(), zap.NewNop())

	var finding *AnomalyFinding
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		buyerID := insertWorkerTestUser(t, ctx, tx, "buyer")
		sellerID := insertWorkerTestUser(t, ctx, tx, "seller")
		firstOrderID := insertWorkerTestOrder(t, ctx, tx, buyerID, sellerID, "paid")
		secondOrderID := insertWorkerTestOrder(t, ctx, tx, buyerID, sellerID, "paid")

		oldest := time.Now().UTC().Add(-time.Duration(DisputeOpenStuckWarnHours+2) * time.Hour)
		newer := oldest.Add(30 * time.Minute)
		oldestDisputeID := insertWorkerTestDispute(t, ctx, tx, firstOrderID, buyerID, sellerID, oldest)
		newerDisputeID := insertWorkerTestDispute(t, ctx, tx, secondOrderID, buyerID, sellerID, newer)

		detected, f, detectErr := rule.Detect(ctx, tx)
		require.NoError(t, detectErr)
		require.True(t, detected)
		require.NotNil(t, f)
		require.NotEqual(t, oldestDisputeID, newerDisputeID)
		finding = f
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, finding)

	first, err := alertSvc.CreateAlert(
		ctx,
		finding.AlertType,
		finding.Severity,
		finding.EntityType,
		finding.EntityID,
		finding.Message,
		finding.Metadata,
		finding.GroupKey,
	)
	require.NoError(t, err)
	require.True(t, first.Created)

	second, err := alertSvc.CreateAlert(
		ctx,
		finding.AlertType,
		finding.Severity,
		finding.EntityType,
		finding.EntityID,
		finding.Message,
		finding.Metadata,
		finding.GroupKey,
	)
	require.NoError(t, err)
	require.False(t, second.Created)

	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM system_alerts
		WHERE alert_type = $1
		  AND group_key = $2
	`, string(alertentity.AlertTypeDisputeOpenStuck), *finding.GroupKey).Scan(&count))
	require.Equal(t, 1, count)
	require.Equal(t, finding.Metadata["oldest_dispute_id"], first.Alert.Metadata["oldest_dispute_id"])
}
