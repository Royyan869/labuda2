//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Connect to database
	connStr := "postgresql://postgres:labuda123@localhost:5432/labuda_db?sslmode=disable"
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	fmt.Println("========================================")
	fmt.Println("REAL DATABASE STATE AUDIT")
	fmt.Println("========================================")

	// 5.1 List all tables
	fmt.Println("\n5.1 ALL TABLES IN PUBLIC SCHEMA:")
	fmt.Println("--------------------------------")
	rows, err := db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name;
	`)
	if err != nil {
		log.Fatal("Failed to query tables:", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatal("Failed to scan row:", err)
		}
		tables = append(tables, tableName)
		fmt.Printf("  - %s\n", tableName)
	}

	// 5.2 Focus on likes tables
	fmt.Println("\n5.2 TABLES CONTAINING 'likes':")
	fmt.Println("--------------------------------")
	likesTables := []string{}
	for _, table := range tables {
		if contains(table, "likes") {
			likesTables = append(likesTables, table)
			fmt.Printf("  ✓ %s\n", table)
		}
	}

	if len(likesTables) == 0 {
		fmt.Println("  ⚠️  NO LIKES TABLES FOUND!")
	}

	// 5.3 Structure of each likes table
	fmt.Println("\n5.3 STRUCTURE OF LIKES TABLES:")
	fmt.Println("-------------------------------")
	for _, table := range likesTables {
		fmt.Printf("\n  TABLE: %s\n", table)
		fmt.Println("  " + string(make([]byte, 50)))

		// Get column information
		colRows, err := db.Query(`
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default
			FROM information_schema.columns
			WHERE table_name = $1
			ORDER BY ordinal_position;
		`, table)
		if err != nil {
			fmt.Printf("  ✗ Error querying columns: %v\n", err)
			continue
		}

		fmt.Println("  Columns:")
		for colRows.Next() {
			var colName, dataType, isNullable string
			var colDefault *string
			if err := colRows.Scan(&colName, &dataType, &isNullable, &colDefault); err != nil {
				fmt.Printf("  ✗ Error scanning column: %v\n", err)
				continue
			}

			nullable := "NULL"
			if isNullable == "NO" {
				nullable = "NOT NULL"
			}
			defaultVal := ""
			if colDefault != nil {
				defaultVal = fmt.Sprintf(" DEFAULT %s", *colDefault)
			}
			fmt.Printf("    - %-20s %-20s %-10s%s\n", colName, dataType, nullable, defaultVal)
		}
		colRows.Close()

		// Get constraint information
		conRows, err := db.Query(`
			SELECT
				con.conname as constraint_name,
				pg_get_constraintdef(con.oid) as constraint_definition
			FROM pg_constraint con
			JOIN pg_class rel ON rel.oid = con.conrelid
			WHERE rel.relname = $1
			ORDER BY con.contype;
		`, table)
		if err != nil {
			fmt.Printf("  ✗ Error querying constraints: %v\n", err)
			continue
		}

		fmt.Println("  Constraints:")
		hasConstraints := false
		for conRows.Next() {
			hasConstraints = true
			var conName, conDef string
			if err := conRows.Scan(&conName, &conDef); err != nil {
				fmt.Printf("  ✗ Error scanning constraint: %v\n", err)
				continue
			}
			fmt.Printf("    - %s: %s\n", conName, conDef)
		}
		if !hasConstraints {
			fmt.Println("    (none)")
		}
		conRows.Close()

		// Get index information
		idxRows, err := db.Query(`
			SELECT
				indexname,
				indexdef
			FROM pg_indexes
			WHERE schemaname = 'public'
			AND tablename = $1
			ORDER BY indexname;
		`, table)
		if err != nil {
			fmt.Printf("  ✗ Error querying indexes: %v\n", err)
			continue
		}

		fmt.Println("  Indexes:")
		hasIndexes := false
		for idxRows.Next() {
			hasIndexes = true
			var idxName, idxDef string
			if err := idxRows.Scan(&idxName, &idxDef); err != nil {
				fmt.Printf("  ✗ Error scanning index: %v\n", err)
				continue
			}
			fmt.Printf("    - %s\n", idxName)
		}
		if !hasIndexes {
			fmt.Println("    (none)")
		}
		idxRows.Close()
	}

	// 5.4 Check for specific constraints
	fmt.Println("\n5.4 ALL CONSTRAINTS ON LIKES TABLES:")
	fmt.Println("------------------------------------")
	conRows, err := db.Query(`
		SELECT
			con.conname as constraint_name,
			rel.relname::text as table_name,
			pg_get_constraintdef(con.oid) as constraint_definition
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		WHERE rel.relname ILIKE '%likes%'
		ORDER BY rel.relname, con.contype;
	`)
	if err != nil {
		log.Fatal("Failed to query constraints:", err)
	}
	defer conRows.Close()

	hasConstraints := false
	for conRows.Next() {
		hasConstraints = true
		var conName, tableName, conDef string
		if err := conRows.Scan(&conName, &tableName, &conDef); err != nil {
			log.Fatal("Failed to scan constraint:", err)
		}
		fmt.Printf("  ✓ Table: %-20s Constraint: %-30s\n", tableName, conName)
		fmt.Printf("    Definition: %s\n", conDef)
	}
	if !hasConstraints {
		fmt.Println("  ⚠️  NO CONSTRAINTS FOUND ON LIKES TABLES!")
	}

	// Check for migration tracking table
	fmt.Println("\nMIGRATION TRACKING TABLE:")
	fmt.Println("-------------------------")
	migRows, err := db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND (table_name LIKE '%migration%' OR table_name LIKE '%schema%');
	`)
	if err != nil {
		fmt.Printf("  ✗ Error querying migration tables: %v\n", err)
	} else {
		defer migRows.Close()
		hasMigrationTable := false
		for migRows.Next() {
			hasMigrationTable = true
			var tableName string
			if err := migRows.Scan(&tableName); err == nil {
				fmt.Printf("  ✓ %s\n", tableName)
			}
		}
		if !hasMigrationTable {
			fmt.Println("  ⚠️  No migration tracking tables found")
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("AUDIT COMPLETE")
	fmt.Println("========================================")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
