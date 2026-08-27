@echo off
echo Setting up Firebase Storage CORS for Labuda Project...
echo.

REM Check if gsutil is installed
gsutil version >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Google Cloud SDK not found!
    echo.
    echo Please install Google Cloud SDK:
    echo 1. Download from: https://cloud.google.com/sdk/docs/install
    echo 2. Run: gcloud auth login
    echo 3. Run this script again
    echo.
    pause
    exit /b 1
)

echo Applying CORS configuration to Firebase Storage...
gsutil cors set cors.json gs://labuda-79de2.firebasestorage.app

if %errorlevel% equ 0 (
    echo.
    echo ✅ CORS configuration applied successfully!
    echo.
    echo You can now test avatar upload/display functionality.
    echo The following origins are now allowed:
    echo   - http://localhost:59379
    echo   - http://localhost:3000
    echo   - https://labuda-79de2.web.app
) else (
    echo.
    echo ❌ Failed to apply CORS configuration.
    echo Please check your authentication and try again.
)

echo.
pause