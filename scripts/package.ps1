param(
	[string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path,
	[string]$OutputDir = (Join-Path (Resolve-Path (Join-Path $PSScriptRoot '..')).Path 'release'),
	[string]$ArchiveName = 'korobokcle-windows-amd64.zip',
	[string]$BinaryName = 'korobokcle.exe'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$env:GOCACHE = Join-Path $RepoRoot '.gocache'
if (-not (Test-Path -LiteralPath $env:GOCACHE)) {
	Ensure-Directory $env:GOCACHE
}

function Write-Step {
	param([string]$Message)
	Write-Host $Message
}

function Ensure-Directory {
	param([string]$Path)
	New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Invoke-CheckedCommand {
	param(
		[string]$Name,
		[scriptblock]$Command
	)

	& $Command
	if ($LASTEXITCODE -ne 0) {
		throw "$Name failed with exit code $LASTEXITCODE"
	}
}

function Copy-SkillDefaults {
	param(
		[string]$SourceRoot,
		[string]$TargetRoot
	)

	$sourceSkillDir = Join-Path (Join-Path $SourceRoot 'skills') 'default'
	if (-not (Test-Path -LiteralPath $sourceSkillDir)) {
		throw "skill directory not found: $sourceSkillDir"
	}

	$targetSkillDir = Join-Path (Join-Path $TargetRoot 'skills') 'default'
	Ensure-Directory $targetSkillDir
	Copy-Item -Path (Join-Path $sourceSkillDir '*') -Destination $targetSkillDir -Recurse -Force
}

Write-Step "Building frontend..."
$frontendBuildDir = Join-Path $env:TEMP ("korobokcle-frontend-dist-" + [guid]::NewGuid().ToString('N'))
Ensure-Directory $frontendBuildDir
Push-Location (Join-Path $RepoRoot 'frontend')
try {
	Invoke-CheckedCommand -Name 'frontend build' -Command { npm run build -- --outDir $frontendBuildDir --emptyOutDir }
}
finally {
	Pop-Location
}

Write-Step "Building backend..."
$stagingDir = Join-Path $env:TEMP ("korobokcle-package-" + [guid]::NewGuid().ToString('N'))
Ensure-Directory $stagingDir

try {
	Ensure-Directory $OutputDir

	Push-Location $RepoRoot
	try {
		Invoke-CheckedCommand -Name 'backend build' -Command { go build -o (Join-Path $stagingDir $BinaryName) ./cmd/korobokcle }
	}
	finally {
		Pop-Location
	}

	Write-Step "Copying runtime assets..."
	$stagingFrontendDistDir = Join-Path (Join-Path $stagingDir 'frontend') 'dist'
	Ensure-Directory $stagingFrontendDistDir
	Copy-Item -Path (Join-Path $frontendBuildDir '*') -Destination $stagingFrontendDistDir -Recurse -Force
	Copy-SkillDefaults -SourceRoot $RepoRoot -TargetRoot $stagingDir

	$archivePath = Join-Path $OutputDir $ArchiveName
	if (Test-Path -LiteralPath $archivePath) {
		Remove-Item -LiteralPath $archivePath -Force
	}

	Write-Step "Creating zip archive..."
	Compress-Archive -Path (Join-Path $stagingDir '*') -DestinationPath $archivePath -Force

	Write-Host "Created $archivePath"
}
finally {
	Remove-Item -LiteralPath $stagingDir -Recurse -Force -ErrorAction SilentlyContinue
	Remove-Item -LiteralPath $frontendBuildDir -Recurse -Force -ErrorAction SilentlyContinue
}
