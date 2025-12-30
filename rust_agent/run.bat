@echo off
echo Building Rust Agent...
cd /d %~dp0
go build -o rust-agent.exe ./cmd

if %errorlevel% neq 0 (
    echo Build failed!
    pause
    exit /b 1
)

echo.
echo Starting Rust Agent in interactive mode...
echo.

rust-agent.exe -i

pause
