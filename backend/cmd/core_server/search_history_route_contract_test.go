package main

import (
	"os"
	"strings"
	"testing"
)

func TestSearchHistoryRoutes_MapClearAllAndDeleteOneSeparately(t *testing.T) {
	src, err := os.ReadFile("routes_core.go")
	if err != nil {
		t.Fatalf("read routes_core.go: %v", err)
	}

	code := string(src)

	clearAll := `searchRoutes.DELETE("/history", deps.SearchHandler.ClearSearchHistory)`
	deleteOne := `searchRoutes.DELETE("/history/:id", deps.SearchHandler.DeleteSearchHistory)`

	if !strings.Contains(code, clearAll) {
		t.Fatalf("missing clear-all route mapping: %s", clearAll)
	}
	if !strings.Contains(code, deleteOne) {
		t.Fatalf("missing delete-one route mapping: %s", deleteOne)
	}
	if strings.Contains(code, `searchRoutes.DELETE("/history", deps.SearchHandler.DeleteSearchHistory)`) {
		t.Fatal("regression: DELETE /history must not be wired to DeleteSearchHistory")
	}
}
