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
	fmt.Println("LOCK DATABASE IDENTITY - AUTOMATED SCRIPT")
	fmt.Println("============================================\n")

	duplicateEmailFound := false
	duplicateFirebaseUIDFound := false
	nullEmailFound := false
	cleanupDone := false
	emailUniqueEnforced := false
	firebaseUIDUniqueEnforced := false
	emailNotNullEnforced := false

	// ========================================
	// TASK 1: INSPECT CURRENT STATE
	// ========================================
	fmt.Println("=== TASK 1: INSPECT CURRENT STATE ===\n")

	// 1A. Check duplicate emails
	fmt.Println("1A. DUPLICATE EMAIL CHECK:")
	var duplicateEmailCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT email, COUNT(*) AS cnt
			FROM users
			GROUP BY email
			HAVING COUNT(*) > 1
		) AS duplicates
	`).Scan(&duplicateEmailCount)
	if err != nil {
		log.Printf("Error checking duplicate emails: %v\n", err)
	} else {
		if duplicateEmailCount > 0 {
			fmt.Printf("   ❗ Found %d duplicate emails!\n", duplicateEmailCount)
			rows, _ := db.Query(`
				SELECT email, COUNT(*) AS cnt
				FROM users
				GROUP BY email
				HAVING COUNT(*) > 1
				ORDER BY cnt DESC
			`)
			for rows.Next() {
				var email string
				var cnt int
				rows.Scan(&email, &cnt)
				fmt.Printf("   - %s: %d duplicates\n", email, cnt)
			}
			rows.Close()
			duplicateEmailFound = true
		} else {
			fmt.Println("   ✅ No duplicate emails found")
		}
	}

	// 1B. Check duplicate firebase_uid
	fmt.Println("\n1B. DUPLICATE FIREBASE_UID CHECK:")
	var duplicateFirebaseCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT firebase_uid, COUNT(*) AS cnt
			FROM users
			WHERE firebase_uid IS NOT NULL
			GROUP BY firebase_uid
			HAVING COUNT(*) > 1
		) AS duplicates
	`).Scan(&duplicateFirebaseCount)
	if err != nil {
		log.Printf("Error checking duplicate firebase_uid: %v\n", err)
	} else {
		if duplicateFirebaseCount > 0 {
			fmt.Printf("   ❗ Found %d duplicate firebase_uid!\n", duplicateFirebaseCount)
			rows, _ := db.Query(`
				SELECT firebase_uid, COUNT(*) AS cnt
				FROM users
				WHERE firebase_uid IS NOT NULL
				GROUP BY firebase_uid
				HAVING COUNT(*) > 1
				ORDER BY cnt DESC
			`)
			for rows.Next() {
				var uid string
				var cnt int
				rows.Scan(&uid, &cnt)
				fmt.Printf("   - %s: %d duplicates\n", uid, cnt)
			}
			rows.Close()
			duplicateFirebaseUIDFound = true
		} else {
			fmt.Println("   ✅ No duplicate firebase_uid found")
		}
	}

	// 1C. Check null or empty emails
	fmt.Println("\n1C. NULL OR EMPTY EMAIL CHECK:")
	var nullEmailCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM users
		WHERE email IS NULL OR email = ''
	`).Scan(&nullEmailCount)
	if err != nil {
		log.Printf("Error checking null emails: %v\n", err)
	} else {
		if nullEmailCount > 0 {
			fmt.Printf("   ❗ Found %d null or empty emails!\n", nullEmailCount)
			nullEmailFound = true
		} else {
			fmt.Println("   ✅ No null or empty emails found")
		}
	}

	// Total users
	var totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	fmt.Printf("\nTOTAL USERS: %d\n", totalUsers)

	// ========================================
	// TASK 2: CLEAN DATA
	// ========================================
	fmt.Println("\n=== TASK 2: CLEAN DATA ===\n")

	// 2A. Normalize emails
	fmt.Println("2A. NORMALIZING EMAILS (lowercase, trim)...")
	result, err := db.Exec(`
		UPDATE users
		SET email = LOWER(TRIM(email))
		WHERE email IS NOT NULL AND email != ''
	`)
	if err != nil {
		log.Printf("Error normalizing emails: %v\n", err)
	} else {
		affected, _ := result.RowsAffected()
		fmt.Printf("   ✅ Normalized %d email(s)\n", affected)
		cleanupDone = true
	}

	// 2B. Handle null/empty emails
	fmt.Println("\n2B. HANDLING NULL/EMPTY EMAILS...")
	if nullEmailCount > 0 {
		fmt.Printf("   ⚠️  Found %d users with null/empty emails\n", nullEmailCount)
		fmt.Println("   ❌ SKIPPING: Manual intervention required")
		fmt.Println("   Options:")
		fmt.Println("   1. DELETE: Delete users with null emails")
		fmt.Println("   2. UPDATE: Set placeholder email")
	} else {
		fmt.Println("   ✅ No null emails to handle")
	}

	// ========================================
	// TASK 3: APPLY CONSTRAINTS
	// ========================================
	fmt.Println("\n=== TASK 3: APPLY CONSTRAINTS ===\n")

	// 3A. Set email to NOT NULL
	fmt.Println("3A. SETTING EMAIL TO NOT NULL...")
	if nullEmailCount > 0 {
		fmt.Println("   ❌ SKIPPED: Still have null emails")
	} else {
		_, err := db.Exec(`ALTER TABLE users ALTER COLUMN email SET NOT NULL`)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
		} else {
			fmt.Println("   ✅ Email column set to NOT NULL")
			emailNotNullEnforced = true
		}
	}

	// 3B. Add UNIQUE constraint on email
	fmt.Println("\n3B. ADDING UNIQUE CONSTRAINT ON EMAIL...")
	if duplicateEmailCount > 0 {
		fmt.Println("   ❌ SKIPPED: Still have duplicate emails")
	} else {
		_, err := db.Exec(`ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email)`)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n", err)
		} else {
			fmt.Println("   ✅ UNIQUE constraint on email added")
			emailUniqueEnforced = true
		}
	}

	// 3C. Check firebase_uid unique index
	fmt.Println("\n3C. CHECKING FIREBASE_UID UNIQUE INDEX...")
	var indexExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'users' AND indexname = 'users_firebase_uid_key'
		)
	`).Scan(&indexExists)
	if err != nil {
		log.Printf("Error checking index: %v\n", err)
	} else {
		if indexExists {
			fmt.Println("   ✅ UNIQUE index on firebase_uid already exists")
			firebaseUIDUniqueEnforced = true
		} else {
			_, err := db.Exec(`CREATE UNIQUE INDEX users_firebase_uid_key ON users(firebase_uid)`)
			if err != nil {
				fmt.Printf("   ❌ Error creating index: %v\n", err)
			} else {
				fmt.Println("   ✅ UNIQUE index on firebase_uid created")
				firebaseUIDUniqueEnforced = true
			}
		}
	}

	// ========================================
	// TASK 4: VERIFICATION
	// ========================================
	fmt.Println("\n=== TASK 4: VERIFICATION ===\n")

	// Show constraints
	fmt.Println("4A. CURRENT CONSTRAINTS:")
	rows, _ := db.Query(`
		SELECT
			conname AS constraint_name,
			pg_get_constraintdef(oid) AS definition
		FROM pg_constraint
		WHERE conrelid = 'users'::regclass
		ORDER BY conname
	`)
	for rows.Next() {
		var name, def string
		rows.Scan(&name, &def)
		fmt.Printf("   - %s: %s\n", name, def)
	}
	rows.Close()

	// ========================================
	// FINAL OUTPUT
	// ========================================
	fmt.Println("\n============================================")
	fmt.Println("FINAL OUTPUT")
	fmt.Println("============================================\n")

	fmt.Printf("DUPLICATE EMAIL FOUND: %s\n", boolToYesNo(duplicateEmailFound))
	fmt.Printf("DUPLICATE FIREBASE_UID FOUND: %s\n", boolToYesNo(duplicateFirebaseUIDFound))
	fmt.Printf("NULL EMAIL FOUND: %s\n", boolToYesNo(nullEmailFound))
	fmt.Println()
	fmt.Printf("CLEANUP DONE: %s\n", boolToYesNo(cleanupDone))
	fmt.Println()
	fmt.Printf("EMAIL UNIQUE ENFORCED: %s\n", boolToYesNo(emailUniqueEnforced))
	fmt.Printf("FIREBASE_UID UNIQUE ENFORCED: %s\n", boolToYesNo(firebaseUIDUniqueEnforced))
	fmt.Printf("EMAIL NOT NULL ENFORCED: %s\n", boolToYesNo(emailNotNullEnforced))
	fmt.Println()

	if duplicateEmailCount == 0 && duplicateFirebaseCount == 0 && nullEmailCount == 0 {
		fmt.Println("DUPLICATE INSERT BLOCKED: YES ✅")
	} else {
		fmt.Println("DUPLICATE INSERT BLOCKED: NO ❌")
		fmt.Println("   (Please resolve data issues first)")
	}
	fmt.Println()

	if duplicateEmailCount > 0 || duplicateFirebaseCount > 0 || nullEmailCount > 0 {
		fmt.Println("⚠️  WARNING: Data issues found!")
		fmt.Println("   Please resolve them before enforcing all constraints.")
	}
}

func boolToYesNo(b bool) string {
	if b {
		return "YES ❗"
	}
	return "NO ✅"
}
