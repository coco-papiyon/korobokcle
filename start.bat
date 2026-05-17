@echo off

cd frontend
cmd /X /C "npm run build"

cd ../
go run ./cmd/korobokcle --debug
REM go run ./cmd/korobokcle
