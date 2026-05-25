@echo off

set KOROBOKCLE_COPILOT_DEBUG=1
set KOROBOKCLE_TOOL_ROOT=tests\data

cd frontend
cmd /X /C "npm run build"

cd ../

REM set KOROBOKCLE_RUN_REAL_COPILOT=1
REM go test ./internal/skill -run TestCopilotCLIProviderRunsGoTestCommandWithRealCopilot -v

go run ./tests/scripts/create-testdata
xcopy skills\default tests\data\skills\default /E /I /Y

go build ./cmd/korobokcle
korobokcle.exe
