@echo off
setlocal EnableExtensions

set "ROOT=%~dp0"
set "ROOT_DIR=%ROOT:~0,-1%"
pushd "%ROOT%" || exit /b 1

if not exist "frontend\node_modules" (
  echo Installing frontend dependencies...
  pushd "frontend" || goto :error
  call npm ci
  if errorlevel 1 goto :error
  popd
)

set "FRONTEND_RUNNING="
for /f "tokens=5" %%P in ('netstat -ano ^| findstr /R /C:":5173 .*LISTENING"') do (
  if not defined FRONTEND_RUNNING set "FRONTEND_RUNNING=%%P"
)
if defined FRONTEND_RUNNING (
  echo Frontend is already running on http://localhost:5173 ^(PID: %FRONTEND_RUNNING%^). Skipping startup.
) else (
  echo Starting frontend at http://localhost:5173...
  start "korobokcle frontend" /D "%ROOT_DIR%\frontend" cmd /k npm run dev
  if errorlevel 1 goto :error
  echo Frontend source changes are applied automatically by Vite HMR.
)

echo Building backend executable...
go build -o "tests\korobokcle.exe" .\cmd\korobokcle
if errorlevel 1 goto :error

echo Running korobokcle from tests directory...
pushd "tests" || goto :error

echo Creating test data...
powershell -NoProfile -ExecutionPolicy Bypass -File "..\create_test_data.ps1" -Root "."
if errorlevel 1 goto :error

echo Starting korobokcle in mock mode at http://localhost:8080...
.\korobokcle.exe --addr :8080 --mock-mode %FORWARD_ARGS%
if errorlevel 1 goto :error

popd
popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
