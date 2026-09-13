package bootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoHTTPBootstrapBackdoor(t *testing.T) {
	// Scan production route files for forbidden bootstrap endpoints.
	forbidden := []string{
		"/bootstrap",
		"/setup/admin",
		"bootstrap/admin",
	}
	// Only production route wiring counts — glob backend/cmd/core_server and internal middleware/handlers
	roots := []string{
		"../../cmd/core_server",
		"../../internal",
	}
	// Walk
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip test files and our bootstrap package itself (which legitimately contains bootstrap word)
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(path, "/platform/bootstrap/") {
				return nil
			}
			if strings.Contains(path, "/platform/capability/bootstrap") {
				return nil
			}
			b, rerr := os.ReadFile(path)
			require.NoError(t, rerr)
			content := strings.ToLower(string(b))
			for _, f := range forbidden {
				assert.NotContains(t, content, strings.ToLower(f), "forbidden HTTP bootstrap route fragment %q found in %s", f, path)
			}
			return nil
		})
		require.NoError(t, err)
	}
}

func TestNoStartupAutoPromotion(t *testing.T) {
	// Ensure core_server main.go and dependencies.go do not auto-promote ADMIN_EMAIL / INITIAL_ADMIN
	forbidden := []string{
		"ADMIN_EMAIL",
		"INITIAL_ADMIN",
		"auto-promote",
	}
	targets := []string{
		"../../cmd/core_server/main.go",
		"../../cmd/core_server/routes_core.go",
		"../../internal/serverboot/dependencies.go",
		"../../internal/serverboot",
	}
	for _, p := range targets {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			_ = filepath.Walk(p, func(path string, _ os.FileInfo, _ error) error {
				if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					b, _ := os.ReadFile(path)
					c := string(b)
					for _, f := range forbidden {
						assert.NotContains(t, c, f, "forbidden auto-promotion marker %q in %s", f, path)
					}
				}
				return nil
			})
		} else {
			b, _ := os.ReadFile(p)
			for _, f := range forbidden {
				assert.NotContains(t, string(b), f)
			}
		}
	}
}
