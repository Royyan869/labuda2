#!/bin/bash
set -e

echo "🔒 Checking EVENT EXECUTION GUARANTEE..."

FILES=$(grep -rl "InsertFollow(" backend/internal/social/graph/application | grep -v "_test.go")

for f in $FILES; do
  # Extract function blocks containing InsertFollow
  awk '
  /func / {
    in_func = 1
    func_block = ""
    next
  }
  in_func {
    func_block = func_block $0 "\n"
  }
  /^}$/ {
    if (in_func && func_block ~ /InsertFollow\(/) {
      if (func_block !~ /EventUserFollowed/) {
        print "❌ BLOCKED: InsertFollow without EventUserFollowed in same function"
        print "File: " FILENAME
        exit 1
      }
      
      # Check if event comes AFTER mutation in the code
      split(func_block, lines, "\n")
      mutation_pos = 0
      event_pos = 0
      
      for (i = 1; i <= length(lines); i++) {
        if (lines[i] ~ /InsertFollow\(/) {
          mutation_pos = i
        }
        if (lines[i] ~ /EventUserFollowed/) {
          event_pos = i
        }
      }
      
      if (mutation_pos > 0 && event_pos > 0 && event_pos < mutation_pos) {
        print "❌ BLOCKED: Event BEFORE mutation in " FILENAME
        print "Event must come AFTER InsertFollow"
        exit 1
      }
      
      # Check for problematic early returns after successful mutation
      # Pattern: InsertFollow succeeds, then return before event
      in_error_check = 0
      mutation_successful = 0
      
      for (i = mutation_pos; i <= event_pos; i++) {
        line = lines[i]
        
        # If we see InsertFollow call (not in if condition)
        if (line ~ /InsertFollow\(/ && line !~ /if/) {
          mutation_successful = 1
        }
        
        # If we see InsertFollow in error check
        if (line ~ /if.*InsertFollow\(/) {
          in_error_check = 1
        }
        
        # Error handling returns are OK (in_error_check context)
        if (line ~ /err != nil/ || line ~ /err :=/) {
          in_error_check = 1
        }
        
        # Non-error-handling return after successful mutation = BAD
        if (mutation_successful && line ~ /return/ && !in_error_check) {
          # Check if this is the error handling return from InsertFollow
          if (line !~ /failed to insert follow/ && line !~ /fmt\.Errorf/ && line !~ /%w/) {
            print "❌ BLOCKED: Early return after successful mutation in " FILENAME
            print "Line: " line
            exit 1
          }
        }
        
        # Reset error check after the if block
        if (line ~ /\}/) {
          in_error_check = 0
        }
      }
    }
    in_func = 0
    func_block = ""
  }
  ' "$f"
done

echo "✅ Event execution guaranteed"
