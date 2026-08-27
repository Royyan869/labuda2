//go:build integration

// Package repository_test proves WithdrawRepository's JOIN queries (GetByID,
// GetBySellerID, GetEligibleForSubmission, GetPendingSettlement, and the
// filtered list query) run against the live schema without the
// "column reference ... is ambiguous" error that previously blocked the
// payout worker. withdrawals, user_profiles, and seller_profiles all define
// created_at/updated_at, so any unqualified reference to those columns in a
// query joining all three is rejected by Postgres at parse time — regardless
// of whether matching profile rows exist. This is the regression guard for
// the PASS 1B payout fix.
package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	financerepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func setupWithdrawTest(t *testing.T) (*testdb.TestDB, *financerepo.WithdrawRepository, func()) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	return tdb, financerepo.NewWithdrawRepository(), cleanup
}

// TestWithdrawRepository_GetEligibleForSubmission_NoAmbiguousColumn is the
// direct regression test for the payout worker break: previously this query
// failed with "column reference \"created_at\" is ambiguous" on every poll
// cycle, so no withdrawal could ever be submitted to the gateway.
func TestWithdrawRepository_GetEligibleForSubmission_NoAmbiguousColumn(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	sellerID := uuid.New()
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, id, sellerID, 100000, 5000, financerepo.WithdrawalStatusProcessing)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var results []*financerepo.Withdrawal
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		results, err = repo.GetEligibleForSubmission(ctx, tx, 10)
		return err
	})
	if err != nil {
		t.Fatalf("GetEligibleForSubmission: %v (ambiguous column regression if this mentions created_at/updated_at)", err)
	}

	found := false
	for _, w := range results {
		if w.ID == id {
			found = true
			if w.Amount != 100000 {
				t.Fatalf("amount mismatch: got %d", w.Amount)
			}
		}
	}
	if !found {
		t.Fatalf("expected withdrawal %s to be eligible for submission, got %d results", id, len(results))
	}
}

// TestWithdrawRepository_GetByID_NoAmbiguousColumn covers the same JOIN shape
// used by GetByID.
func TestWithdrawRepository_GetByID_NoAmbiguousColumn(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	sellerID := uuid.New()
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, id, sellerID, 50000, 1000, financerepo.WithdrawalStatusRequested)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var found *financerepo.Withdrawal
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		found, err = repo.GetByID(ctx, tx, id)
		return err
	})
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.ID != id {
		t.Fatalf("ID mismatch: got %v want %v", found.ID, id)
	}
}

// TestWithdrawRepository_GetBySellerID_NoAmbiguousColumn covers the same JOIN
// shape used by GetBySellerID.
func TestWithdrawRepository_GetBySellerID_NoAmbiguousColumn(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	sellerID := uuid.New()
	id := uuid.New()
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, id, sellerID, 75000, 2000, financerepo.WithdrawalStatusSettled)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var results []*financerepo.Withdrawal
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		results, err = repo.GetBySellerID(ctx, tx, sellerID)
		return err
	})
	if err != nil {
		t.Fatalf("GetBySellerID: %v", err)
	}
	if len(results) != 1 || results[0].ID != id {
		t.Fatalf("expected 1 result with ID %v, got %+v", id, results)
	}
}

// TestWithdrawRepository_GetPendingSettlement_NoAmbiguousColumn covers the
// same JOIN shape used by GetPendingSettlement.
func TestWithdrawRepository_GetPendingSettlement_NoAmbiguousColumn(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	sellerID := uuid.New()
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, id, sellerID, 20000, 500, financerepo.WithdrawalStatusSubmitted)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var results []*financerepo.Withdrawal
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		results, err = repo.GetPendingSettlement(ctx, tx, 10)
		return err
	})
	if err != nil {
		t.Fatalf("GetPendingSettlement: %v", err)
	}
	found := false
	for _, w := range results {
		if w.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected withdrawal %s in pending settlement results", id)
	}
}

// TestWithdrawRepository_ListWithFilters_EmptyReturnsNoError is the direct
// regression test for the admin Withdrawals page HTTP 500: the default
// ORDER BY clause referenced the unqualified "created_at" column, which is
// ambiguous once this query LEFT JOINs user_profiles and seller_profiles
// (both of which also define created_at/updated_at). Postgres rejects that
// at parse time, so ListWithFilters failed on every single call — including
// against an empty table — not just when rows existed.
func TestWithdrawRepository_ListWithFilters_EmptyReturnsNoError(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	filters := financerepo.WithdrawalListFilters{
		SortBy:   "created_at",
		SortDesc: true,
		Page:     1,
		PageSize: 20,
	}

	var results []*financerepo.Withdrawal
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		results, err = repo.ListWithFilters(ctx, tx, filters)
		return err
	})
	if err != nil {
		t.Fatalf("ListWithFilters on empty table: %v (ambiguous column regression if this mentions created_at/updated_at)", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results on empty table, got %d", len(results))
	}

	var total int64
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		total, err = repo.CountWithFilters(ctx, tx, filters)
		return err
	})
	if err != nil {
		t.Fatalf("CountWithFilters on empty table: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected count 0 on empty table, got %d", total)
	}
}

// TestWithdrawRepository_ListWithFilters_WithDataNoAmbiguousColumn proves the
// same query succeeds once a row exists, using every whitelisted sort column
// (the bug affected created_at/updated_at specifically, since those are the
// columns shared by all three joined tables).
func TestWithdrawRepository_ListWithFilters_WithDataNoAmbiguousColumn(t *testing.T) {
	tdb, repo, cleanup := setupWithdrawTest(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	sellerID := uuid.New()
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.Create(ctx, tx, id, sellerID, 30000, 1500, financerepo.WithdrawalStatusRequested)
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, sortBy := range []string{"created_at", "updated_at", "amount", "status", "submitted_at", "settled_at"} {
		filters := financerepo.WithdrawalListFilters{
			SortBy:   sortBy,
			SortDesc: true,
			Page:     1,
			PageSize: 20,
		}
		var results []*financerepo.Withdrawal
		err := tdb.WithTx(ctx, func(tx db.Tx) error {
			var err error
			results, err = repo.ListWithFilters(ctx, tx, filters)
			return err
		})
		if err != nil {
			t.Fatalf("ListWithFilters sort_by=%s: %v", sortBy, err)
		}
		found := false
		for _, w := range results {
			if w.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("sort_by=%s: expected withdrawal %s in results, got %d rows", sortBy, id, len(results))
		}
	}

	var total int64
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		total, err = repo.CountWithFilters(ctx, tx, financerepo.WithdrawalListFilters{Page: 1, PageSize: 20})
		return err
	})
	if err != nil {
		t.Fatalf("CountWithFilters: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected count >= 1, got %d", total)
	}
}
