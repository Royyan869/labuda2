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

	// Check columns in users table
	rows, _ := pool.Query(ctx, `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'users' 
		ORDER BY ordinal_position
	`)
	defer rows.Close()

	fmt.Println("Columns in users table:")
	for rows.Next() {
		var colName, dataType string
		rows.Scan(&colName, &dataType)
		fmt.Printf("  - %s: %s\n", colName, dataType)
	}
}
