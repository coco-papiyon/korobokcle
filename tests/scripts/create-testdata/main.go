package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/storage/sqlite"
)

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		fail(err)
	}

	fixtureRoot := filepath.Join(repoRoot, "tests", "data")
	if err := os.RemoveAll(fixtureRoot); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(fixtureRoot, 0o755); err != nil {
		fail(err)
	}

	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "coco-papiyon/korobokcle", Branch: "main", Workers: 1},
	}
	files.ToolCommands.Commands = []config.ToolCommand{
		{
			Name:     "fixture-test",
			Command:  "export KOROBOKCLE_PORT=8081\n./test.sh",
			Resident: false,
		},
	}
	files.WatchRules.Rules = []config.WatchRule{
		{
			ID:             "rule-1",
			Name:           "Fixture Issue Rule",
			Repositories:   []string{"coco-papiyon/korobokcle"},
			Target:         "issue",
			Labels:         []string{"ai:design"},
			ExcludeDraftPR: true,
			SkillSet:       "default",
			TestProfile:    "go-default",
			ToolCommand:    "fixture-test",
			Enabled:        true,
		},
		{
			ID:             "rule-2",
			Name:           "Fixture Review Rule",
			Repositories:   []string{"coco-papiyon/korobokcle"},
			Target:         "pull_request",
			Labels:         []string{"ai:review"},
			ExcludeDraftPR: true,
			SkillSet:       "default",
			TestProfile:    "go-default",
			ToolCommand:    "fixture-test",
			Enabled:        true,
		},
	}

	if err := writeConfigFiles(fixtureRoot, files); err != nil {
		fail(err)
	}
	if err := writeReadme(fixtureRoot); err != nil {
		fail(err)
	}

	store, err := sqlite.Open(filepath.Join(fixtureRoot, "data", "korobokcle.db"))
	if err != nil {
		fail(err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			fail(closeErr)
		}
	}()

	ctx := context.Background()
	for _, fixture := range buildFixtures() {
		if err := store.UpsertJob(ctx, fixture.job); err != nil {
			fail(err)
		}
		for _, event := range fixture.events {
			if err := store.AppendEvent(ctx, event); err != nil {
				fail(err)
			}
		}
		if err := writeArtifacts(fixtureRoot, files.App.ArtifactsDir, fixture.job.ID, fixture.artifacts); err != nil {
			fail(err)
		}
	}
}

type jobFixture struct {
	job       domain.Job
	events    []domain.Event
	artifacts []artifactFile
}

type artifactFile struct {
	worker  string
	name    string
	content string
}

func buildFixtures() []jobFixture {
	repository := "coco-papiyon/korobokcle"
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)

	designJob := domain.Job{
		ID:           "fixture-design-ready",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 101,
		State:        domain.StateWaitingDesignApproval,
		Title:        "Add test fixture viewer",
		BranchName:   "issue_101",
		WatchRuleID:  "rule-1",
		CreatedAt:    base,
		UpdatedAt:    base.Add(3 * time.Minute),
	}

	implementationJob := domain.Job{
		ID:           "fixture-implementation-ready",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 102,
		State:        domain.StateWaitingFinalApproval,
		Title:        "Support manual test fixtures",
		BranchName:   "issue_102",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(10 * time.Minute),
		UpdatedAt:    base.Add(18 * time.Minute),
	}

	failedJob := domain.Job{
		ID:           "fixture-failed",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 103,
		State:        domain.StateFailed,
		Title:        "Reproduce flaky approval flow",
		BranchName:   "issue_103",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(20 * time.Minute),
		UpdatedAt:    base.Add(28 * time.Minute),
	}

	reviewedJob := domain.Job{
		ID:           "fixture-review-completed",
		Type:         domain.JobTypePRReview,
		Repository:   repository,
		GitHubNumber: 104,
		State:        domain.StateReviewReady,
		Title:        "Review API response cleanup",
		BranchName:   "korobokcle/pr-review-104",
		WatchRuleID:  "rule-2",
		CreatedAt:    base.Add(30 * time.Minute),
		UpdatedAt:    base.Add(35 * time.Minute),
	}

	deletedJobDeletedAt := base.Add(45 * time.Minute)
	deletedJob := domain.Job{
		ID:           "fixture-deleted",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 105,
		State:        domain.StateWaitingDesignApproval,
		Title:        "Archive removed fixture job",
		BranchName:   "issue_105",
		WatchRuleID:  "rule-1",
		DeletedAt:    &deletedJobDeletedAt,
		CreatedAt:    base.Add(40 * time.Minute),
		UpdatedAt:    base.Add(44 * time.Minute),
	}

	return []jobFixture{
		{
			job: designJob,
			events: []domain.Event{
				domainEvent(designJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 101, designJob.Title, domain.TargetIssue), base),
				domainEvent(designJob.ID, "design_started", string(domain.StateDetected), string(domain.StateDesignRunning), `{"provider":"mock","model":""}`, base.Add(1*time.Minute)),
				domainEvent(designJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), `{"artifactDir":"artifacts/jobs/fixture-design-ready/design","skill":"design"}`, base.Add(2*time.Minute)),
				domainEvent(designJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), `{"artifactDir":"artifacts/jobs/fixture-design-ready/design","skill":"design"}`, base.Add(3*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerDesign, name: "result.md", content: "# Design\n\n- Add a dedicated fixture dataset.\n- Keep states visible on the dashboard.\n"},
				{worker: artifacts.WorkerDesign, name: "stdout.log", content: "design worker started\nfixture data prepared\n"},
				{worker: artifacts.WorkerDesign, name: "stderr.log", content: ""},
			},
		},
		{
			job: implementationJob,
			events: []domain.Event{
				domainEvent(implementationJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 102, implementationJob.Title, domain.TargetIssue), base.Add(10*time.Minute)),
				domainEvent(implementationJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), `{"artifactDir":"artifacts/jobs/fixture-implementation-ready/design","skill":"design"}`, base.Add(11*time.Minute)),
				domainEvent(implementationJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), `{"artifactDir":"artifacts/jobs/fixture-implementation-ready/design","skill":"design"}`, base.Add(12*time.Minute)),
				domainEvent(implementationJob.ID, "design_approved", string(domain.StateWaitingDesignApproval), string(domain.StateImplementationRunning), `{"comment":"looks good"}`, base.Add(13*time.Minute)),
				domainEvent(implementationJob.ID, "implementation_ready", string(domain.StateImplementationRunning), string(domain.StateImplementationReady), `{"artifactDir":"artifacts/jobs/fixture-implementation-ready/implementation"}`, base.Add(17*time.Minute)),
				domainEvent(implementationJob.ID, "waiting_final_approval", string(domain.StateImplementationReady), string(domain.StateWaitingFinalApproval), `{"artifactDir":"artifacts/jobs/fixture-implementation-ready/implementation"}`, base.Add(18*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerDesign, name: "result.md", content: "# Design\n\nImplementation can proceed.\n"},
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Implementation Summary\n\n- Added fixture generator.\n- Added fixture documentation.\n"},
				{worker: artifacts.WorkerImplementation, name: "test-report.json", content: "{\n  \"success\": true,\n  \"commands\": [\n    {\n      \"command\": \"go test ./...\",\n      \"success\": true\n    }\n  ]\n}\n"},
				{worker: artifacts.WorkerImplementation, name: "stdout.log", content: "implementation worker started\nall checks passed\n"},
				{worker: artifacts.WorkerImplementation, name: "stderr.log", content: ""},
			},
		},
		{
			job: failedJob,
			events: []domain.Event{
				domainEvent(failedJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 103, failedJob.Title, domain.TargetIssue), base.Add(20*time.Minute)),
				domainEvent(failedJob.ID, "design_approved", string(domain.StateWaitingDesignApproval), string(domain.StateImplementationRunning), `{"comment":"retry failing case"}`, base.Add(21*time.Minute)),
				domainEvent(failedJob.ID, "implementation_failed", string(domain.StateImplementationRunning), string(domain.StateFailed), `{"error":"go test ./... failed: expected 200, got 500"}`, base.Add(28*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Implementation Attempt\n\nA panic occurs during approval handling.\n"},
				{worker: artifacts.WorkerImplementation, name: "test-report.json", content: "{\n  \"success\": false,\n  \"commands\": [\n    {\n      \"command\": \"go test ./...\",\n      \"success\": false,\n      \"stderr\": \"expected 200, got 500\"\n    }\n  ]\n}\n"},
				{worker: artifacts.WorkerImplementation, name: "stdout.log", content: "implementation worker started\nrunning go test ./...\n"},
				{worker: artifacts.WorkerImplementation, name: "stderr.log", content: "FAIL\tinternal/web\tapproval handler returned 500\n"},
			},
		},
		{
			job: reviewedJob,
			events: []domain.Event{
				domainEvent(reviewedJob.ID, "pull_request_matched", "", string(domain.StateCollectingContext), pullRequestPayload(repository, 104, reviewedJob.Title), base.Add(30*time.Minute)),
				domainEvent(reviewedJob.ID, "review_started", string(domain.StateCollectingContext), string(domain.StateReviewRunning), `{"provider":"mock","model":""}`, base.Add(31*time.Minute)),
				domainEvent(reviewedJob.ID, "review_ready", string(domain.StateReviewRunning), string(domain.StateReviewReady), `{"artifactDir":"artifacts/jobs/fixture-review-completed/review","skill":"review"}`, base.Add(35*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerReview, name: "result.md", content: "# Review Summary\n\n- API response shape is consistent.\n- No blocking issues found.\n"},
				{worker: artifacts.WorkerReview, name: "stdout.log", content: "review worker started\nreview completed\n"},
				{worker: artifacts.WorkerReview, name: "stderr.log", content: ""},
			},
		},
		{
			job: deletedJob,
			events: []domain.Event{
				domainEvent(deletedJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 105, deletedJob.Title, domain.TargetIssue), base.Add(40*time.Minute)),
				domainEvent(deletedJob.ID, "design_started", string(domain.StateDetected), string(domain.StateDesignRunning), `{"provider":"mock","model":""}`, base.Add(41*time.Minute)),
				domainEvent(deletedJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), `{"artifactDir":"artifacts/jobs/fixture-deleted/design","skill":"design"}`, base.Add(42*time.Minute)),
				domainEvent(deletedJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), `{"artifactDir":"artifacts/jobs/fixture-deleted/design","skill":"design"}`, base.Add(43*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerDesign, name: "result.md", content: "# Design\n\n- Demonstrate deleted job filtering.\n- Keep deleted jobs visible in the fixture dataset.\n"},
				{worker: artifacts.WorkerDesign, name: "stdout.log", content: "design worker started\ndeleted job fixture prepared\n"},
				{worker: artifacts.WorkerDesign, name: "stderr.log", content: ""},
			},
		},
	}
}

func writeConfigFiles(root string, files config.Files) error {
	targets := []struct {
		path  string
		value any
	}{
		{path: filepath.Join(root, "config", "app.yaml"), value: files.App},
		{path: filepath.Join(root, "config", "watch-rules.yaml"), value: files.WatchRules},
		{path: filepath.Join(root, "config", "notifications.yaml"), value: files.Notifications},
		{path: filepath.Join(root, "config", "test-profiles.yaml"), value: files.TestProfiles},
		{path: filepath.Join(root, "config", "tool-commands.yaml"), value: files.ToolCommands},
	}

	for _, target := range targets {
		raw, err := yaml.Marshal(target.value)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target.path, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeArtifacts(root string, artifactsDir string, jobID string, files []artifactFile) error {
	for _, file := range files {
		path := filepath.Join(artifacts.WorkerDir(root, artifactsDir, jobID, file.worker), file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeReadme(root string) error {
	const body = `# Test Data

動作確認用の fixture です。` + "`KOROBOKCLE_TOOL_ROOT=test/data`" + ` を指定して起動すると、
この配下の ` + "`config/`" + `、` + "`data/`" + `、` + "`artifacts/`" + ` を使って UI を確認できます。

含まれるジョブ:

- ` + "`fixture-design-ready`" + `: 設計済み。状態は ` + "`waiting_design_approval`" + `
- ` + "`fixture-implementation-ready`" + `: 実装済み。状態は ` + "`waiting_final_approval`" + `
- ` + "`fixture-failed`" + `: エラー状態。状態は ` + "`failed`" + `
- ` + "`fixture-review-completed`" + `: レビュー実行済みで承認待ち。状態は ` + "`review_ready`" + `
- ` + "`fixture-deleted`" + `: 削除済み。状態は ` + "`waiting_design_approval`" + `

再生成:

` + "```powershell" + `
go run ./scripts/create-testdata
` + "```" + `
`
	return os.WriteFile(filepath.Join(root, "README.md"), []byte(body), 0o644)
}

func domainEvent(jobID string, eventType string, stateFrom string, stateTo string, payload string, createdAt time.Time) domain.Event {
	return domain.Event{
		JobID:     jobID,
		EventType: eventType,
		StateFrom: stateFrom,
		StateTo:   stateTo,
		Payload:   payload,
		CreatedAt: createdAt,
	}
}

func issuePayload(repository string, number int, title string, target domain.MonitoredTarget) string {
	return marshalJSON(map[string]any{
		"ruleId":     "rule-1",
		"ruleName":   "Fixture Issue Rule",
		"repository": repository,
		"number":     number,
		"url":        fmt.Sprintf("https://github.com/%s/issues/%d", repository, number),
		"target":     target,
		"title":      title,
		"body":       "Fixture issue body used for manual verification.",
		"author":     "fixture-user",
		"labels":     []string{"ai:design"},
		"assignees":  []string{"fixture-user"},
		"branchName": fmt.Sprintf("issue_%d", number),
		"baseBranch": "main",
	})
}

func pullRequestPayload(repository string, number int, title string) string {
	return marshalJSON(map[string]any{
		"ruleId":     "rule-2",
		"ruleName":   "Fixture Review Rule",
		"repository": repository,
		"number":     number,
		"url":        fmt.Sprintf("https://github.com/%s/pull/%d", repository, number),
		"target":     domain.TargetPullRequest,
		"title":      title,
		"body":       "Fixture pull request body used for manual verification.",
		"author":     "fixture-reviewer",
		"labels":     []string{"ai:review"},
		"assignees":  []string{"fixture-reviewer"},
		"branchName": "feature/review-cleanup",
		"baseBranch": "main",
	})
}

func marshalJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		fail(err)
	}
	return string(raw)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
