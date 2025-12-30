@echo off
cd /d %~dp0
go build -o rust-agent.exe ./cmd
if %errorlevel% neq 0 (
    echo Build failed!
    pause
    exit /b 1
)
echo Build successful!
rust-agent.exe %*
