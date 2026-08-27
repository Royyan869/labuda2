#!/bin/bash
set -e

echo "🔒 Checking FLOW: InsertFollow → EventUserFollowed (same function)"

FILES=$(grep -rl "InsertFollow(" backend/internal/social/graph/application | grep -v "_test.go")

for f in $FILES; do
  awk '
  /func / {
    in_func = 1
    func_block = $0
    next
  }
  in_func {
    func_block = func_block "\n" $0
  }
  /^}$/ {
    if (in_func && func_block ~ /InsertFollow\(/) {
      if (func_block !~ /EventUserFollowed/) {
        print "❌ BLOCKED: InsertFollow without EventUserFollowed in same function"
        print "File: " FILENAME
        print "Function block:"
        print func_block
        exit 1
      }
    }
    in_func = 0
    func_block = ""
  }
  ' "$f"
done

echo "✅ Flow consistency OK"
