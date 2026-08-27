package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "host=localhost port=5432 user=labuda password=labuda123 dbname=labuda sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("📋 OUTBOX TABLE STRUCTURE")
	fmt.Println("══════════════════════════════════════════════════════════")

	rows, err := db.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = 'outbox'
		ORDER BY ordinal_position
	`)
	if err != nil {
		log.Fatal("Error querying columns:", err)
	}
	defer rows.Close()

	for rows.Next() {
		var colName, dataType, nullable string
		rows.Scan(&colName, &dataType, &nullable)
		fmt.Printf("  • %s: %s (nullable: %s)\n", colName, dataType, nullable)
	}

	// Check constraints
	fmt.Println("\n📋 OUTBOX CONSTRAINTS")
	fmt.Println("══════════════════════════════════════════════════════════")

	constraints, err := db.Query(`
		SELECT constraint_name, constraint_type
		FROM information_schema.table_constraints
		WHERE table_name = 'outbox'
		ORDER BY constraint_type
	`)
	if err == nil {
		for constraints.Next() {
			var constraintName, constraintType string
			constraints.Scan(&constraintName, &constraintType)
			fmt.Printf("  • %s: %s\n", constraintType, constraintName)
		}
		constraints.Close()
	}

	// Check current data
	var count int
	db.QueryRow("SELECT COUNT(*) FROM outbox").Scan(&count)
	fmt.Printf("\n📊 Total outbox events: %d\n", count)

	if count > 0 {
		rows, err := db.Query("SELECT id, event_type, aggregate_id, status FROM outbox LIMIT 3")
		if err == nil {
			fmt.Println("📋 Sample outbox events:")
			for rows.Next() {
				var id, eventType, aggregateID, status string
				rows.Scan(&id, &eventType, &aggregateID, &status)
				fmt.Printf("  • ID: %s, Type: %s, Aggregate: %s, Status: %s\n", id, eventType, aggregateID, status)
			}
			rows.Close()
		}
	}
}