@echo off

set KOROBOKCLE_COPILOT_DEBUG=1
set KOROBOKCLE_TOOL_ROOT=exec\base

cd frontend
cmd /X /C "npm run build"

cd ../

REM set KOROBOKCLE_RUN_REAL_COPILOT=1
REM go test ./internal/skill -run TestCopilotCLIProviderRunsGoTestCommandWithRealCopilot -v

xcopy skills\default exec\base\skills\default /E /I /Y

go build ./cmd/korobokcle
korobokcle.exe
