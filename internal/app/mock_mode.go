package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/web"
)

type FileMockJobSource struct {
	path   string
	logger workflowLogger
}

type MockSkillGenerator struct {
	baseDir string
}

func NewMockSkillGenerator(baseDir string) *MockSkillGenerator {
	return &MockSkillGenerator{baseDir: baseDir}
}

func (g *MockSkillGenerator) SkillStatus(context.Context) ([]domain.SkillStatus, error) {
	discovered, err := (&SkillGenerator{baseDir: g.baseDir}).discoverSkills()
	if err != nil {
		return nil, err
	}
	return buildSkillStatuses(g.baseDir, discovered, nil), nil
}

func (g *MockSkillGenerator) GenerateSkills(ctx context.Context, req domain.SkillGenerationRequest) (domain.SkillGenerationResult, error) {
	targets := make(map[domain.SkillPurpose]struct{}, len(req.ForcePurposes))
	for _, purpose := range req.ForcePurposes {
		targets[purpose] = struct{}{}
	}
	if len(targets) == 0 {
		statuses, err := g.SkillStatus(ctx)
		if err != nil {
			return domain.SkillGenerationResult{}, err
		}
		for _, status := range statuses {
			if !status.Exists {
				targets[status.Purpose] = struct{}{}
			}
		}
	}
	for _, definition := range issueDrivenSkillDefinitions {
		if _, ok := targets[definition.purpose]; !ok {
			continue
		}
		if err := g.writeMockSkill(definition, req); err != nil {
			return domain.SkillGenerationResult{}, err
		}
	}
	statuses, err := g.SkillStatus(ctx)
	if err != nil {
		return domain.SkillGenerationResult{}, err
	}
	return domain.SkillGenerationResult{
		Provider: domain.AIProviderCodex,
		Skills:   statuses,
		Message:  "モックモードでスキルを生成しました。",
	}, nil
}

func (g *MockSkillGenerator) writeMockSkill(definition skillDefinition, req domain.SkillGenerationRequest) error {
	dir := filepath.Join(g.baseDir, ".agents", "skills", definition.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	commands := strings.TrimSpace(req.TestCommand)
	if commands == "" {
		commands = "mock-mode"
	}
	content := strings.Join([]string{
		"---",
		"name: " + definition.name,
		"description: " + definition.displayName + "の出力形式を規定するモックスキル。",
		"---",
		"<!-- generated-by: korobokcle -->",
		"<!-- korobokcle-purpose: " + string(definition.purpose) + " -->",
		"",
		"## 必須出力形式",
		"概要、要件、設計、変更対象ファイル、テスト計画、リスク、変更内容、テスト結果、残課題、指摘事項、確認事項を必要な作業種別に応じて出力する。",
		"",
		"## テストコマンド",
		commands,
		"",
	}, "\n")
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}

func NewFileMockJobSource(path string, logger workflowLogger) *FileMockJobSource {
	return &FileMockJobSource{path: path, logger: logger}
}

func (s *FileMockJobSource) List(context.Context) ([]domain.Job, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read mock jobs: %w", err)
	}
	var jobs []domain.Job
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return nil, fmt.Errorf("decode mock jobs: %w", err)
	}
	if s.logger != nil {
		s.logger.Infof("mock source: loaded jobs=%d path=%s", len(jobs), s.path)
	}
	return jobs, nil
}

type MockWorkflowProcessor struct {
	store    JobStore
	feedback DesignFeedbackStore
	baseDir  string
	logger   workflowLogger
}

func NewMockWorkflowProcessorFactory(store JobStore, feedback DesignFeedbackStore, baseDir string, logger workflowLogger) WorkerProcessorFactory {
	return func() WorkerProcessor {
		return &MockWorkflowProcessor{store: store, feedback: feedback, baseDir: baseDir, logger: logger}
	}
}

func (p *MockWorkflowProcessor) Start(context.Context) error { return nil }

func (p *MockWorkflowProcessor) Stop(context.Context) error { return nil }

func (p *MockWorkflowProcessor) Process(ctx context.Context, job domain.Job) error {
	runningState := domain.RunningStateForKind(job.Kind, job.State)
	readyState := domain.ReadyStateForKind(job.Kind, job.State)
	if runningState == domain.StateFailed || readyState == domain.StateFailed {
		return fmt.Errorf("unsupported mock job kind: %s", job.Kind)
	}
	if !job.State.CanTransitionTo(runningState) && job.State != runningState {
		return fmt.Errorf("invalid mock transition: %s -> %s", job.State, runningState)
	}
	job = markJobState(job, runningState)
	if err := p.store.Upsert(ctx, job); err != nil {
		return err
	}
	artifactPath, err := mockArtifactPath(p.baseDir, job)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return fmt.Errorf("create mock artifact dir: %w", err)
	}
	if err := os.WriteFile(artifactPath, []byte(p.mockArtifact(ctx, job)), 0o644); err != nil {
		return fmt.Errorf("write mock artifact: %w", err)
	}
	diffPath, err := mockSourceDiffPath(p.baseDir, job)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(diffPath), 0o755); err != nil {
		return fmt.Errorf("create mock diff dir: %w", err)
	}
	if err := os.WriteFile(diffPath, []byte(p.mockSourceDiff(ctx, job, artifactPath)), 0o644); err != nil {
		return fmt.Errorf("write mock diff: %w", err)
	}
	job = markJobState(job, readyState)
	if err := p.store.Upsert(ctx, job); err != nil {
		return err
	}
	if p.logger != nil {
		p.logger.Infof("mock workflow complete job=%s state=%s artifact=%s diff=%s", job.ID, job.State, artifactPath, diffPath)
	}
	return nil
}

func (p *MockWorkflowProcessor) mockArtifact(ctx context.Context, job domain.Job) string {
	feedback := ""
	if p.feedback != nil {
		if content, ok, err := p.feedback.Load(ctx, job.ID); err == nil && ok {
			feedback = strings.TrimSpace(content)
		}
	}
	lines := []string{
		"# " + job.Title,
		"",
		"## 概要",
		fmt.Sprintf("モックモードで生成した %s の成果物です。", job.Kind),
		"",
		"## 変更内容",
		"- 実 AI は呼び出していません。",
		"- GitHub へのコメント、ラベル更新、PR 作成は行いません。",
		"",
		"## テスト結果",
		"- mock-mode: 成功",
		fmt.Sprintf("- generated_at: %s", time.Now().Format(time.RFC3339)),
		"",
		"## 残課題",
		"- 画面確認用のダミーデータです。",
	}
	if feedback != "" {
		lines = append(lines, "", "## ユーザコメント", feedback)
	}
	return strings.Join(lines, "\n")
}

type MockArtifactActionService struct {
	store    JobStore
	manager  *WorkerManager
	feedback DesignFeedbackStore
	baseDir  string
	monitor  RepositoryMonitor
}

func NewMockArtifactActionService(store JobStore, manager *WorkerManager, feedback DesignFeedbackStore, baseDir string, monitor RepositoryMonitor) *MockArtifactActionService {
	return &MockArtifactActionService{store: store, manager: manager, feedback: feedback, baseDir: baseDir, monitor: monitor}
}

func (s *MockArtifactActionService) GetArtifact(ctx context.Context, id string) (web.DesignArtifact, error) {
	job, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	if !ok {
		return web.DesignArtifact{}, fmt.Errorf("job not found")
	}
	path, err := mockArtifactPath(s.baseDir, job)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	return web.DesignArtifact{Content: string(raw), Path: path}, nil
}

func (s *MockArtifactActionService) GetSourceDiff(ctx context.Context, id string) (web.JobSourceDiff, error) {
	job, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return web.JobSourceDiff{}, err
	}
	if !ok {
		return web.JobSourceDiff{}, fmt.Errorf("job not found")
	}
	path, err := mockSourceDiffPath(s.baseDir, job)
	if err != nil {
		return web.JobSourceDiff{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			artifactPath, artifactErr := mockArtifactPath(s.baseDir, job)
			if artifactErr != nil {
				return web.JobSourceDiff{}, artifactErr
			}
			artifactRaw, artifactErr := os.ReadFile(artifactPath)
			if artifactErr != nil {
				return web.JobSourceDiff{}, artifactErr
			}
			content := s.mockSourceDiff(ctx, job, artifactPath)
			if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr == nil {
				raw = []byte(content)
			} else {
				raw = artifactRaw
			}
		} else {
			return web.JobSourceDiff{}, err
		}
	}
	return web.JobSourceDiff{
		Content: string(raw),
		Path:    jobSourceDiffTargetPath(job),
		BaseRef: "mock",
	}, nil
}

func (s *MockArtifactActionService) UpdateArtifact(ctx context.Context, id, content string) (web.DesignArtifact, error) {
	job, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	if !ok {
		return web.DesignArtifact{}, fmt.Errorf("job not found")
	}
	if !supportsArtifactEditing(job) {
		return web.DesignArtifact{}, fmt.Errorf("artifact editing is not supported for this job")
	}
	path, err := mockArtifactPath(s.baseDir, job)
	if err != nil {
		return web.DesignArtifact{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return web.DesignArtifact{}, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return web.DesignArtifact{}, err
	}
	return web.DesignArtifact{Content: content, Path: path}, nil
}

func (s *MockArtifactActionService) mockSourceDiff(ctx context.Context, job domain.Job, artifactPath string) string {
	lines := []string{
		"diff --git a/mock-source.txt b/mock-source.txt",
		"index 1111111..2222222 100644",
		"--- a/mock-source.txt",
		"+++ b/mock-source.txt",
		"@@ -1,14 +1,14 @@",
		" # " + job.Title,
		" ## Summary",
		"  context line 1",
		"  context line 2",
		"  context line 3",
		"  context line 4",
		"-This is a mock artifact.",
		"+This is a mock artifact for " + string(job.State) + ".",
		"  This line stays unchanged.",
		"  This line stays unchanged too.",
		" ## Changes",
		"-This artifact is generated as mock test data.",
		"+This artifact is generated as mock test data for UI testing.",
		"  This line stays unchanged.",
		"  This line stays unchanged too.",
		"  This line stays unchanged three.",
		"  This line stays unchanged four.",
	}
	return strings.Join(lines, "\n") + "\n"
}

func (s *MockArtifactActionService) ApproveArtifact(ctx context.Context, id, userComment string) (domain.Job, error) {
	return s.completeReadyJob(ctx, id)
}

func (s *MockArtifactActionService) RequestChanges(ctx context.Context, id, userComment string) (domain.Job, error) {
	return s.completeReadyJob(ctx, id)
}

func (s *MockArtifactActionService) RerunArtifact(ctx context.Context, id, userComment string) (domain.Job, error) {
	job, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("job not found")
	}
	runningState := rerunRunningState(job)
	if runningState == domain.StateFailed {
		return domain.Job{}, fmt.Errorf("job is not ready for rerun")
	}
	if s.feedback != nil {
		if err := s.feedback.Save(ctx, job.ID, userComment); err != nil {
			return domain.Job{}, err
		}
	}
	job.ErrorMessage = ""
	job = markJobState(job, runningState)
	if err := s.store.Upsert(ctx, job); err != nil {
		return domain.Job{}, err
	}
	if s.manager != nil {
		if err := s.manager.Submit(job); err != nil {
			return domain.Job{}, err
		}
	} else if s.monitor != nil {
		if err := s.monitor.PollNow(ctx); err != nil {
			return domain.Job{}, err
		}
	}
	return job, nil
}

func (s *MockArtifactActionService) completeReadyJob(ctx context.Context, id string) (domain.Job, error) {
	job, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("job not found")
	}
	if !isReadyState(job.State) {
		return domain.Job{}, fmt.Errorf("job is not ready for approval")
	}
	job = markJobState(job, domain.StateCompleted)
	if err := s.store.Upsert(ctx, job); err != nil {
		return domain.Job{}, err
	}
	if s.feedback != nil {
		if err := s.feedback.Delete(ctx, job.ID); err != nil {
			return domain.Job{}, err
		}
	}
	if s.monitor != nil {
		if err := s.monitor.PollNow(ctx); err != nil {
			return domain.Job{}, err
		}
	}
	return job, nil
}

func mockArtifactPath(baseDir string, job domain.Job) (string, error) {
	if artifactSubdir(job) == "" {
		return "", fmt.Errorf("job is not supported")
	}
	return filepath.Join(baseDir, ".workspace", artifactSubdir(job), fmt.Sprintf("%d_%s.md", job.Number, sanitizePart(job.Title))), nil
}

func mockSourceDiffPath(baseDir string, job domain.Job) (string, error) {
	if artifactSubdir(job) == "" {
		return "", fmt.Errorf("job is not supported")
	}
	return filepath.Join(baseDir, ".workspace", artifactSubdir(job), fmt.Sprintf("%d_%s.diff", job.Number, sanitizePart(job.Title))), nil
}

func (p *MockWorkflowProcessor) mockSourceDiff(ctx context.Context, job domain.Job, artifactPath string) string {
	lines := []string{
		"diff --git a/mock-source.txt b/mock-source.txt",
		"index 1111111..2222222 100644",
		"--- a/mock-source.txt",
		"+++ b/mock-source.txt",
		"@@ -1,7 +1,7 @@",
		" # " + job.Title,
		" ## Summary",
		"-This is a mock artifact.",
		"+This is a mock artifact for " + string(job.State) + ".",
		" ## Changes",
		"-This artifact is generated as mock test data.",
		"+This artifact is generated as mock test data for UI testing.",
		"  This line stays unchanged.",
		" ## Test Results",
	}
	return strings.Join(lines, "\n") + "\n"
}
