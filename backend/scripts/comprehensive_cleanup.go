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

	tables := []string{
		"orders", "order_items", "order_ratings", "order_shipping_proofs",
		"listings", "listing_shipping_options", "listing_views",
		"shipping_coverages", "shipping_city_overrides", "shipping_options", "shipping_quotes",
		"addresses",
	}

	for _, table := range tables {
		pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE true", table))
		fmt.Printf("Cleaned %s\n", table)
	}

	fmt.Println("✅ Complete cleanup finished!")
}
