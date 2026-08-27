package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://" + os.Getenv("DB_USER") + ":" + os.Getenv("DB_PASSWORD") + "@" + os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT") + "/" + os.Getenv("DB_NAME")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := func(title, sql string) {
		fmt.Println("== " + title + " ==")
		rows, err := pool.Query(ctx, sql)
		if err != nil {
			fmt.Println("  ERR:", err)
			return
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				fmt.Println("  scan err:", err)
				return
			}
			fmt.Println("  " + s)
			n++
		}
		if n == 0 {
			fmt.Println("  (none)")
		}
	}

	q("fixed_price_sales exists", `SELECT 'yes' FROM pg_tables WHERE schemaname='public' AND tablename='fixed_price_sales'`)
	q("for_sales exists", `SELECT 'yes' FROM pg_tables WHERE schemaname='public' AND tablename='for_sales'`)
	q("fixed_price_sale_status_enum exists", `SELECT 'yes' FROM pg_type WHERE typname='fixed_price_sale_status_enum'`)
	q("for_sale_status_enum exists", `SELECT 'yes' FROM pg_type WHERE typname='for_sale_status_enum'`)
	q("fixed_price columns", `SELECT table_name||'.'||column_name FROM information_schema.columns WHERE table_schema='public' AND column_name LIKE 'fixed_price%'`)
	q("uniq_active_fixed_price_sale_per_product", `SELECT 'yes' FROM pg_indexes WHERE schemaname='public' AND indexname='uniq_active_fixed_price_sale_per_product'`)
	q("trg_fixed_price_sales_single_active_channel", `SELECT 'yes' FROM pg_trigger WHERE NOT tgisinternal AND tgname='trg_fixed_price_sales_single_active_channel'`)
	q("order_source_enum has listing", `SELECT 'yes' FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='listing'`)
	q("order_source_enum has fixed_price_sale", `SELECT 'yes' FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='fixed_price_sale'`)
	q("saved_items check", `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='saved_items'::regclass AND conname='saved_items_target_type_check'`)
}
