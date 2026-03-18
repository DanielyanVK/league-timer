@echo off
echo === LoL Asset Downloader ===

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH.
    pause
    exit /b 1
)

go run ./cmd/download-assets
pause
