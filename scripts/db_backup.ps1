# ============================================
# PostgreSQL Backup Script (Windows/PowerShell)
# Labuda Project - Automatic Daily Backup
# ============================================

$ErrorActionPreference = "Stop"

# Configuration
$CONTAINER_NAME = "labuda-postgres"
$DB_NAME = "labuda"
$DB_USER = "labuda"
$BACKUP_DIR = "./backups"
$RETENTION_DAYS = 7

# Create backup directory if not exists
if (-not (Test-Path $BACKUP_DIR)) {
    New-Item -ItemType Directory -Path $BACKUP_DIR | Out-Null
    Write-Host "Created backup directory: $BACKUP_DIR" -ForegroundColor Green
}

# Generate timestamp
$TIMESTAMP = Get-Date -Format "yyyyMMdd_HHmmss"
$BACKUP_FILE = "$BACKUP_DIR\labuda_$TIMESTAMP.dump"

Write-Host "=========================================="
Write-Host "PostgreSQL Backup Started"
Write-Host "Timestamp: $TIMESTAMP"
Write-Host "=========================================="

# Check if Docker container is running
$containerCheck = docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}"
if (-not $containerCheck) {
    Write-Host "ERROR: Docker container '$CONTAINER_NAME' is not running!" -ForegroundColor Red
    Write-Host "Please start PostgreSQL with: docker compose up -d postgres"
    exit 1
}

# Run pg_dump inside Docker container and pipe output to host file
Write-Host "Running pg_dump on container: $CONTAINER_NAME" -ForegroundColor Cyan
Write-Host "Writing backup to: $BACKUP_FILE" -ForegroundColor Cyan

docker exec $CONTAINER_NAME pg_dump -U $DB_USER -d $DB_NAME -F c > $BACKUP_FILE

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: pg_dump failed with exit code $LASTEXITCODE" -ForegroundColor Red
    exit 1
}

# ============================================
# INTEGRITY CHECKS
# ============================================

# Check file exists and not empty
Write-Host "Running integrity checks..." -ForegroundColor Cyan

if (!(Test-Path $BACKUP_FILE)) {
    Write-Host "ERROR: Backup file is missing!" -ForegroundColor Red
    exit 1
}

$fileSize = (Get-Item $BACKUP_FILE).Length
if ($fileSize -eq 0) {
    Write-Host "ERROR: Backup file is empty!" -ForegroundColor Red
    Remove-Item $BACKUP_FILE -Force
    exit 1
}

# Copy file to container for validation (pg_restore needs file access inside container)
docker cp $BACKUP_FILE "${CONTAINER_NAME}:/tmp/backup_verify.dump"

# Validate dump file integrity using pg_restore -l (list contents)
docker exec $CONTAINER_NAME pg_restore -l /tmp/backup_verify.dump > $null 2>&1

$validateExitCode = $LASTEXITCODE

# Clean up verification file
docker exec $CONTAINER_NAME rm -f /tmp/backup_verify.dump

if ($validateExitCode -ne 0) {
    Write-Host "ERROR: Backup file is corrupted or invalid!" -ForegroundColor Red
    Remove-Item $BACKUP_FILE -Force
    exit 1
}

Write-Host "Integrity check passed: valid PostgreSQL dump file" -ForegroundColor Green
Write-Host "----------------------------------------" -ForegroundColor Cyan

# Get file size (re-calculate after checks)
$fileSize = (Get-Item $BACKUP_FILE).Length
$fileSizeMB = [math]::Round($fileSize / 1MB, 2)

Write-Host "=========================================="
Write-Host "Backup completed successfully!" -ForegroundColor Green
Write-Host "File: $BACKUP_FILE"
Write-Host "Size: $fileSizeMB MB"
Write-Host "=========================================="

# Delete old backups (retention policy)
Write-Host "Applying retention policy: keeping last $RETENTION_DAYS days..." -ForegroundColor Cyan

$cutoffDate = (Get-Date).AddDays(-$RETENTION_DAYS)
$deletedCount = 0

Get-ChildItem -Path $BACKUP_DIR -Filter "labuda_*.dump" | Where-Object {
    $_.LastWriteTime -lt $cutoffDate
} | ForEach-Object {
    Write-Host "Deleting old backup: $($_.Name)" -ForegroundColor Yellow
    Remove-Item $_.FullName -Force
    $deletedCount++
}

if ($deletedCount -gt 0) {
    Write-Host "Deleted $deletedCount old backup(s)" -ForegroundColor Green
} else {
    Write-Host "No old backups to delete" -ForegroundColor Gray
}

Write-Host "=========================================="
Write-Host "Backup process completed!"
Write-Host "=========================================="
