//go:build ignore

package main

import (
	"context"
	"fmt"
	"time"

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

	// Get seller ID
	var sellerID string
	pool.QueryRow(ctx, "SELECT id FROM users WHERE firebase_uid = $1", "mock-user917").Scan(&sellerID)

	// Delete and recreate with proper dates
	pool.Exec(ctx, `DELETE FROM seller_subscriptions WHERE user_id = $1`, sellerID)

	expiresAt := time.Now().AddDate(1, 0, 0) // 1 year from now

	pool.Exec(ctx, `
		INSERT INTO seller_subscriptions (user_id, status, started_at, expires_at, duration_days, amount_paid, currency, payment_id)
		VALUES ($1, 'active', NOW(), $2, 365, 50000000, 'IDR', gen_random_uuid())
	`, sellerID, expiresAt)

	fmt.Printf("✅ Fixed subscription for seller %s\n", sellerID)
	fmt.Printf("   Expires: %s\n", expiresAt.Format("2006-01-02"))
}
