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

echo Building frontend...
pushd "frontend" || goto :error
call npm run build
if errorlevel 1 goto :error
popd

echo Updating static files...
powershell -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path 'static') { Remove-Item -Recurse -Force 'static' }; New-Item -ItemType Directory -Path 'static' | Out-Null; Copy-Item -Recurse -Force 'frontend\dist\*' 'static\'"
if errorlevel 1 goto :error

echo Creating test data...
powershell -NoProfile -ExecutionPolicy Bypass -File ".\create_test_data.ps1" -Root ".\tests"
if errorlevel 1 goto :error

echo Starting korobokcle in mock mode...
go run .\cmd\korobokcle --addr :8081 --tool-dir "%ROOT_DIR%" --base-dir "%ROOT_DIR%\tests" --work-dir "%ROOT_DIR%\tests" --mock-mode %*
if errorlevel 1 goto :error

popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
