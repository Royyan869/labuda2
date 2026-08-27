# ============================================================================
# System Integration Validation - Master Script (Windows PowerShell)
# ============================================================================
# This script runs all validation flows and generates a final report
#
# Usage: .\run_all_flows.ps1
# ============================================================================

# Error action preference
$ErrorActionPreference = "Stop"

# Database configuration
$DB_HOST = if ($env:DB_HOST) { $env:DB_HOST } else { "localhost" }
$DB_PORT = if ($env:DB_PORT) { $env:DB_PORT } else { "5432" }
$DB_USER = if ($env:DB_USER) { $env:DB_USER } else { "labuda" }
$DB_NAME = if ($env:DB_NAME) { $env:DB_NAME } else { "labuda" }

# Script directory
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
$OUTPUT_DIR = Join-Path $SCRIPT_DIR "results"
$TIMESTAMP = Get-Date -Format "yyyyMMdd_HHmmss"

# Create output directory
New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null

Write-Host "============================================================================" -ForegroundColor Blue
Write-Host "SYSTEM INTEGRATION VALIDATION - MASTER SCRIPT" -ForegroundColor Blue
Write-Host "============================================================================" -ForegroundColor Blue
Write-Host ""
Write-Host "Database: $DB_USER@$DB_HOST`:$DB_PORT/$DB_NAME"
Write-Host "Output: $OUTPUT_DIR/validation_$TIMESTAMP.log"
Write-Host ""

# Test database connection
Write-Host "Testing database connection..." -ForegroundColor Yellow
try {
    $null = psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT 1;" 2>&1 | Out-Null
    Write-Host "✓ Database connection successful" -ForegroundColor Green
} catch {
    Write-Host "✗ Database connection failed" -ForegroundColor Red
    Write-Host "Please check your database configuration and try again."
    exit 1
}
Write-Host ""

# Array of flows
$FLOWS = @(
    "flow1_negotiation_chat_order_notification.sql"
    "flow2_moderation_content_effect.sql"
    "flow3_moderation_listing_order_safety.sql"
    "flow4_retry_idempotency_validation.sql"
    "flow5_outbox_health_check.sql"
)

$FLOW_NAMES = @(
    "Negotiation → Chat → Order → Notification"
    "Moderation → Content Effect"
    "Moderation → Listing → Order Safety"
    "Retry & Idempotency Validation"
    "Outbox Health Check"
)

# Results array
$FLOW_RESULTS = @()

# Run each flow
for ($i = 0; $i -lt $FLOWS.Count; $i++) {
    $FLOW_FILE = $FLOWS[$i]
    $FLOW_NAME = $FLOW_NAMES[$i]

    Write-Host "============================================================================" -ForegroundColor Blue
    Write-Host "RUNNING: $FLOW_NAME" -ForegroundColor Blue
    Write-Host "============================================================================" -ForegroundColor Blue
    Write-Host ""

    $FLOW_OUTPUT = Join-Path $OUTPUT_DIR "flow$($i+1)_${TIMESTAMP}.log"

    try {
        psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f (Join-Path $SCRIPT_DIR $FLOW_FILE) *>&1 | Out-File -FilePath $FLOW_OUTPUT
        $FLOW_RESULTS += "PASS"
        Write-Host "✓ ${FLOW_NAME}: PASSED" -ForegroundColor Green
    } catch {
        $FLOW_RESULTS += "FAIL"
        Write-Host "✗ ${FLOW_NAME}: FAILED" -ForegroundColor Red
        Write-Host "  Check $FLOW_OUTPUT for details"
    }
    Write-Host ""
}

# Generate final report
Write-Host "============================================================================" -ForegroundColor Blue
Write-Host "FINAL VALIDATION REPORT" -ForegroundColor Blue
Write-Host "============================================================================" -ForegroundColor Blue
Write-Host ""

Write-Host "Summary:"
Write-Host ""

for ($i = 0; $i -lt $FLOW_NAMES.Count; $i++) {
    $FLOW_NAME = $FLOW_NAMES[$i]
    $RESULT = $FLOW_RESULTS[$i]

    if ($RESULT -eq "PASS") {
        Write-Host "  ✓ $FLOW_NAME : PASSED" -ForegroundColor Green
    } else {
        Write-Host "  ✗ $FLOW_NAME : FAILED" -ForegroundColor Red
    }
}
Write-Host ""

# Calculate overall result
$ALL_PASSED = $true
foreach ($RESULT in $FLOW_RESULTS) {
    if ($RESULT -ne "PASS") {
        $ALL_PASSED = $false
        break
    }
}

Write-Host "============================================================================" -ForegroundColor Blue
if ($ALL_PASSED) {
    Write-Host "SYSTEM INTEGRATED: YES" -ForegroundColor Green
    Write-Host "ALL FLOWS VALIDATED SUCCESSFULLY" -ForegroundColor Green
} else {
    Write-Host "SYSTEM INTEGRATED: NO" -ForegroundColor Red
    Write-Host "SOME FLOWS FAILED VALIDATION" -ForegroundColor Red
}
Write-Host "============================================================================" -ForegroundColor Blue
Write-Host ""

# Save final report
$FINAL_REPORT = Join-Path $OUTPUT_DIR "validation_report_${TIMESTAMP}.txt"
@"
SYSTEM INTEGRATION VALIDATION REPORT
====================================
Date: $(Get-Date)
Database: $DB_USER@$DB_HOST`:$DB_PORT/$DB_NAME

Results:
"@ | Out-File -FilePath $FINAL_REPORT

for ($i = 0; $i -lt $FLOW_NAMES.Count; $i++) {
    $FLOW_NAME = $FLOW_NAMES[$i]
    $RESULT = $FLOW_RESULTS[$i]
    "  $FLOW_NAME : $RESULT" | Out-File -FilePath $FINAL_REPORT -Append
}

@"

Overall: $(if ($ALL_PASSED) { "PASS" } else { "FAIL" })
"@ | Out-File -FilePath $FINAL_REPORT -Append

Write-Host "Detailed logs saved to:"
Write-Host "  $OUTPUT_DIR\"
Write-Host ""
Write-Host "Full report saved to:"
Write-Host "  $FINAL_REPORT"
Write-Host ""

# Exit with appropriate code
if ($ALL_PASSED) {
    exit 0
} else {
    exit 1
}
