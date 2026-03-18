@echo off
echo === Building Spell Timer ===

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH.
    pause
    exit /b 1
)

echo [1/2] Downloading dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo [ERROR] go mod tidy failed.
    pause
    exit /b 1
)

echo [2/2] Building spell-timer.exe...
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
