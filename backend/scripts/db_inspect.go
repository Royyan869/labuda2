//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	// Database connection
	db, err := sql.Open("postgres", "host=localhost port=5432 user=labuda password=labuda123 dbname=labuda sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("Cannot connect to database:", err)
	}

	fmt.Println("=== TASK 1: INSPECT CURRENT STATE ===\n")

	// 1A. Duplicate email
	fmt.Println("1A. DUPLICATE EMAIL:")
	rows1, err := db.Query(`
		SELECT email, COUNT(*) AS cnt
		FROM users
		GROUP BY email
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
	} else {
		count := 0
		for rows1.Next() {
			var email string
			var cnt int
			rows1.Scan(&email, &cnt)
			fmt.Printf("  Email: %s, Count: %d\n", email, cnt)
			count++
		}
		rows1.Close()
		if count == 0 {
			fmt.Println("  No duplicate emails found ✅")
		}
		fmt.Println()
	}

	// 1B. Duplicate firebase_uid
	fmt.Println("1B. DUPLICATE FIREBASE_UID:")
	rows2, err := db.Query(`
		SELECT firebase_uid, COUNT(*) AS cnt
		FROM users
		WHERE firebase_uid IS NOT NULL
		GROUP BY firebase_uid
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
	} else {
		count := 0
		for rows2.Next() {
			var uid string
			var cnt int
			rows2.Scan(&uid, &cnt)
			fmt.Printf("  Firebase UID: %s, Count: %d\n", uid, cnt)
			count++
		}
		rows2.Close()
		if count == 0 {
			fmt.Println("  No duplicate firebase_uid found ✅")
		}
		fmt.Println()
	}

	// 1C. Null or empty email
	fmt.Println("1C. NULL OR EMPTY EMAIL:")
	var nullCount int
	err = db.QueryRow(`
		SELECT COUNT(*) AS null_or_empty_email
		FROM users
		WHERE email IS NULL OR email = ''
	`).Scan(&nullCount)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
	} else {
		if nullCount == 0 {
			fmt.Printf("  No null or empty emails found ✅\n\n")
		} else {
			fmt.Printf("  Found %d null or empty emails ❗\n\n", nullCount)
		}
	}

	// Check total users
	var totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	fmt.Printf("TOTAL USERS: %d\n", totalUsers)
}
