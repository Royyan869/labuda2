#!/bin/bash
set -e

# B4.2 / G2 — evaluator package MUST NOT import "database/sql".
#
# Doctrine: docs/contracts/governance-constitution.md §3 (F1, F2, F3) +
# §8.1 G2. The evaluator is a pure decision module. Importing
# "database/sql" implies the evaluator owns query authority, which
# violates the locked rule "evaluator never fetches internally" and the
# operational lock in docs/contracts/viewer-context.md §2.4.
#
# This guard is anti-drift only. Existing transitional debt on the
# evaluator package (the pgxpool runners on feed and content-detail
# pilots) is bounded separately and remains in shadow until the
# corresponding rebuild batches (C1, D1) per constitution §9. This
# guard does not enforce against pgxpool today; it enforces only
# the cleaner-still rule for "database/sql".

echo "🔒 G2: evaluator MUST NOT import database/sql"

EVALUATOR_DIR="backend/internal/governance/evaluator"

if [ ! -d "$EVALUATOR_DIR" ]; then
  echo "✅ G2 skipped: $EVALUATOR_DIR not present"
  exit 0
fi

# Match the exact Go import path "database/sql" inside *.go files,
# excluding *_test.go where a fixture might legitimately import the
# stdlib for mock builders. The evaluator decision functions and
# runners are non-test code; that is what this guard locks.
VIOLATIONS=$(grep -rn '"database/sql"' "$EVALUATOR_DIR" \
  --include='*.go' 2>/dev/null \
  | grep -v '_test\.go' || true)

if [ -n "$VIOLATIONS" ]; then
  echo "❌ BLOCKED: evaluator package imports \"database/sql\""
  echo ""
  echo "Forbidden pattern (G2): import \"database/sql\" inside $EVALUATOR_DIR"
  echo "See: docs/contracts/governance-constitution.md §3 F1–F3, §8.1 G2"
  echo ""
  echo "Offending files:"
  echo "$VIOLATIONS"
  exit 1
fi

echo "✅ G2 passed: no database/sql imports in evaluator package"
