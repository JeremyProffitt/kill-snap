@echo off
setlocal enabledelayedexpansion

REM ============================================
REM Kill-Snap Data Cleanup Script
REM Deletes all images and projects from DynamoDB
REM and all objects from S3
REM ============================================

REM Configuration - Update these values as needed
set "S3_BUCKET=kill-snap"
set "IMAGE_TABLE=kill-snap-ImageMetadata"
set "PROJECTS_TABLE=kill-snap-Projects"
set "REVIEW_GROUPS_TABLE=kill-snap-ReviewGroups"

REM Colors for output (using ANSI escape codes)
echo.
echo ============================================
echo  KILL-SNAP DATA CLEANUP SCRIPT
echo ============================================
echo.
echo This script will DELETE ALL DATA from:
echo   - DynamoDB Table: %IMAGE_TABLE%
echo   - DynamoDB Table: %PROJECTS_TABLE%
echo   - DynamoDB Table: %REVIEW_GROUPS_TABLE%
echo   - S3 Bucket: %S3_BUCKET%
echo.
echo ============================================
echo.

REM Confirmation prompt
set /p "CONFIRM=Are you sure you want to delete ALL data? (yes/no): "
if /i not "%CONFIRM%"=="yes" (
    echo.
    echo Operation cancelled.
    exit /b 0
)

echo.
echo Starting cleanup...
echo.

REM ============================================
REM Step 1: Delete all items from ImageMetadata table
REM ============================================
echo [1/4] Deleting all items from %IMAGE_TABLE%...

REM Scan and delete all items from ImageMetadata table
for /f "tokens=*" %%i in ('aws dynamodb scan --table-name %IMAGE_TABLE% --projection-expression "ImageGUID" --output text --query "Items[].ImageGUID.S"') do (
    echo   Deleting image: %%i
    aws dynamodb delete-item --table-name %IMAGE_TABLE% --key "{\"ImageGUID\": {\"S\": \"%%i\"}}" >nul 2>&1
)

echo   Done deleting from %IMAGE_TABLE%
echo.

REM ============================================
REM Step 2: Delete all items from Projects table
REM ============================================
echo [2/4] Deleting all items from %PROJECTS_TABLE%...

REM Scan and delete all items from Projects table
for /f "tokens=*" %%i in ('aws dynamodb scan --table-name %PROJECTS_TABLE% --projection-expression "ProjectID" --output text --query "Items[].ProjectID.S"') do (
    echo   Deleting project: %%i
    aws dynamodb delete-item --table-name %PROJECTS_TABLE% --key "{\"ProjectID\": {\"S\": \"%%i\"}}" >nul 2>&1
)

echo   Done deleting from %PROJECTS_TABLE%
echo.

REM ============================================
REM Step 3: Delete all items from ReviewGroups table
REM ============================================
echo [3/4] Deleting all items from %REVIEW_GROUPS_TABLE%...

REM Scan and delete all items from ReviewGroups table (composite key: ReviewID + ImageGUID)
for /f "tokens=1,2" %%i in ('aws dynamodb scan --table-name %REVIEW_GROUPS_TABLE% --projection-expression "ReviewID, ImageGUID" --output text --query "Items[].[ReviewID.S, ImageGUID.S]"') do (
    echo   Deleting review group item: %%i / %%j
    aws dynamodb delete-item --table-name %REVIEW_GROUPS_TABLE% --key "{\"ReviewID\": {\"S\": \"%%i\"}, \"ImageGUID\": {\"S\": \"%%j\"}}" >nul 2>&1
)

echo   Done deleting from %REVIEW_GROUPS_TABLE%
echo.

REM ============================================
REM Step 4: Delete all objects from S3 bucket
REM ============================================
echo [4/4] Deleting all objects from S3 bucket: %S3_BUCKET%...

REM Delete all objects (including versions if versioning is enabled)
aws s3 rm s3://%S3_BUCKET% --recursive

echo   Done deleting from S3 bucket
echo.

REM ============================================
REM Completion
REM ============================================
echo ============================================
echo  CLEANUP COMPLETE
echo ============================================
echo.
echo All data has been deleted from:
echo   - %IMAGE_TABLE%
echo   - %PROJECTS_TABLE%
echo   - %REVIEW_GROUPS_TABLE%
echo   - s3://%S3_BUCKET%
echo.

endlocal
