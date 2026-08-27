#!/bin/bash
set -e

echo "🔒 Checking mutation without event..."

# Only check application layer where business logic lives
FILES=$(grep -rl "InsertFollow(" backend/internal/social/graph/application 2>/dev/null | grep -v "_test.go")

if [ -z "$FILES" ]; then
  echo "✅ No InsertFollow in application layer"
  exit 0
fi

for f in $FILES; do
  if ! grep -q "EventUserFollowed" "$f"; then
    echo "❌ BLOCKED: InsertFollow mutation without EventUserFollowed in $f"
    exit 1
  fi
done

echo "✅ Mutation-event consistency OK"
