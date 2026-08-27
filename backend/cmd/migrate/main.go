package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
)

// DBConfig holds database connection configuration
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func LoadDBConfig() (*DBConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return &DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.Name,
		SSLMode:  cfg.Database.SSLMode,
	}, nil
}

func (c *DBConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type Migration struct {
	Version int
	Name    string
	UpSQL   string
}

func main() {
	log.Println("========================================")
	log.Println("  MIGRATION RUNNER (PGX + PROPER PARSER)")
	log.Println("========================================")

	cfg, err := LoadDBConfig()
	if err != nil {
		log.Fatal("Failed to load database configuration:", err)
	}

	dsn := cfg.GetDSN()
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal("Failed to parse database configuration:", err)
	}
	poolConfig.MaxConns = 15
	poolConfig.MinConns = 5

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal("Failed to create pool:", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("Ping failed:", err)
	}
	log.Println("DB Connected!")

	// Create schema_migrations table
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// Enable UUID extension
	log.Println("Ensuring pgcrypto extension...")
	pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto")

	// Load migration
	migrations, _ := loadMigrations()

	for _, m := range migrations {
		var exists bool
		pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", m.Version).Scan(&exists)

		if exists {
			log.Printf("Migration %d already applied\n", m.Version)
			continue
		}

		log.Printf("Applying migration: %d_%s.up.sql...\n", m.Version, m.Name)

		// Split SQL into statements
		statements := splitSQLStatements(m.UpSQL)
		log.Printf("  Split into %d statements\n", len(statements))

		// Detect ADD VALUE migrations: new enum values cannot be used in the same tx.
		// Run non-transactionally so the value is committed before CREATE INDEX.
		hasAddValue := strings.Contains(strings.ToUpper(m.UpSQL), "ADD VALUE")

		if hasAddValue {
			for i, stmt := range statements {
				if strings.TrimSpace(stmt) == "" {
					continue
				}
				log.Printf("  Executing statement %d (no-tx)...\n", i+1)
				if _, err := pool.Exec(ctx, stmt); err != nil {
					log.Printf("Statement %d failed: %v\n", i+1, err)
					log.Printf("Full SQL:\n%s\n", stmt)
					os.Exit(1)
				}
			}
			pool.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name)
		} else {
			tx, _ := pool.Begin(ctx)

			for i, stmt := range statements {
				if strings.TrimSpace(stmt) == "" {
					continue
				}
				log.Printf("  Executing statement %d...\n", i+1)
				if _, err := tx.Exec(ctx, stmt); err != nil {
					tx.Rollback(ctx)
					log.Printf("Statement %d failed: %v\n", i+1, err)
					log.Printf("Full SQL:\n%s\n", stmt)
					os.Exit(1)
				}
			}

			tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name)
			tx.Commit(ctx)
		}
		log.Printf("Migration %d applied successfully!\n", m.Version)
	}

	// List tables
	log.Println("\n========================================")
	log.Println("  TABLES IN PUBLIC SCHEMA")
	log.Println("========================================")

	rows, _ := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name`)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	for _, t := range tables {
		log.Printf("  - %s\n", t)
	}

	// Verification
	log.Println("\n========================================")
	log.Println("  VERIFICATION")
	log.Println("========================================")

	expected := map[string]bool{
		"ledger_transactions":    false,
		"ledger_entries":         false,
		"financial_accounts":     false,
		"reconciliation_results": false,
		"outbox":                 false,
		"withdrawals":            false,
		"contents":               false,
		"shipping_options":       false,
		"billing_transactions":   false,
	}

	for _, t := range tables {
		if _, ok := expected[t]; ok {
			expected[t] = true
		}
	}

	for name, exists := range expected {
		if exists {
			log.Printf("✓ %s\n", name)
		} else {
			log.Printf("✗ %s - MISSING\n", name)
		}
	}

	log.Println("\n========================================")
	log.Println("  MIGRATION COMPLETE")
	log.Println("========================================")
}

// splitSQLStatements splits SQL into individual statements
// This properly handles:
// - CREATE FUNCTION with $$ delimiters
// - Multi-line comments (/* */)
// - Single-line comments (--)
// - Strings with quotes
// - Nested parentheses
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	runes := []rune(sql)
	n := len(runes)

	i := 0
	for i < n {
		r := runes[i]

		// Single-line comment --
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			// Skip until newline
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}

		// Multi-line comment /* */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i < n-1 && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}

		// String literal
		if r == '\'' {
			current.WriteRune(r)
			i++
			for i < n {
				current.WriteRune(runes[i])
				if runes[i] == '\'' {
					// Check for escaped quote ''
					if i+1 < n && runes[i+1] == '\'' {
						current.WriteRune(runes[i+1]) // write second ' of ''
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
			continue
		}

		// Dollar-quoted string $$
		if r == '$' && i+1 < n && runes[i+1] == '$' {
			// Find the closing $$
			current.WriteString("$$")
			i += 2
			for i < n-1 && !(runes[i] == '$' && runes[i+1] == '$') {
				current.WriteRune(runes[i])
				i++
			}
			if i < n-1 {
				current.WriteString("$$")
				i += 2
			}
			continue
		}

		// Statement terminator (semicolon)
		if r == ';' {
			current.WriteRune(r)
			stmt := current.String()
			stmt = cleanupStatement(stmt)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			i++
			continue
		}

		current.WriteRune(r)
		i++
	}

	// Add remaining content
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		stmt = cleanupStatement(stmt)
		if strings.TrimSpace(stmt) != "" {
			statements = append(statements, strings.TrimSpace(stmt))
		}
	}

	return statements
}

// cleanupStatement removes trailing commas before closing parentheses
// and other common SQL cleanup issues
func cleanupStatement(stmt string) string {
	// Remove trailing commas before closing parentheses
	// Pattern: comma + whitespace + closing paren + optional whitespace + semicolon
	// This handles: "column TYPE, );" -> "column TYPE);"

	// First, handle ",);" -> ");"
	re1 := regexp.MustCompile(`,\s*\);`)
	stmt = re1.ReplaceAllString(stmt, ");")

	// Then handle ",\n)" -> "\n)" (comma before newline before paren)
	re2 := regexp.MustCompile(`,\s*\n\s*\)`)
	stmt = re2.ReplaceAllString(stmt, "\n)")

	// Handle ", )" at end of statement
	re3 := regexp.MustCompile(`,(\s*\))\s*;`)
	stmt = re3.ReplaceAllString(stmt, "$1;")

	// Clean up multiple empty lines
	re4 := regexp.MustCompile(`\n\s*\n\s*\n+`)
	stmt = re4.ReplaceAllString(stmt, "\n")

	return stmt
}

func truncateSQL(sql string, maxLen int) string {
	sql = regexp.MustCompile(`\s+`).ReplaceAllString(sql, " ")
	sql = strings.TrimSpace(sql)
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}

func loadMigrations() ([]Migration, error) {
	migrationsDir := "migrations"

	// Read all files in migrations directory
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatal("Failed to read migrations directory:", err)
	}

	var migrationFiles []string

	// Filter *.up.sql files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			migrationFiles = append(migrationFiles, filepath.Join(migrationsDir, name))
		}
	}

	// Sort by filename ascending
	sort.Strings(migrationFiles)

	var migrations []Migration

	// Load each migration file
	for _, filePath := range migrationFiles {
		filename := filepath.Base(filePath)

		// Extract version from filename (000001_xxx.up.sql -> 1)
		versionStr := strings.Split(filename, "_")[0]
		var version int
		_, err := fmt.Sscanf(versionStr, "%d", &version)
		if err != nil {
			log.Printf("Warning: Could not parse version from %s, skipping\n", filename)
			continue
		}

		// Extract name (remove .up.sql suffix)
		name := strings.TrimPrefix(filename, versionStr+"_")
		name = strings.TrimSuffix(name, ".up.sql")

		// Read file content
		upSQL, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatal("Failed to read migration file:", filePath, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			UpSQL:   string(upSQL),
		})
	}

	return migrations, nil
}
