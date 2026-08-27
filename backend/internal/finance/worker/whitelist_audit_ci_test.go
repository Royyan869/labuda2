package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// These tests are CI-safe: they use nil repo (dev mode) and require no DB.
// They verify that the in-process audit log records every mutation correctly
// and that NewWhitelistManager / Add / Remove never fail with nil repo.

func TestWhitelistManagerInitRecordsEntry(t *testing.T) {
	log := zap.NewNop()
	auditLog := NewWhitelistAuditLog(log, nil) // nil repo = dev mode, no DB

	seller1 := uuid.MustParse("a1000000-0000-0000-0000-000000000001")
	seller2 := uuid.MustParse("a1000000-0000-0000-0000-000000000002")

	wm, err := NewWhitelistManager(context.Background(), []uuid.UUID{seller1, seller2},
		"system:startup", "ci-test", auditLog)
	if err != nil {
		t.Fatalf("NewWhitelistManager with nil repo must not fail: %v", err)
	}
	if wm.Size() != 2 {
		t.Fatalf("expected size 2, got %d", wm.Size())
	}

	entries := auditLog.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 INITIALIZED entry, got %d", len(entries))
	}
	if entries[0].EventType != WhitelistEventInitialized {
		t.Fatalf("expected WHITELIST_INITIALIZED, got %s", entries[0].EventType)
	}
	if entries[0].WhitelistSize != 2 {
		t.Fatalf("expected whitelist_size 2 in audit entry, got %d", entries[0].WhitelistSize)
	}
}

func TestWhitelistManagerAddRecordsEntry(t *testing.T) {
	log := zap.NewNop()
	auditLog := NewWhitelistAuditLog(log, nil)

	wm, _ := NewWhitelistManager(context.Background(), nil, "system:startup", "ci-test", auditLog)

	seller := uuid.MustParse("a2000000-0000-0000-0000-000000000001")
	if err := wm.Add(context.Background(), seller, "admin:test", "ci add"); err != nil {
		t.Fatalf("Add with nil repo must not fail: %v", err)
	}

	if !wm.IsWhitelisted(seller) {
		t.Fatal("seller should be whitelisted after Add")
	}
	entries := auditLog.ForSeller(seller)
	if len(entries) != 1 || entries[0].EventType != WhitelistEventSellerAdded {
		t.Fatalf("expected 1 SELLER_ADDED entry for seller, got %v", entries)
	}
}

func TestWhitelistManagerRemoveRecordsEntry(t *testing.T) {
	log := zap.NewNop()
	auditLog := NewWhitelistAuditLog(log, nil)

	seller := uuid.MustParse("a3000000-0000-0000-0000-000000000001")
	wm, _ := NewWhitelistManager(context.Background(), []uuid.UUID{seller}, "system:startup", "ci-test", auditLog)

	if err := wm.Remove(context.Background(), seller, "admin:test", "ci remove"); err != nil {
		t.Fatalf("Remove with nil repo must not fail: %v", err)
	}

	if wm.IsWhitelisted(seller) {
		t.Fatal("seller should not be whitelisted after Remove")
	}
	entries := auditLog.ForSeller(seller)
	if len(entries) != 1 || entries[0].EventType != WhitelistEventSellerRemoved {
		t.Fatalf("expected 1 SELLER_REMOVED entry for seller, got %v", entries)
	}
}

func TestWhitelistManagerEmptyInitAllowed(t *testing.T) {
	log := zap.NewNop()
	auditLog := NewWhitelistAuditLog(log, nil)

	wm, err := NewWhitelistManager(context.Background(), nil, "system:startup", "empty-ci", auditLog)
	if err != nil {
		t.Fatalf("NewWhitelistManager with empty list must not fail: %v", err)
	}
	if wm.Size() != 0 {
		t.Fatalf("expected size 0, got %d", wm.Size())
	}
	if wm.IsWhitelisted(uuid.New()) {
		t.Fatal("empty whitelist must not allow any seller")
	}
}


