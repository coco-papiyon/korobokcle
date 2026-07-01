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

Write-Host "Test data created: $rootPath"
Write-Host "Run: go run ./cmd/korobokcle --tool-dir . --base-dir tests --work-dir tests --mock-mode"
