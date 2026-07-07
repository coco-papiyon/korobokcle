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
$workspaceRepoDir = Join-Path $rootPath "workspace/mock-owner_mock-repo"
if (Test-Path $workspaceRepoDir) {
  Remove-Item -LiteralPath $workspaceRepoDir -Recurse -Force
}
$artifactDirs = @(
  ".workspace/design",
  ".workspace/implementation",
  ".workspace/review",
  ".workspace/review_fix_design",
  ".workspace/review_fix_implementation",
  ".workspace/pr_conflict"
)
foreach ($artifactDir in $artifactDirs) {
  $fullArtifactDir = Join-Path $rootPath $artifactDir
  if (Test-Path $fullArtifactDir) {
    Get-ChildItem -Path $fullArtifactDir -File -ErrorAction SilentlyContinue | Remove-Item -Force
  }
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

function Get-TimeText {
  param(
    [datetime]$BaseTime,
    [int]$OffsetMinutes
  )
  return $BaseTime.AddMinutes($OffsetMinutes).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}

function New-IssueContext {
  param(
    [int]$Number,
    [string]$Title,
    [string]$State
  )
  return @"
#$Number $Title

Mock issue for testing state: $State
"@
}

function New-JobEntry {
  param(
    [string]$Id,
    [string]$Kind,
    [string]$State,
    [int]$Number,
    [string]$Title,
    [int]$Order,
    [string]$SubStatus = "",
    [string]$FailedFromState = "",
    [string]$ErrorMessage = "",
    [switch]$IncludeIssueContext
  )

  $job = [ordered]@{
    id = $Id
    kind = $Kind
    state = $State
    repository = "mock-owner/mock-repo"
    number = $Number
    title = $Title
    fetchedAt = Get-TimeText -BaseTime ([datetime]"2026-07-01T00:00:00Z") -OffsetMinutes ($Order * 10)
    updatedAt = Get-TimeText -BaseTime ([datetime]"2026-07-01T03:00:05Z") -OffsetMinutes ($Order * 10)
  }

  if ($IncludeIssueContext) {
    $job.issueContext = New-IssueContext -Number $Number -Title $Title -State $State
  }
  if ($SubStatus) {
    $job.subStatus = $SubStatus
  }
  if ($FailedFromState) {
    $job.failedFromState = $FailedFromState
  }
  if ($ErrorMessage) {
    $job.errorMessage = $ErrorMessage
  }
  return $job
}

$jobs = @(
  (New-JobEntry -Id "issue-101" -Kind "issue_design" -State "detected" -Number 101 -Title "design-detected" -Order 0 -IncludeIssueContext),
  (New-JobEntry -Id "issue-102" -Kind "issue_design" -State "design_running" -Number 102 -Title "design-running" -Order 1 -IncludeIssueContext),
  (New-JobEntry -Id "issue-103" -Kind "issue_design" -State "design_ready" -Number 103 -Title "design-ready" -Order 2 -IncludeIssueContext),
  (New-JobEntry -Id "issue-104" -Kind "issue_design" -State "design_approved" -Number 104 -Title "design-approved" -Order 3 -IncludeIssueContext),
  (New-JobEntry -Id "issue-105" -Kind "issue_design" -State "completed" -Number 105 -Title "design-completed" -Order 4 -IncludeIssueContext),
  (New-JobEntry -Id "issue-106" -Kind "issue_design" -State "failed" -Number 106 -Title "design-failed" -Order 5 -FailedFromState "design_running" -ErrorMessage "mock design failure" -IncludeIssueContext),
  (New-JobEntry -Id "issue-201" -Kind "issue_implementation" -State "implementation_running" -Number 201 -Title "implementation-running" -Order 6 -SubStatus "検証(2回目)" -IncludeIssueContext),
  (New-JobEntry -Id "issue-202" -Kind "issue_implementation" -State "implementation_ready" -Number 202 -Title "implementation-ready" -Order 7 -IncludeIssueContext),
  (New-JobEntry -Id "issue-203" -Kind "issue_implementation" -State "implementation_approved" -Number 203 -Title "implementation-approved" -Order 8 -IncludeIssueContext),
  (New-JobEntry -Id "issue-204" -Kind "issue_implementation" -State "pr_created" -Number 204 -Title "implementation-pr-created" -Order 9 -IncludeIssueContext),
  (New-JobEntry -Id "issue-205" -Kind "issue_implementation" -State "implementation_running" -Number 205 -Title "implementation-awaiting-permission" -Order 10 -SubStatus "コマンド許可待ち" -IncludeIssueContext),
  (New-JobEntry -Id "pr-301" -Kind "pr_review" -State "review_running" -Number 301 -Title "review-running" -Order 11),
  (New-JobEntry -Id "pr-302" -Kind "pr_review" -State "review_ready" -Number 302 -Title "review-ready" -Order 12),
  (New-JobEntry -Id "pr-303" -Kind "pr_review" -State "review_approved" -Number 303 -Title "review-approved" -Order 13),
  (New-JobEntry -Id "pr-304" -Kind "pr_feedback" -State "pr_review_comment" -Number 304 -Title "review-comment" -Order 14),
  (New-JobEntry -Id "pr-508" -Kind "pr_review" -State "review_ready" -Number 508 -Title "review-awaiting-user-response" -Order 26),
  (New-JobEntry -Id "pr-401" -Kind "pr_conflict" -State "pr_conflict" -Number 401 -Title "conflict-detected" -Order 15),
  (New-JobEntry -Id "pr-402" -Kind "pr_conflict" -State "pr_conflict_running" -Number 402 -Title "conflict-running" -Order 16),
  (New-JobEntry -Id "pr-403" -Kind "pr_conflict" -State "pr_conflict_ready" -Number 403 -Title "conflict-ready" -Order 17),
  (New-JobEntry -Id "pr-404" -Kind "pr_conflict" -State "pr_conflict_resolved" -Number 404 -Title "conflict-resolved" -Order 18),
  (New-JobEntry -Id "pr-501" -Kind "pr_feedback" -State "review_fix_design_running" -Number 501 -Title "review-fix-design-running" -Order 19),
  (New-JobEntry -Id "pr-502" -Kind "pr_feedback" -State "review_fix_design_ready" -Number 502 -Title "review-fix-design-ready" -Order 20),
  (New-JobEntry -Id "pr-503" -Kind "pr_feedback" -State "review_fix_design_approved" -Number 503 -Title "review-fix-design-approved" -Order 21),
  (New-JobEntry -Id "pr-504" -Kind "pr_feedback" -State "review_fix_implementation_running" -Number 504 -Title "review-fix-implementation-running" -Order 22),
  (New-JobEntry -Id "pr-505" -Kind "pr_feedback" -State "review_fix_implementation_ready" -Number 505 -Title "review-fix-implementation-ready" -Order 23),
  (New-JobEntry -Id "pr-506" -Kind "pr_feedback" -State "review_fix_implementation_approved" -Number 506 -Title "review-fix-implementation-approved" -Order 24),
  (New-JobEntry -Id "pr-507" -Kind "pr_feedback" -State "review_fixed" -Number 507 -Title "review-fixed" -Order 25)
)
Write-TextNoBom -Path (Join-Path $rootPath "db/jobs.json") -Value ($jobs | ConvertTo-Json -Depth 10)

$mockJobs = @($jobs | ForEach-Object { $_ })
Write-TextNoBom -Path (Join-Path $rootPath "db/mock_jobs.json") -Value ($mockJobs | ConvertTo-Json -Depth 10)

function Write-Artifact {
  param(
    [string]$SubDir,
    [int]$Number,
    [string]$SafeTitle,
    [string]$Title,
    [string]$Kind,
    [string]$State
  )
  $content = if ($Number -eq 203) {
    @"
# $Title

## Summary
This is a $Kind artifact for UI testing at state: $State.

## Changes
- This artifact is generated as mock test data.
- Use it to test approve, rerun, and request-changes UI actions.
- It also verifies markdown rendering inside the chat view.

## Result
| Item | Value |
| --- | --- |
| Status | approved |
| Role | implementer |
| Loop | 1 |

> The chat preview should keep the summary readable.

    mock preview ready

<p>HTML preview enabled.</p>

## Test Results
- create_test_data.ps1: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## Remaining
- Mock mode does not post to GitHub.
"@
  } elseif ($Number -eq 508) {
    @"
# $Title

## 概要
レビューが完了し、ユーザの応答待ちになっているテストデータです。

## ユーザ応答待ち
- 承認、修正依頼、再実行のいずれかを選択してください。
- チャット入力欄に追記したコメントは、そのまま操作時のコメントとして利用できます。

## 変更内容
- AI への指示をチャットで送信する画面確認用の fixture です。
- 結果画面に頼らず、会話の流れで待機状態を把握できます。

## テスト結果
- create_test_data.ps1: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## 残課題
- ユーザの操作を待っています。
"@
  } else {
    @"
# $Title

## Summary
This is a $Kind artifact for UI testing at state: $State.

## Changes
- This artifact is generated as mock test data.
- Use it to test approve, rerun, and request-changes UI actions.

## Test Results
- create_test_data.ps1: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## Remaining
- Mock mode does not post to GitHub.
"@
  }
  $path = Join-Path $rootPath ".workspace/$SubDir/${Number}_${SafeTitle}.md"
  Write-TextNoBom -Path $path -Value $content
}

function Write-Diff {
  param(
    [string]$SubDir,
    [int]$Number,
    [string]$SafeTitle,
    [string]$Title,
    [string]$Kind,
    [string]$State
  )
  $content = if ($Number -eq 203) {
    @"
diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # $Title
 ## Summary
  context line 1
  context line 2
  context line 3
  context line 4
-This is a mock artifact.
+This is a mock artifact for $State.
 This line stays unchanged.
 This line stays unchanged too.
 ## Changes
-This artifact is generated as mock test data.
+This artifact is generated as mock test data for UI testing.
 This line stays unchanged.
 This line stays unchanged too.
 This line stays unchanged three.
 This line stays unchanged four.
@@ -16,6 +16,6 @@
  keep the chat preview readable.
-Old html preview.
+<p>HTML preview enabled.</p>
"@
  } elseif ($Number -eq 508) {
    @"
diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # $Title
 ## Summary
  context line 1
  context line 2
  context line 3
  context line 4
-This is a mock artifact.
+This is a mock artifact waiting for user response.
 This line stays unchanged.
 This line stays unchanged too.
 ## Changes
-This artifact is generated as mock test data.
+This artifact is generated as mock test data for chat response testing.
 This line stays unchanged.
 This line stays unchanged too.
 This line stays unchanged three.
 This line stays unchanged four.
"@
  } else {
    @"
diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # $Title
 ## Summary
  context line 1
  context line 2
  context line 3
  context line 4
-This is a mock artifact.
+This is a mock artifact for $State.
 This line stays unchanged.
 This line stays unchanged too.
 ## Changes
-This artifact is generated as mock test data.
+This artifact is generated as mock test data for UI testing.
 This line stays unchanged.
 This line stays unchanged too.
This line stays unchanged three.
This line stays unchanged four.
"@
  }
  $path = Join-Path $rootPath ".workspace/$SubDir/${Number}_${SafeTitle}.diff"
  Write-TextNoBom -Path $path -Value $content
}

function Get-ArtifactSubDir {
  param(
    [hashtable]$Job
  )
  switch ($Job.kind) {
    "issue_design" { return "design" }
    "issue_implementation" { return "implementation" }
    "pr_review" { return "review" }
    "pr_conflict" { return "pr_conflict" }
    "pr_feedback" { return "review_fix_implementation" }
    default { return "" }
  }
}

function Should-WriteArtifact {
  param(
    [hashtable]$Job
  )
  $inspectableStates = @(
    "design_ready",
    "design_approved",
    "implementation_ready",
    "implementation_approved",
    "pr_created",
    "review_ready",
    "review_approved",
    "review_fix_design_approved",
    "review_fix_implementation_ready",
    "review_fix_implementation_approved",
    "review_fixed",
    "pr_conflict_ready",
    "pr_conflict_resolved",
    "completed"
  )
  return $inspectableStates -contains $Job.state
}

foreach ($job in $jobs) {
  if (-not (Should-WriteArtifact -Job $job)) {
    continue
  }
  $artifactSubDir = Get-ArtifactSubDir -Job $job
  if (-not $artifactSubDir) {
    continue
  }
  Write-Artifact -SubDir $artifactSubDir -Number $job.number -SafeTitle $job.title -Title $job.title -Kind $job.kind -State $job.state
  Write-Diff -SubDir $artifactSubDir -Number $job.number -SafeTitle $job.title -Title $job.title -Kind $job.kind -State $job.state
}

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

function Get-LogSubDir {
  param(
    [hashtable]$Job
  )
  switch ($Job.kind) {
    "issue_design" { return "design" }
    "issue_implementation" { return "implementation" }
    "pr_review" { return "review" }
    "pr_conflict" { return "pr_conflict" }
    "pr_feedback" {
      if ($Job.state -like "review_fix_design*") {
        return "review_fix_design"
      }
      return "review_fix_implementation"
    }
    default { return "" }
  }
}

foreach ($job in $jobs) {
  if ($job.state -eq "detected") {
    continue
  }
  $logSubDir = Get-LogSubDir -Job $job
  if (-not $logSubDir) {
    continue
  }

  $activity = @"
=== 2026-07-01T04:00:00Z request job=$($job.id) kind=$($job.kind) state=$($job.state) ===
provider: codex
model: default
working_dir: tests

[prompt]
Mock fixture for $($job.title)
"@
  if ($job.number -eq 205) {
    $activity = @"
=== 2026-07-01T04:00:00Z request job=$($job.id) kind=$($job.kind) state=$($job.state) ===
provider: codex
model: default
working_dir: tests

[system]
You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown.

[assistant]
Command permission is required before continuing.

[request_permission]
command: npm test
status: awaiting_permission
message: npm test を実行してよいですか？
"@
  }
  if ($job.number -eq 508) {
    $activity = @"
=== 2026-07-01T04:00:00Z request job=$($job.id) kind=$($job.kind) state=$($job.state) ===
provider: codex
model: default
working_dir: tests

[system]
You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown.

[user]
The review is complete and the job is waiting for a user response. Keep the chat state visible until the user replies.

[assistant]
Waiting for user approval or change request.
"@
  }
  if ($job.number -eq 203) {
    $activity = @"
=== 2026-07-01T04:00:00Z request job=$($job.id) kind=$($job.kind) state=$($job.state) ===
provider: codex
model: default
working_dir: tests

[system]
You are an autonomous software engineer. Follow the repository instructions with minimal extra process. Edit the repository directly and report the result in concise Japanese Markdown.

[user]
Implement the job detail chat preview fixture.

[assistant]
Ready for approval after markdown rendering is verified.
"@
  }
  Write-SingleRoleLogs -SubDir $logSubDir -Number $job.number -Role "agent" -Activity $activity -Stdout "agent stdout: $($job.state)" -Stderr "agent stderr: none"

  if ($job.kind -eq "issue_implementation") {
    $verificationSummary = if ($job.number -eq 205) {
      "コマンド許可待ちで処理が停止しています。"
    } elseif ($job.subStatus -like "検証*") {
      "検証中の状態で停止しています。"
    } else {
      "モックの検証ログです。"
    }
    if ($job.number -eq 203) {
      $verificationSummary = "チャット表示の見本として、Markdown と HTML を含む成果物を確認しました。"
    }
    $verificationStatus = "changes_requested"
    if ($job.number -eq 203) {
      $verificationStatus = "passed"
    } elseif ($job.number -eq 205) {
      $verificationStatus = "awaiting_permission"
    }
    Write-LogGroup -SubDir $logSubDir -Number $job.number -Role "verifier" -Attempt 2 -Activity @"
=== 2026-07-01T04:01:00Z verification job=$($job.id) kind=$($job.kind) state=$($job.state) ===
status: $verificationStatus
feedback: 追加の確認が必要です。
summary: $verificationSummary
"@ -Stdout "verifier stdout: $($job.state)" -Stderr "verifier stderr: none"
  }
}

Write-Host "Test data created: $rootPath"
Write-Host "Run: go run ./cmd/korobokcle --tool-dir . --base-dir tests --work-dir tests --mock-mode"
