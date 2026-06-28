package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type workflowLogger interface {
	Infof(string, ...any)
	Debugf(string, ...any)
}

type WorkflowProcessor struct {
	store    JobStore
	settings SettingsStore
	baseDir  string
	toolDir  string
	logger   workflowLogger
}

func NewWorkflowProcessor(store JobStore, settings SettingsStore, baseDir, toolDir string, logger workflowLogger) JobProcessor {
	processor := &WorkflowProcessor{
		store:    store,
		settings: settings,
		baseDir:  baseDir,
		toolDir:  toolDir,
		logger:   logger,
	}
	return processor.Process
}

func (p *WorkflowProcessor) Process(ctx context.Context, job domain.Job) error {
	runningState := domain.RunningStateForKind(job.Kind, job.State)
	readyState := domain.ReadyStateForKind(job.Kind, job.State)
	if runningState == domain.StateFailed || readyState == domain.StateFailed {
		return fmt.Errorf("unsupported job kind for workflow: %s", job.Kind)
	}

	if p.logger != nil {
		p.logger.Infof("workflow start job=%s kind=%s state=%s", job.ID, job.Kind, job.State)
		p.logger.Debugf("workflow job detail id=%s repository=%s number=%d title=%q", job.ID, job.Repository, job.Number, job.Title)
	}

	updated, err := p.transitionState(ctx, job, runningState)
	if err != nil {
		return err
	}
	job = updated

	settings, err := p.loadSettings(ctx)
	if err != nil {
		return err
	}

	artifactPath, err := p.artifactPath(job)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	content := p.renderArtifact(job, settings, runningState, readyState)
	if err := os.WriteFile(artifactPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}

	if p.logger != nil {
		p.logger.Debugf("workflow artifact written job=%s path=%s", job.ID, artifactPath)
	}

	job, err = p.transitionState(ctx, job, readyState)
	if err != nil {
		return err
	}

	if p.logger != nil {
		p.logger.Infof("workflow complete job=%s state=%s", job.ID, job.State)
	}
	return nil
}

func (p *WorkflowProcessor) transitionState(ctx context.Context, job domain.Job, next domain.JobState) (domain.Job, error) {
	if next == "" {
		return job, nil
	}
	if job.State != next && !job.State.CanTransitionTo(next) {
		return domain.Job{}, fmt.Errorf("invalid workflow transition: %s -> %s", job.State, next)
	}
	job.State = next
	if p.store == nil {
		return job, nil
	}
	if err := p.store.Upsert(ctx, job); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (p *WorkflowProcessor) loadSettings(ctx context.Context) (domain.WatchSettings, error) {
	if p.settings == nil {
		return domain.WatchSettings{}, nil
	}
	settings, err := p.settings.Load(ctx)
	if err != nil {
		return domain.WatchSettings{}, err
	}
	return domain.NormalizeWatchSettings(settings), nil
}

func (p *WorkflowProcessor) artifactPath(job domain.Job) (string, error) {
	dir := artifactSubdir(job)
	if dir == "" {
		return "", fmt.Errorf("unsupported job kind: %s", job.Kind)
	}
	return filepath.Join(p.baseDir, ".workspace", dir, fmt.Sprintf("%d_%s.md", job.Number, sanitizePart(job.Title))), nil
}

func (p *WorkflowProcessor) renderArtifact(job domain.Job, settings domain.WatchSettings, runningState, readyState domain.JobState) string {
	provider := settings.AIProvider.DisplayName()
	model := selectedModel(settings, providerKey(settings.AIProvider))
	phase := artifactSubdir(job)
	lines := []string{
		fmt.Sprintf("# %s", job.Title),
		"",
		fmt.Sprintf("- Job ID: %s", job.ID),
		fmt.Sprintf("- Kind: %s", job.Kind),
		fmt.Sprintf("- Repository: %s", job.Repository),
		fmt.Sprintf("- Number: #%d", job.Number),
		fmt.Sprintf("- Phase: %s", phase),
		fmt.Sprintf("- Provider: %s", provider),
		fmt.Sprintf("- Model: %s", model),
		fmt.Sprintf("- Running State: %s", runningState),
		fmt.Sprintf("- Ready State: %s", readyState),
		fmt.Sprintf("- Generated At: %s", time.Now().Format(time.RFC3339)),
		"",
		"## Output",
		"- ここに実際の AI 実行結果を保存する。",
	}
	return strings.Join(lines, "\n")
}

func providerKey(provider domain.AIProvider) string {
	switch provider {
	case domain.AIProviderGitHubCopilot:
		return "githubCopilot"
	default:
		return "codex"
	}
}

func selectedModel(settings domain.WatchSettings, key string) string {
	var selection domain.ModelSelection
	switch key {
	case "githubCopilot":
		selection = settings.Models.GitHubCopilot
	default:
		selection = settings.Models.Codex
	}
	if selection.Mode == domain.ModelModeCustom && strings.TrimSpace(selection.Value) != "" {
		return selection.Value
	}
	return "default"
}

func artifactSubdir(job domain.Job) string {
	switch job.Kind {
	case domain.JobKindIssueDesign:
		return "design"
	case domain.JobKindIssueImplementation:
		return "implementation"
	case domain.JobKindPRReview:
		return "review"
	case domain.JobKindPRFeedback:
		return "review_fix_design"
	default:
		return ""
	}
}

func sanitizePart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "#", "-", ".", "-", ",", "-", "(", "-", ")", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	return value
}
