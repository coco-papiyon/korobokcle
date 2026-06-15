package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	const (
		fixtureRepository = "coco-papiyon/dummy"
		fixtureWorkDir    = "source/coco-papiyon-dummy"
	)

	fixtureRoot := filepath.Join(repoRoot, "tests", "data")
	if err := os.RemoveAll(fixtureRoot); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(fixtureRoot, 0o755); err != nil {
		fail(err)
	}

	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{
			Repository:            fixtureRepository,
			Branch:                "",
			WorkDir:               "",
			ImplementationWorkers: 1,
			ImprovementEnabled:    false,
			ImprovementBranch:     "",
			ImprovementDir:        "",
		},
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
			ID:             "default-issues",
			Name:           "Default Issue Rule",
			Repositories:   []string{fixtureRepository},
			Target:         "issue",
			ProjectName:    "",
			Labels:         []string{"ai:design"},
			ProjectFilters: nil,
			ExcludeDraftPR: true,
			Provider:       "",
			Model:          "",
			SkillSet:       "default",
			TestProfile:    "go-default",
			ToolCommand:    "",
			Enabled:        false,
		},
		{
			ID:             "default-prs",
			Name:           "Default PR Rule",
			Repositories:   []string{fixtureRepository},
			Target:         "pull_request",
			ProjectName:    "",
			Labels:         []string{"ai:review"},
			ProjectFilters: nil,
			ExcludeDraftPR: true,
			Provider:       "",
			Model:          "",
			SkillSet:       "default",
			TestProfile:    "go-default",
			ToolCommand:    "",
			Enabled:        false,
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
	for _, fixture := range buildFixtures(files.App.ArtifactsDir, fixtureRepository) {
		if err := store.UpsertJob(ctx, fixture.job); err != nil {
			fail(err)
		}
		for _, event := range fixture.events {
			if err := store.AppendEvent(ctx, event); err != nil {
				fail(err)
			}
		}
		if err := writeRepositoryArtifacts(fixtureRoot, files.App.ArtifactsDir, fixtureWorkDir, fixture.job.Repository, fixture.job.GitHubNumber, fixture.job.Title, fixture.artifacts, fixture.workspaceFiles); err != nil {
			fail(err)
		}
	}

	if err := initializeFixtureSourceRepository(filepath.Join(fixtureRoot, fixtureWorkDir)); err != nil {
		fail(err)
	}
}

type jobFixture struct {
	job            domain.Job
	events         []domain.Event
	artifacts      []artifactFile
	workspaceFiles []workspaceFile
}

type artifactFile struct {
	worker  string
	name    string
	content string
}

type workspaceFile struct {
	path    string
	content string
}

func buildFixtures(artifactsDir string, repository string) []jobFixture {
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)

	registeredJob := domain.Job{
		ID:           "fixture-issue-registered",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 100,
		State:        domain.StateDetected,
		Title:        "登録直後",
		BranchName:   "issue_100",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(-5 * time.Minute),
		UpdatedAt:    base.Add(-5 * time.Minute),
	}

	designJob := domain.Job{
		ID:           "fixture-design-ready",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 101,
		State:        domain.StateWaitingDesignApproval,
		Title:        "設計済み",
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
		Title:        "実装済み",
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
		Title:        "エラー状態",
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
		Title:        "レビュー済み",
		BranchName:   "korobokcle/pr-review-104",
		WatchRuleID:  "rule-2",
		CreatedAt:    base.Add(30 * time.Minute),
		UpdatedAt:    base.Add(35 * time.Minute),
	}

	prCreatedJob := domain.Job{
		ID:           "fixture-pr-created",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 106,
		State:        domain.StateCompleted,
		Title:        "PR 作成済み",
		BranchName:   "issue_106",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(36 * time.Minute),
		UpdatedAt:    base.Add(39 * time.Minute),
	}

	prCommentAnalysisRunningJob := domain.Job{
		ID:           "fixture-pr-comment-analysis-running",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 108,
		State:        domain.StateDesignRunning,
		Title:        "PRコメント分析中",
		BranchName:   "issue_108",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(52 * time.Minute),
		UpdatedAt:    base.Add(54 * time.Minute),
	}

	prCommentAnalysisReadyJob := domain.Job{
		ID:           "fixture-pr-comment-analysis-ready",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 109,
		State:        domain.StateWaitingDesignApproval,
		Title:        "PRコメント分析済み",
		BranchName:   "issue_109",
		WatchRuleID:  "rule-1",
		CreatedAt:    base.Add(55 * time.Minute),
		UpdatedAt:    base.Add(58 * time.Minute),
	}

	prFeedbackJob := domain.Job{
		ID:           "fixture-pr-feedback-completed",
		Type:         domain.JobTypePRFeedback,
		Repository:   repository,
		GitHubNumber: 107,
		State:        domain.StateCompleted,
		Title:        "PR フィードバック完了",
		BranchName:   "feature/review-feedback-107",
		WatchRuleID:  "rule-2",
		CreatedAt:    base.Add(46 * time.Minute),
		UpdatedAt:    base.Add(50 * time.Minute),
	}

	deletedJobDeletedAt := base.Add(45 * time.Minute)
	deletedJob := domain.Job{
		ID:           "fixture-deleted",
		Type:         domain.JobTypeIssue,
		Repository:   repository,
		GitHubNumber: 105,
		State:        domain.StateWaitingDesignApproval,
		Title:        "削除済み",
		BranchName:   "issue_105",
		WatchRuleID:  "rule-1",
		DeletedAt:    &deletedJobDeletedAt,
		CreatedAt:    base.Add(40 * time.Minute),
		UpdatedAt:    base.Add(44 * time.Minute),
	}

	return []jobFixture{
		{
			job: registeredJob,
			events: []domain.Event{
				domainEvent(registeredJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 100, registeredJob.Title, domain.TargetIssue), base.Add(-5*time.Minute)),
			},
		},
		{
			job: designJob,
			events: []domain.Event{
				domainEvent(designJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 101, designJob.Title, domain.TargetIssue), base),
				domainEvent(designJob.ID, "design_started", string(domain.StateDetected), string(domain.StateDesignRunning), `{"provider":"mock","model":""}`, base.Add(1*time.Minute)),
				domainEvent(designJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), artifactPayload(artifactsDir, repository, 101, artifacts.WorkerDesign, "design"), base.Add(2*time.Minute)),
				domainEvent(designJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), artifactPayload(artifactsDir, repository, 101, artifacts.WorkerDesign, "design"), base.Add(3*time.Minute)),
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
				domainEvent(implementationJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), artifactPayload(artifactsDir, repository, 102, artifacts.WorkerDesign, "design"), base.Add(11*time.Minute)),
				domainEvent(implementationJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), artifactPayload(artifactsDir, repository, 102, artifacts.WorkerDesign, "design"), base.Add(12*time.Minute)),
				domainEvent(implementationJob.ID, "design_approved", string(domain.StateWaitingDesignApproval), string(domain.StateImplementationRunning), `{"comment":"looks good"}`, base.Add(13*time.Minute)),
				domainEvent(implementationJob.ID, "implementation_ready", string(domain.StateImplementationRunning), string(domain.StateImplementationReady), artifactPayload(artifactsDir, repository, 102, artifacts.WorkerImplementation, ""), base.Add(17*time.Minute)),
				domainEvent(implementationJob.ID, "waiting_final_approval", string(domain.StateImplementationReady), string(domain.StateWaitingFinalApproval), artifactPayload(artifactsDir, repository, 102, artifacts.WorkerImplementation, ""), base.Add(18*time.Minute)),
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
			job: prCreatedJob,
			events: []domain.Event{
				domainEvent(prCreatedJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 106, prCreatedJob.Title, domain.TargetIssue), base.Add(36*time.Minute)),
				domainEvent(prCreatedJob.ID, "pr_created", string(domain.StatePRCreating), string(domain.StateCompleted), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 106, artifacts.WorkerPR),
					"url":         githubPullURL(repository, 106),
					"pullNumber":  106,
					"title":       prCreatedJob.Title,
					"head":        prCreatedJob.BranchName,
				}), base.Add(39*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        githubPullURL(repository, 106),
					"pullNumber": 106,
					"repository": repository,
					"branchName": prCreatedJob.BranchName,
					"title":      prCreatedJob.Title,
					"pushed":     true,
				})},
				{worker: artifacts.WorkerPR, name: "gh-pr-comments.json", content: marshalJSON(map[string]any{
					"pullNumber": 106,
					"comments": []map[string]any{
						{
							"author":    "fixture-reviewer",
							"body":      "Looks good to me.",
							"url":       githubPullCommentURL(repository, 106, 1),
							"createdAt": "2026-05-20T09:39:00Z",
						},
						{
							"author":    "fixture-user",
							"body":      "Addressed in the follow-up commit.",
							"url":       githubPullCommentURL(repository, 106, 2),
							"createdAt": "2026-05-20T09:40:00Z",
						},
					},
				})},
				{worker: artifacts.WorkerImprovement, name: "input.md", content: improvementInputMarkdown(prCreatedJob, "final_rejected", []string{"implementation", "fix"}, "繰り返しレビューされる修正方針を恒久化する")},
				{worker: artifacts.WorkerImprovement, name: "context.json", content: marshalJSON(map[string]any{
					"jobID":       prCreatedJob.ID,
					"repository":  repository,
					"issueNumber": prCreatedJob.GitHubNumber,
					"title":       prCreatedJob.Title,
					"phases":      []string{"implementation", "fix"},
					"source": map[string]any{
						"eventType": "final_rejected",
						"comment":   "同じ指摘が繰り返されているので恒久化する",
					},
				})},
				{worker: artifacts.WorkerImprovement, name: "notes.md", content: "# 生成メモ\n\n- mode: ai\n- job: fixture-pr-created\n"},
				{worker: artifacts.WorkerImprovement, name: "decision.json", content: marshalJSON(map[string]any{
					"decision":    "draft_created",
					"reason":      "",
					"updatedAt":   "2026-05-20T09:39:00Z",
					"sourceEvent": "final_rejected",
				})},
			},
			workspaceFiles: []workspaceFile{
				{path: improvementDraftWorkspacePath(prCreatedJob), content: improvementDraftMarkdown("PR 作成前に最終確認を固定化する", "PR 作成前に変更点セルフレビューとテスト結果確認を必ず行う。")},
			},
		},
		{
			job: prCommentAnalysisRunningJob,
			events: []domain.Event{
				domainEvent(prCommentAnalysisRunningJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 108, prCommentAnalysisRunningJob.Title, domain.TargetIssue), base.Add(52*time.Minute)),
				domainEvent(prCommentAnalysisRunningJob.ID, "pr_created", string(domain.StatePRCreating), string(domain.StateCompleted), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 108, artifacts.WorkerPR),
					"url":         githubPullURL(repository, 108),
					"pullNumber":  108,
					"title":       prCommentAnalysisRunningJob.Title,
					"head":        prCommentAnalysisRunningJob.BranchName,
				}), base.Add(53*time.Minute)),
				domainEvent(prCommentAnalysisRunningJob.ID, "pr_comment_analysis_requested", string(domain.StateCompleted), string(domain.StateDesignRunning), marshalJSON(map[string]any{
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please simplify the conditional.",
						"url":       githubPullCommentURL(repository, 108, 1),
						"createdAt": "2026-05-20T09:53:00Z",
					},
				}), base.Add(54*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Implementation Summary\n\n- Previous implementation result kept for analysis context.\n"},
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        githubPullURL(repository, 108),
					"pullNumber": 108,
					"repository": repository,
					"branchName": prCommentAnalysisRunningJob.BranchName,
					"title":      prCommentAnalysisRunningJob.Title,
					"pushed":     true,
				})},
				{worker: artifacts.WorkerPR, name: "gh-pr-comments.json", content: marshalJSON(map[string]any{
					"pullNumber": 108,
					"comments": []map[string]any{
						{
							"author":    "fixture-reviewer",
							"body":      "Please simplify the conditional.",
							"url":       githubPullCommentURL(repository, 108, 1),
							"createdAt": "2026-05-20T09:53:00Z",
						},
					},
				})},
			},
		},
		{
			job: prCommentAnalysisReadyJob,
			events: []domain.Event{
				domainEvent(prCommentAnalysisReadyJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 109, prCommentAnalysisReadyJob.Title, domain.TargetIssue), base.Add(55*time.Minute)),
				domainEvent(prCommentAnalysisReadyJob.ID, "pr_created", string(domain.StatePRCreating), string(domain.StateCompleted), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 109, artifacts.WorkerPR),
					"url":         githubPullURL(repository, 109),
					"pullNumber":  109,
					"title":       prCommentAnalysisReadyJob.Title,
					"head":        prCommentAnalysisReadyJob.BranchName,
				}), base.Add(56*time.Minute)),
				domainEvent(prCommentAnalysisReadyJob.ID, "pr_comment_analysis_requested", string(domain.StateCompleted), string(domain.StateDesignRunning), marshalJSON(map[string]any{
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please split this logic into a helper.",
						"url":       githubPullCommentURL(repository, 109, 1),
						"createdAt": "2026-05-20T09:56:00Z",
					},
				}), base.Add(57*time.Minute)),
				domainEvent(prCommentAnalysisReadyJob.ID, "pr_comment_analysis_ready", string(domain.StateDesignRunning), string(domain.StateWaitingDesignApproval), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 109, artifacts.WorkerPR),
					"pullNumber":  109,
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please split this logic into a helper.",
						"url":       githubPullCommentURL(repository, 109, 1),
						"createdAt": "2026-05-20T09:56:00Z",
					},
				}), base.Add(58*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Previous Implementation\n\n- Keep the previous implementation result for analysis comparison.\n"},
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        githubPullURL(repository, 109),
					"pullNumber": 109,
					"repository": repository,
					"branchName": prCommentAnalysisReadyJob.BranchName,
					"title":      prCommentAnalysisReadyJob.Title,
					"pushed":     true,
				})},
				{worker: artifacts.WorkerPR, name: "gh-pr-comments.json", content: marshalJSON(map[string]any{
					"pullNumber": 109,
					"comments": []map[string]any{
						{
							"author":    "fixture-reviewer",
							"body":      "Please split this logic into a helper.",
							"url":       githubPullCommentURL(repository, 109, 1),
							"createdAt": "2026-05-20T09:56:00Z",
						},
					},
				})},
				{worker: artifacts.WorkerPR, name: "result.md", content: "# PR Comment Analysis\n\n- Split the logic into a helper.\n- Keep the current behavior unchanged.\n"},
				{worker: artifacts.WorkerImprovement, name: "input.md", content: improvementInputMarkdown(prCommentAnalysisReadyJob, "pr_comment_analysis_ready", []string{"fix"}, "PR コメントから恒久改善へ昇格した例")},
				{worker: artifacts.WorkerImprovement, name: "context.json", content: marshalJSON(map[string]any{
					"jobID":       prCommentAnalysisReadyJob.ID,
					"repository":  repository,
					"issueNumber": prCommentAnalysisReadyJob.GitHubNumber,
					"title":       prCommentAnalysisReadyJob.Title,
					"phases":      []string{"fix"},
					"source": map[string]any{
						"eventType": "pr_comment_analysis_ready",
						"comment":   "Please split this logic into a helper.",
						"author":    "fixture-reviewer",
						"url":       githubPullCommentURL(repository, 109, 1),
					},
				})},
				{worker: artifacts.WorkerImprovement, name: "notes.md", content: "# 生成メモ\n\n- mode: ai\n- job: fixture-pr-comment-analysis-ready\n"},
				{worker: artifacts.WorkerImprovement, name: "implementation-prompt.md", content: "implement .improvement/design.md"},
				{worker: artifacts.WorkerImprovement, name: "result.md", content: improvementDraftMarkdown("複雑な条件分岐は helper に抽出する", "条件分岐が長くなったら helper 関数へ切り出し、呼び出し側の責務を狭く保つ。")},
				{worker: artifacts.WorkerImprovement, name: "approval.json", content: marshalJSON(map[string]any{
					"status":     "approved",
					"comment":    "今後も適用する",
					"approvedAt": "2026-05-20T09:58:30Z",
				})},
				{worker: artifacts.WorkerImprovement, name: "decision.json", content: marshalJSON(map[string]any{
					"decision":    "approved",
					"reason":      "今後も適用する",
					"updatedAt":   "2026-05-20T09:58:30Z",
					"sourceEvent": "pr_comment_analysis_ready",
				})},
			},
			workspaceFiles: []workspaceFile{
				{path: improvementDraftWorkspacePath(prCommentAnalysisReadyJob), content: improvementDraftMarkdown("複雑な条件分岐は helper に抽出する", "条件分岐が長くなったら helper 関数へ切り出し、呼び出し側の責務を狭く保つ。")},
				{path: filepath.Join(".improvement", "design.md"), content: "# 改善実装結果\n\n## 修正箇所一覧\n\n- improvement workspace のソースコードを直接修正\n- `.improvement/design.md` を最終要約として更新\n\n## 変更したファイル\n\n- `src/example.go`\n- `.improvement/design.md`\n\n## 追加した処理\n\n- 改善案を直接コードに反映する処理\n\n## 変更した処理\n\n- 既存の実装を改善案ベースへ置き換え\n\n## 動作確認\n\n- mock provider の出力を書き込み可能であることを確認\n\n## 懸念点・残課題\n\n- 実コードの変更は mock のため行っていません。\n"},
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
				{worker: artifacts.WorkerImprovement, name: "input.md", content: improvementInputMarkdown(failedJob, "final_rejected", []string{"implementation"}, "一時的な失敗で恒久化しない例")},
				{worker: artifacts.WorkerImprovement, name: "context.json", content: marshalJSON(map[string]any{
					"jobID":       failedJob.ID,
					"repository":  repository,
					"issueNumber": failedJob.GitHubNumber,
					"title":       failedJob.Title,
					"phases":      []string{"implementation"},
					"source": map[string]any{
						"eventType": "final_rejected",
						"comment":   "",
					},
				})},
				{worker: artifacts.WorkerImprovement, name: "decision.json", content: marshalJSON(map[string]any{
					"decision":    "no_improvement_needed",
					"reason":      "comment was empty",
					"updatedAt":   "2026-05-20T09:28:00Z",
					"sourceEvent": "final_rejected",
				})},
			},
		},
		{
			job: deletedJob,
			events: []domain.Event{
				domainEvent(deletedJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 105, deletedJob.Title, domain.TargetIssue), base.Add(40*time.Minute)),
				domainEvent(deletedJob.ID, "design_started", string(domain.StateDetected), string(domain.StateDesignRunning), `{"provider":"mock","model":""}`, base.Add(41*time.Minute)),
				domainEvent(deletedJob.ID, "design_ready", string(domain.StateDesignRunning), string(domain.StateDesignReady), artifactPayload(artifactsDir, repository, 105, artifacts.WorkerDesign, "design"), base.Add(42*time.Minute)),
				domainEvent(deletedJob.ID, "waiting_design_approval", string(domain.StateDesignReady), string(domain.StateWaitingDesignApproval), artifactPayload(artifactsDir, repository, 105, artifacts.WorkerDesign, "design"), base.Add(43*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerDesign, name: "result.md", content: "# Design\n\n- Demonstrate deleted job filtering.\n- Keep deleted jobs visible in the fixture dataset.\n"},
				{worker: artifacts.WorkerDesign, name: "stdout.log", content: "design worker started\ndeleted job fixture prepared\n"},
				{worker: artifacts.WorkerDesign, name: "stderr.log", content: ""},
			},
		},
		{
			job: reviewedJob,
			events: []domain.Event{
				domainEvent(reviewedJob.ID, "pull_request_matched", "", string(domain.StateCollectingContext), pullRequestPayload(repository, 104, reviewedJob.Title), base.Add(30*time.Minute)),
				domainEvent(reviewedJob.ID, "review_started", string(domain.StateCollectingContext), string(domain.StateReviewRunning), `{"provider":"mock","model":""}`, base.Add(31*time.Minute)),
				domainEvent(reviewedJob.ID, "review_ready", string(domain.StateReviewRunning), string(domain.StateReviewReady), artifactPayload(artifactsDir, repository, 104, artifacts.WorkerReview, "review"), base.Add(35*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerReview, name: "result.md", content: "# Review Summary\n\n- API response shape is consistent.\n- No blocking issues found.\n"},
				{worker: artifacts.WorkerReview, name: "stdout.log", content: "review worker started\nreview completed\n"},
				{worker: artifacts.WorkerReview, name: "stderr.log", content: ""},
			},
		},
		{
			job: prFeedbackJob,
			events: []domain.Event{
				domainEvent(prFeedbackJob.ID, "pull_request_review_matched", "", string(domain.StateImplementationRunning), marshalJSON(map[string]any{
					"ruleId":     "rule-2",
					"ruleName":   "Fixture Review Rule",
					"repository": repository,
					"number":     107,
					"url":        githubPullURL(repository, 107),
					"target":     domain.TargetPullRequestReview,
					"title":      prFeedbackJob.Title,
					"body":       "Fixture PR review body used for manual verification.",
					"author":     "fixture-reviewer",
					"labels":     []string{"ai:review"},
					"assignees":  []string{"fixture-reviewer"},
					"branchName": prFeedbackJob.BranchName,
					"baseBranch": "main",
					"reviewComments": []map[string]any{
						{
							"id":        9001,
							"author":    "fixture-reviewer",
							"body":      "Please tighten this logic.",
							"path":      "internal/app/pr_worker.go",
							"line":      120,
							"url":       githubPullDiscussionURL(repository, 107, "discussion_r1"),
							"createdAt": "2026-05-20T09:46:00Z",
							"updatedAt": "2026-05-20T09:46:00Z",
						},
					},
				}), base.Add(46*time.Minute)),
				domainEvent(prFeedbackJob.ID, "implementation_ready", string(domain.StateImplementationRunning), string(domain.StateImplementationReady), artifactPayload(artifactsDir, repository, 107, artifacts.WorkerImplementation, "review"), base.Add(47*time.Minute)),
				domainEvent(prFeedbackJob.ID, "waiting_final_approval", string(domain.StateImplementationReady), string(domain.StateWaitingFinalApproval), artifactPayload(artifactsDir, repository, 107, artifacts.WorkerImplementation, "review"), base.Add(48*time.Minute)),
				domainEvent(prFeedbackJob.ID, "final_approved", string(domain.StateWaitingFinalApproval), string(domain.StatePRCreating), `{"comment":"review feedback addressed"}`, base.Add(49*time.Minute)),
				domainEvent(prFeedbackJob.ID, "pr_updated", string(domain.StatePRCreating), string(domain.StateCompleted), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 107, artifacts.WorkerPR),
					"url":         githubPullURL(repository, 107),
					"pullNumber":  107,
					"title":       prFeedbackJob.Title,
					"head":        prFeedbackJob.BranchName,
				}), base.Add(50*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Review Feedback Implementation\n\n- Addressed reviewer comments.\n"},
				{worker: artifacts.WorkerImplementation, name: "stdout.log", content: "implementation worker started\nreview feedback applied\n"},
				{worker: artifacts.WorkerImplementation, name: "stderr.log", content: ""},
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        githubPullURL(repository, 107),
					"pullNumber": 107,
					"repository": repository,
					"branchName": prFeedbackJob.BranchName,
					"title":      prFeedbackJob.Title,
					"pushed":     true,
				})},
				{worker: artifacts.WorkerPR, name: "gh-pr-comments.json", content: marshalJSON(map[string]any{
					"pullNumber": 107,
					"comments": []map[string]any{
						{
							"author":    "fixture-reviewer",
							"body":      "Thanks for the fix.",
							"url":       githubPullCommentURL(repository, 107, 1),
							"createdAt": "2026-05-20T09:50:00Z",
						},
					},
				})},
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

func writeRepositoryArtifacts(root string, artifactsDir string, workDirSetting string, repository string, issueNumber int, title string, files []artifactFile, workspaceFiles []workspaceFile) error {
	workDir := artifacts.RepositoryWorkerWorkDir(root, artifactsDir, repository, workDirSetting)
	for _, file := range files {
		path := filepath.Join(artifacts.RepositoryWorkerJobPhaseDir(root, artifactsDir, repository, issueNumber, file.worker), file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
		if file.name == "result.md" && file.worker != artifacts.WorkerPR && file.worker != artifacts.WorkerImprovement {
			workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, file.worker, issueNumber, title)
			if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(workingPath, []byte(file.content), 0o644); err != nil {
				return err
			}
		}
	}
	for _, file := range workspaceFiles {
		path := filepath.Join(workDir, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func artifactPayload(artifactsDir string, repository string, issueNumber int, worker string, phase string) string {
	artifactDir := artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, issueNumber, worker)
	payload := map[string]any{
		"artifactDir": artifactDir,
	}
	if strings.TrimSpace(phase) != "" {
		payload["skill"] = phase
	}
	return marshalJSON(payload)
}

func improvementInputMarkdown(job domain.Job, sourceEventType string, phases []string, comment string) string {
	return fmt.Sprintf(`# Improvement Input

- jobId: %s
- repository: %s
- issueNumber: %d
- sourceEventType: %s
- phases: %s

## Comment

%s
`, job.ID, job.Repository, job.GitHubNumber, sourceEventType, strings.Join(phases, ", "), comment)
}

func improvementDraftMarkdown(title string, policy string) string {
	return fmt.Sprintf(`## タイトル

%s

## 汎化した方針案

%s
`, title, policy)
}

func improvementDraftWorkspacePath(job domain.Job) string {
	return filepath.Join(".improvement", "draft", artifacts.RepositoryWorkerImprovementDraftFileName(job.ID, job.Title))
}

func approvedImprovementMarkdown(job domain.Job, title string, phases []string, body string) string {
	return fmt.Sprintf(`---
id: %s
title: %s
scope: repository
phases:
%s
status: active
updatedAt: 2026-05-20T09:58:30Z
source:
  jobID: %s
  issueNumber: %d
  repository: %s
  event: improvement_approved
---

%s
`, slugify(title), title, yamlList(phases), job.ID, job.GitHubNumber, job.Repository, body)
}

func yamlList(values []string) string {
	if len(values) == 0 {
		return "  - implementation"
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, "  - "+value)
	}
	return strings.Join(lines, "\n")
}

func slugify(value string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "@", "-", "?", "-", "#", "-", "　", "-")
	normalized := replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "improvement"
	}
	return normalized
}

func writeReadme(root string) error {
	const body = `# Test Data

動作確認用の fixture です。` + "`KOROBOKCLE_TOOL_ROOT=test/data`" + ` を指定して起動すると、
この配下の ` + "`config/`" + `、` + "`data/`" + `、` + "`artifacts/`" + ` を使って UI を確認できます。
監視対象リポジトリは設定していないため、ここでは監視処理は動作しません。
	repository worker の作業ディレクトリは ` + "`source/coco-papiyon-dummy`" + `、成果物は ` + "`artifacts/coco-papiyon-dummy/jobs/issue_<issue番号>/`" + ` 配下に出力されます。
作業ディレクトリには、AI の ` + "`result.md`" + ` を ` + "`design/issue_<issue番号>_...md`" + ` などの形式で複製しています。

含まれるジョブ:

` + `| 概要 | 種別 | GitHub 番号 | 状態 | 主なイベント | 主な成果物 |
| --- | --- | ---: | --- | --- | --- |
| issue 登録直後 (fixture-issue-registered) | issue | 100 | detected | issue_matched | なし |
| 設計済み (fixture-design-ready) | issue | 101 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |
| 実装済み (fixture-implementation-ready) | issue | 102 | waiting_final_approval | issue_matched, design_ready, waiting_design_approval, design_approved, implementation_ready, waiting_final_approval | design/result.md, implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log |
| エラー状態 (fixture-failed) | issue | 103 | failed | issue_matched, design_approved, implementation_failed | implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log |
| レビュー実行済み (fixture-review-completed) | pr_review | 104 | review_ready | pull_request_matched, review_started, review_ready | review/result.md, review/stdout.log, review/stderr.log |
| PR 作成済み (fixture-pr-created) | issue | 106 | completed | issue_matched, pr_created | pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析中 (fixture-pr-comment-analysis-running) | issue | 108 | design_running | issue_matched, pr_created, pr_comment_analysis_requested | implementation/result.md, pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析済み (fixture-pr-comment-analysis-ready) | issue | 109 | waiting_design_approval | issue_matched, pr_created, pr_comment_analysis_requested, pr_comment_analysis_ready | implementation/result.md, pr/result.json, pr/gh-pr-comments.json, pr/result.md |
| PR フィードバック完了 (fixture-pr-feedback-completed) | pr_feedback | 107 | completed | pull_request_review_matched, implementation_ready, waiting_final_approval, final_approved, pr_updated | implementation/result.md, implementation/stdout.log, implementation/stderr.log, pr/result.json, pr/gh-pr-comments.json |
| 削除済み (fixture-deleted) | issue | 105 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |` + `

改善 fixture:

- ` + "`fixture-pr-created`" + `: ` + "`draft_created`" + `。shared workdir の ` + "`.improvement/draft/<job-id>_<title>.md`" + ` に下書きがあります。
- ` + "`fixture-pr-comment-analysis-ready`" + `: ` + "`approved`" + `。shared workdir の ` + "`.improvement/design.md`" + ` に承認済み指示があります。
- ` + "`fixture-failed`" + `: ` + "`no_improvement_needed`" + `。job artifact の ` + "`improvement/decision.json`" + ` に理由があります。

再生成:

` + "```powershell" + `
go run ./scripts/create-testdata
` + "```" + `
	`
	return os.WriteFile(filepath.Join(root, "README.md"), []byte(body), 0o644)
}

func initializeFixtureSourceRepository(workDir string) error {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	if err := runGitCommand(workDir, "init", "--initial-branch=main"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# Dummy fixture source\n"), 0o644); err != nil {
		return err
	}
	if err := runGitCommand(workDir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "add", "-A"); err != nil {
		return err
	}
	if err := runGitCommand(workDir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "commit", "-m", "fixture source"); err != nil {
		return err
	}
	return nil
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %w: %s", args, err, strings.TrimSpace(string(out)))
	}
	return nil
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

func githubIssueURL(repository string, number int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", repository, number)
}

func githubPullURL(repository string, number int) string {
	return fmt.Sprintf("https://github.com/%s/pull/%d", repository, number)
}

func githubPullCommentURL(repository string, number int, commentIndex int) string {
	return fmt.Sprintf("https://github.com/%s/pull/%d#issuecomment-%d", repository, number, commentIndex)
}

func githubPullDiscussionURL(repository string, number int, discussionID string) string {
	return fmt.Sprintf("https://github.com/%s/pull/%d#%s", repository, number, discussionID)
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
