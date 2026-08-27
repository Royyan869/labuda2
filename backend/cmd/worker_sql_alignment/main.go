package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/config"
)

const probeDBPrefix = "labuda_worker_sql_alignment_"

var probeDBPattern = regexp.MustCompile(`^labuda_worker_sql_alignment_[a-z0-9_]+$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var explicitDBName string
	flag.StringVar(&explicitDBName, "db-name", "", "explicit disposable PostgreSQL database name to create and destroy")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	probeDBName := strings.TrimSpace(explicitDBName)
	if probeDBName == "" {
		probeDBName = generateProbeDBName()
	}
	if !probeDBPattern.MatchString(probeDBName) {
		return fmt.Errorf("refusing unsafe probe database name %q; expected %s", probeDBName, probeDBPattern.String())
	}
	if probeDBName == cfg.Database.Name {
		return fmt.Errorf("refusing to reuse main database %q as the disposable probe", probeDBName)
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ensureProbeDatabase(ctx, cfg, probeDBName); err != nil {
		return err
	}

	cleanupDone := false
	defer func() {
		if cleanupDone {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = dropProbeDatabase(cleanupCtx, cfg, probeDBName)
	}()

	fmt.Printf("worker SQL alignment probe database: %s\n", probeDBName)

	testCmd := exec.Command("go", "test", "-tags", "worker_sql_alignment", "./internal/worker/...")
	testCmd.Dir = moduleRoot
	testCmd.Env = append(os.Environ(),
		"DB_TEST_NAME="+probeDBName,
		"TEST_MODE=true",
	)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("worker SQL alignment tests failed: %w", err)
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	if err := dropProbeDatabase(cleanupCtx, cfg, probeDBName); err != nil {
		return err
	}
	cleanupDone = true

	fmt.Println("worker SQL alignment proof completed")
	return nil
}

func generateProbeDBName() string {
	var entropy [4]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		entropy = [4]byte{0, 0, 0, 0}
	}

	return probeDBPrefix + time.Now().UTC().Format("20060102_150405") + "_" + hex.EncodeToString(entropy[:])
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not locate go.mod from %s", dir)
}

func ensureProbeDatabase(ctx context.Context, cfg *config.Config, dbName string) error {
	adminConn, err := pgx.Connect(ctx, adminDSN(cfg))
	if err != nil {
		return fmt.Errorf("connect to admin database: %w", err)
	}
	defer adminConn.Close(ctx)

	if _, err := adminConn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName)); err != nil {
		return fmt.Errorf("drop stale probe database %q: %w", dbName, err)
	}
	if _, err := adminConn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, dbName)); err != nil {
		return fmt.Errorf("create probe database %q: %w", dbName, err)
	}

	return nil
}

func dropProbeDatabase(ctx context.Context, cfg *config.Config, dbName string) error {
	adminConn, err := pgx.Connect(ctx, adminDSN(cfg))
	if err != nil {
		return fmt.Errorf("connect to admin database for cleanup: %w", err)
	}
	defer adminConn.Close(ctx)

	_, _ = adminConn.Exec(ctx, `SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName)

	if _, err := adminConn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName)); err != nil {
		return fmt.Errorf("drop probe database %q: %w", dbName, err)
	}
	return nil
}

func adminDSN(cfg *config.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.SSLMode,
	)
}
