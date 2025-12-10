@echo off
REM ============================================
REM Kill-Snap Data Cleanup Script
REM Builds and runs the Go cleanup tool
REM ============================================

echo Building cleanup tool...
cd /d "%~dp0tools\cleanup"

go build -o cleanup.exe .
if errorlevel 1 (
    echo Failed to build cleanup tool
    exit /b 1
)

echo.
cleanup.exe
cd /d "%~dp0"
