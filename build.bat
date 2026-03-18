@echo off
echo === Building Spell Timer ===

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH.
    pause
    exit /b 1
)

echo [1/4] Downloading dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo [ERROR] go mod tidy failed.
    pause
    exit /b 1
)

echo [2/4] Downloading/updating assets and generating icon...
go run ./cmd/download-assets
if %errorlevel% neq 0 (
    echo [WARNING] Asset download had issues, continuing build...
)

echo [3/4] Embedding icon into exe...
where rsrc >nul 2>nul
if %errorlevel% neq 0 (
    echo        Installing rsrc tool...
    go install github.com/akavel/rsrc@latest
)
if exist ico\icon.ico (
    rsrc -ico ico\icon.ico -o rsrc_windows_amd64.syso 2>nul
    if %errorlevel% equ 0 (
        echo        Icon embedded.
    ) else (
        echo        [WARNING] Icon embedding failed, exe will have no icon.
    )
) else (
    echo        [WARNING] ico\icon.ico not found, skipping.
)

echo [4/4] Building spell-timer.exe...
go build -ldflags "-H windowsgui -s -w" -o spell-timer.exe .
if %errorlevel% neq 0 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)

echo.
echo [OK] Built spell-timer.exe
for %%A in (spell-timer.exe) do echo      Size: %%~zA bytes
echo.
pause
