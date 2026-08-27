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

	fmt.Println("📋 USERS TABLE STRUCTURE")
	fmt.Println("══════════════════════════════════════════════════════════")

	rows, err := db.Query(`
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = 'users'
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

	// Check if there are any existing users
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	fmt.Printf("\n📊 Total users: %d\n", count)

	// Get sample user IDs if any exist
	if count > 0 {
		rows, err := db.Query("SELECT id, email FROM users LIMIT 3")
		if err == nil {
			fmt.Println("📋 Sample users:")
			for rows.Next() {
				var id string
				var email string
				rows.Scan(&id, &email)
				fmt.Printf("  • ID: %s, Email: %s\n", id, email)
			}
			rows.Close()
		}
	}
}