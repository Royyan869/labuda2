# PostgreSQL Backup Setup - Labuda Project

## STEP 1 — DATABASE ENVIRONMENT ✓ IDENTIFIED

| Setting | Value |
|---------|-------|
| **Platform** | Docker Container |
| **Container Name** | `labuda-postgres` |
| **Database Host** | localhost |
| **Database Port** | 5432 |
| **Database Name** | `labuda` |
| **Database User** | `labuda` |

## STEP 2 — BACKUP SCRIPT ✓ CREATED

### Files Created:

```
scripts/
├── db_backup.sh          # Linux/macOS/WSL backup script
├── db_backup.ps1         # Windows PowerShell backup script
├── db_restore.sh         # Linux/macOS/WSL restore script
├── db_restore.ps1        # Windows PowerShell restore script
├── backups/              # Backup storage directory (auto-created)
└── README.md             # Full documentation
```

## STEP 3 — TEST BACKUP (User Action Required)

### Run this command to test:

**Windows (PowerShell):**
```powershell
cd C:\Project\labuda\scripts
.\db_backup.ps1
```

**Linux/macOS/WSL:**
```bash
cd /path/to/labuda/scripts
chmod +x db_backup.sh
./db_backup.sh
```

### Expected Output:
```
==========================================
PostgreSQL Backup Started
Timestamp: 20250220_143045
==========================================
Running pg_dump on container: labuda-postgres
Copying backup from container to host...
Cleaning up temporary file in container...
==========================================
Backup completed successfully!
File: ./backups/labuda_20250220_143045.dump
Size: X.XX MB
==========================================
Applying retention policy: keeping last 7 days...
==========================================
Backup process completed!
==========================================
```

### Verify backup created:
```powershell
# Windows
dir scripts\backups

# Linux/macOS
ls -lh scripts/backups
```

## STEP 4 — SETUP AUTO DAILY BACKUP

### Option A: Windows Task Scheduler (Recommended for Windows)

1. Open Task Scheduler:
   ```
   Win+R → taskschd.msc → Enter
   ```

2. Create Task:
   - **Name**: `Labuda PostgreSQL Backup`
   - **General** → Select "Run whether user is logged in or not"
   - **Triggers** → New → Daily at 3:00 AM
   - **Actions** → New:
     - Program: `powershell.exe`
     - Arguments: `-ExecutionPolicy Bypass -File "C:\Project\labuda\scripts\db_backup.ps1"`
     - Start in: `C:\Project\labuda\scripts`

3. Verify: Right-click task → "Run" to test

### Option B: Cron (Linux/macOS/WSL)

```bash
crontab -e

# Add this line:
0 3 * * * cd /c/Project/labuda/scripts && ./db_backup.sh >> /tmp/labuda_backup.log 2>&1
```

## STEP 5 — RETENTION POLICY ✓ INCLUDED

- **Retention**: 7 days (configurable)
- **Auto-delete**: Old backups removed after each backup
- **Location**: `scripts/backups/`

To change retention, edit the script:
```
$RETENTION_DAYS = 7    # PowerShell
RETENTION_DAYS=7       # Bash
```

## STEP 6 — VERIFY RESTORE (User Action Required)

### Test restore to a temporary database:

**Windows:**
```powershell
cd C:\Project\labuda\scripts
.\db_restore.ps1 backups\labuda_20250220_143045.dump labuda_restore_test
```

**Linux/macOS/WSL:**
```bash
cd /path/to/labuda/scripts
./db_restore.sh backups/labuda_20250220_143045.dump labuda_restore_test
```

### Verify restore:
```bash
docker exec labuda-postgres psql -U labuda -d labuda_restore_test -c "\dt"
```

### Clean up test database:
```bash
docker exec labuda-postgres psql -U labuda -c "DROP DATABASE labuda_restore_test;"
```

## FINAL STATUS

| Item | Status |
|------|--------|
| Database Environment Identified | ✓ |
| Backup Script Created | ✓ |
| Restore Script Created | ✓ |
| Retention Policy Implemented | ✓ |
| Documentation Created | ✓ |
| Backup Tested | ⚠️ Requires Docker |
| Restore Verified | ⚠️ Requires Docker |
| Auto Backup Scheduled | ⚠️ User to configure |

**BACKUP READY: YES** (Scripts ready, requires testing with running Docker)

## QUICK REFERENCE

| Action | Command |
|--------|---------|
| **Manual Backup** | `scripts/db_backup.ps1` or `./scripts/db_backup.sh` |
| **Restore to Test DB** | `scripts/db_restore.ps1 backups\labuda_XXX.dump labuda_test` |
| **Restore to Main DB** | `scripts/db_restore.ps1 backups\labuda_XXX.dump` |
| **List Backups** | `dir scripts\backups` or `ls -lh scripts/backups` |
| **Start PostgreSQL** | `docker compose up -d postgres` |

## PRODUCTION CONSIDERATIONS

1. **Off-site Backup**: Consider copying backups to S3, GCS, or external storage
2. **Encryption**: Encrypt backups if containing sensitive data
3. **Monitoring**: Add logging/alerting for backup failures
4. **Testing**: Schedule regular restore testing
5. **Documentation**: Keep this file updated with any changes
