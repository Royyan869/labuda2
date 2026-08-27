@echo off
echo ========================================
echo Firebase CLI Fix & Deployment Script
echo ========================================
echo.

echo Step 1: Checking Node.js version...
node --version
echo.

echo Step 2: Reinstalling Firebase Tools...
echo This may take a few minutes...
call npm uninstall -g firebase-tools
call npm install -g firebase-tools@latest
echo.

echo Step 3: Verify Firebase CLI...
firebase --version
echo.

echo Step 4: Building functions...
cd functions
call npm run build
if errorlevel 1 (
    echo ERROR: Build failed!
    pause
    exit /b 1
)
echo Build successful!
echo.

echo Step 5: Deploying to Firebase...
echo Waiting 5 seconds before deployment...
timeout /t 5
echo.

firebase deploy --only functions --project labuda-79de2
echo.

echo ========================================
echo Deployment Complete!
echo ========================================
echo.
echo Please check Firebase Console to verify deployment
pause
