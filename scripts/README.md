# PostgreSQL Backup & Restore Scripts

Automatic PostgreSQL backup solution for the Labuda project.

## Features

- **Full database backup** - Backs up entire database, no per-table management
- **Automatic retention policy** - Keeps backups for 7 days by default
- **Zero maintenance** - No changes needed when new tables are added
- **Cross-platform** - Works on Windows, Linux, macOS, and WSL
- **Docker-native** - Uses Docker exec, no PostgreSQL client needed on host

## Files

| File | Purpose |
|------|---------|
| `db_backup.sh` | Backup script for Linux/macOS/WSL |
| `db_backup.ps1` | Backup script for Windows PowerShell |
| `db_restore.sh` | Restore script for Linux/macOS/WSL |
| `db_restore.ps1` | Restore script for Windows PowerShell |

## Quick Start

### Manual Backup

**Windows (PowerShell):**
```powershell
cd scripts
.\db_backup.ps1
```

**Linux/macOS/WSL:**
```bash
cd scripts
chmod +x db_backup.sh
./db_backup.sh
```

### Manual Restore

**Windows (PowerShell):**
```powershell
cd scripts
# Restore to main database (requires confirmation)
.\db_restore.ps1 backups\labuda_20250220_030000.dump

# Restore to test database (safe)
.\db_restore.ps1 backups\labuda_20250220_030000.dump labuda_restore_test
```

**Linux/macOS/WSL:**
```bash
cd scripts
# Restore to main database (requires confirmation)
./db_restore.sh backups/labuda_20250220_030000.dump

# Restore to test database (safe)
./db_restore.sh backups/labuda_20250220_030000.dump labuda_restore_test
```

## Setting Up Automatic Daily Backup

### Windows Task Scheduler

1. Open Task Scheduler (`taskschd.msc`)
2. Click "Create Task" on the right
3. General tab:
   - Name: `Labuda PostgreSQL Backup`
   - Select "Run whether user is logged in or not"
4. Triggers tab:
   - Click "New"
   - Begin the task: "Daily"
   - Start: 3:00:00 AM
   - Click OK
5. Actions tab:
   - Click "New"
   - Program: `powershell.exe`
   - Arguments: `-ExecutionPolicy Bypass -File "C:\Project\labuda\scripts\db_backup.ps1"`
   - Start in: `C:\Project\labuda\scripts`
   - Click OK
6. Click OK to create the task

### Linux/macOS/WSL (Cron)

```bash
# Edit crontab
crontab -e

# Add this line for daily backup at 3:00 AM
0 3 * * * cd /path/to/labuda/scripts && ./db_backup.sh >> /var/log/labuda_backup.log 2>&1
```

## Configuration

Edit the backup script to change these settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `CONTAINER_NAME` | `labuda-postgres` | Docker container name |
| `DB_NAME` | `labuda` | Database name |
| `DB_USER` | `labuda` | Database user |
| `RETENTION_DAYS` | `7` | Days to keep backups |

## Backup Location

Backups are stored in: `scripts/backups/`

Format: `labuda_YYYYMMDD_HHMMSS.dump`

Example: `labuda_20250220_030000.dump`

## Verifying Backups

1. Check the backups directory:
   ```powershell
   ls scripts\backups
   ```

2. Verify file contents (list tables):
   ```bash
   docker exec labuda-postgres pg_restore -l backups/latest.dump
   ```

3. Test restore to a temporary database (see Restore section above)

## Production Deployment Considerations

1. **Off-site storage**: Copy backups to S3, GCS, or similar
2. **Encryption**: Encrypt backups if they contain sensitive data
3. **Monitoring**: Set up alerts for backup failures
4. **Testing**: Regularly test restore procedures
5. **Retention**: Adjust retention based on compliance requirements

## Troubleshooting

**Container not running:**
```bash
docker compose up -d postgres
```

**Permission denied on script:**
```bash
chmod +x scripts/db_backup.sh
```

**Disk space full:**
- Reduce RETENTION_DAYS
- Move backups to external storage
