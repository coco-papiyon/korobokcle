@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "ROOT=%~dp0"
pushd "%ROOT%" || exit /b 1

set "BACKEND_PORT=8081"
set "FORWARD_ARGS="
set "NEXT_IS_BACKEND_PORT="
for %%A in (%*) do (
  if defined NEXT_IS_BACKEND_PORT (
    set "BACKEND_PORT=%%~A"
    set "NEXT_IS_BACKEND_PORT="
  ) else if /I "%%~A"=="--backend-port" (
    set "NEXT_IS_BACKEND_PORT=1"
  ) else if /I "%%~A"=="-p" (
    set "NEXT_IS_BACKEND_PORT=1"
  ) else (
    set "FORWARD_ARGS=!FORWARD_ARGS! %%~A"
  )
)
if "%BACKEND_PORT:~0,1%"==":" set "BACKEND_PORT=%BACKEND_PORT:~1%"

if not exist "frontend\node_modules" (
  echo Installing frontend dependencies...
  pushd "frontend" || goto :error
  call npm ci
  if errorlevel 1 goto :error
  popd
)

echo Building frontend...
pushd "frontend" || goto :error
call npm run build
if errorlevel 1 goto :error
popd

echo Syncing frontend build to tests static contents...
if not exist "tests\static" mkdir "tests\static"
robocopy "frontend\dist" "tests\static" /MIR /NFL /NDL /NJH /NJS /NC /NS /NP
set "ROBOCOPY_EXIT=%errorlevel%"
if %ROBOCOPY_EXIT% GEQ 8 goto :error

echo Building backend executable...
go build -o "tests\korobokcle.exe" .\cmd\korobokcle
if errorlevel 1 goto :error

echo Running korobokcle from tests directory...
pushd "tests" || goto :error

echo Creating test data...
go run ..\tests\scripts\create-testdata -root "."
if errorlevel 1 goto :error

echo Starting korobokcle in mock mode...
.\korobokcle.exe --addr :%BACKEND_PORT% --mock-mode %FORWARD_ARGS%
if errorlevel 1 goto :error

popd
popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
