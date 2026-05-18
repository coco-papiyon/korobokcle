@echo off

set KOROBOKCLE_COPILOT_DEBUG=1

cd frontend
cmd /X /C "npm run build"

cd ../

REM set KOROBOKCLE_RUN_REAL_COPILOT=1
REM go test ./internal/skill -run TestCopilotCLIProviderRunsGoTestCommandWithRealCopilot -v

REM go run ./cmd/korobokcle --debug
go run ./cmd/korobokcle
