#!/bin/bash
# Create test database (labuda_test) if it doesn't exist.
# This script is run by postgres container on startup.
#
# Usage: docker compose -f docker-compose.yml -f docker-compose.test.yml up

set -e

echo "==> Creating test database..."

# Check if labuda_test exists
RESULT=$(psql -U labuda -tc "SELECT 1 FROM pg_database WHERE datname='labuda_test'" 2>/dev/null || echo "0")

if [ "$RESULT" = "1" ]; then
    echo "==> Test database 'labuda_test' already exists"
else
    echo "==> Creating test database 'labuda_test'..."
    createdb -U labuda labuda_test
    echo "==> Test database 'labuda_test' created successfully"
fi

echo "==> Test database setup complete"
