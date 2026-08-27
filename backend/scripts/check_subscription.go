//go:build ignore

package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
)

func main() {
	cfg, _ := config.Load()
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode)

	poolConfig, _ := pgxpool.ParseConfig(dsn)
	ctx := context.Background()

	pool, _ := pgxpool.NewWithConfig(ctx, poolConfig)
	defer pool.Close()

	// Get seller user ID
	var sellerID string
	pool.QueryRow(ctx, "SELECT id FROM users WHERE firebase_uid = $1", "mock-user917").Scan(&sellerID)
	fmt.Printf("Seller ID: %s\n\n", sellerID)

	// Check subscription
	var status string
	var expiresAt string
	pool.QueryRow(ctx, "SELECT status, expires_at FROM seller_subscriptions WHERE user_id = $1", sellerID).Scan(&status, &expiresAt)
	fmt.Printf("Subscription Status: %s\n", status)
	fmt.Printf("Expires At: %s\n", expiresAt)

	// Check all subscriptions
	fmt.Println("\nAll seller_subscriptions:")
	rows, _ := pool.Query(ctx, "SELECT user_id, status, expires_at FROM seller_subscriptions")
	defer rows.Close()
	for rows.Next() {
		var uid, st, exp string
		rows.Scan(&uid, &st, &exp)
		fmt.Printf("  %s: %s (expires: %s)\n", uid, st, exp)
	}
}
