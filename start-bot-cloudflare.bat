@echo off
setlocal
cd /d "%~dp0"
powershell.exe -ExecutionPolicy Bypass -File ".\scripts\start-bot-cloudflare.ps1"
pause
