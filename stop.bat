@echo off
title Coal Governance Platform - Stopper
echo ========================================================
echo  Stopping Coal Governance Platform Services...
echo ========================================================
echo.

echo Stopping services on ports 5000, 8080, 8000...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetTCPConnection -LocalPort 5000, 8080, 8000 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }"

wmic process where "commandline like '%%app.py%%'" call terminate >nul 2>&1
wmic process where "commandline like '%%mine_data_generator%%'" call terminate >nul 2>&1
taskkill /f /im main.exe >nul 2>&1

echo.
echo All Coal Governance services stopped successfully.
pause
