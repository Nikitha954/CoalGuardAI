@echo off
title Coal Governance Platform - Launcher
echo ========================================================
echo  Coal Governance Platform - Starting All Services
echo ========================================================
echo.

:: 1. Start Go Backend
echo [1/4] Starting Go Backend on port 8080...
start "Coal Governance - Backend (Port 8080)" cmd /k "cd /d %~dp0backend && go run main.go"

:: 2. Start Python AI Service
echo [2/4] Starting Python AI Service on port 5000...
start "Coal Governance - AI Service (Port 5000)" cmd /k "cd /d %~dp0ai-service && python app.py"

:: 3. Start Synthetic Operational Data Simulator
echo [3/4] Starting Telemetry Data Simulator...
start "Coal Governance - Mine Simulator" cmd /k "cd /d %~dp0simulator && python mine_data_generator.py"

:: 4. Start Static Frontend HTTP Server
echo [4/4] Starting Frontend Web Server on port 8000...
start "Coal Governance - Frontend (Port 8000)" cmd /k "cd /d %~dp0frontend && python -m http.server 8000"

:: Wait 3 seconds and open browser
timeout /t 3 /nobreak >nul
echo Opening web application at http://localhost:8000 ...
start http://localhost:8000

echo.
echo ========================================================
echo  All services have been launched in separate windows!
echo  - Frontend:   http://localhost:8000
echo  - Backend:    http://localhost:8080/api/health
echo  - AI Service: http://localhost:5000/health
echo.
echo  Demo Credentials:
echo    Email:    admin@coal.gov
echo    Password: Coal@2026
echo ========================================================
pause
