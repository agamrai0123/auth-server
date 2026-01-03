@echo off
REM 10 Million Request Load Test Script (Windows)
REM =============================================
REM This script runs the complete 10M load test with proper orchestration

setlocal enabledelayedexpansion

cd /d "d:\work-projects\auth-server" || exit /b 1

echo.
echo ==========================================
echo 10 MILLION REQUEST LOAD TEST
echo ==========================================
echo.

REM Configuration
set CONCURRENCY=%1
if "%CONCURRENCY%"=="" set CONCURRENCY=500

set REQUESTS_PER_WORKER=%2
if "%REQUESTS_PER_WORKER%"=="" set REQUESTS_PER_WORKER=20000

set /a TOTAL_REQUESTS=%CONCURRENCY% * %REQUESTS_PER_WORKER%
set SERVER_URL=http://localhost:8080

echo Configuration:
echo   Concurrency: %CONCURRENCY% workers
echo   Requests/worker: %REQUESTS_PER_WORKER%
echo   Total requests: %TOTAL_REQUESTS%
echo   Server URL: %SERVER_URL%
echo.

REM Check if load-test executable exists
if not exist "load-test.exe" (
    echo ERROR: load-test.exe not found
    echo Building load-test...
    go build -o load-test.exe load-test.go
    if !ERRORLEVEL! neq 0 (
        echo Build failed!
        exit /b 1
    )
)

REM Verify load-test is ready
echo Verifying load-test executable...
dir load-test.exe
echo.

REM Check if server is reachable
echo Checking if server is reachable at %SERVER_URL%...
curl -s "%SERVER_URL%/health" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo. Server is reachable
) else (
    echo. WARNING: Server might not be running at %SERVER_URL%
    echo.   Make sure to run: go run main.go
    echo.   Continuing anyway...
)
echo.

REM Run the load test
echo ==========================================
echo STARTING 10M REQUEST LOAD TEST
echo ==========================================
echo Start time: %date% %time%
echo.

REM Generate timestamp for results file
for /f "tokens=2-4 delims=/ " %%a in ('date /t') do (set mydate=%%c%%a%%b)
for /f "tokens=1-2 delims=/:" %%a in ('time /t') do (set mytime=%%a%%b)
set RESULTS_FILE=LOAD_TEST_10M_RESULTS_%mydate%_%mytime%.txt

echo Running load test...
echo.

REM Run load test (Windows batch doesn't have good time measurement, so we'll skip detailed timing)
call load-test.exe -url="%SERVER_URL%" -concurrency=%CONCURRENCY% -requests=%REQUESTS_PER_WORKER% > "%RESULTS_FILE%" 2>&1

REM Summary
echo.
echo ==========================================
echo LOAD TEST COMPLETED
echo ==========================================
echo End time: %date% %time%
echo Results saved to: %RESULTS_FILE%
echo.
echo Next steps:
echo 1. Review results in: %RESULTS_FILE%
echo 2. Check success rate
echo 3. Review latency statistics
echo 4. Check per-endpoint performance
echo.
echo To view results:
echo   type %RESULTS_FILE%
echo.

endlocal
