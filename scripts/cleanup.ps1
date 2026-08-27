#==============================================
# Labuda Project - Cleanup Script
#==============================================
# Run weekly to prevent disk space bloat
# Usage: .\scripts\cleanup.ps1
#==============================================

$ErrorActionPreference = "SilentlyContinue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "LABUDA CLEANUP SCRIPT" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

#==============================================
# 1. Hapus Load Test Results > 7 hari
#==============================================
Write-Host "=== 1. LOAD TEST RESULTS (>7 days) ===" -ForegroundColor Yellow

$loadTestPath = 'C:\Project\labuda\backend\tests\load'
if (Test-Path $loadTestPath) {
    $cutoffDate = (Get-Date).AddDays(-7)
    $filesDeleted = 0
    $spaceReclaimed = 0

    Get-ChildItem $loadTestPath -Recurse -File -Include '*.json','*.html' -ErrorAction SilentlyContinue | Where-Object {
        $_.LastWriteTime -lt $cutoffDate -and $_.Length -gt 10MB
    } | ForEach-Object {
        $spaceReclaimed += $_.Length
        Remove-Item $_.FullName -Force
        $filesDeleted++
        Write-Host "  Deleted: $($_.FullName.Replace('C:\Project\labuda\', ''))"
    }

    Write-Host "  Deleted $filesDeleted files, reclaimed $([math]::Round($spaceReclaimed/1MB, 2)) MB" -ForegroundColor Green
} else {
    Write-Host "  Load test path not found" -ForegroundColor Gray
}

#==============================================
# 2. Truncate Log Files > 50MB
#==============================================
Write-Host "`n=== 2. LOG FILES (>50MB) ===" -ForegroundColor Yellow

$logThreshold = 50MB
$logFiles = Get-ChildItem 'C:\Project\labuda' -Recurse -Filter '*.log' -ErrorAction SilentlyContinue | Where-Object { $_.Length -gt $logThreshold }

if ($logFiles) {
    foreach ($log in $logFiles) {
        $sizeBefore = $log.Length

        # Backup before truncate
        $backupPath = "$($log.FullName).bak"
        Copy-Item $log.FullName $backupPath -Force

        # Truncate
        Clear-Content $log.FullName

        Write-Host "  Truncated: $($log.FullName.Replace('C:\Project\labuda\', ''))"
        Write-Host "    Before: $([math]::Round($sizeBefore/1MB, 2)) MB -> After: 0 KB"
    }
    Write-Host "  Processed $($logFiles.Count) log files" -ForegroundColor Green
} else {
    Write-Host "  No large log files found" -ForegroundColor Gray
}

#==============================================
# 3. Clean Gradle Build Cache (optional)
#==============================================
Write-Host "`n=== 3. GRADLE BUILD CACHE ===" -ForegroundColor Yellow

$gradleCaches = Join-Path $env:USERPROFILE '.gradle\caches'
if (Test-Path $gradleCaches) {
    $cacheSize = (Get-ChildItem $gradleCaches -Recurse -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum

    if ($cacheSize -gt 3GB) {
        Write-Host "  Gradle cache is large: $([math]::Round($cacheSize/1GB, 2)) GB"
        Write-Host "  Run 'Remove-Item `$env:USERPROFILE\.gradle\caches -Recurse -Force' to clean"
        Write-Host "  (Skipped - requires rebuild)" -ForegroundColor Yellow
    } else {
        Write-Host "  Gradle cache is OK: $([math]::Round($cacheSize/1MB, 2)) MB" -ForegroundColor Green
    }
}

#==============================================
# 4. Clean Flutter Build (optional)
#==============================================
Write-Host "`n=== 4. FLUTTER BUILD ===" -ForegroundColor Yellow

$flutterBuild = 'C:\Project\labuda\apps\mobile\build'
if (Test-Path $flutterBuild) {
    $buildSize = (Get-ChildItem $flutterBuild -Recurse -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
    Write-Host "  Flutter build size: $([math]::Round($buildSize/1MB, 2)) MB"
    Write-Host "  Run 'flutter clean' to clean (Skipped - requires rebuild)" -ForegroundColor Yellow
}

#==============================================
# 5. Show Current Disk Space
#==============================================
Write-Host "`n=== 5. DISK SPACE ===" -ForegroundColor Yellow

$drive = Get-PSDrive C
$freeGB = [math]::Round($drive.Free/1GB, 2)
$usedPercent = [math]::Round(($drive.Used/($drive.Free+$drive.Used))*100, 1)

Write-Host "  Free:  $freeGB GB"
Write-Host "  Used:  $usedPercent%"

if ($freeGB -lt 15) {
    Write-Host "  WARNING: Low disk space!" -ForegroundColor Red
} else {
    Write-Host "  OK" -ForegroundColor Green
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Cleanup completed!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
