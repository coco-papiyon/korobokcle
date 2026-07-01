@echo off
setlocal EnableExtensions

set "ROOT=%~dp0"
set "ROOT_DIR=%ROOT:~0,-1%"
pushd "%ROOT%" || exit /b 1

if not exist "frontend\node_modules" (
  echo Installing frontend dependencies...
  pushd "frontend" || goto :error
  call npm install
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
echo Backend runs in this window.

echo Creating test data...
powershell -NoProfile -ExecutionPolicy Bypass -File ".\create_test_data.ps1" -Root ".\tests"
if errorlevel 1 goto :error

echo Starting korobokcle in mock mode...
go run .\cmd\korobokcle --tool-dir "%ROOT_DIR%" --base-dir "%ROOT_DIR%\tests" --work-dir "%ROOT_DIR%\tests" --mock-mode %*
if errorlevel 1 goto :error

popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
