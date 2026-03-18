@echo off
echo === Building Spell Timer ===

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH.
    pause
    exit /b 1
)

echo [1/3] Downloading dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo [ERROR] go mod tidy failed.
    pause
    exit /b 1
)

echo [2/3] Downloading/updating assets...
go run ./cmd/download-assets
if %errorlevel% neq 0 (
    echo [WARNING] Asset download had issues, continuing build...
)

echo [3/3] Building spell-timer.exe...
go build -ldflags "-H windowsgui -s -w" -o spell-timer.exe .
if %errorlevel% neq 0 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)

echo.
echo [OK] Built spell-timer.exe
echo      Size: 
for %%A in (spell-timer.exe) do echo      %%~zA bytes
echo.
pause
