#!/bin/bash
set -e

# B4.2 / G5 — evaluator.ViewerContextShadow and evaluator.TargetContextShadow
# MUST NOT be referenced outside the evaluator package itself.
#
# Doctrine: docs/contracts/governance-constitution.md §3 F5 / F18 +
# §8.1 G5. The shadow type is TRANSITIONAL DEBT bounded to
# backend/internal/governance/evaluator/* until pilots are rebuilt on
# the canonical viewercontext.ViewerContext (constitution §9 batches
# C1 and D1). Any new cross-package consumer is a forbidden spread of
# the transitional type into runtime authority.
#
# Detection strategy: the type is qualified `evaluator.ViewerContextShadow`
# (and `evaluator.TargetContextShadow`) only when referenced from
# outside the evaluator package. Inside the package it is unqualified.
# A bare qualified-form match anywhere in the tree is therefore a
# guaranteed cross-package consumer.

echo "🔒 G5: ViewerContextShadow forbidden outside evaluator package"

# Match qualified references only — these can only appear in importers
# of the evaluator package. *_test.go files in the evaluator package
# itself use the unqualified form and are not matched by this pattern.
PATTERN='evaluator\.(ViewerContextShadow|TargetContextShadow)'

VIOLATIONS=$(grep -rnE "$PATTERN" backend \
  --include='*.go' 2>/dev/null || true)

if [ -n "$VIOLATIONS" ]; then
  echo "❌ BLOCKED: shadow type referenced outside evaluator package"
  echo ""
  echo "Forbidden pattern (G5): evaluator.ViewerContextShadow / evaluator.TargetContextShadow"
  echo "consumed by a non-evaluator package. The shadow type is"
  echo "transitional debt scoped to backend/internal/governance/evaluator/*"
  echo "until pilot rebuilds (C1, D1)."
  echo "See: docs/contracts/governance-constitution.md §3 F5, §8.1 G5"
  echo ""
  echo "Offending references:"
  echo "$VIOLATIONS"
  exit 1
fi

echo "✅ G5 passed: no cross-package shadow type consumers"
