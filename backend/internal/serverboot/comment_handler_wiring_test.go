package serverboot

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/database"
)

func loadEnvFromParents(t *testing.T) {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			if err := godotenv.Load(candidate); err != nil {
				t.Fatalf("load env: %v", err)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func TestInitServices_WiresCommentListContentService(t *testing.T) {
	loadEnvFromParents(t)

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("skipping: config load failed: %v", err)
	}

	log, err := logger.NewDevelopment()
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		t.Skipf("skipping: local Postgres unavailable: %v", err)
	}
	defer database.CloseDB(db, log)

	deps := InitServices(ctx, db, nil, nil, nil, log, cfg, false)
	if deps == nil {
		t.Fatal("InitServices returned nil dependencies")
	}
	if deps.CommentHandler == nil {
		t.Fatal("InitServices returned nil CommentHandler")
	}

	contentService := reflect.ValueOf(deps.CommentHandler).Elem().FieldByName("contentService")
	if !contentService.IsValid() {
		t.Fatal("CommentHandler.contentService field missing")
	}
	if contentService.IsNil() {
		t.Fatal("CommentHandler.contentService is nil; ListComments would panic on the visibility gate")
	}
}

// TestInitServices_WiresSellerCapabilityChecker is the Scope 1B/1C production
// wiring proof. It verifies that the CommentService built by InitServices has
// a non-nil sellerCapabilityChecker wired from the real production roleChecker.
// If future wiring omits or nil-replaces this field, the test fails.
func TestInitServices_WiresSellerCapabilityChecker(t *testing.T) {
	loadEnvFromParents(t)

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("skipping: config load failed: %v", err)
	}

	log, err := logger.NewDevelopment()
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		t.Skipf("skipping: local Postgres unavailable: %v", err)
	}
	defer database.CloseDB(db, log)

	deps := InitServices(ctx, db, nil, nil, nil, log, cfg, false)
	if deps == nil {
		t.Fatal("InitServices returned nil dependencies")
	}
	if deps.CommentHandler == nil {
		t.Fatal("InitServices returned nil CommentHandler")
	}

	// Navigate: CommentHandler.commentService → CommentService.sellerCapabilityChecker
	commentServiceField := reflect.ValueOf(deps.CommentHandler).Elem().FieldByName("commentService")
	if !commentServiceField.IsValid() {
		t.Fatal("CommentHandler.commentService field missing — structure drifted")
	}
	if commentServiceField.IsNil() {
		t.Fatal("CommentHandler.commentService is nil — wiring broken")
	}

	// The commentService field is itself an interface/pointer; dereference to
	// reach the CommentService struct.
	svcValue := reflect.Indirect(commentServiceField)
	sccField := svcValue.FieldByName("sellerCapabilityChecker")
	if !sccField.IsValid() {
		t.Fatal("CommentService.sellerCapabilityChecker field missing — structure drifted (did the field get renamed?)")
	}
	if sccField.IsNil() {
		t.Fatal("SCOPE 1B/1C WIRING REGRESSION: sellerCapabilityChecker is nil. " +
			"dependencies.go must pass a non-nil roleChecker to NewCommentService. " +
			"Without this, AddListingReferenceComment panics on nil checker or (worse) skips market-authority enforcement.")
	}
}
