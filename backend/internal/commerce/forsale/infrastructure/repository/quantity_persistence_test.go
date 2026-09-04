//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
)

// PASS_19E: proves for_sales.quantity_available is real, persisted
// state — not derived from status on every read (the PASS_19D-discovered
// bug). Requires PostgreSQL — run with: go test -tags integration

func insertSellerFixture(ctx context.Context, t *testing.T, testDB *testdb.TestDB, sellerID uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, account_status, created_at, updated_at, role
			)
			VALUES ($1, $2, $3, 'active', $4, $4, 'user')
		`, sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local", now); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO seller_profiles (
				id, user_id, store_name, tier, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'active', $5, $5)
		`, uuid.New(), sellerID, "Seller Store", sellerEntity.TierBasic, now)
		return err
	})
	if err != nil {
		t.Fatalf("seller fixture: %v", err)
	}
}

// TestForSaleRepository_Create_PersistsRealQuantity proves a
// multi-quantity for_sale survives Create+reload with its real quantity, not
// a status-derived 1/0.
func TestForSaleRepository_Create_PersistsRealQuantity(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	insertSellerFixture(ctx, t, testDB, sellerID)

	repo := NewForSaleRepository()

	var for_saleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := entity.NewForSale(
			sellerID, "Batch Koi", "5 units available", []byte(`[]`), "Kohaku",
			nil, nil, nil, nil, nil, []string{"global"},
			entity.ForSaleTypeFixedPrice, money.New(300000), 5, false,
			entity.ForSaleVisibilityPublic,
			nil, entity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		if err := for_sale.Publish(); err != nil {
			return err
		}
		if err := repo.Create(ctx, tx, for_sale); err != nil {
			return err
		}
		for_saleID = for_sale.ID
		return nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 5 {
			t.Fatalf("reloaded QuantityAvailable = %d, want 5", reloaded.QuantityAvailable)
		}
		if !reloaded.IsAvailable() {
			t.Fatal("reloaded for_sale with quantity=5 must be available")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
}

// TestForSaleRepository_ReduceRestoreCycle_PersistsThroughUpdateStock
// proves the full multi-quantity lifecycle round-trips through UpdateStock —
// the exact method order_creation_service/order_completion_service call.
func TestForSaleRepository_ReduceRestoreCycle_PersistsThroughUpdateStock(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	insertSellerFixture(ctx, t, testDB, sellerID)

	repo := NewForSaleRepository()

	var for_saleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := entity.NewForSale(
			sellerID, "Batch Koi", "5 units available", []byte(`[]`), "Kohaku",
			nil, nil, nil, nil, nil, []string{"global"},
			entity.ForSaleTypeFixedPrice, money.New(300000), 5, false,
			entity.ForSaleVisibilityPublic,
			nil, entity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		if err := for_sale.Publish(); err != nil {
			return err
		}
		if err := repo.Create(ctx, tx, for_sale); err != nil {
			return err
		}
		for_saleID = for_sale.ID
		return nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Buy 3 of 5 — stays active with 2 left.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := repo.GetForUpdate(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if err := for_sale.ReduceQuantity(3); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, for_sale)
	})
	if err != nil {
		t.Fatalf("reduce 3: %v", err)
	}
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 2 {
			t.Fatalf("after reduce 3: QuantityAvailable = %d, want 2", reloaded.QuantityAvailable)
		}
		if reloaded.Status != entity.ForSaleStatusActive {
			t.Fatalf("after reduce 3: Status = %s, want active", reloaded.Status)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload after reduce 3: %v", err)
	}

	// Buy remaining 2 — quantity 0, status sold.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := repo.GetForUpdate(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if err := for_sale.ReduceQuantity(2); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, for_sale)
	})
	if err != nil {
		t.Fatalf("reduce remaining 2: %v", err)
	}
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 0 {
			t.Fatalf("after exhausting stock: QuantityAvailable = %d, want 0", reloaded.QuantityAvailable)
		}
		if reloaded.Status != entity.ForSaleStatusSold {
			t.Fatalf("after exhausting stock: Status = %s, want sold", reloaded.Status)
		}
		if reloaded.IsAvailable() {
			t.Fatal("sold for_sale must not be available")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload after exhausting stock: %v", err)
	}

	// Cancel one unit — order_completion_service.restoreForSaleStock path.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := repo.GetForUpdate(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if err := for_sale.RestoreQuantity(1); err != nil {
			return err
		}
		return repo.UpdateStock(ctx, tx, for_sale)
	})
	if err != nil {
		t.Fatalf("restore 1: %v", err)
	}
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 1 {
			t.Fatalf("after restore 1: QuantityAvailable = %d, want 1", reloaded.QuantityAvailable)
		}
		if reloaded.Status != entity.ForSaleStatusActive {
			t.Fatalf("after restore 1: Status = %s, want active again", reloaded.Status)
		}
		if !reloaded.IsAvailable() {
			t.Fatal("restored for_sale with quantity=1 must be available again")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload after restore: %v", err)
	}
}

// TestForSaleRepository_OversellRejected_DBStateUnchanged proves an
// oversell attempt is rejected before any write, and the DB row is untouched.
func TestForSaleRepository_OversellRejected_DBStateUnchanged(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	insertSellerFixture(ctx, t, testDB, sellerID)

	repo := NewForSaleRepository()

	var for_saleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := entity.NewForSale(
			sellerID, "Single Koi", "unique item", []byte(`[]`), "Showa",
			nil, nil, nil, nil, nil, []string{"global"},
			entity.ForSaleTypeFixedPrice, money.New(300000), 1, false,
			entity.ForSaleVisibilityPublic,
			nil, entity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		if err := for_sale.Publish(); err != nil {
			return err
		}
		if err := repo.Create(ctx, tx, for_sale); err != nil {
			return err
		}
		for_saleID = for_sale.ID
		return nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := repo.GetForUpdate(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		reduceErr := for_sale.ReduceQuantity(3) // only 1 available
		if reduceErr == nil {
			t.Fatal("expected InsufficientQuantityError, got nil")
		}
		if _, ok := reduceErr.(*entity.InsufficientQuantityError); !ok {
			t.Fatalf("expected *InsufficientQuantityError, got %T: %v", reduceErr, reduceErr)
		}
		// Oversell rejection must never reach UpdateStock — nothing to persist.
		return nil
	})
	if err != nil {
		t.Fatalf("oversell check tx: %v", err)
	}

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 1 {
			t.Fatalf("DB quantity must be unchanged after rejected oversell: got %d, want 1", reloaded.QuantityAvailable)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload after rejected oversell: %v", err)
	}
}

// TestForSaleRepository_UniqueItemDefault_QuantityOne proves the
// unique-item (default) case still persists quantity=1 correctly — the
// common case (most koi for_sales are unique) must not regress.
func TestForSaleRepository_UniqueItemDefault_QuantityOne(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	insertSellerFixture(ctx, t, testDB, sellerID)

	repo := NewForSaleRepository()

	var for_saleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := entity.NewForSale(
			sellerID, "Unique Koi", "one of a kind", []byte(`[]`), "Sanke",
			nil, nil, nil, nil, nil, []string{"global"},
			entity.ForSaleTypeFixedPrice, money.New(500000), 1, false,
			entity.ForSaleVisibilityPublic,
			nil, entity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		if err := for_sale.Publish(); err != nil {
			return err
		}
		if err := repo.Create(ctx, tx, for_sale); err != nil {
			return err
		}
		for_saleID = for_sale.ID
		return nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 1 {
			t.Fatalf("unique-item QuantityAvailable = %d, want 1", reloaded.QuantityAvailable)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
}

// TestForSaleRepository_DirectQuantityEdit_PersistsThroughUpdate
// proves the seller-initiated PUT /for_sales/:id quantity edit path (a direct
// assignment, not ReduceQuantity/RestoreQuantity) persists through Update —
// previously silently discarded by derivedQuantity(status).
func TestForSaleRepository_DirectQuantityEdit_PersistsThroughUpdate(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	insertSellerFixture(ctx, t, testDB, sellerID)

	repo := NewForSaleRepository()

	var for_saleID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := entity.NewForSale(
			sellerID, "Restock Koi", "seller increases stock", []byte(`[]`), "Kohaku",
			nil, nil, nil, nil, nil, []string{"global"},
			entity.ForSaleTypeFixedPrice, money.New(300000), 1, false,
			entity.ForSaleVisibilityPublic,
			nil, entity.PreparationTimeImmediate, nil,
		)
		if err != nil {
			return err
		}
		if err := for_sale.Publish(); err != nil {
			return err
		}
		if err := repo.Create(ctx, tx, for_sale); err != nil {
			return err
		}
		for_saleID = for_sale.ID
		return nil
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Seller directly edits quantity from 1 to 10 (matches the PUT handler's
	// "for_sale.QuantityAvailable = *req.Quantity" direct-assignment path).
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		for_sale, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		for_sale.QuantityAvailable = 10
		return repo.Update(ctx, tx, for_sale)
	})
	if err != nil {
		t.Fatalf("direct quantity edit: %v", err)
	}

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		reloaded, err := repo.GetByID(ctx, tx, for_saleID)
		if err != nil {
			return err
		}
		if reloaded.QuantityAvailable != 10 {
			t.Fatalf("after direct edit: QuantityAvailable = %d, want 10", reloaded.QuantityAvailable)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reload after direct edit: %v", err)
	}
}
