package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

const repositoryName = "mock-owner/mock-repo"

type watchSettingsFixture struct {
	Repository          string                  `json:"repository"`
	AIProvider          string                  `json:"aiProvider"`
	StartupCommand      string                  `json:"startupCommand"`
	ResidentMode        bool                    `json:"residentMode"`
	PollIntervalSeconds int                     `json:"pollIntervalSeconds"`
	BaseBranch          string                  `json:"baseBranch"`
	BranchNamePattern   string                  `json:"branchNamePattern"`
	AIAllowedCommands   []string                `json:"aiAllowedCommands"`
	Models              map[string]modelFixture `json:"models"`
	Issue               searchConditionFixture  `json:"issue"`
	PullRequest         searchConditionFixture  `json:"pullRequest"`
}

type modelFixture struct {
	Mode string `json:"mode"`
}

type searchConditionFixture struct {
	LabelIncludes []string `json:"labelIncludes"`
	LabelExcludes []string `json:"labelExcludes"`
	TitleContains []string `json:"titleContains"`
	Authors       []string `json:"authors"`
	Assignees     []string `json:"assignees"`
}

type artifactJob struct {
	job             domain.Job
	includeContext  bool
	artifactSubDir  string
	logSubDir       string
	writeArtifact   bool
	verificationLog bool
}

func main() {
	defaultRoot := filepath.Join(".", "tests")
	rootFlag := flag.String("root", defaultRoot, "root directory for generated test data")
	flag.Parse()

	rootPath, err := filepath.Abs(*rootFlag)
	if err != nil {
		exitf("resolve root: %v", err)
	}

	if err := generate(rootPath); err != nil {
		exitf("%v", err)
	}

	fmt.Printf("Test data created: %s\n", rootPath)
}

func generate(rootPath string) error {
	if err := ensureDirs(rootPath); err != nil {
		return err
	}
	if err := cleanupLegacyData(rootPath); err != nil {
		return err
	}
	if err := cleanupArtifacts(rootPath); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(rootPath, "config", "settings.json"), newSettingsFixture()); err != nil {
		return err
	}

	jobs := buildJobs()
	jobList := make([]domain.Job, 0, len(jobs))
	for _, entry := range jobs {
		jobList = append(jobList, entry.job)
	}
	if err := writeJSON(filepath.Join(rootPath, "db", "jobs.json"), jobList); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(rootPath, "db", "mock_jobs.json"), jobList); err != nil {
		return err
	}

	for _, entry := range jobs {
		if entry.writeArtifact {
			if err := writeArtifact(rootPath, entry); err != nil {
				return err
			}
			if err := writeDiff(rootPath, entry); err != nil {
				return err
			}
		}
		if err := writeLogs(rootPath, entry); err != nil {
			return err
		}
	}

	return nil
}

func ensureDirs(rootPath string) error {
	dirs := []string{
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
		".workspace/review_fix_implementation",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(rootPath, dir), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func cleanupLegacyData(rootPath string) error {
	logsDir := filepath.Join(rootPath, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read logs dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			if removeErr := os.RemoveAll(filepath.Join(logsDir, entry.Name())); removeErr != nil {
				return fmt.Errorf("remove legacy log dir %s: %w", entry.Name(), removeErr)
			}
		}
	}

	legacyDirs := []string{
		filepath.Join(rootPath, "workspace", "mock-owner-mock-repo"),
		filepath.Join(rootPath, "workspace", "mock-owner_mock-repo"),
	}
	for _, dir := range legacyDirs {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove legacy workspace %s: %w", dir, err)
		}
	}
	return nil
}

func cleanupArtifacts(rootPath string) error {
	artifactDirs := []string{
		".workspace/design",
		".workspace/implementation",
		".workspace/review",
		".workspace/review_fix_design",
		".workspace/review_fix_implementation",
		".workspace/pr_conflict",
	}
	for _, dir := range artifactDirs {
		fullDir := filepath.Join(rootPath, dir)
		entries, err := os.ReadDir(fullDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read artifact dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if removeErr := os.Remove(filepath.Join(fullDir, entry.Name())); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove artifact %s/%s: %w", dir, entry.Name(), removeErr)
			}
		}
	}
	return nil
}

func newSettingsFixture() watchSettingsFixture {
	startupCommand := `..\..\..\..\start_mock_app.bat`
	if runtime.GOOS != "windows" {
		startupCommand = "../../../../start_mock_app.sh"
	}
	return watchSettingsFixture{
		Repository:          repositoryName,
		AIProvider:          "codex",
		StartupCommand:      startupCommand,
		ResidentMode:        true,
		PollIntervalSeconds: 3600,
		BaseBranch:          "main",
		BranchNamePattern:   "issue_#<issueNumber>",
		AIAllowedCommands:   []string{"go test ./...", "cd frontend && npm test"},
		Models: map[string]modelFixture{
			"codex":         {Mode: "default"},
			"githubCopilot": {Mode: "default"},
		},
		Issue: searchConditionFixture{
			LabelIncludes: []string{},
			LabelExcludes: []string{},
			TitleContains: []string{},
			Authors:       []string{},
			Assignees:     []string{},
		},
		PullRequest: searchConditionFixture{
			LabelIncludes: []string{},
			LabelExcludes: []string{},
			TitleContains: []string{},
			Authors:       []string{},
			Assignees:     []string{},
		},
	}
}

func buildJobs() []artifactJob {
	return []artifactJob{
		newJob("issue-101", domain.JobKindIssueDesign, domain.StateDetected, 101, "design-detected", 0, "", "", "", true),
		newJob("issue-102", domain.JobKindIssueDesign, domain.StateDesignRunning, 102, "design-running", 1, "", "", "", true),
		newJob("issue-103", domain.JobKindIssueDesign, domain.StateDesignReady, 103, "design-ready", 2, "", "", "", true),
		newJob("issue-104", domain.JobKindIssueDesign, domain.StateDesignApproved, 104, "design-approved", 3, "", "", "", true),
		newJob("issue-105", domain.JobKindIssueDesign, domain.StateCompleted, 105, "design-completed", 4, "", "", "", true),
		newJob("issue-106", domain.JobKindIssueDesign, domain.StateFailed, 106, "design-failed", 5, "", domain.StateDesignRunning, "mock design failure", true),
		newJob("issue-201", domain.JobKindIssueImplementation, domain.StateImplementationRunning, 201, "implementation-running", 6, "検証(2回目)", "", "", true),
		newJob("issue-202", domain.JobKindIssueImplementation, domain.StateImplementationReady, 202, "implementation-ready", 7, "", "", "", true),
		newJob("issue-203", domain.JobKindIssueImplementation, domain.StateImplementationApproved, 203, "implementation-approved", 8, "", "", "", true),
		newJob("issue-204", domain.JobKindIssueImplementation, domain.StatePRCreated, 204, "implementation-pr-created", 9, "", "", "", true),
		newJob("issue-205", domain.JobKindIssueImplementation, domain.StateImplementationRunning, 205, "implementation-awaiting-permission", 10, "コマンド許可待ち", "", "", true),
		newJob("pr-301", domain.JobKindPRReview, domain.StateReviewRunning, 301, "review-running", 11, "", "", "", false),
		newJob("pr-302", domain.JobKindPRReview, domain.StateReviewReady, 302, "review-ready", 12, "", "", "", false),
		newJob("pr-303", domain.JobKindPRReview, domain.StateReviewApproved, 303, "review-approved", 13, "", "", "", false),
		newJob("pr-304", domain.JobKindPRFeedback, domain.StatePRReviewComment, 304, "review-comment", 14, "", "", "", false),
		newJob("pr-508", domain.JobKindPRReview, domain.StateReviewReady, 508, "review-awaiting-user-response", 26, "", "", "", false),
		newJob("pr-401", domain.JobKindPRConflict, domain.StatePRConflict, 401, "conflict-detected", 15, "", "", "", false),
		newJob("pr-402", domain.JobKindPRConflict, domain.StatePRConflictRunning, 402, "conflict-running", 16, "", "", "", false),
		newJob("pr-403", domain.JobKindPRConflict, domain.StatePRConflictReady, 403, "conflict-ready", 17, "", "", "", false),
		newJob("pr-404", domain.JobKindPRConflict, domain.StatePRConflictResolved, 404, "conflict-resolved", 18, "", "", "", false),
		newJob("pr-501", domain.JobKindPRFeedback, domain.StateReviewFixDesignRunning, 501, "review-fix-design-running", 19, "", "", "", false),
		newJob("pr-502", domain.JobKindPRFeedback, domain.StateReviewFixDesignReady, 502, "review-fix-design-ready", 20, "", "", "", false),
		newJob("pr-503", domain.JobKindPRFeedback, domain.StateReviewFixDesignApproved, 503, "review-fix-design-approved", 21, "", "", "", false),
		newJob("pr-504", domain.JobKindPRFeedback, domain.StateReviewFixImplementationRunning, 504, "review-fix-implementation-running", 22, "", "", "", false),
		newJob("pr-505", domain.JobKindPRFeedback, domain.StateReviewFixImplementationReady, 505, "review-fix-implementation-ready", 23, "", "", "", false),
		newJob("pr-506", domain.JobKindPRFeedback, domain.StateReviewFixImplementationApproved, 506, "review-fix-implementation-approved", 24, "", "", "", false),
		newJob("pr-507", domain.JobKindPRFeedback, domain.StateReviewFixed, 507, "review-fixed", 25, "", "", "", false),
	}
}

func newJob(id string, kind domain.JobKind, state domain.JobState, number int, title string, order int, subStatus string, failedFromState domain.JobState, errorMessage string, includeContext bool) artifactJob {
	job := domain.Job{
		ID:         id,
		Kind:       kind,
		State:      state,
		SubStatus:  subStatus,
		Repository: repositoryName,
		Number:     number,
		Title:      title,
		FetchedAt:  timeText(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), order*10),
		UpdatedAt:  timeText(time.Date(2026, 7, 1, 3, 0, 5, 0, time.UTC), order*10),
	}
	if includeContext {
		job.IssueContext = newIssueContext(number, title, state)
	}
	if failedFromState != "" {
		job.FailedFromState = failedFromState
	}
	if errorMessage != "" {
		job.ErrorMessage = errorMessage
	}

	artifactSubDir := artifactSubDirForKind(kind)
	logSubDir := logSubDirForJob(job)
	return artifactJob{
		job:             job,
		includeContext:  includeContext,
		artifactSubDir:  artifactSubDir,
		logSubDir:       logSubDir,
		writeArtifact:   shouldWriteArtifact(state) && artifactSubDir != "",
		verificationLog: kind == domain.JobKindIssueImplementation && state != domain.StateDetected,
	}
}

func timeText(base time.Time, offsetMinutes int) time.Time {
	return base.Add(time.Duration(offsetMinutes) * time.Minute).UTC()
}

func newIssueContext(number int, title string, state domain.JobState) string {
	return fmt.Sprintf("#%d %s\n\nMock issue for testing state: %s\n", number, title, state)
}

func artifactSubDirForKind(kind domain.JobKind) string {
	switch kind {
	case domain.JobKindIssueDesign:
		return "design"
	case domain.JobKindIssueImplementation:
		return "implementation"
	case domain.JobKindPRReview:
		return "review"
	case domain.JobKindPRConflict:
		return "pr_conflict"
	case domain.JobKindPRFeedback:
		return "review_fix_implementation"
	default:
		return ""
	}
}

func logSubDirForJob(job domain.Job) string {
	switch job.Kind {
	case domain.JobKindIssueDesign:
		return "design"
	case domain.JobKindIssueImplementation:
		return "implementation"
	case domain.JobKindPRReview:
		return "review"
	case domain.JobKindPRConflict:
		return "pr_conflict"
	case domain.JobKindPRFeedback:
		if strings.HasPrefix(string(job.State), "review_fix_design") {
			return "review_fix_design"
		}
		return "review_fix_implementation"
	default:
		return ""
	}
}

func shouldWriteArtifact(state domain.JobState) bool {
	switch state {
	case domain.StateDesignReady,
		domain.StateDesignApproved,
		domain.StateImplementationReady,
		domain.StateImplementationApproved,
		domain.StatePRCreated,
		domain.StateReviewReady,
		domain.StateReviewApproved,
		domain.StateReviewFixDesignApproved,
		domain.StateReviewFixImplementationReady,
		domain.StateReviewFixImplementationApproved,
		domain.StateReviewFixed,
		domain.StatePRConflictReady,
		domain.StatePRConflictResolved,
		domain.StateCompleted:
		return true
	default:
		return false
	}
}

func writeArtifact(rootPath string, entry artifactJob) error {
	content := genericArtifactContent(entry)
	switch entry.job.Number {
	case 203:
		content = implementationApprovedArtifactContent(entry)
	case 508:
		content = awaitingUserArtifactContent(entry)
	}
	path := filepath.Join(rootPath, ".workspace", entry.artifactSubDir, fmt.Sprintf("%d_%s.md", entry.job.Number, entry.job.Title))
	return writeText(path, content)
}

func genericArtifactContent(entry artifactJob) string {
	return fmt.Sprintf(`# %s

## Summary
This is a %s artifact for UI testing at state: %s.

## Changes
- This artifact is generated as mock test data.
- Use it to test approve, rerun, and request-changes UI actions.

## Test Results
- go run ./tests/scripts/create-testdata: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## Remaining
- Mock mode does not post to GitHub.
`, entry.job.Title, entry.job.Kind, entry.job.State)
}

func implementationApprovedArtifactContent(entry artifactJob) string {
	return fmt.Sprintf(`# %s

## Summary
This is a %s artifact for UI testing at state: %s.

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
- go run ./tests/scripts/create-testdata: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## Remaining
- Mock mode does not post to GitHub.
`, entry.job.Title, entry.job.Kind, entry.job.State)
}

func awaitingUserArtifactContent(entry artifactJob) string {
	return fmt.Sprintf(`# %s

## 概要
レビューが完了し、ユーザの応答待ちになっているテストデータです。

## ユーザ応答待ち
- 承認、修正依頼、再実行のいずれかを選択してください。
- チャット入力欄に追記したコメントは、そのまま操作時のコメントとして利用できます。

## 変更内容
- AI への指示をチャットで送信する画面確認用の fixture です。
- 結果画面に頼らず、会話の流れで待機状態を把握できます。

## テスト結果
- go run ./tests/scripts/create-testdata: success
- unchanged line 1
- unchanged line 2
- unchanged line 3
- unchanged line 4
- unchanged line 5

## 残課題
- ユーザの操作を待っています。
`, entry.job.Title)
}

func writeDiff(rootPath string, entry artifactJob) error {
	content := genericDiffContent(entry)
	switch entry.job.Number {
	case 203:
		content = implementationApprovedDiffContent(entry)
	case 508:
		content = awaitingUserDiffContent(entry)
	}
	path := filepath.Join(rootPath, ".workspace", entry.artifactSubDir, fmt.Sprintf("%d_%s.diff", entry.job.Number, entry.job.Title))
	return writeText(path, content)
}

func genericDiffContent(entry artifactJob) string {
	return fmt.Sprintf(`diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # %s
 ## Summary
  context line 1
  context line 2
  context line 3
  context line 4
-This is a mock artifact.
+This is a mock artifact for %s.
 This line stays unchanged.
 This line stays unchanged too.
 ## Changes
-This artifact is generated as mock test data.
+This artifact is generated as mock test data for UI testing.
 This line stays unchanged.
 This line stays unchanged too.
This line stays unchanged three.
This line stays unchanged four.
`, entry.job.Title, entry.job.State)
}

func implementationApprovedDiffContent(entry artifactJob) string {
	return fmt.Sprintf(`diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # %s
 ## Summary
  context line 1
  context line 2
  context line 3
  context line 4
-This is a mock artifact.
+This is a mock artifact for %s.
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
`, entry.job.Title, entry.job.State)
}

func awaitingUserDiffContent(entry artifactJob) string {
	return fmt.Sprintf(`diff --git a/mock-source.txt b/mock-source.txt
index 1111111..2222222 100644
--- a/mock-source.txt
+++ b/mock-source.txt
@@ -1,14 +1,14 @@
 # %s
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
`, entry.job.Title)
}

func writeLogs(rootPath string, entry artifactJob) error {
	if entry.job.State == domain.StateDetected || entry.logSubDir == "" {
		return nil
	}

	activity := agentActivity(entry.job)
	if err := writeLogGroup(rootPath, entry.logSubDir, entry.job.Number, roleAgent, 1, activity, fmt.Sprintf("agent stdout: %s", entry.job.State), "agent stderr: none"); err != nil {
		return err
	}

	if entry.job.Kind == domain.JobKindIssueImplementation {
		status := "changes_requested"
		summary := "モックの検証ログです。"
		if entry.job.Number == 205 {
			status = "awaiting_permission"
			summary = "コマンド許可待ちで処理が停止しています。"
		} else if strings.HasPrefix(entry.job.SubStatus, "検証") {
			summary = "検証中の状態で停止しています。"
		}
		if entry.job.Number == 203 {
			status = "passed"
			summary = "チャット表示の見本として、Markdown と HTML を含む成果物を確認しました。"
		}
		verificationActivity := fmt.Sprintf(`=== 2026-07-01T04:01:00Z verification job=%s kind=%s state=%s ===
status: %s
feedback: 追加の確認が必要です。
summary: %s
`, entry.job.ID, entry.job.Kind, entry.job.State, status, summary)
		if err := writeLogGroup(rootPath, entry.logSubDir, entry.job.Number, roleVerifier, 2, verificationActivity, fmt.Sprintf("verifier stdout: %s", entry.job.State), "verifier stderr: none"); err != nil {
			return err
		}
	}

	return nil
}

const (
	roleAgent    = "agent"
	roleVerifier = "verifier"
)

func agentActivity(job domain.Job) string {
	switch job.Number {
	case 205:
		return fmt.Sprintf(`=== 2026-07-01T04:00:00Z request job=%s kind=%s state=%s ===
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
`, job.ID, job.Kind, job.State)
	case 508:
		return fmt.Sprintf(`=== 2026-07-01T04:00:00Z request job=%s kind=%s state=%s ===
provider: codex
model: default
working_dir: tests

[system]
You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown.

[user]
The review is complete and the job is waiting for a user response. Keep the chat state visible until the user replies.

[assistant]
Waiting for user approval or change request.
`, job.ID, job.Kind, job.State)
	case 203:
		return fmt.Sprintf(`=== 2026-07-01T04:00:00Z request job=%s kind=%s state=%s ===
provider: codex
model: default
working_dir: tests

[system]
You are an autonomous software engineer. Follow the repository instructions with minimal extra process. Edit the repository directly and report the result in concise Japanese Markdown.

[user]
Implement the job detail chat preview fixture.

[assistant]
Ready for approval after markdown rendering is verified.
`, job.ID, job.Kind, job.State)
	default:
		return fmt.Sprintf(`=== 2026-07-01T04:00:00Z request job=%s kind=%s state=%s ===
provider: codex
model: default
working_dir: tests

[prompt]
Mock fixture for %s
`, job.ID, job.Kind, job.State, job.Title)
	}
}

func writeLogGroup(rootPath, subDir string, number int, role string, attempt int, activity, stdout, stderr string) error {
	prefix := fmt.Sprintf("%s_attempt-%d", subDir, attempt)
	if role != "" {
		prefix += "_" + role
	}
	jobPrefix := "issue"
	switch subDir {
	case "review", "review_fix_design", "review_fix_implementation", "pr_conflict":
		jobPrefix = "pr"
	}
	jobID := fmt.Sprintf("%s-%d", jobPrefix, number)
	logDir := filepath.Join(rootPath, "workspace", "mock-owner_mock-repo", jobID, "logs")
	if err := writeText(filepath.Join(logDir, prefix+".log"), activity); err != nil {
		return err
	}
	if err := writeText(filepath.Join(logDir, prefix+"_stdout.log"), stdout); err != nil {
		return err
	}
	if err := writeText(filepath.Join(logDir, prefix+"_stderr.log"), stderr); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("marshal json %s: %w", path, err)
	}
	data := buffer.Bytes()
	if len(data) == 0 {
		data = []byte("{}\n")
	}
	return writeBytes(path, data)
}

func writeText(path, value string) error {
	return writeBytes(path, []byte(value))
}

func writeBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
