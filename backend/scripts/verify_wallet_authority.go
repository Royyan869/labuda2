//go:build ignore
// +build ignore

// WALLET AUTHORITY VERIFICATION SCRIPT
//
// This script enforces that Wallet is the ONLY domain allowed to:
// - Mutate available_balance or held_balance
// - Call AtomicUpdateAvailableBalance
// - Call TransferHeldToAvailable
//
// USAGE: go run scripts/verify_wallet_authority.go
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

// ForbiddenPatterns are patterns that should ONLY appear in wallet domain
var ForbiddenPatterns = []struct {
	Pattern      string
	Description  string
	AllowedInDir string
}{
	{
		Pattern:      "available_balance",
		Description:  "Direct balance field access",
		AllowedInDir: "wallet",
	},
	{
		Pattern:      "held_balance",
		Description:  "Direct balance field access",
		AllowedInDir: "wallet",
	},
	{
		Pattern:      "AtomicUpdateAvailableBalance",
		Description:  "Balance mutation method",
		AllowedInDir: "wallet",
	},
	{
		Pattern:      "TransferHeldToAvailable",
		Description:  "Balance transfer method",
		AllowedInDir: "wallet",
	},
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

	fmt.Println("🔍 WALLET AUTHORITY VERIFICATION")
	fmt.Println("================================")
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
						if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
							continue
						}

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

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Print results
	if len(violations) == 0 {
		fmt.Println("✅ ALL CHECKS PASSED")
		fmt.Println("\nNo forbidden balance mutations found outside wallet domain.")
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

	fmt.Println("🚨 ACTION REQUIRED:")
	fmt.Println("   All money operations MUST go through WalletService.")
	fmt.Println("   Direct balance mutation is forbidden outside wallet domain.")

	os.Exit(1)
}
