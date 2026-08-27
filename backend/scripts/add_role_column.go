//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode)

	poolConfig, _ := pgxpool.ParseConfig(dsn)
	ctx := context.Background()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Add role column if it doesn't exist.
	_, err = pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'`)
	if err != nil {
		log.Printf("Failed to add role column: %v", err)
	} else {
		fmt.Println("Added role column to users table")
	}

	// Normalize any legacy seller markers to user role.
	// Seller identity now comes from seller_profiles, not users.role.
	_, err = pool.Exec(ctx, `UPDATE users SET role = 'user' WHERE firebase_uid LIKE '%seller%'`)
	if err != nil {
		log.Printf("Failed to normalize seller-like accounts to user role: %v", err)
	} else {
		fmt.Println("Normalized seller-like accounts to user role")
	}

	_, err = pool.Exec(ctx, `UPDATE users SET role = 'user' WHERE firebase_uid LIKE '%buyer%'`)
	if err != nil {
		log.Printf("Failed to update buyer role: %v", err)
	} else {
		fmt.Println("Updated buyer role")
	}
}
