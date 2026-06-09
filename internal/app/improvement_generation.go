package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

const (
	improvementDecisionDraftCreated        = "draft_created"
	improvementDecisionNoImprovementNeeded = "no_improvement_needed"

	improvementSourceDesignRejected      = "design_rejected"
	improvementSourceFinalRejected       = "final_rejected"
	improvementSourcePRCommentAnalysis   = "pr_comment_analysis_ready"
	improvementSourceDesignRerun         = "design_rerun_requested"
	improvementSourceImplementationRerun = "implementation_rerun_requested"
	improvementSourcePRRerun             = "pr_rerun_requested"
	improvementSourceReviewRerun         = "review_rerun_requested"
)

type improvementSourceInput struct {
	EventType string `json:"eventType"`
	Comment   string `json:"comment"`
	Author    string `json:"author,omitempty"`
	URL       string `json:"url,omitempty"`
}

type improvementContextData struct {
	JobID                string                     `json:"jobId"`
	Repository           string                     `json:"repository"`
	IssueNumber          int                        `json:"issueNumber"`
	Title                string                     `json:"title"`
	Source               improvementSourceInput     `json:"source"`
	Phases               []string                   `json:"phases"`
	RelatedJobIDs        []string                   `json:"relatedJobIds"`
	ExistingImprovements []improvementExistingEntry `json:"existingImprovements"`
	GeneratedAt          string                     `json:"generatedAt"`
}

type improvementExistingEntry struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Path  string   `json:"path"`
	Phase []string `json:"phases"`
}

type improvementDecision struct {
	Decision    string `json:"decision"`
	Reason      string `json:"reason,omitempty"`
	SourceEvent string `json:"sourceEvent"`
	UpdatedAt   string `json:"updatedAt"`
}

type improvementGenerationResult struct {
	Decision string
	Reason   string
}

type improvementDraftResult struct {
	Draft string
	Notes string
}

type improvementPromptContext struct {
	JobID                 string                     `json:"jobId"`
	Repository            string                     `json:"repository"`
	IssueNumber           int                        `json:"issueNumber"`
	Title                 string                     `json:"title"`
	Source                improvementSourceInput     `json:"source"`
	Phases                []string                   `json:"phases"`
	ExistingImprovements  []improvementExistingEntry `json:"existingImprovements"`
	InputMarkdown         string                     `json:"inputMarkdown"`
	CurrentResultMarkdown string                     `json:"currentResultMarkdown,omitempty"`
}

func generateImprovementDraft(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, jobID string, sourceEventType string, logger *log.Logger) (improvementGenerationResult, error) {
	job, events, err := orch.JobDetail(ctx, jobID)
	if err != nil {
		return improvementGenerationResult{}, err
	}

	repositoryConfig, ok := resolveMonitoredRepository(cfg, job.Repository)
	if !ok || !repositoryConfig.ImprovementEnabled {
		return improvementGenerationResult{Decision: improvementDecisionNoImprovementNeeded, Reason: "improvement feature is disabled"}, nil
	}

	source, err := resolveImprovementSource(events, sourceEventType)
	if err != nil {
		return improvementGenerationResult{}, err
	}

	improvementWorkspaceDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(cfg.Root(), cfg.App().ArtifactsDir, job.Repository)
	workFiles := repositoryImprovementWorkFiles(improvementWorkspaceDir, repositoryConfig.ImprovementWorkDir)
	artifactFiles := repositoryImprovementArtifactFiles(cfg.Root(), cfg.App().ArtifactsDir, job.Repository, job.GitHubNumber)

	if err := os.MkdirAll(workFiles.DraftDir, 0o755); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := os.MkdirAll(workFiles.Dir, 0o755); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := os.MkdirAll(artifactFiles.Dir, 0o755); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := os.MkdirAll(artifactFiles.DraftDir, 0o755); err != nil {
		return improvementGenerationResult{}, err
	}

	existingImprovements, err := loadExistingImprovements(improvementWorkspaceDir, repositoryConfig.ImprovementDir)
	if err != nil {
		return improvementGenerationResult{}, err
	}

	phases := inferImprovementPhases(source.EventType)
	contextData := improvementContextData{
		JobID:                job.ID,
		Repository:           job.Repository,
		IssueNumber:          job.GitHubNumber,
		Title:                job.Title,
		Source:               source,
		Phases:               phases,
		RelatedJobIDs:        []string{job.ID},
		ExistingImprovements: existingImprovements,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}

	inputMarkdown := buildImprovementInputMarkdown(job, source, phases)
	if err := writeImprovementFile(workFiles.InputPath, []byte(inputMarkdown)); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := writeImprovementFile(artifactFiles.InputPath, []byte(inputMarkdown)); err != nil {
		return improvementGenerationResult{}, err
	}

	contextRaw, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return improvementGenerationResult{}, err
	}
	if err := writeImprovementFile(workFiles.ContextPath, contextRaw); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := writeImprovementFile(artifactFiles.ContextPath, contextRaw); err != nil {
		return improvementGenerationResult{}, err
	}

	comment := strings.TrimSpace(source.Comment)
	if comment == "" {
		decision := improvementDecision{
			Decision:    improvementDecisionNoImprovementNeeded,
			Reason:      "source comment is empty",
			SourceEvent: source.EventType,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if err := writeImprovementDecisionFiles(workFiles, artifactFiles, decision); err != nil {
			return improvementGenerationResult{}, err
		}
		if logger != nil {
			logger.Printf("improvement draft skipped job_id=%s source=%s reason=%s", job.ID, source.EventType, decision.Reason)
		}
		return improvementGenerationResult{Decision: decision.Decision, Reason: decision.Reason}, nil
	}

	currentResultMarkdown, err := loadCurrentImprovementResult(comment, workFiles, artifactFiles)
	if err != nil {
		return improvementGenerationResult{}, err
	}

	draftResult, err := generateImprovementDraftContent(ctx, cfg, job, repositoryConfig, source, phases, existingImprovements, inputMarkdown, currentResultMarkdown, improvementWorkspaceDir, artifactFiles, logger)
	if err != nil {
		return improvementGenerationResult{}, err
	}
	draft := draftResult.Draft
	if err := writeImprovementFile(workFiles.DraftPath, []byte(draft)); err != nil {
		return improvementGenerationResult{}, err
	}
	if err := writeImprovementFile(artifactFiles.DraftPath, []byte(draft)); err != nil {
		return improvementGenerationResult{}, err
	}
	if strings.TrimSpace(draftResult.Notes) != "" {
		if err := writeImprovementFile(workFiles.NotesPath, []byte(draftResult.Notes)); err != nil {
			return improvementGenerationResult{}, err
		}
		if err := writeImprovementFile(artifactFiles.NotesPath, []byte(draftResult.Notes)); err != nil {
			return improvementGenerationResult{}, err
		}
	}

	decision := improvementDecision{
		Decision:    improvementDecisionDraftCreated,
		SourceEvent: source.EventType,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeImprovementDecisionFiles(workFiles, artifactFiles, decision); err != nil {
		return improvementGenerationResult{}, err
	}
	if logger != nil {
		logger.Printf("improvement draft created job_id=%s source=%s draft=%s", job.ID, source.EventType, workFiles.DraftPath)
	}
	return improvementGenerationResult{Decision: decision.Decision}, nil
}

func generateImprovementDraftContent(
	ctx context.Context,
	cfg *config.Service,
	job domain.Job,
	repositoryConfig config.MonitoredRepository,
	source improvementSourceInput,
	phases []string,
	existingImprovements []improvementExistingEntry,
	inputMarkdown string,
	currentResultMarkdown string,
	workDir string,
	artifactFiles improvementArtifactFiles,
	logger *log.Logger,
) (improvementDraftResult, error) {
	aiResult, err := generateImprovementDraftWithAI(ctx, cfg, job, source, phases, existingImprovements, inputMarkdown, currentResultMarkdown, workDir, artifactFiles)
	if err == nil {
		if logger != nil {
			logger.Printf("improvement draft generated by provider job_id=%s source=%s", job.ID, source.EventType)
		}
		return aiResult, nil
	}
	if logger != nil {
		logger.Printf("improvement draft provider generation failed job_id=%s source=%s error=%v", job.ID, source.EventType, err)
	}
	fallbackDraft := buildImprovementDraft(job, source, phases)
	fallbackNotes := buildImprovementFallbackNotes(job, repositoryConfig, source, err)
	return improvementDraftResult{
		Draft: fallbackDraft,
		Notes: fallbackNotes,
	}, nil
}

func generateImprovementDraftWithAI(
	ctx context.Context,
	cfg *config.Service,
	job domain.Job,
	source improvementSourceInput,
	phases []string,
	existingImprovements []improvementExistingEntry,
	inputMarkdown string,
	currentResultMarkdown string,
	workDir string,
	artifactFiles improvementArtifactFiles,
) (improvementDraftResult, error) {
	execution, err := resolveExecutionConfig(cfg, job.WatchRuleID)
	if err != nil {
		return improvementDraftResult{}, err
	}
	promptContext := improvementPromptContext{
		JobID:                 job.ID,
		Repository:            job.Repository,
		IssueNumber:           job.GitHubNumber,
		Title:                 job.Title,
		Source:                source,
		Phases:                phases,
		ExistingImprovements:  existingImprovements,
		InputMarkdown:         inputMarkdown,
		CurrentResultMarkdown: currentResultMarkdown,
	}
	prompt, err := skill.RenderSkillPrompt(cfg.Root(), "default/improvement_consideration", promptContext)
	if err != nil {
		return improvementDraftResult{}, err
	}
	generationDir := filepath.Join(artifactFiles.Dir, "generation")
	if err := os.MkdirAll(generationDir, 0o755); err != nil {
		return improvementDraftResult{}, err
	}
	promptPath := filepath.Join(generationDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return improvementDraftResult{}, err
	}
	rawContext, err := json.MarshalIndent(promptContext, "", "  ")
	if err != nil {
		return improvementDraftResult{}, err
	}
	if err := os.WriteFile(filepath.Join(generationDir, "context.json"), rawContext, 0o644); err != nil {
		return improvementDraftResult{}, err
	}

	provider, err := skill.ProviderFor(execution.Provider)
	if err != nil {
		return improvementDraftResult{}, err
	}
	request := skill.AIRequest{
		SkillName:         "improvement_consideration",
		Prompt:            prompt,
		Model:             execution.Model,
		WorkDir:           workDir,
		ArtifactDir:       generationDir,
		OutputPath:        filepath.Join(generationDir, "result.md"),
		CopilotAllowTools: cfg.App().CopilotAllowTools,
	}
	result, err := provider.Run(ctx, request)
	if err != nil {
		return improvementDraftResult{}, err
	}
	if err := os.WriteFile(filepath.Join(generationDir, "stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return improvementDraftResult{}, err
	}
	if err := os.WriteFile(filepath.Join(generationDir, "stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return improvementDraftResult{}, err
	}
	draft := strings.TrimSpace(result.Output)
	if draft == "" {
		return improvementDraftResult{}, fmt.Errorf("provider returned empty improvement draft")
	}
	return improvementDraftResult{
		Draft: draft + "\n",
		Notes: buildImprovementAINotes(job, source, execution),
	}, nil
}

func resolveMonitoredRepository(cfg *config.Service, repository string) (config.MonitoredRepository, bool) {
	for _, monitored := range cfg.App().MonitoredRepositories {
		if !repositoryMatches(repository, monitored.Repository) {
			continue
		}
		return monitored, true
	}
	return config.MonitoredRepository{}, false
}

func resolveImprovementSource(events []domain.Event, requestedEventType string) (improvementSourceInput, error) {
	for i := len(events) - 1; i >= 0; i-- {
		if requestedEventType != "" && events[i].EventType != requestedEventType {
			continue
		}
		if source, ok := improvementSourceFromEvent(events[i]); ok {
			return source, nil
		}
	}
	return improvementSourceInput{}, fmt.Errorf("improvement source event %q not found", requestedEventType)
}

func improvementSourceFromEvent(event domain.Event) (improvementSourceInput, bool) {
	switch event.EventType {
	case improvementSourceDesignRejected, improvementSourceFinalRejected, improvementSourceDesignRerun, improvementSourceImplementationRerun, improvementSourcePRRerun, improvementSourceReviewRerun:
		var payload struct {
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return improvementSourceInput{}, false
		}
		return improvementSourceInput{
			EventType: event.EventType,
			Comment:   strings.TrimSpace(payload.Comment),
		}, true
	case improvementSourcePRCommentAnalysis:
		var payload struct {
			Comment struct {
				Author string `json:"author"`
				Body   string `json:"body"`
				URL    string `json:"url"`
			} `json:"comment"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return improvementSourceInput{}, false
		}
		return improvementSourceInput{
			EventType: event.EventType,
			Comment:   strings.TrimSpace(payload.Comment.Body),
			Author:    strings.TrimSpace(payload.Comment.Author),
			URL:       strings.TrimSpace(payload.Comment.URL),
		}, true
	}
	return improvementSourceInput{}, false
}

func inferImprovementPhases(sourceEventType string) []string {
	switch sourceEventType {
	case improvementSourceDesignRejected, improvementSourceDesignRerun:
		return []string{"design"}
	case improvementSourceFinalRejected, improvementSourcePRCommentAnalysis, improvementSourceImplementationRerun:
		return []string{"implementation", "fix"}
	case improvementSourceReviewRerun:
		return []string{"review"}
	case improvementSourcePRRerun:
		return []string{"implementation"}
	default:
		return []string{"implementation"}
	}
}

func loadExistingImprovements(workDir string, configuredDir string) ([]improvementExistingEntry, error) {
	improvementsDir := artifacts.RepositoryWorkerImprovementsDir(workDir, configuredDir)
	entries, err := filepath.Glob(filepath.Join(improvementsDir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	out := make([]improvementExistingEntry, 0, len(entries))
	for _, entry := range entries {
		raw, err := os.ReadFile(entry)
		if err != nil {
			return nil, err
		}
		document, err := ParseImprovementMarkdown(raw)
		if err != nil {
			continue
		}
		out = append(out, improvementExistingEntry{
			ID:    document.FrontMatter.ID,
			Title: document.FrontMatter.Title,
			Path:  entry,
			Phase: append([]string(nil), document.FrontMatter.Phases...),
		})
	}
	return out, nil
}

func buildImprovementInputMarkdown(job domain.Job, source improvementSourceInput, phases []string) string {
	var lines []string
	lines = append(lines, "# 改善案入力")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- job: %s", job.ID))
	lines = append(lines, fmt.Sprintf("- repository: %s", job.Repository))
	lines = append(lines, fmt.Sprintf("- issueNumber: %d", job.GitHubNumber))
	lines = append(lines, fmt.Sprintf("- sourceEvent: %s", source.EventType))
	lines = append(lines, fmt.Sprintf("- phases: %s", strings.Join(phases, ", ")))
	if source.Author != "" {
		lines = append(lines, fmt.Sprintf("- author: %s", source.Author))
	}
	if source.URL != "" {
		lines = append(lines, fmt.Sprintf("- url: %s", source.URL))
	}
	lines = append(lines, "")
	lines = append(lines, "## 元コメント")
	lines = append(lines, "")
	lines = append(lines, strings.TrimSpace(source.Comment))
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func loadCurrentImprovementResult(comment string, workFiles improvementWorkFiles, artifactFiles improvementArtifactFiles) (string, error) {
	if strings.TrimSpace(comment) == "" {
		return "", nil
	}
	if raw, err := os.ReadFile(workFiles.DraftPath); err == nil {
		trimmed := strings.TrimSpace(string(raw))
		if trimmed != "" {
			return trimmed, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if raw, err := os.ReadFile(artifactFiles.DraftPath); err == nil {
		trimmed := strings.TrimSpace(string(raw))
		if trimmed != "" {
			return trimmed, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return "", nil
}

func buildImprovementDraft(job domain.Job, source improvementSourceInput, phases []string) string {
	comment := strings.TrimSpace(source.Comment)
	title := improvementDraftTitle(source.EventType, job.Title)
	category := improvementDraftCategory(phases, comment)
	return strings.TrimSpace(fmt.Sprintf(`# 改善方針案

## タイトル

%s

## 適用対象

- repository: %s
- phases: %s
- sourceEvent: %s

## 改善項目

- %s
  - %s

## 汎化メモ

- 元コメントの意図を維持したまま、1 回限りの指示ではなく継続適用できる改善項目に言い換える。
- 具体例ではなく、同種の作業で再利用できる表現を優先する。

## 元コメント

%s
`, title, job.Repository, strings.Join(phases, ", "), source.EventType, category, comment, comment)) + "\n"
}

func buildImprovementAINotes(job domain.Job, source improvementSourceInput, execution skill.ExecutionConfig) string {
	return strings.TrimSpace(fmt.Sprintf(`# 生成メモ

- mode: ai
- job: %s
- sourceEvent: %s
- provider: %s
- model: %s
`, job.ID, source.EventType, execution.Provider, firstNonEmpty(execution.Model, "(default)"))) + "\n"
}

func buildImprovementFallbackNotes(job domain.Job, repository config.MonitoredRepository, source improvementSourceInput, generationErr error) string {
	return strings.TrimSpace(fmt.Sprintf(`# 生成メモ

- mode: fallback
- job: %s
- sourceEvent: %s
- repository: %s
- reason: %s
`, job.ID, source.EventType, repository.Repository, generationErr.Error())) + "\n"
}

func improvementDraftTitle(sourceEventType string, fallbackTitle string) string {
	switch sourceEventType {
	case improvementSourceDesignRejected:
		return "設計差し戻しから抽出した改善方針"
	case improvementSourceFinalRejected:
		return "最終差し戻しから抽出した改善方針"
	case improvementSourcePRCommentAnalysis:
		return "PR コメント分析から抽出した改善方針"
	default:
		if strings.TrimSpace(fallbackTitle) != "" {
			return fallbackTitle
		}
		return "改善方針案"
	}
}

func improvementDraftCategory(phases []string, comment string) string {
	trimmed := strings.TrimSpace(comment)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(trimmed, "画面") || strings.Contains(trimmed, "配置") || strings.Contains(trimmed, "レイアウト") || strings.Contains(trimmed, "ボタン") || strings.Contains(lower, "layout"):
		return "画面レイアウト方針"
	}
	for _, phase := range phases {
		switch strings.TrimSpace(strings.ToLower(phase)) {
		case "design":
			return "設計方針"
		case "review":
			return "レビュー方針"
		case "implementation", "fix":
			return "実装方針"
		}
	}
	return "改善方針"
}

func writeImprovementDecisionFiles(workFiles improvementWorkFiles, artifactFiles improvementArtifactFiles, decision improvementDecision) error {
	raw, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		return err
	}
	if err := writeImprovementFile(artifactFiles.DecisionPath, raw); err != nil {
		return err
	}
	return writeImprovementFile(workFiles.DecisionPath, raw)
}

func writeImprovementFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
