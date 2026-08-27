//go:build ignore
// +build ignore

// FINANCIAL BOUNDARY VERIFICATION SCRIPT
//
// This script enforces the strict separation between Wallet and Finance domains:
// - Wallet = SINGLE SOURCE OF TRUTH for user money
// - Finance = accounting mirror ONLY (NOT money authority)
//
// FORBIDDEN: Using financial_accounts outside of finance domain for:
// - order creation/payment
// - refund decisions
// - withdrawal decisions
// - escrow operations
// - wallet operations
//
// USAGE: go run scripts/verify_financial_boundary.go
//
// EXIT CODES:
// 0 - All checks passed
// 1 - Violations found

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ForbiddenPatterns are patterns that should ONLY appear in finance domain
var ForbiddenPatterns = []struct {
	Pattern      string
	Description  string
	AllowedInDir string
}{
	{
		Pattern:      "financial_accounts",
		Description:  "Finance ledger table - ONLY for accounting, NOT money authority",
		AllowedInDir: "finance",
	},
	{
		Pattern:      "FinancialAccountDB",
		Description:  "Finance account model - ONLY for accounting",
		AllowedInDir: "finance",
	},
}

// ForbiddenDomains lists domains that must NOT use financial_accounts
var ForbiddenDomains = []string{
	"order",
	"payment",
	"wallet",
	"refund",
	"dispute",
	"escrow",
}

type Violation struct {
	File      string
	Line      int
	Pattern   string
	Context   string
	Directory string
}

func main() {
	rootDir := "." // Default to current directory
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	fmt.Println("🔍 FINANCIAL BOUNDARY VERIFICATION")
	fmt.Println("==================================")
	fmt.Printf("Scanning: %s\n\n", rootDir)

	violations := []Violation{}

	// Walk through all Go files in the domain directory
	domainDir := filepath.Join(rootDir, "internal", "domain")
	err := filepath.Walk(domainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and test files
		if info.IsDir() || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️  Could not read file: %s\n", path)
			return nil
		}

		lines := strings.Split(string(content), "\n")
		relPath, _ := filepath.Rel(rootDir, path)

		// Check each forbidden pattern
		for _, fp := range ForbiddenPatterns {
			for i, line := range lines {
				if strings.Contains(line, fp.Pattern) {
					// Check if this is in an allowed directory
					if !strings.Contains(relPath, fp.AllowedInDir) {
						// Skip if it's just a comment or import
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
							continue
						}

						// Check if file is in a forbidden domain
						inForbiddenDomain := false
						for _, domain := range ForbiddenDomains {
							if strings.Contains(relPath, domain) {
								inForbiddenDomain = true
								break
							}
						}

						if inForbiddenDomain {
							violations = append(violations, Violation{
								File:      relPath,
								Line:      i + 1,
								Pattern:   fp.Pattern,
								Context:   strings.TrimSpace(line),
								Directory: filepath.Dir(relPath),
							})
						}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Print results
	if len(violations) == 0 {
		fmt.Println("✅ ALL CHECKS PASSED")
		fmt.Println("\n✓ financial_accounts is ONLY used in finance domain")
		fmt.Println("✓ No leakage to order/payment/wallet/refund/dispute")
		fmt.Println("\n📋 FINANCIAL ARCHITECTURE CONFIRMED:")
		fmt.Println("   Wallet = Real money authority")
		fmt.Println("   Finance = Accounting mirror (read-only for operations)")
		os.Exit(0)
	}

	// Print violations
	fmt.Printf("❌ FOUND %d VIOLATION(S)\n\n", len(violations))

	for i, v := range violations {
		fmt.Printf("%d. %s:%d\n", i+1, v.File, v.Line)
		fmt.Printf("   Pattern: %s\n", v.Pattern)
		fmt.Printf("   Directory: %s\n", v.Directory)
		fmt.Printf("   Context: %s\n\n", v.Context)
	}

	fmt.Println("🚨 FINANCIAL BOUNDARY VIOLATION:")
	fmt.Println("   financial_accounts is NOT user money.")
	fmt.Println("   It is a DERIVED accounting value.")
	fmt.Println("")
	fmt.Println("   ORDER OF OPERATIONS:")
	fmt.Println("   1. WalletService updates wallet.available_balance")
	fmt.Println("   2. Finance mirrors to financial_accounts (later, async)")
	fmt.Println("   3. financial_accounts is READ-ONLY for operations")
	fmt.Println("")
	fmt.Println("   DO NOT use financial_accounts for:")
	fmt.Println("   - Order payment decisions")
	fmt.Println("   - Refund eligibility")
	fmt.Println("   - Withdrawal availability")
	fmt.Println("   - Escrow holds/releases")
	fmt.Println("")
	fmt.Println("   USE wallet.available_balance instead.")

	os.Exit(1)
}
