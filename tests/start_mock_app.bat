@echo off
setlocal EnableExtensions

set "SCRIPT_DIR=%~dp0"
pushd "%SCRIPT_DIR%" || exit /b 1

if not exist "mock-app\package.json" (
  echo tests\mock-app not found. Run create_test_data.ps1 first.
  popd
  exit /b 1
)

set "APP_DIR=%SCRIPT_DIR%mock-app"
echo Changing to script directory: %SCRIPT_DIR%
echo Starting mock app with npm run dev...
cd /d "%APP_DIR%" || goto :error
npm run dev
if errorlevel 1 goto :error

popd
exit /b 0

:error
set "EXIT_CODE=%errorlevel%"
popd
exit /b %EXIT_CODE%
