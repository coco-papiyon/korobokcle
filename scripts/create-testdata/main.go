package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
			Repository:         "coco-papiyon/korobokcle",
			Branch:             "main",
			WorkDir:            "artifacts/workers/coco-papiyon-korobokcle/work",
			Workers:            1,
			ImprovementEnabled: true,
			ImprovementBranch:  "develop",
			ImprovementDir:     ".improvements",
			ImprovementWorkDir: ".improvement",
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
	for _, fixture := range buildFixtures(files.App.ArtifactsDir) {
		if err := store.UpsertJob(ctx, fixture.job); err != nil {
			fail(err)
		}
		for _, event := range fixture.events {
			if err := store.AppendEvent(ctx, event); err != nil {
				fail(err)
			}
		}
		if err := writeRepositoryArtifacts(fixtureRoot, files.App.ArtifactsDir, files.App.MonitoredRepositories[0].WorkDir, fixture.job.Repository, fixture.job.GitHubNumber, fixture.job.Title, fixture.artifacts); err != nil {
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
	worker      string
	name        string
	content     string
	destination string
}

func buildFixtures(artifactsDir string) []jobFixture {
	repository := "coco-papiyon/korobokcle"
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
					"url":         "https://github.com/coco-papiyon/korobokcle/pull/106",
					"pullNumber":  106,
					"title":       prCreatedJob.Title,
					"head":        prCreatedJob.BranchName,
				}), base.Add(39*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        "https://github.com/coco-papiyon/korobokcle/pull/106",
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
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/106#issuecomment-1",
							"createdAt": "2026-05-20T09:39:00Z",
						},
						{
							"author":    "fixture-user",
							"body":      "Addressed in the follow-up commit.",
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/106#issuecomment-2",
							"createdAt": "2026-05-20T09:40:00Z",
						},
					},
				})},
				{worker: "improvement", name: "input.md", content: "ボタンを左に配置し、右に説明文を配置してほしい。\n"},
				{worker: "improvement", name: "context.json", content: marshalJSON(map[string]any{
					"jobId":       prCreatedJob.ID,
					"repository":  repository,
					"issueNumber": 106,
					"title":       prCreatedJob.Title,
					"jobType":     string(prCreatedJob.Type),
					"comment":     "ボタンを左に配置し、右に説明文を配置してほしい。",
					"createdAt":   "2026-05-20T09:40:30Z",
				})},
				{worker: "improvement", name: "draft.md", content: "# 画面レイアウト方針\n\n- 操作要素は左、補足説明は右に配置する。\n- モーダルでは主要情報を開いた時点で表示する。\n"},
				{worker: "improvement", name: "decision.json", content: marshalJSON(map[string]any{
					"decision":  "draft_created",
					"reason":    "",
					"updatedAt": "2026-05-20T09:41:00Z",
				})},
				{worker: "improvement", name: "issue_106_画面レイアウト方針.md", content: "# 画面レイアウト方針\n\n- 操作要素は左、補足説明は右に配置する。\n- モーダルでは主要情報を開いた時点で表示する。\n", destination: "improvement-draft"},
			},
		},
		{
			job: prCommentAnalysisRunningJob,
			events: []domain.Event{
				domainEvent(prCommentAnalysisRunningJob.ID, "issue_matched", "", string(domain.StateDetected), issuePayload(repository, 108, prCommentAnalysisRunningJob.Title, domain.TargetIssue), base.Add(52*time.Minute)),
				domainEvent(prCommentAnalysisRunningJob.ID, "pr_created", string(domain.StatePRCreating), string(domain.StateCompleted), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 108, artifacts.WorkerPR),
					"url":         "https://github.com/coco-papiyon/korobokcle/pull/108",
					"pullNumber":  108,
					"title":       prCommentAnalysisRunningJob.Title,
					"head":        prCommentAnalysisRunningJob.BranchName,
				}), base.Add(53*time.Minute)),
				domainEvent(prCommentAnalysisRunningJob.ID, "pr_comment_analysis_requested", string(domain.StateCompleted), string(domain.StateDesignRunning), marshalJSON(map[string]any{
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please simplify the conditional.",
						"url":       "https://github.com/coco-papiyon/korobokcle/pull/108#issuecomment-1",
						"createdAt": "2026-05-20T09:53:00Z",
					},
				}), base.Add(54*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Implementation Summary\n\n- Previous implementation result kept for analysis context.\n"},
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        "https://github.com/coco-papiyon/korobokcle/pull/108",
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
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/108#issuecomment-1",
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
					"url":         "https://github.com/coco-papiyon/korobokcle/pull/109",
					"pullNumber":  109,
					"title":       prCommentAnalysisReadyJob.Title,
					"head":        prCommentAnalysisReadyJob.BranchName,
				}), base.Add(56*time.Minute)),
				domainEvent(prCommentAnalysisReadyJob.ID, "pr_comment_analysis_requested", string(domain.StateCompleted), string(domain.StateDesignRunning), marshalJSON(map[string]any{
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please split this logic into a helper.",
						"url":       "https://github.com/coco-papiyon/korobokcle/pull/109#issuecomment-1",
						"createdAt": "2026-05-20T09:56:00Z",
					},
				}), base.Add(57*time.Minute)),
				domainEvent(prCommentAnalysisReadyJob.ID, "pr_comment_analysis_ready", string(domain.StateDesignRunning), string(domain.StateWaitingDesignApproval), marshalJSON(map[string]any{
					"artifactDir": artifacts.RepositoryWorkerJobPhaseDir("", artifactsDir, repository, 109, artifacts.WorkerPR),
					"pullNumber":  109,
					"comment": map[string]any{
						"author":    "fixture-reviewer",
						"body":      "Please split this logic into a helper.",
						"url":       "https://github.com/coco-papiyon/korobokcle/pull/109#issuecomment-1",
						"createdAt": "2026-05-20T09:56:00Z",
					},
				}), base.Add(58*time.Minute)),
			},
			artifacts: []artifactFile{
				{worker: artifacts.WorkerImplementation, name: "result.md", content: "# Previous Implementation\n\n- Keep the previous implementation result for analysis comparison.\n"},
				{worker: artifacts.WorkerPR, name: "result.json", content: marshalJSON(map[string]any{
					"url":        "https://github.com/coco-papiyon/korobokcle/pull/109",
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
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/109#issuecomment-1",
							"createdAt": "2026-05-20T09:56:00Z",
						},
					},
				})},
				{worker: artifacts.WorkerPR, name: "result.md", content: "# PR Comment Analysis\n\n- Split the logic into a helper.\n- Keep the current behavior unchanged.\n"},
				{worker: "improvement", name: "input.md", content: "PR コメントをもとに、継続利用できる実装方針へ整理してほしい。\n"},
				{worker: "improvement", name: "context.json", content: marshalJSON(map[string]any{
					"jobId":       prCommentAnalysisReadyJob.ID,
					"repository":  repository,
					"issueNumber": 109,
					"title":       prCommentAnalysisReadyJob.Title,
					"jobType":     string(prCommentAnalysisReadyJob.Type),
					"comment":     "PR コメントをもとに、継続利用できる実装方針へ整理してほしい。",
					"createdAt":   "2026-05-20T09:57:30Z",
				})},
				{worker: "improvement", name: "draft.md", content: "# ロジック分割方針\n\n- 条件分岐が増える処理は helper 関数へ分割する。\n- 既存の振る舞いを変えないことを先に確認する。\n"},
				{worker: "improvement", name: "result.md", content: "# ロジック分割方針\n\n- 条件分岐が増える処理は helper 関数へ分割する。\n- 既存の振る舞いを変えないことを先に確認する。\n"},
				{worker: "improvement", name: "approval.json", content: marshalJSON(map[string]any{
					"status":     "approved",
					"comment":    "この方針で継続利用する",
					"approvedAt": "2026-05-20T09:58:30Z",
				})},
				{worker: "improvement", name: "decision.json", content: marshalJSON(map[string]any{
					"decision":  "approved",
					"reason":    "",
					"updatedAt": "2026-05-20T09:58:30Z",
				})},
				{worker: "improvement", name: "issue_109_ロジック分割方針.md", content: "# ロジック分割方針\n\n- 条件分岐が増える処理は helper 関数へ分割する。\n- 既存の振る舞いを変えないことを先に確認する。\n", destination: "improvement-draft"},
				{worker: "improvement", name: "issue_109_ロジック分割方針.md", content: improvementApprovedDocument(repository, 109, "ロジック分割方針", prCommentAnalysisReadyJob.ID, []string{"implementation", "fix"}, "# ロジック分割方針\n\n- 条件分岐が増える処理は helper 関数へ分割する。\n- 既存の振る舞いを変えないことを先に確認する。\n", "2026-05-20T09:58:30Z"), destination: "improvement-approved"},
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
				{worker: "improvement", name: "input.md", content: "この件は一時的な失敗なので恒久改善は不要なら不要と判断してほしい。\n"},
				{worker: "improvement", name: "context.json", content: marshalJSON(map[string]any{
					"jobId":       failedJob.ID,
					"repository":  repository,
					"issueNumber": 103,
					"title":       failedJob.Title,
					"jobType":     string(failedJob.Type),
					"comment":     "この件は一時的な失敗なので恒久改善は不要なら不要と判断してほしい。",
					"createdAt":   "2026-05-20T09:28:30Z",
				})},
				{worker: "improvement", name: "decision.json", content: marshalJSON(map[string]any{
					"decision":  "no_improvement_needed",
					"reason":    "一時的なテスト失敗であり、継続利用する改善方針は不要です。",
					"updatedAt": "2026-05-20T09:29:00Z",
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
					"url":        "https://github.com/coco-papiyon/korobokcle/pull/107",
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
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/107#discussion_r1",
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
					"url":         "https://github.com/coco-papiyon/korobokcle/pull/107",
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
					"url":        "https://github.com/coco-papiyon/korobokcle/pull/107",
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
							"url":       "https://github.com/coco-papiyon/korobokcle/pull/107#issuecomment-1",
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

func writeRepositoryArtifacts(root string, artifactsDir string, workDirSetting string, repository string, issueNumber int, title string, files []artifactFile) error {
	workDir := artifacts.RepositoryWorkerWorkDir(root, artifactsDir, repository, workDirSetting)
	for _, file := range files {
		switch file.destination {
		case "improvement-draft":
			path := filepath.Join(artifacts.RepositoryWorkerImprovementDir(workDir, ".improvement"), file.name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
				return err
			}
			continue
		case "improvement-approved":
			path := filepath.Join(artifacts.RepositoryWorkerImprovementApprovedDir(workDir, ".improvements"), file.name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
				return err
			}
			continue
		}
		path := filepath.Join(artifacts.RepositoryWorkerJobPhaseDir(root, artifactsDir, repository, issueNumber, file.worker), file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
		if file.name == "result.md" && file.worker != artifacts.WorkerPR {
			workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, file.worker, issueNumber, title)
			if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(workingPath, []byte(file.content), 0o644); err != nil {
				return err
			}
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

func improvementApprovedDocument(repository string, issueNumber int, title string, jobID string, phases []string, body string, updatedAt string) string {
	if len(phases) == 0 {
		phases = []string{"design", "implementation"}
	}
	raw, err := yaml.Marshal(map[string]any{
		"id":        fmt.Sprintf("issue-%d", issueNumber),
		"title":     title,
		"scope":     "repository",
		"phases":    phases,
		"status":    "active",
		"updatedAt": updatedAt,
		"source": map[string]any{
			"jobId":       jobID,
			"issueNumber": issueNumber,
			"repository":  repository,
			"event":       "improvement_approved",
		},
	})
	if err != nil {
		fail(err)
	}
	return "---\n" + string(raw) + "---\n\n" + strings.TrimSpace(body) + "\n"
}

func writeReadme(root string) error {
	const body = `# Test Data

動作確認用の fixture です。` + "`KOROBOKCLE_TOOL_ROOT=test/data`" + ` を指定して起動すると、
この配下の ` + "`config/`" + `、` + "`data/`" + `、` + "`artifacts/`" + ` を使って UI を確認できます。
repository worker の作業ディレクトリは ` + "`artifacts/workers/coco-papiyon-korobokcle/work`" + `、成果物は ` + "`artifacts/workers/coco-papiyon-korobokcle/jobs/issue_<issue番号>/`" + ` 配下に出力されます。
作業ディレクトリには、AI の ` + "`result.md`" + ` を ` + "`design/issue_<issue番号>_...md`" + ` などの形式で複製しています。
改善機能も有効化済みで、承認前 draft は ` + "`work/.improvement/`" + `、承認済み方針は ` + "`work/.improvements/`" + ` で確認できます。

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

改善点画面用レコード:

` + `- ` + "`fixture-pr-created`" + `: ` + "`draft_created`" + ` の改善案
- ` + "`fixture-pr-comment-analysis-ready`" + `: ` + "`approved`" + ` の改善案
- ` + "`fixture-failed`" + `: ` + "`no_improvement_needed`" + ` の改善案
- 承認前 draft は ` + "`work/.improvement/*.md`" + `、承認済み方針は ` + "`work/.improvements/*.md`" + ` に生成済み
` + `

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
