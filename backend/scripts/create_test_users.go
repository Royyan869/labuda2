package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	connStr := "host=localhost port=5432 user=labuda password=labuda123 dbname=labuda sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create 20 test users for parallel testing
	fmt.Println("Creating 20 test users for parallel testing...")
	for i := 0; i < 20; i++ {
		userID := uuid.New()
		_, err := db.ExecContext(ctx, `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, is_id_verified, is_farm_verified, role, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), false, 'active', false, false, 'user', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING
		`, userID, fmt.Sprintf("firebase_test_%d", i), fmt.Sprintf("test_user_%d@example.com", i))

		if err != nil {
			log.Printf("Error creating user %d: %v", i, err)
		} else {
			fmt.Printf("✅ Created user %d: %s\n", i+1, userID)
		}
	}

	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	fmt.Printf("\n📊 Total users in database: %d\n", userCount)
}
