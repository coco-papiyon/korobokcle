param(
  [string]$Root = (Join-Path (Get-Location) "tests")
)

$ErrorActionPreference = "Stop"

$rootPath = [System.IO.Path]::GetFullPath($Root)
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Write-TextNoBom {
  param(
    [string]$Path,
    [string]$Value
  )
  $fullPath = [System.IO.Path]::GetFullPath($Path)
  $parent = [System.IO.Path]::GetDirectoryName($fullPath)
  if ($parent) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
  }
  [System.IO.File]::WriteAllText($fullPath, $Value, $script:utf8NoBom)
}

$dirs = @(
  "config",
  "db",
  "prompt",
  "workspace",
  "workspace/design_feedback",
  "state",
  "logs",
  "logs/skill",
  ".workspace/design",
  ".workspace/implementation",
  ".workspace/review",
  ".workspace/review_fix_design",
  ".workspace/review_fix_implementation"
)

foreach ($dir in $dirs) {
  New-Item -ItemType Directory -Force -Path (Join-Path $rootPath $dir) | Out-Null
}

$legacyJobLogDirs = Get-ChildItem -Path (Join-Path $rootPath "logs") -Directory -ErrorAction SilentlyContinue |
  Where-Object { $_.Name -match '^\d+$' }
foreach ($legacyDir in $legacyJobLogDirs) {
  Remove-Item -LiteralPath $legacyDir.FullName -Recurse -Force
}

$legacyWorkspaceRepoDir = Join-Path $rootPath "workspace/mock-owner-mock-repo"
if (Test-Path $legacyWorkspaceRepoDir) {
  Remove-Item -LiteralPath $legacyWorkspaceRepoDir -Recurse -Force
}

$settings = @{
  repository = "mock-owner/mock-repo"
  aiProvider = "codex"
  pollIntervalSeconds = 3600
  baseBranch = "main"
  branchNamePattern = "issue_#<issueNumber>"
  aiAllowedCommands = @("go test ./...", "cd frontend && npm test")
  models = @{
    codex = @{ mode = "default" }
    githubCopilot = @{ mode = "default" }
  }
  issue = @{
    labelIncludes = @()
    labelExcludes = @()
    titleContains = @()
    authors = @()
    assignees = @()
  }
  pullRequest = @{
    labelIncludes = @()
    labelExcludes = @()
    titleContains = @()
    authors = @()
    assignees = @()
  }
}
Write-TextNoBom -Path (Join-Path $rootPath "config/settings.json") -Value ($settings | ConvertTo-Json -Depth 10)

$jobs = @(
  @{
    id = "issue-101"
    kind = "issue_design"
    state = "design_ready"
    repository = "mock-owner/mock-repo"
    number = 101
    title = "login-page-improvements"
    issueContext = @"
#101 login-page-improvements

Improve the login page layout, spacing, and accessibility.
"@
    fetchedAt = "2026-07-01T00:00:00Z"
    updatedAt = "2026-07-01T03:04:05Z"
  },
  @{
    id = "issue-102"
    kind = "issue_implementation"
    state = "implementation_ready"
    repository = "mock-owner/mock-repo"
    number = 102
    title = "job-detail-panel-improvements"
    issueContext = @"
#102 job-detail-panel-improvements

Refine the job detail panel, align cards, and improve readability.
"@
    fetchedAt = "2026-07-01T00:10:00Z"
    updatedAt = "2026-07-01T03:14:05Z"
  },
  @{
    id = "pr-201"
    kind = "pr_review"
    state = "review_ready"
    repository = "mock-owner/mock-repo"
    number = 201
    title = "add-filter-conditions"
    fetchedAt = "2026-07-01T00:20:00Z"
    updatedAt = "2026-07-01T03:24:05Z"
  },
  @{
    id = "pr-202"
    kind = "pr_feedback"
    state = "review_fix_design_ready"
    repository = "mock-owner/mock-repo"
    number = 202
    title = "review-feedback-fix"
    fetchedAt = "2026-07-01T00:30:00Z"
    updatedAt = "2026-07-01T03:34:05Z"
  }
)
Write-TextNoBom -Path (Join-Path $rootPath "db/jobs.json") -Value ($jobs | ConvertTo-Json -Depth 10)

$mockJobs = @(
  @{
    id = "issue-301"
    kind = "issue_design"
    state = "detected"
    repository = "mock-owner/mock-repo"
    number = 301
    title = "mock-detected-design"
    issueContext = @"
#301 mock-detected-design

Mock issue for testing the detected state.
"@
    fetchedAt = "2026-07-01T00:40:00Z"
    updatedAt = "2026-07-01T03:44:05Z"
  },
  @{
    id = "issue-302"
    kind = "issue_implementation"
    state = "design_approved"
    repository = "mock-owner/mock-repo"
    number = 302
    title = "mock-detected-implementation"
    issueContext = @"
#302 mock-detected-implementation

Mock issue for testing implementation flow.
"@
    fetchedAt = "2026-07-01T00:50:00Z"
    updatedAt = "2026-07-01T03:54:05Z"
  },
  @{
    id = "pr-401"
    kind = "pr_review"
    state = "review_running"
    repository = "mock-owner/mock-repo"
    number = 401
    title = "mock-pr-review"
    fetchedAt = "2026-07-01T01:00:00Z"
    updatedAt = "2026-07-01T04:04:05Z"
  }
)
Write-TextNoBom -Path (Join-Path $rootPath "db/mock_jobs.json") -Value ($mockJobs | ConvertTo-Json -Depth 10)

function Write-Artifact {
  param(
    [string]$SubDir,
    [int]$Number,
    [string]$SafeTitle,
    [string]$Title,
    [string]$Kind
  )
  $content = @"
# $Title

## Summary
This is a $Kind artifact for UI testing.

## Changes
- This artifact is generated as mock test data.
- Use it to test approve, rerun, and request-changes UI actions.

## Test Results
- create_test_data.ps1: success

## Remaining
- Mock mode does not post to GitHub.
"@
  $path = Join-Path $rootPath ".workspace/$SubDir/${Number}_${SafeTitle}.md"
  Write-TextNoBom -Path $path -Value $content
}

Write-Artifact -SubDir "design" -Number 101 -SafeTitle "login-page-improvements" -Title "login-page-improvements" -Kind "design"
Write-Artifact -SubDir "implementation" -Number 102 -SafeTitle "job-detail-panel-improvements" -Title "job-detail-panel-improvements" -Kind "implementation"
Write-Artifact -SubDir "review" -Number 201 -SafeTitle "add-filter-conditions" -Title "add-filter-conditions" -Kind "review"
Write-Artifact -SubDir "review_fix_design" -Number 202 -SafeTitle "review-feedback-fix" -Title "review-feedback-fix" -Kind "review feedback design"

function Write-LogGroup {
  param(
    [string]$SubDir,
    [int]$Number,
    [string]$Role,
    [int]$Attempt,
    [string]$Activity,
    [string]$Stdout,
    [string]$Stderr
  )
  $prefix = "${SubDir}_attempt-$Attempt"
  if ($Role) {
    $prefix = "${prefix}_$Role"
  }
  $repoId = "mock-owner_mock-repo"
  $jobPrefix = if ($SubDir -eq "review" -or $SubDir -eq "review_fix_design" -or $SubDir -eq "review_fix_implementation" -or $SubDir -eq "pr_conflict") { "pr" } else { "issue" }
  $jobId = "$jobPrefix-$Number"
  $logDir = Join-Path $rootPath "workspace/$repoId/$jobId/logs"
  Write-TextNoBom -Path (Join-Path $logDir "$prefix.log") -Value $Activity
  Write-TextNoBom -Path (Join-Path $logDir "$prefix`_stdout.log") -Value $Stdout
  Write-TextNoBom -Path (Join-Path $logDir "$prefix`_stderr.log") -Value $Stderr
}

function Write-SingleRoleLogs {
  param(
    [string]$SubDir,
    [int]$Number,
    [string]$Role,
    [string]$Activity,
    [string]$Stdout,
    [string]$Stderr
  )
  Write-LogGroup -SubDir $SubDir -Number $Number -Role $Role -Attempt 1 -Activity $Activity -Stdout $Stdout -Stderr $Stderr
}

Write-SingleRoleLogs -SubDir "design" -Number 101 -Role "agent" -Activity @"
=== 2026-07-01T04:00:00Z request job=issue-101 kind=issue_design state=design_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Design the login page.
"@ -Stdout "design stdout: mock design run" -Stderr "design stderr: none"

Write-LogGroup -SubDir "implementation" -Number 102 -Role "agent" -Attempt 1 -Activity @"
=== 2026-07-01T04:10:00Z request job=issue-102 kind=issue_implementation state=implementation_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Implement the job detail panel improvements.
"@ -Stdout "implementation stdout: attempt 1" -Stderr "implementation stderr: none"

Write-LogGroup -SubDir "implementation" -Number 102 -Role "verifier" -Attempt 1 -Activity @"
=== 2026-07-01T04:11:00Z verification job=issue-102 kind=issue_implementation state=implementation_running ===
status: passed
feedback:
summary: 検証を通過しました。
"@ -Stdout "verifier stdout: tests passed" -Stderr "verifier stderr: none"

Write-SingleRoleLogs -SubDir "review" -Number 201 -Role "agent" -Activity @"
=== 2026-07-01T04:20:00Z request job=pr-201 kind=pr_review state=review_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Review the filter conditions.
"@ -Stdout "review stdout: mock review run" -Stderr "review stderr: none"

Write-SingleRoleLogs -SubDir "review_fix_design" -Number 202 -Role "agent" -Activity @"
=== 2026-07-01T04:30:00Z request job=pr-202 kind=pr_feedback state=review_fix_design_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Fix the feedback design.
"@ -Stdout "review-fix-design stdout: mock run" -Stderr "review-fix-design stderr: none"

Write-SingleRoleLogs -SubDir "design" -Number 301 -Role "agent" -Activity @"
=== 2026-07-01T04:40:00Z request job=issue-301 kind=issue_design state=design_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Detected issue for design.
"@ -Stdout "design stdout: detected mock" -Stderr "design stderr: none"

Write-LogGroup -SubDir "implementation" -Number 302 -Role "agent" -Attempt 1 -Activity @"
=== 2026-07-01T04:50:00Z request job=issue-302 kind=issue_implementation state=implementation_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Mock implementation run.
"@ -Stdout "implementation stdout: attempt 1" -Stderr "implementation stderr: none"

Write-LogGroup -SubDir "implementation" -Number 302 -Role "verifier" -Attempt 1 -Activity @"
=== 2026-07-01T04:51:00Z verification job=issue-302 kind=issue_implementation state=implementation_running ===
status: changes_requested
feedback: テストを追加してください。
summary: 追加テストが必要です。
"@ -Stdout "verifier stdout: request changes" -Stderr "verifier stderr: none"

Write-SingleRoleLogs -SubDir "review" -Number 401 -Role "agent" -Activity @"
=== 2026-07-01T05:00:00Z request job=pr-401 kind=pr_review state=review_running ===
provider: codex
model: default
working_dir: tests

[prompt]
Mock PR review.
"@ -Stdout "review stdout: mock review" -Stderr "review stderr: none"

Write-Host "Test data created: $rootPath"
Write-Host "Run: go run ./cmd/korobokcle --tool-dir . --base-dir tests --work-dir tests --mock-mode"
