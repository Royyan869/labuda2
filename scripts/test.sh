#!/bin/bash
# Run tests with isolated test database.
#
# Usage:
#   ./scripts/test.sh                    # Run all tests
#   ./scripts/test.sh -run TestExpiry   # Run specific test
#   ./scripts/test.sh -v ./tests/...    # Run tests in package with verbose output

set -e

# Change to backend directory
cd "$(dirname "$0")/.." || exit 1
cd backend || exit 1

echo "==> Running tests with isolated test database..."
echo "    Test DB: labuda_test"

# Run tests with TEST_MODE flag
TEST_MODE=true go test "$@" -v
