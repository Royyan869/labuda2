# Run tests with isolated test database.
#
# Usage:
#   .\scripts\test.ps1                    # Run all tests
#   .\scripts\test.ps1 -run TestExpiry   # Run specific test
#   .\scripts\test.ps1 -v .\tests\...    # Run tests in package with verbose output

$ErrorActionPreference = "Stop"

# Change to backend directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location "$scriptDir\.."
Set-Location backend

Write-Host "==> Running tests with isolated test database..."
Write-Host "    Test DB: labuda_test"

# Set TEST_MODE environment variable
$env:TEST_MODE = "true"

# Run tests
go test $args -v
