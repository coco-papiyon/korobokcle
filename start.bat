@echo off

cd frontend
cmd /X /C "npm run build"

cd ../
REM go run ./cmd/korobokcle --debug
go run ./cmd/korobokcle
