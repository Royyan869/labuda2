# ============================================
# PostgreSQL Restore Script (Windows/PowerShell)
# Labuda Project - Database Restore
# ============================================

$ErrorActionPreference = "Stop"

# Configuration
$CONTAINER_NAME = "labuda-postgres"
$DB_NAME = "labuda"
$DB_USER = "labuda"

# Check if backup file is provided
if ($args.Count -eq 0) {
    Write-Host "ERROR: Please specify the backup file to restore" -ForegroundColor Red
    Write-Host "Usage: .\db_restore.ps1 <backup_file.dump> [target_database_name]"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\db_restore.ps1 backups\labuda_20250220_030000.dump"
    Write-Host "  .\db_restore.ps1 backups\labuda_20250220_030000.dump labuda_restore_test"
    exit 1
}

$BACKUP_FILE = $args[0]
$TARGET_DB = if ($args.Count -ge 2) { $args[1] } else { $DB_NAME }

# Check if backup file exists
if (-not (Test-Path $BACKUP_FILE)) {
    Write-Host "ERROR: Backup file not found: $BACKUP_FILE" -ForegroundColor Red
    exit 1
}

Write-Host "=========================================="
Write-Host "PostgreSQL Restore"
Write-Host "Backup file: $BACKUP_FILE"
Write-Host "Target database: $TARGET_DB"
Write-Host "=========================================="

# Check if Docker container is running
$containerCheck = docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}"
if (-not $containerCheck) {
    Write-Host "ERROR: Docker container '$CONTAINER_NAME' is not running!" -ForegroundColor Red
    Write-Host "Please start PostgreSQL with: docker compose up -d postgres"
    exit 1
}

# Warning if restoring to main database
if ($TARGET_DB -eq $DB_NAME) {
    Write-Host "WARNING: You are about to overwrite the main database '$DB_NAME'!" -ForegroundColor Yellow
    Write-Host "This will DELETE ALL existing data." -ForegroundColor Yellow
    Write-Host ""
    $confirmation = Read-Host "Are you sure you want to continue? (type 'yes' to confirm)"
    if ($confirmation -ne "yes") {
        Write-Host "Restore cancelled."
        exit 0
    }
}

# If restoring to a test database, create it first
if ($TARGET_DB -ne $DB_NAME) {
    Write-Host "Creating test database: $TARGET_DB" -ForegroundColor Cyan
    docker exec $CONTAINER_NAME psql -U $DB_USER -c "DROP DATABASE IF EXISTS $TARGET_DB;"
    docker exec $CONTAINER_NAME psql -U $DB_USER -c "CREATE DATABASE $TARGET_DB;"
}

# Copy backup to container
Write-Host "Copying backup file to container..." -ForegroundColor Cyan
docker cp "$BACKUP_FILE" "${CONTAINER_NAME}:/tmp/labuda_restore.dump"

# Restore
Write-Host "Restoring database..." -ForegroundColor Cyan
$restoreCmd = "docker exec $CONTAINER_NAME pg_restore -U $DB_USER -d $TARGET_DB -c --if-exists /tmp/labuda_restore.dump"
Invoke-Expression $restoreCmd

if ($LASTEXITCODE -eq 0) {
    Write-Host "=========================================="
    Write-Host "Restore completed successfully!" -ForegroundColor Green
    Write-Host "Database: $TARGET_DB"
    Write-Host "=========================================="
} else {
    Write-Host "ERROR: Restore failed" -ForegroundColor Red
    exit 1
}

# Clean up
docker exec $CONTAINER_NAME rm -f /tmp/labuda_restore.dump
