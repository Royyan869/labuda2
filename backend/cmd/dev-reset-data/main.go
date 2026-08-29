// Command dev-reset-data resets the LOCAL/DEV database to a clean state while
// preserving a whitelist of owner accounts (by email).
//
// Semantics (owner-approved, 2026-07-03 addendum; actors table dropped in
// PHASE 1 CLEANUP migration 000011, 2026-07-10):
//   - Preserve the whitelisted accounts ONLY: identity + minimal capability
//     rows needed for login and role (users, user_profiles, user_capabilities,
//     seller_profiles, seller_subscriptions, seller_verifications,
//     support_admins).
//   - Delete ALL domain/runtime data, including rows owned by the preserved
//     accounts (orders, listings, content, chats, payments, ledger, ...).
//   - Preserve static config/reference tables required by runtime.
//   - Never touch schema_migrations. Never touch Firebase Auth.
//
// Default mode is dry-run. Destructive mode requires --execute.
// Fail-closed: any table present in the DB but absent from the classification
// below is reported and blocks --execute.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// Table classification. Every table in the public schema MUST appear in exactly
// one of these sets. Unknown tables block execute mode.
// ─────────────────────────────────────────────────────────────────────────────

// neverTouch: not read, not written.
var neverTouch = []string{
	"schema_migrations",
}

// referenceTables: static config/reference data required by runtime. Fully preserved.
var referenceTables = []string{
	"platform_configs",
	"configs",
	"seller_subscription_configs",
	"promotion_packages",
}

// identityTables: user-scoped rows preserved ONLY for whitelisted users.
// Rows for all other users are removed via ON DELETE CASCADE when their users
// row is deleted. These are the minimal rows for login + role/capability:
//   - users row itself (firebase_uid = auth provider linkage)
//   - user_profiles (username / app-boot profile)
//   - user_capabilities (admin governance capabilities)
//   - seller_profiles + seller_subscriptions + seller_verifications
//     (the 4-gate HasActiveSellerCapability chain)
//   - support_admins (admin support authority)
var identityTables = []string{
	"users",
	"user_profiles",
	"user_capabilities",
	"seller_profiles",
	"seller_subscriptions",
	"seller_verifications",
	"support_admins",
}

// domainTables: all runtime/domain data. Fully wiped, including rows owned by
// whitelisted users. financial_accounts is wiped too: core_server bootstrap
// (EnsureSystemAccounts) recreates the system accounts with seed balances.
var domainTables = []string{
	"account_balances",
	"addresses",
	"admin_audit_logs",
	"appeals",
	"auction_bids",
	"auctions",
	"audit_events",
	"auth_refresh_sessions",
	"bank_accounts",
	"billing_transactions",
	"buyer_bnr_strikes",
	"chat_messages",
	"chat_read_states",
	"chat_rooms",
	"coins_transactions",
	"comment_likes",
	"comments",
	"content_hashtags",
	"content_likes",
	"content_media",
	"contents",
	"discount_usages",
	"discounts",
	"dispute_freezes",
	"dispute_media",
	"disputes",
	"escrows",
	"external_product_media",
	"external_product_review_history",
	"external_products",
	"fcm_tokens",
	"financial_accounts",
	"for_sales",
	"idempotency_records",
	"ledger_entries",
	"ledger_transactions",
	"listing_shipping_options",
	"listing_views",
	"listings",
	"moderation_cases",
	"negotiation_messages",
	"negotiation_price_history",
	"negotiation_sessions",
	"notification_delivery_log",
	"notifications",
	"order_items",
	"order_overdue_reminders",
	"order_ratings",
	"order_shipping_proofs",
	"order_summaries",
	"orders",
	"outbox",
	"outbox_archive",
	"payment_attempts",
	"payment_webhook_events",
	"payments",
	"payout_whitelist_audit_logs",
	"pricing_tokens",
	"processed_ban_events",
	"product_shipping_options",
	"products",
	"projection_tracker",
	"promotion_events",
	"promotion_instances",
	"promotion_ownerships",
	"push_retry_queue",
	"reconciliation_results",
	"refund_evidence",
	"refunds",
	"saved_items",
	"search_history",
	"seller_monthly_metrics",
	"seller_reputation_state",
	"shipping_city_overrides",
	"shipping_coverages",
	"shipping_options",
	"shipping_quotes",
	"support_ticket_events",
	"support_tickets",
	"system_alerts",
	"user_blocks",
	"user_coin_balance",
	"user_follows",
	"user_mutes",
	"user_warnings",
	"verification_documents",
	"wallets",
	"withdrawals",
}

type keptUser struct {
	ID          string
	Email       string
	FirebaseUID string
	Role        string
	Status      string
}

func main() {
	dryRun := flag.Bool("dry-run", false, "report what would be deleted/preserved; no writes (default mode)")
	execute := flag.Bool("execute", false, "perform the destructive reset (requires exclusive intent; overridden by --dry-run)")
	keepEmails := flag.String("keep-email", "", "comma-separated emails of accounts to preserve (required)")
	allowMissing := flag.Bool("allow-missing-keep", false, "allow --execute even if a keep-email has no users row")
	flag.Parse()

	if *keepEmails == "" {
		log.Fatal("--keep-email is required")
	}
	mode := "DRY-RUN"
	if *execute && !*dryRun {
		mode = "EXECUTE"
	}

	var kept []string
	for _, e := range strings.Split(*keepEmails, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		log.Fatal("--keep-email produced an empty whitelist")
	}
	if len(kept) != 3 {
		log.Fatalf("--keep-email must specify exactly 3 owner-test accounts (got %d); "+
			"this tool is scoped to the 3 canonical owner-test accounts per docs/owner-test-runtime.md", len(kept))
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config load failed: ", err)
	}
	host := cfg.Database.Host
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		log.Fatalf("refusing to run against non-local DB host %q — this tool is local/dev only", host)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password,
		cfg.Database.Name, cfg.Database.SSLMode)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("pool create failed: ", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal("DB ping failed: ", err)
	}

	fmt.Printf("================================================================\n")
	fmt.Printf("  DEV DATA RESET — mode: %s\n", mode)
	fmt.Printf("  database: %s@%s:%s/%s\n", cfg.Database.User, host, cfg.Database.Port, cfg.Database.Name)
	fmt.Printf("================================================================\n\n")

	// ── Discover tables and validate classification (fail closed) ──
	dbTables, err := listTables(ctx, pool)
	if err != nil {
		log.Fatal("table discovery failed: ", err)
	}
	classified := map[string]string{}
	for _, t := range neverTouch {
		classified[t] = "NEVER_TOUCH"
	}
	for _, t := range referenceTables {
		classified[t] = "REFERENCE_PRESERVED"
	}
	for _, t := range identityTables {
		classified[t] = "IDENTITY_WHITELIST"
	}
	for _, t := range domainTables {
		classified[t] = "DOMAIN_WIPED"
	}

	var unknown, missing []string
	dbSet := map[string]bool{}
	for _, t := range dbTables {
		dbSet[t] = true
		if _, ok := classified[t]; !ok {
			unknown = append(unknown, t)
		}
	}
	for t := range classified {
		if !dbSet[t] {
			missing = append(missing, t)
		}
	}
	sort.Strings(unknown)
	sort.Strings(missing)

	// ── Resolve whitelist accounts ──
	rows, err := pool.Query(ctx,
		`SELECT id, email, firebase_uid, role, account_status FROM users WHERE lower(email) = ANY($1)`, kept)
	if err != nil {
		log.Fatal("whitelist lookup failed: ", err)
	}
	found := map[string]keptUser{}
	var keptIDs []string
	for rows.Next() {
		var u keptUser
		if err := rows.Scan(&u.ID, &u.Email, &u.FirebaseUID, &u.Role, &u.Status); err != nil {
			log.Fatal("whitelist scan failed: ", err)
		}
		found[strings.ToLower(u.Email)] = u
		keptIDs = append(keptIDs, u.ID)
	}
	rows.Close()

	fmt.Println("── 1. WHITELIST ACCOUNTS ──────────────────────────────────────")
	var missingKeep []string
	for _, e := range kept {
		if u, ok := found[e]; ok {
			username := lookupUsername(ctx, pool, u.ID)
			fmt.Printf("  PRESERVED  %s\n", e)
			fmt.Printf("             user_id=%s firebase_uid=%s role=%s status=%s username=%s\n",
				u.ID, u.FirebaseUID, u.Role, u.Status, username)
		} else {
			missingKeep = append(missingKeep, e)
			fmt.Printf("  MISSING    %s — NO users row in this database\n", e)
		}
	}
	fmt.Println()

	// ── Users that would be deleted ──
	fmt.Println("── 2. USER ROWS TO DELETE (not on whitelist) ──────────────────")
	delRows, err := pool.Query(ctx,
		`SELECT id, email, firebase_uid, role FROM users WHERE NOT (lower(email) = ANY($1)) ORDER BY created_at`, kept)
	if err != nil {
		log.Fatal("user enumeration failed: ", err)
	}
	deleteUserCount := 0
	for delRows.Next() {
		var id, email, fuid, role string
		if err := delRows.Scan(&id, &email, &fuid, &role); err != nil {
			log.Fatal("user scan failed: ", err)
		}
		deleteUserCount++
		fmt.Printf("  DELETE     %s (role=%s, id=%s)\n", email, role, id)
	}
	delRows.Close()
	if deleteUserCount == 0 {
		fmt.Println("  (none)")
	}
	fmt.Println()

	// ── Per-table plan with counts ──
	fmt.Println("── 3. TABLE PLAN ──────────────────────────────────────────────")
	report := func(list []string, action string) int64 {
		var total int64
		for _, t := range list {
			if !dbSet[t] {
				continue
			}
			n := countRows(ctx, pool, t)
			total += n
			if n > 0 {
				fmt.Printf("  %-20s %-32s %6d rows\n", action, t, n)
			}
		}
		return total
	}
	wiped := report(domainTables, "DOMAIN_WIPE_ALL")
	fmt.Println()
	fmt.Println("  Identity tables (rows kept ONLY for whitelist users; all other")
	fmt.Println("  users' rows deleted via user-row cascade):")
	report(identityTables, "IDENTITY_FILTER")
	fmt.Println()
	fmt.Println("  Reference tables (fully preserved):")
	report(referenceTables, "REFERENCE_KEEP")
	fmt.Println()
	fmt.Println("  Never touched: " + strings.Join(neverTouch, ", "))
	fmt.Println()

	if len(missing) > 0 {
		fmt.Println("  Note: classified but absent from DB (skipped): " + strings.Join(missing, ", "))
		fmt.Println()
	}

	fmt.Println("── 4. UNCLASSIFIED TABLES (block execute) ─────────────────────")
	if len(unknown) == 0 {
		fmt.Println("  (none — every table is classified)")
	} else {
		for _, t := range unknown {
			fmt.Printf("  UNKNOWN    %s (%d rows) — needs owner decision\n", t, countRows(ctx, pool, t))
		}
	}
	fmt.Println()

	fmt.Printf("── 5. SUMMARY ─────────────────────────────────────────────────\n")
	fmt.Printf("  domain rows to wipe:    %d\n", wiped)
	fmt.Printf("  user rows to delete:    %d\n", deleteUserCount)
	fmt.Printf("  whitelist matched:      %d/%d\n", len(found), len(kept))
	fmt.Printf("  schema_migrations:      untouched\n")
	fmt.Printf("  Firebase Auth:          untouched (this tool never calls Firebase)\n")

	if mode != "EXECUTE" {
		fmt.Printf("  destructive execution:  NOT PERFORMED (dry-run)\n")
		fmt.Println("================================================================")
		return
	}

	// ── EXECUTE MODE ──
	if len(unknown) > 0 {
		log.Fatal("refusing to execute: unclassified tables present (fail-closed)")
	}
	if len(missingKeep) > 0 && !*allowMissing {
		log.Fatalf("refusing to execute: %d whitelist email(s) have no users row: %s\n"+
			"(re-run with --allow-missing-keep to proceed anyway)",
			len(missingKeep), strings.Join(missingKeep, ", "))
	}

	fmt.Println()
	fmt.Println("── EXECUTING DESTRUCTIVE RESET ────────────────────────────────")
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// 1. Clear dangling reviewer references on preserved verification rows
		//    (reviewed_by is NO ACTION and may point at a user being deleted).
		if len(keptIDs) > 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE seller_verifications SET reviewed_by = NULL
				WHERE seller_id = ANY($1::uuid[])
				  AND reviewed_by IS NOT NULL
				  AND NOT (reviewed_by = ANY($1::uuid[]))`, keptIDs); err != nil {
				return fmt.Errorf("clearing dangling reviewed_by: %w", err)
			}
		}

		// 2. Wipe all domain tables in one TRUNCATE (no CASCADE: if anything
		//    outside this list still references these tables, fail closed).
		var present []string
		for _, t := range domainTables {
			if dbSet[t] {
				present = append(present, quoteIdent(t))
			}
		}
		if len(present) > 0 {
			if _, err := tx.Exec(ctx, "TRUNCATE TABLE "+strings.Join(present, ", ")); err != nil {
				return fmt.Errorf("domain truncate: %w", err)
			}
		}

		// 3. Delete all non-whitelisted users; identity rows cascade.
		ct, err := tx.Exec(ctx, `DELETE FROM users WHERE NOT (lower(email) = ANY($1))`, kept)
		if err != nil {
			return fmt.Errorf("user deletion: %w", err)
		}
		fmt.Printf("  deleted %d user rows (+ cascaded identity rows)\n", ct.RowsAffected())
		return nil
	})
	if err != nil {
		log.Fatal("EXECUTE FAILED (transaction rolled back, nothing changed): ", err)
	}

	fmt.Println("  reset committed successfully")
	fmt.Println("  NOTE: start core_server once to re-bootstrap system financial")
	fmt.Println("  accounts (EnsureSystemAccounts) and run ./cmd/seed if reference")
	fmt.Println("  seed data is needed.")
	fmt.Println("================================================================")
	_ = os.Stdout.Sync()
}

func listTables(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func countRows(ctx context.Context, pool *pgxpool.Pool, table string) int64 {
	var n int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quoteIdent(table)).Scan(&n); err != nil {
		log.Fatalf("count failed for %s: %v", table, err)
	}
	return n
}

// lookupUsername reads the app-boot profile name for a user, if the profile
// table and row exist. Best-effort: returns "-" when absent.
func lookupUsername(ctx context.Context, pool *pgxpool.Pool, userID string) string {
	var username *string
	err := pool.QueryRow(ctx,
		`SELECT username FROM user_profiles WHERE user_id = $1`, userID).Scan(&username)
	if err != nil || username == nil {
		return "-"
	}
	return *username
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
