#!/bin/bash
# ============================================
# PostgreSQL Restore Script (Linux/macOS/WSL)
# Labuda Project - Database Restore
# ============================================

set -e

# Configuration
CONTAINER_NAME="labuda-postgres"
DB_NAME="labuda"
DB_USER="labuda"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if backup file is provided
if [ -z "$1" ]; then
    echo -e "${RED}ERROR: Please specify the backup file to restore${NC}"
    echo "Usage: $0 <backup_file.dump> [target_database_name]"
    echo ""
    echo "Examples:"
    echo "  $0 backups/labuda_20250220_030000.dump"
    echo "  $0 backups/labuda_20250220_030000.dump labuda_restore_test"
    exit 1
fi

BACKUP_FILE="$1"
TARGET_DB="${2:-$DB_NAME}"

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    echo -e "${RED}ERROR: Backup file not found: $BACKUP_FILE${NC}"
    exit 1
fi

echo "=========================================="
echo "PostgreSQL Restore"
echo "Backup file: $BACKUP_FILE"
echo "Target database: $TARGET_DB"
echo "=========================================="

# Check if Docker container is running
if ! docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${RED}ERROR: Docker container '$CONTAINER_NAME' is not running!${NC}"
    echo "Please start PostgreSQL with: docker compose up -d postgres"
    exit 1
fi

# Warning if restoring to main database
if [ "$TARGET_DB" = "$DB_NAME" ]; then
    echo -e "${YELLOW}WARNING: You are about to overwrite the main database '$DB_NAME'!${NC}"
    echo -e "${YELLOW}This will DELETE ALL existing data.${NC}"
    echo ""
    read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirmation
    if [ "$confirmation" != "yes" ]; then
        echo "Restore cancelled."
        exit 0
    fi
fi

# If restoring to a test database, create it first
if [ "$TARGET_DB" != "$DB_NAME" ]; then
    echo -e "${CYAN}Creating test database: $TARGET_DB${NC}"
    docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -c "DROP DATABASE IF EXISTS $TARGET_DB;"
    docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -c "CREATE DATABASE $TARGET_DB;"
fi

# Copy backup to container
echo -e "${CYAN}Copying backup file to container...${NC}"
docker cp "$BACKUP_FILE" "${CONTAINER_NAME}:/tmp/labuda_restore.dump"

# Restore
echo -e "${CYAN}Restoring database...${NC}"
docker exec "$CONTAINER_NAME" pg_restore -U "$DB_USER" -d "$TARGET_DB" -c --if-exists /tmp/labuda_restore.dump

if [ $? -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "Restore completed successfully!"
    echo "Database: $TARGET_DB"
    echo "==========================================${NC}"
else
    echo -e "${RED}ERROR: Restore failed${NC}"
    exit 1
fi

# Clean up
docker exec "$CONTAINER_NAME" rm -f /tmp/labuda_restore.dump
