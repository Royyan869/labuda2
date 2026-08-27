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
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("Cannot ping database:", err)
	}

	fmt.Println("============================================")
	fmt.Println("🚨 STEP A (REVISED) — LOCK DATABASE IDENTITY")
	fmt.Println("============================================")
	fmt.Println("MODE: SAFE (Preserves signup flow)")
	fmt.Println("")

	// Track status
	duplicateEmailFound := false
	duplicateFirebaseUIDFound := false
	nullEmailFound := false
	cleanupDone := false
	emailUniqueEnforced := false
	firebaseUIDUniqueEnforced := false
	duplicateInsertBlocked := false

	// ========================================
	// 🔍 TASK 1 — INSPECT
	// ========================================
	fmt.Println("🔍 TASK 1 — INSPECT")
	fmt.Println("")

	// 1A. Check duplicate emails (exclude null/empty)
	fmt.Println("1A. DUPLICATE EMAIL CHECK (exclude null/empty):")
	rows, err := db.Query(`
		SELECT email, COUNT(*) AS cnt
		FROM users
		WHERE email IS NOT NULL AND email != ''
		GROUP BY email
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		count := 0
		for rows.Next() {
			var email string
			var cnt int
			rows.Scan(&email, &cnt)
			fmt.Printf("   ❗ %s: %d duplicates\n", email, cnt)
			count++
		}
		rows.Close()
		if count == 0 {
			fmt.Println("   ✅ No duplicate emails found")
		} else {
			duplicateEmailFound = true
		}
	}
	fmt.Println("")

	// 1B. Check duplicate firebase_uid
	fmt.Println("1B. DUPLICATE FIREBASE_UID CHECK:")
	rows, err = db.Query(`
		SELECT firebase_uid, COUNT(*) AS cnt
		FROM users
		WHERE firebase_uid IS NOT NULL
		GROUP BY firebase_uid
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		count := 0
		for rows.Next() {
			var uid string
			var cnt int
			rows.Scan(&uid, &cnt)
			fmt.Printf("   ❗ %s: %d duplicates\n", uid, cnt)
			count++
		}
		rows.Close()
		if count == 0 {
			fmt.Println("   ✅ No duplicate firebase_uid found")
		} else {
			duplicateFirebaseUIDFound = true
		}
	}
	fmt.Println("")

	// 1C. Check null/empty emails
	fmt.Println("1C. NULL/EMPTY EMAIL CHECK:")
	var nullEmailCount int
	err = db.QueryRow(`
		SELECT COUNT(*) AS null_or_empty_email
		FROM users
		WHERE email IS NULL OR email = ''
	`).Scan(&nullEmailCount)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		if nullEmailCount == 0 {
			fmt.Println("   ✅ No null or empty emails")
		} else {
			fmt.Printf("   ❗ Found %d null or empty emails\n", nullEmailCount)
			nullEmailFound = true
		}
	}
	fmt.Println("")

	// Total users
	var totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	fmt.Printf("📊 TOTAL USERS: %d\n", totalUsers)
	fmt.Println("")

	// ========================================
	// 🧹 TASK 2 — CLEAN (SAFE)
	// ========================================
	if duplicateEmailFound || duplicateFirebaseUIDFound {
		fmt.Println("🧹 TASK 2 — CLEAN (SAFE)")
		fmt.Println("")

		// 2A. Normalize email
		fmt.Println("2A. NORMALIZE EMAIL (lowercase + trim):")
		result, err := db.Exec(`
			UPDATE users
			SET email = LOWER(TRIM(email))
			WHERE email IS NOT NULL AND email != ''
		`)
		if err != nil {
			log.Printf("   ❌ Error: %v\n", err)
		} else {
			affected, _ := result.RowsAffected()
			fmt.Printf("   ✅ Normalized %d email(s)\n", affected)
		}
		fmt.Println("")

		// 2B. Resolve duplicate email
		if duplicateEmailFound {
			fmt.Println("2B. RESOLVE DUPLICATE EMAIL (MANUAL):")
			fmt.Println("   ⚠️  Duplicate emails found - NEEDS MANUAL CLEANUP")
			fmt.Println("")
			fmt.Println("   For each duplicate email:")
			fmt.Println("   - KEEP: created_at paling awal")
			fmt.Println("   - DELETE: sisanya")
			fmt.Println("")

			// Show duplicates with details
			rows, _ := db.Query(`
				SELECT
					email,
					ARRAY_AGG(id ORDER BY created_at) AS user_ids,
					ARRAY_AGG(created_at ORDER BY created_at) AS created_dates,
					COUNT(*) AS cnt
				FROM users
				WHERE email IS NOT NULL AND email != ''
				GROUP BY email
				HAVING COUNT(*) > 1
				ORDER BY cnt DESC
			`)
			for rows.Next() {
				var email string
				var userIDs, createdDates []string
				var cnt int
				rows.Scan(&email, &userIDs, &createdDates, &cnt)
				fmt.Printf("   Email: %s (%d duplicates)\n", email, cnt)
				for i := 0; i < len(userIDs); i++ {
					keep := "🗑️  DELETE"
					if i == 0 {
						keep = "✅ KEEP"
					}
					fmt.Printf("      %s %s (created: %s)\n", keep, userIDs[i], createdDates[i])
				}
				fmt.Println("")
			}
			rows.Close()
		}

		// 2C. Resolve duplicate firebase_uid
		if duplicateFirebaseUIDFound {
			fmt.Println("2C. RESOLVE DUPLICATE FIREBASE_UID (MANUAL):")
			fmt.Println("   ⚠️  Duplicate firebase_uid found - NEEDS MANUAL CLEANUP")
			fmt.Println("")

			// Show duplicates with details
			rows, _ := db.Query(`
				SELECT
					firebase_uid,
					ARRAY_AGG(id ORDER BY created_at) AS user_ids,
					ARRAY_AGG(created_at ORDER BY created_at) AS created_dates,
					COUNT(*) AS cnt
				FROM users
				WHERE firebase_uid IS NOT NULL
				GROUP BY firebase_uid
				HAVING COUNT(*) > 1
				ORDER BY cnt DESC
			`)
			for rows.Next() {
				var uid string
				var userIDs, createdDates []string
				var cnt int
				rows.Scan(&uid, &userIDs, &createdDates, &cnt)
				fmt.Printf("   Firebase UID: %s (%d duplicates)\n", uid, cnt)
				for i := 0; i < len(userIDs); i++ {
					keep := "🗑️  DELETE"
					if i == 0 {
						keep = "✅ KEEP"
					}
					fmt.Printf("      %s %s (created: %s)\n", keep, userIDs[i], createdDates[i])
				}
				fmt.Println("")
			}
			rows.Close()
		}

		fmt.Println("⚠️  PAUSE: Please resolve duplicates manually, then re-run this script")
		fmt.Println("")
		return
	}

	// ========================================
	// 🔒 TASK 3 — APPLY CONSTRAINT (FINAL)
	// ========================================
	fmt.Println("🔒 TASK 3 — APPLY CONSTRAINT (FINAL)")
	fmt.Println("")

	// 3A. EMAIL UNIQUE (nullable allowed)
	fmt.Println("3A. EMAIL UNIQUE (nullable allowed):")
	fmt.Println("   Creating partial unique index: WHERE email IS NOT NULL AND email != ''")
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx
		ON users(email)
		WHERE email IS NOT NULL AND email != ''
	`)
	if err != nil {
		log.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Println("   ✅ Partial unique index on email created")
		emailUniqueEnforced = true
	}
	fmt.Println("")

	// 3B. FIREBASE_UID UNIQUE
	fmt.Println("3B. FIREBASE_UID UNIQUE:")
	fmt.Println("   Creating partial unique index: WHERE firebase_uid IS NOT NULL")
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS users_firebase_uid_unique_idx
		ON users(firebase_uid)
		WHERE firebase_uid IS NOT NULL
	`)
	if err != nil {
		log.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Println("   ✅ Partial unique index on firebase_uid created")
		firebaseUIDUniqueEnforced = true
	}
	fmt.Println("")

	// ========================================
	// ⚡ TASK 4 — VERIFY
	// ========================================
	fmt.Println("⚡ TASK 4 — VERIFY")
	fmt.Println("")

	// Test 1: Duplicate email (should fail)
	fmt.Println("4A. TEST DUPLICATE EMAIL (should fail):")
	testID1 := "test-email-1"
	testID2 := "test-email-2"
	testEmail := "duplicate-test@example.com"

	// Cleanup test data if exists
	db.Exec("DELETE FROM users WHERE id = $1 OR id = $2", testID1, testID2)

	// Insert first test email
	db.Exec("INSERT INTO users (id, email) VALUES ($1, $2)", testID1, testEmail)

	// Try to insert duplicate email
	_, err = db.Exec("INSERT INTO users (id, email) VALUES ($1, $2)", testID2, testEmail)
	if err != nil {
		fmt.Println("   ✅ Duplicate email BLOCKED (as expected)")
		fmt.Printf("   Error: %v\n", err)
		duplicateInsertBlocked = true
	} else {
		fmt.Println("   ❌ Duplicate email NOT blocked (UNEXPECTED!)")
	}

	// Cleanup
	db.Exec("DELETE FROM users WHERE id = $1 OR id = $2", testID1, testID2)
	fmt.Println("")

	// Test 2: Duplicate firebase_uid (should fail)
	fmt.Println("4B. TEST DUPLICATE FIREBASE_UID (should fail):")
	testUID1 := "test-firebase-1"
	testUID2 := "test-firebase-2"
	testFirebaseUID := "test-firebase-uid-123"

	// Cleanup test data if exists
	db.Exec("DELETE FROM users WHERE id = $1 OR id = $2", testUID1, testUID2)

	// Insert first test firebase_uid
	db.Exec("INSERT INTO users (id, email, firebase_uid) VALUES ($1, $2, $3)", testUID1, "a@test.com", testFirebaseUID)

	// Try to insert duplicate firebase_uid
	_, err = db.Exec("INSERT INTO users (id, email, firebase_uid) VALUES ($1, $2, $3)", testUID2, "b@test.com", testFirebaseUID)
	if err != nil {
		fmt.Println("   ✅ Duplicate firebase_uid BLOCKED (as expected)")
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Println("   ❌ Duplicate firebase_uid NOT blocked (UNEXPECTED!)")
	}

	// Cleanup
	db.Exec("DELETE FROM users WHERE id = $1 OR id = $2", testUID1, testUID2)
	fmt.Println("")

	// Show current indexes
	fmt.Println("4C. CURRENT INDEXES ON USERS TABLE:")
	rows, _ = db.Query(`
		SELECT
			indexname AS index_name,
			indexdef AS index_definition
		FROM pg_indexes
		WHERE tablename = 'users'
		ORDER BY indexname
	`)
	for rows.Next() {
		var name, def string
		rows.Scan(&name, &def)
		fmt.Printf("   - %s\n",     name)
		fmt.Printf("     %s\n", def)
	}
	rows.Close()
	fmt.Println("")

	// ========================================
	// 💣 FINAL OUTPUT (WAJIB)
	// ========================================
	fmt.Println("💣 FINAL OUTPUT (WAJIB)")
	fmt.Println("============================================")
	fmt.Println("")

	fmt.Printf("DUPLICATE EMAIL FOUND: %s\n", boolToYesNo(duplicateEmailFound))
	fmt.Printf("DUPLICATE FIREBASE_UID FOUND: %s\n", boolToYesNo(duplicateFirebaseUIDFound))
	fmt.Printf("NULL EMAIL FOUND: %s\n", boolToYesNo(nullEmailFound))
	fmt.Println("")

	fmt.Printf("CLEANUP DONE: %s\n", boolToYesNo(cleanupDone))
	fmt.Println("")

	fmt.Printf("EMAIL UNIQUE ENFORCED: %s\n", boolToYesNo(emailUniqueEnforced))
	fmt.Printf("FIREBASE_UID UNIQUE ENFORCED: %s\n", boolToYesNo(firebaseUIDUniqueEnforced))
	fmt.Println("EMAIL NOT NULL ENFORCED: NO (EXPECTED) ✅")
	fmt.Println("")

	fmt.Printf("DUPLICATE INSERT BLOCKED: %s\n", boolToYesNo(duplicateInsertBlocked))
	fmt.Println("")

	// ========================================
	// 🎯 GOAL
	// ========================================
	fmt.Println("============================================")
	if emailUniqueEnforced && firebaseUIDUniqueEnforced && duplicateInsertBlocked {
		fmt.Println("🎯 GOAL ACHIEVED")
		fmt.Println("✅ Duplicate user secara DB = IMPOSSIBLE")
		fmt.Println("✅ Signup flow preserved (email + Google)")
	} else {
		fmt.Println("⚠️  GOAL NOT FULLY ACHIEVED")
		fmt.Println("   Please check the output above for issues")
	}
	fmt.Println("============================================")
}

func boolToYesNo(b bool) string {
	if b {
		return "YES ❗"
	}
	return "NO ✅"
}
