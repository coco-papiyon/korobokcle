@echo off
setlocal EnableExtensions

set "ROOT=%~dp0"
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

echo Starting korobokcle...
go run .\cmd\korobokcle %*
if errorlevel 1 goto :error

popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
