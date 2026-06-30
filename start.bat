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

echo Starting backend at http://localhost:8080...
set "FRONTEND_RUNNING="
for /f "tokens=5" %%P in ('powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetTCPConnection -LocalPort 5173 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess"') do set "FRONTEND_RUNNING=%%P"
if defined FRONTEND_RUNNING (
  echo Frontend is already running on http://localhost:5173 ^(PID: %FRONTEND_RUNNING%^). Skipping startup.
) else (
  echo Starting frontend at http://localhost:5173...
  start "korobokcle frontend" /D "%ROOT_DIR%\frontend" cmd /k npm run dev
  if errorlevel 1 goto :error
  echo Frontend source changes are applied automatically by Vite HMR.
)
echo Backend runs in this window.

go run .\cmd\korobokcle --tool-dir "%ROOT_DIR%" --work-dir "%ROOT_DIR%" %*
if errorlevel 1 goto :error

popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
