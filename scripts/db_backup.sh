#!/bin/bash
# ============================================
# PostgreSQL Backup Script (Linux/macOS/WSL)
# Labuda Project - Automatic Daily Backup
# ============================================

set -e

# Configuration
CONTAINER_NAME="labuda-postgres"
DB_NAME="labuda"
DB_USER="labuda"
BACKUP_DIR="./backups"
RETENTION_DAYS=7

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Create backup directory if not exists
mkdir -p "$BACKUP_DIR"

# Generate timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/labuda_$TIMESTAMP.dump"

echo "=========================================="
echo "PostgreSQL Backup Started"
echo "Timestamp: $TIMESTAMP"
echo "=========================================="

# Check if Docker container is running
if ! docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${RED}ERROR: Docker container '$CONTAINER_NAME' is not running!${NC}"
    echo "Please start PostgreSQL with: docker compose up -d postgres"
    exit 1
fi

# Run pg_dump inside Docker container and pipe output to host file
echo -e "${CYAN}Running pg_dump on container: $CONTAINER_NAME${NC}"
echo -e "${CYAN}Writing backup to: $BACKUP_FILE${NC}"

docker exec "$CONTAINER_NAME" pg_dump -U "$DB_USER" -d "$DB_NAME" -F c > "$BACKUP_FILE"

if [ $? -ne 0 ]; then
    echo -e "${RED}ERROR: pg_dump failed${NC}"
    exit 1
fi

# ============================================
# INTEGRITY CHECKS
# ============================================

# Check file exists and not empty
echo -e "${CYAN}Running integrity checks...${NC}"

if [ ! -s "$BACKUP_FILE" ]; then
    echo -e "${RED}ERROR: Backup file is empty or missing!${NC}"
    exit 1
fi

# Copy file to container for validation (pg_restore needs file access inside container)
docker cp "$BACKUP_FILE" "${CONTAINER_NAME}:/tmp/backup_verify.dump"

# Validate dump file integrity using pg_restore -l (list contents)
docker exec "$CONTAINER_NAME" pg_restore -l /tmp/backup_verify.dump > /dev/null 2>&1

VALIDATE_EXIT_CODE=$?

# Clean up verification file
docker exec "$CONTAINER_NAME" rm -f /tmp/backup_verify.dump

if [ $VALIDATE_EXIT_CODE -ne 0 ]; then
    echo -e "${RED}ERROR: Backup file is corrupted or invalid!${NC}"
    rm -f "$BACKUP_FILE"
    exit 1
fi

echo -e "${GREEN}Integrity check passed: valid PostgreSQL dump file${NC}"
echo -e "${CYAN}----------------------------------------${NC}"

# Get file size
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    FILE_SIZE=$(stat -f%z "$BACKUP_FILE" 2>/dev/null || echo "0")
    FILE_SIZE_MB=$(awk "BEGIN {printf \"%.2f\", $FILE_SIZE/1024/1024}")
else
    # Linux
    FILE_SIZE=$(stat -c%s "$BACKUP_FILE" 2>/dev/null || echo "0")
    FILE_SIZE_MB=$(awk "BEGIN {printf \"%.2f\", $FILE_SIZE/1024/1024}")
fi

echo -e "${GREEN}=========================================="
echo "Backup completed successfully!"
echo "File: $BACKUP_FILE"
echo "Size: ${FILE_SIZE_MB} MB"
echo "==========================================${NC}"

# Delete old backups (retention policy)
echo -e "${CYAN}Applying retention policy: keeping last $RETENTION_DAYS days...${NC}"

DELETED_COUNT=0
find "$BACKUP_DIR" -type f -name "labuda_*.dump" -mtime +$RETENTION_DAYS -print | while read -r old_backup; do
    echo -e "${YELLOW}Deleting old backup: $(basename "$old_backup")${NC}"
    rm -f "$old_backup"
    DELETED_COUNT=$((DELETED_COUNT + 1))
done

echo -e "${GREEN}=========================================="
echo "Backup process completed!"
echo "==========================================${NC}"
