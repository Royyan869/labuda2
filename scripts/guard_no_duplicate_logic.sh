#!/bin/bash
set -e

echo "🔒 Checking duplicate DOMAIN WRITE (follow)..."

# Hanya cek IMPLEMENTATION di infrastructure layer (actual database write)
IMPL_COUNT=$(grep -r "func (.*) InsertFollow(" backend/internal 2>/dev/null \
  | grep -v "_test.go" \
  | grep "/infrastructure/" \
  | wc -l)

if [ "$IMPL_COUNT" -gt 1 ]; then
  echo "❌ BLOCKED: Multiple FOLLOW WRITE implementations detected ($IMPL_COUNT)"
  grep -r "func (.*) InsertFollow(" backend/internal | grep -v "_test.go" | grep "/infrastructure/"
  exit 1
fi

echo "✅ Single follow write implementation"
