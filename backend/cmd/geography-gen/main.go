// Command geography-gen builds the canonical Geography seed from the
// authoritative source dataset.
//
// Usage (from backend/):
//
//	go run ./cmd/geography-gen
//	go run ./cmd/geography-gen -src internal/platform/geography/source \
//	    -out internal/platform/geography/data/geography.tsv.gz
//
// Generation FAILS (non-zero exit) on any source data-integrity violation,
// including a child whose parent is absent. It never silently drops a record.
package main

import (
	"flag"
	"log"
	"path/filepath"

	gen "github.com/labuda/backend/internal/platform/geography/generator"
)

func main() {
	src := flag.String("src", filepath.Join("internal", "platform", "geography", "source"), "authoritative source dataset directory")
	out := flag.String("out", filepath.Join("internal", "platform", "geography", "data", "geography.tsv.gz"), "output seed path")
	flag.Parse()

	source, err := gen.ReadSource(*src)
	if err != nil {
		log.Fatalf("read source: %v", err)
	}

	rows, err := gen.Build(source)
	if err != nil {
		log.Fatalf("GENERATION FAILURE: %v", err)
	}

	if err := gen.WriteGzipTSV(*out, rows); err != nil {
		log.Fatalf("write seed: %v", err)
	}

	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Level]++
	}
	log.Printf("geography seed written: %s provinces=%d regencies=%d districts=%d villages=%d total=%d",
		*out, counts[gen.LevelProvince], counts[gen.LevelRegency], counts[gen.LevelDistrict], counts[gen.LevelVillage], len(rows))
}
