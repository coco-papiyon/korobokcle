package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	feedback DesignFeedbackStore
	baseDir  string
	toolDir  string
	logger   workflowLogger
	runner   AIRunner
	verifier AIRunner
	contexts JobContextLoader
}

type managedAIRunner interface {
	Start(context.Context, domain.AIProvider, string) error
	Stop(context.Context) error
}

func NewWorkflowProcessorFactory(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger) WorkerProcessorFactory {
	return func() WorkerProcessor {
		return newWorkflowProcessorWithVerifier(store, settings, feedback, baseDir, toolDir, logger, NewHTTPAIRunner(nil, logger), NewHTTPAIRunner(nil, logger), &GitHubJobContextLoader{})
	}
}

func NewWorkflowProcessor(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger) JobProcessor {
	processor := newWorkflowProcessorWithVerifier(store, settings, feedback, baseDir, toolDir, logger, NewHTTPAIRunner(nil, logger), NewHTTPAIRunner(nil, logger), &GitHubJobContextLoader{})
	return processor.Process
}

func NewWorkflowProcessorWithLoopDeps(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger, implementer, verifier AIRunner, contexts JobContextLoader) JobProcessor {
	processor := newWorkflowProcessorWithVerifier(store, settings, feedback, baseDir, toolDir, logger, implementer, verifier, contexts)
	return processor.Process
}

func NewWorkflowProcessorWithDeps(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger, runner AIRunner, contexts JobContextLoader) JobProcessor {
	processor := newWorkflowProcessor(store, settings, feedback, baseDir, toolDir, logger, runner, contexts)
	return processor.Process
}

func newWorkflowProcessorWithVerifier(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger, runner, verifier AIRunner, contexts JobContextLoader) *WorkflowProcessor {
	processor := newWorkflowProcessor(store, settings, feedback, baseDir, toolDir, logger, runner, contexts)
	processor.verifier = verifier
	return processor
}

func newWorkflowProcessor(store JobStore, settings SettingsStore, feedback DesignFeedbackStore, baseDir, toolDir string, logger workflowLogger, runner AIRunner, contexts JobContextLoader) *WorkflowProcessor {
	return &WorkflowProcessor{
		store:    store,
		settings: settings,
		feedback: feedback,
		baseDir:  baseDir,
		toolDir:  toolDir,
		logger:   logger,
		runner:   runner,
		contexts: contexts,
	}
}

func (p *WorkflowProcessor) Start(ctx context.Context) error {
	runner, ok := p.runner.(managedAIRunner)
	if !ok {
		return nil
	}
	settings, err := p.loadSettings(ctx)
	if err != nil {
		return err
	}
	if err := runner.Start(ctx, settings.AIProvider, p.baseDir); err != nil {
		return err
	}
	verifier, ok := p.verifier.(managedAIRunner)
	if !ok {
		return nil
	}
	if err := verifier.Start(ctx, verifierProviderForSettings(settings), p.baseDir); err != nil {
		_ = runner.Stop(context.Background())
		return fmt.Errorf("start verifier: %w", err)
	}
	return nil
}

func (p *WorkflowProcessor) Stop(ctx context.Context) error {
	runner, ok := p.runner.(managedAIRunner)
	if !ok {
		return nil
	}
	implementerErr := runner.Stop(ctx)
	var verifierErr error
	if verifier, ok := p.verifier.(managedAIRunner); ok {
		verifierErr = verifier.Stop(ctx)
	}
	return errors.Join(implementerErr, verifierErr)
}

func (p *WorkflowProcessor) Process(ctx context.Context, job domain.Job) (retErr error) {
	runningState := domain.RunningStateForKind(job.Kind, job.State)
	readyState := domain.ReadyStateForKind(job.Kind, job.State)
	if runningState == domain.StateFailed || readyState == domain.StateFailed {
		return fmt.Errorf("unsupported job kind for workflow: %s", job.Kind)
	}

	if p.logger != nil {
		p.logger.Infof("workflow start job=%s kind=%s state=%s", job.ID, job.Kind, job.State)
		p.logger.Debugf("workflow job detail id=%s repository=%s number=%d title=%q", job.ID, job.Repository, job.Number, job.Title)
	}

	job.ErrorMessage = ""
	job.FailedFromState = ""
	updated, err := p.transitionState(ctx, job, runningState)
	if err != nil {
		return err
	}
	job = updated
	defer func() {
		if retErr == nil || p.store == nil {
			return
		}
		job.FailedFromState = job.State
		job = markJobSubStatus(job, "")
		job = markJobState(job, domain.StateFailed)
		job.ErrorMessage = retErr.Error()
		if err := p.store.Upsert(context.Background(), job); err != nil && p.logger != nil {
			p.logger.Infof("persist workflow failure failed job=%s error=%v", job.ID, err)
		}
	}()

	settings, err := p.loadSettings(ctx)
	if err != nil {
		return err
	}
	feedback, _ := p.loadFeedback(ctx, job.ID)

	artifactPath, err := p.artifactPath(job)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	workDir, branch, err := p.workDirForJob(ctx, job, settings)
	if err != nil {
		return err
	}
	contextText, err := p.loadJobContext(ctx, job)
	if err != nil {
		return err
	}
	if strings.TrimSpace(job.IssueContext) == "" && strings.TrimSpace(contextText) != "" && p.store != nil {
		job.IssueContext = contextText
		if err := p.store.Upsert(ctx, job); err != nil {
			return err
		}
	}
	content, err := p.runAI(ctx, job, settings, feedback, contextText, workDir, branch, runningState, readyState)
	if err != nil {
		return err
	}
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
	job = markJobState(job, next)
	if p.store == nil {
		return job, nil
	}
	if err := p.store.Upsert(ctx, job); err != nil {
		return domain.Job{}, err
	}
	return job, nil
}

func (p *WorkflowProcessor) persistJob(ctx context.Context, job domain.Job) error {
	if p.store == nil {
		return nil
	}
	return p.store.Upsert(ctx, job)
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

func (p *WorkflowProcessor) loadJobContext(ctx context.Context, job domain.Job) (string, error) {
	if strings.TrimSpace(job.IssueContext) != "" {
		return job.IssueContext, nil
	}
	if p.contexts == nil {
		return "", nil
	}
	return p.contexts.Load(ctx, job)
}

func (p *WorkflowProcessor) loadFeedback(ctx context.Context, jobID string) (string, bool) {
	if p.feedback == nil {
		return "", false
	}
	content, ok, err := p.feedback.Load(ctx, jobID)
	if err != nil || !ok {
		return "", false
	}
	return content, true
}

func (p *WorkflowProcessor) runAI(ctx context.Context, job domain.Job, settings domain.WatchSettings, feedback string, contextText string, workDir string, branch string, runningState, readyState domain.JobState) (string, error) {
	if job.Kind == domain.JobKindIssueImplementation && p.verifier != nil {
		return p.runImplementationLoop(ctx, job, settings, feedback, contextText, workDir, branch, runningState, readyState)
	}
	return p.runSingleAI(ctx, job, settings, feedback, contextText, workDir, branch, runningState, readyState, 1, "agent")
}

func (p *WorkflowProcessor) runSingleAI(ctx context.Context, job domain.Job, settings domain.WatchSettings, feedback string, contextText string, workDir string, branch string, runningState, readyState domain.JobState, attempt int, role string) (string, error) {
	if p.runner == nil {
		return "", fmt.Errorf("AI runner is not configured")
	}
	stdoutLog, stderrLog, err := p.openAIProcessLogs(job, attempt, role)
	if err != nil {
		return "", err
	}
	defer stdoutLog.Close()
	defer stderrLog.Close()

	provider, model := resolveJobAISelectionForRole(settings, job, role)
	prompt := p.buildPrompt(job, settings, feedback, contextText, workDir, branch, runningState, readyState)
	req := AIRequest{
		Provider:        provider,
		Model:           model,
		System:          systemPromptForJob(job),
		Prompt:          prompt,
		WorkingDir:      workDir,
		ExpectPatch:     implementationJob(job),
		Stdout:          stdoutLog,
		Stderr:          stderrLog,
		AllowedCommands: settings.AIAllowedCommands,
	}
	p.appendIssueAILog(job, attempt, role, "request", strings.Join([]string{
		fmt.Sprintf("provider: %s", provider),
		fmt.Sprintf("model: %s", displayModel(model)),
		fmt.Sprintf("working_dir: %s", workDir),
		fmt.Sprintf("branch: %s", branch),
		"",
		"[system]",
		req.System,
		"",
		"[prompt]",
		prompt,
	}, "\n"))
	response, err := p.runner.Run(ctx, req)
	if err != nil {
		if parseErr, ok := err.(*AIResponseParseError); ok {
			p.appendIssueAILog(job, attempt, role, "response_error", strings.Join([]string{
				fmt.Sprintf("error: %s", parseErr.Error()),
				"",
				"[raw_response]",
				parseErr.RawOutput,
			}, "\n"))
		} else {
			p.appendIssueAILog(job, attempt, role, "response_error", fmt.Sprintf("error: %v", err))
		}
		return "", err
	}
	p.appendIssueAILog(job, attempt, role, "response", strings.Join([]string{
		"[artifact_markdown]",
		response.ArtifactMarkdown,
		"",
		"[git_diff]",
		response.GitDiff,
		"",
		"[raw_output]",
		response.RawOutput,
	}, "\n"))
	if implementationJob(job) && strings.TrimSpace(response.GitDiff) != "" {
		if err := p.applyGitDiff(ctx, workDir, response.GitDiff); err != nil {
			p.appendIssueAILog(job, attempt, role, "apply_diff_error", fmt.Sprintf("error: %v\n\n[git_diff]\n%s", err, response.GitDiff))
			return "", err
		}
	}
	return p.decorateArtifact(job, response.ArtifactMarkdown), nil
}

type implementationVerification struct {
	Status   string `json:"status"`
	Feedback string `json:"feedback"`
	Summary  string `json:"summary"`
}

func (p *WorkflowProcessor) runImplementationLoop(ctx context.Context, job domain.Job, settings domain.WatchSettings, feedback, contextText, workDir, branch string, runningState, readyState domain.JobState) (string, error) {
	maxAttempts := settings.ImplementationLoopCount
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	loopFeedback := strings.TrimSpace(feedback)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		job = markJobSubStatus(job, fmt.Sprintf("実装(%d回目)", attempt))
		if err := p.persistJob(ctx, job); err != nil {
			return "", err
		}
		attemptFeedback := loopFeedback
		if attempt > 1 {
			attemptFeedback = strings.TrimSpace(strings.Join([]string{
				attemptFeedback,
				fmt.Sprintf("検証者エージェントの指摘を反映する再実装です（%d/%d回目）。", attempt, maxAttempts),
			}, "\n\n"))
		}
		artifact, err := p.runSingleAI(ctx, job, settings, attemptFeedback, contextText, workDir, branch, runningState, readyState, attempt, "agent")
		if err != nil {
			return "", fmt.Errorf("implementation attempt %d: %w", attempt, err)
		}
		job = markJobSubStatus(job, fmt.Sprintf("検証(%d回目)", attempt))
		if err := p.persistJob(ctx, job); err != nil {
			return "", err
		}
		verification, err := p.verifyImplementation(ctx, job, settings, contextText, workDir, branch, attempt, maxAttempts, artifact)
		if err != nil {
			return "", fmt.Errorf("verification attempt %d: %w", attempt, err)
		}
		p.appendIssueAILog(job, attempt, "verifier", "verification", fmt.Sprintf("status: %s\nfeedback: %s\nsummary: %s", verification.Status, verification.Feedback, verification.Summary))
		if verification.Status == "passed" {
			return strings.TrimSpace(artifact + "\n\n## 検証者エージェントの判定\n" + verification.Summary), nil
		}
		loopFeedback = strings.TrimSpace(verification.Feedback)
		if loopFeedback == "" {
			loopFeedback = strings.TrimSpace(verification.Summary)
		}
	}
	return "", fmt.Errorf("implementation verification did not pass after %d attempts: %s", maxAttempts, loopFeedback)
}

func (p *WorkflowProcessor) verifyImplementation(ctx context.Context, job domain.Job, settings domain.WatchSettings, contextText, workDir, branch string, attempt, maxAttempts int, implementationArtifact string) (implementationVerification, error) {
	stdoutLog, stderrLog, err := p.openAIRoleProcessLogs(job, attempt, "verifier")
	if err != nil {
		return implementationVerification{}, err
	}
	defer stdoutLog.Close()
	defer stderrLog.Close()
	provider, model := resolveJobAISelectionForRole(settings, job, "verifier")
	prompt := strings.Join([]string{
		fmt.Sprintf("job_id: %s", job.ID),
		fmt.Sprintf("attempt: %d/%d", attempt, maxAttempts),
		fmt.Sprintf("working_dir: %s", workDir),
		fmt.Sprintf("branch: %s", branch),
		"",
		"GitHub context:", strings.TrimSpace(contextText),
		"",
		"Implementation agent summary:", strings.TrimSpace(implementationArtifact),
		"",
		"Inspect the current changes in working_dir and run the tests required by the design and repository instructions.",
		"Do not edit, create, delete, or format repository files. Your role is verification only.",
		"Return only one JSON object: {\"status\":\"passed|changes_requested\",\"feedback\":\"specific instructions for the implementer\",\"summary\":\"Japanese verification summary\"}.",
		"Use passed only when the implementation and required tests are acceptable.",
	}, "\n")
	response, err := p.verifier.Run(ctx, AIRequest{
		Provider: provider, Model: model,
		System: "You are an independent software verification agent. Inspect and test the implementation without modifying the repository.",
		Prompt: prompt, WorkingDir: workDir, Stdout: stdoutLog, Stderr: stderrLog,
		AllowedCommands: settings.AIAllowedCommands,
	})
	if err != nil {
		return implementationVerification{}, err
	}
	return parseImplementationVerification(response.RawOutput)
}

func parseImplementationVerification(raw string) (implementationVerification, error) {
	cleaned := strings.TrimSpace(stripCodeFence(raw, "json"))
	if extracted, ok := extractFirstJSONObject(cleaned); ok {
		cleaned = extracted
	}
	var result implementationVerification
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return result, fmt.Errorf("decode verifier response: %w", err)
	}
	result.Status = strings.ToLower(strings.TrimSpace(result.Status))
	if result.Status != "passed" && result.Status != "changes_requested" {
		return result, fmt.Errorf("invalid verifier status %q", result.Status)
	}
	if strings.TrimSpace(result.Summary) == "" {
		return result, fmt.Errorf("verifier summary is empty")
	}
	return result, nil
}

func (p *WorkflowProcessor) openAIProcessLogs(job domain.Job, attempt int, role string) (*os.File, *os.File, error) {
	return p.openAIRoleProcessLogs(job, attempt, role)
}

func (p *WorkflowProcessor) openAIRoleProcessLogs(job domain.Job, attempt int, role string) (*os.File, *os.File, error) {
	logDir := jobLogDir(p.toolDir, job)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create AI process log dir: %w", err)
	}
	prefix := jobLogFilePrefix(job, attempt, role)
	stdoutLog, err := os.OpenFile(filepath.Join(logDir, prefix+"_stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open AI stdout log: %w", err)
	}
	stderrLog, err := os.OpenFile(filepath.Join(logDir, prefix+"_stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = stdoutLog.Close()
		return nil, nil, fmt.Errorf("open AI stderr log: %w", err)
	}
	return stdoutLog, stderrLog, nil
}

func (p *WorkflowProcessor) buildPrompt(job domain.Job, settings domain.WatchSettings, feedback string, contextText string, workDir string, branch string, runningState, readyState domain.JobState) string {
	phase := artifactSubdir(job)
	provider, model := resolveJobAISelectionForRole(settings, job, "agent")
	lines := []string{
		fmt.Sprintf("phase: %s", phase),
		fmt.Sprintf("job_id: %s", job.ID),
		fmt.Sprintf("job_kind: %s", job.Kind),
		fmt.Sprintf("repository: %s", job.Repository),
		fmt.Sprintf("number: %d", job.Number),
		fmt.Sprintf("title: %s", job.Title),
		fmt.Sprintf("provider: %s", provider),
		fmt.Sprintf("model: %s", displayModel(model)),
		fmt.Sprintf("running_state: %s", runningState),
		fmt.Sprintf("ready_state: %s", readyState),
		fmt.Sprintf("working_dir: %s", workDir),
	}
	if branch != "" {
		lines = append(lines, fmt.Sprintf("branch: %s", branch))
	}
	if skillName := skillNameForJob(job); skillName != "" {
		if skill := p.loadSkillInstructions(workDir, skillName); skill != "" {
			lines = append(lines,
				"",
				fmt.Sprintf("Mandatory Agent Skill instructions (%s):", skillName),
				skill,
				"",
				"Follow all processing steps and the required output format above.",
				"Do not return progress updates as the final response. Complete the work and return only the required final Markdown.",
			)
		}
	}
	lines = append(lines,
		"",
		"GitHub context:",
		strings.TrimSpace(contextText),
	)

	if implementationJob(job) {
		lines = append(lines,
			"",
			"All repository file reads, edits, and commands must use working_dir as the repository root.",
			"Do not access the original repository root or any path outside working_dir.",
			"Use paths relative to working_dir whenever possible.",
		)
		if job.Kind == domain.JobKindPRConflict {
			lines = append(lines,
				"",
				"Resolve the merge conflicts directly in working_dir.",
				"Keep the intent of both issues and branches in mind while editing.",
			)
		}
		designPath := p.relatedDesignPath(job)
		if designPath != "" {
			if raw, err := os.ReadFile(designPath); err == nil {
				lines = append(lines, "", "Existing design artifact:", string(raw))
			}
		}
		lines = append(lines,
			"",
			"Repository files:",
			p.repoFileList(workDir),
			"",
			"Implement the requested changes directly in working_dir.",
			"Run appropriate tests or checks after editing.",
			"Return only a Markdown summary in Japanese. Do not return JSON or a git diff.",
		)
	} else {
		lines = append(lines,
			"",
			"Return only Markdown in Japanese.",
		)
	}

	if strings.TrimSpace(feedback) != "" {
		lines = append(lines, "", "User comment:", strings.TrimSpace(feedback))
	}
	return strings.Join(lines, "\n")
}

func skillNameForJob(job domain.Job) string {
	switch job.Kind {
	case domain.JobKindIssueDesign:
		return "design-from-issue"
	case domain.JobKindIssueImplementation:
		return "implement-from-design"
	case domain.JobKindPRReview:
		return "review-pull-request"
	case domain.JobKindPRFeedback:
		return "review-comment-fix"
	case domain.JobKindPRConflict:
		return "resolve-pr-conflicts"
	default:
		return ""
	}
}

func (p *WorkflowProcessor) loadSkillInstructions(workDir, skillName string) string {
	for _, root := range []string{workDir, p.baseDir} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		for _, parent := range []string{".agents", ".github"} {
			path := filepath.Join(root, parent, "skills", skillName, "SKILL.md")
			raw, err := os.ReadFile(path)
			if err == nil && strings.TrimSpace(string(raw)) != "" {
				return strings.TrimSpace(string(raw))
			}
		}
	}
	return ""
}

func (p *WorkflowProcessor) decorateArtifact(job domain.Job, artifact string) string {
	return strings.Join([]string{
		fmt.Sprintf("# %s", job.Title),
		"",
		stripLeadingH1(artifact),
	}, "\n")
}

func stripLeadingH1(artifact string) string {
	trimmed := strings.TrimSpace(artifact)
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || !strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(lines[1:], "\n"))
}

func systemPromptForJob(job domain.Job) string {
	if job.Kind == domain.JobKindPRConflict {
		return "You are an autonomous software engineer. Resolve merge conflicts carefully, preserve both issue intents when possible, and report the result in concise Japanese Markdown."
	}
	if implementationJob(job) {
		return "You are an autonomous software engineer. Follow the repository instructions with minimal extra process. Edit the repository directly and report the result in concise Japanese Markdown."
	}
	return "You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown."
}

func implementationJob(job domain.Job) bool {
	return job.Kind == domain.JobKindIssueImplementation || job.Kind == domain.JobKindPRConflict || (job.Kind == domain.JobKindPRFeedback && (job.State == domain.StatePRReviewComment || job.State == domain.StateReviewFixImplementationRunning || job.State == domain.StateReviewFixImplementationReady || job.State == domain.StateReviewFixImplementationApproved || job.State == domain.StateReviewFixDesignApproved))
}

func (p *WorkflowProcessor) workDirForJob(ctx context.Context, job domain.Job, settings domain.WatchSettings) (string, string, error) {
	if !implementationJob(job) {
		return p.baseDir, "", nil
	}
	branch := renderBranchName(settings.BranchNamePattern, job.Number)
	baseBranch := ""
	if job.Kind == domain.JobKindPRConflict {
		var err error
		branch, baseBranch, err = pullRequestBranches(ctx, job)
		if err != nil {
			return "", "", err
		}
	}
	worktreeBranch := branch
	worktreePath := implementationWorktreePath(p.toolDir, job)
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", "", fmt.Errorf("create worktree parent: %w", err)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err == nil {
		if job.Kind == domain.JobKindPRConflict && mergeInProgress(ctx, worktreePath) {
			return worktreePath, branch, nil
		}
		currentBranchName, currentErr := currentBranch(ctx, worktreePath)
		if currentErr == nil && strings.TrimSpace(currentBranchName) != "" {
			dirty, dirtyErr := gitHasChanges(ctx, worktreePath)
			if dirtyErr != nil {
				return "", "", dirtyErr
			}
			if dirty {
				if p.logger != nil {
					p.logger.Infof("workflow reuse dirty worktree job=%s path=%s branch=%s", job.ID, worktreePath, currentBranchName)
				}
				return worktreePath, currentBranchName, nil
			}
			if err := syncBranchFromRemote(ctx, worktreePath, currentBranchName); err != nil {
				return "", "", err
			}
			if job.Kind == domain.JobKindPRConflict {
				if err := prepareConflictMerge(ctx, worktreePath, baseBranch); err != nil {
					return "", "", err
				}
			}
			return worktreePath, currentBranchName, nil
		}
		if err := syncBranchFromRemote(ctx, worktreePath, worktreeBranch); err != nil {
			return "", "", err
		}
		return worktreePath, worktreeBranch, nil
	}
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		if pruneErr := runGit(ctx, p.baseDir, "worktree", "prune"); pruneErr != nil {
			return "", "", fmt.Errorf("prune stale worktrees: %w", pruneErr)
		}
	}
	if err := addImplementationWorktree(ctx, p.baseDir, worktreeBranch, worktreePath); err != nil {
		if !strings.Contains(err.Error(), "already used by worktree") {
			return "", "", fmt.Errorf("create worktree: %w", err)
		}
		worktreeBranch = implementationWorktreeBranchName(branch, job)
		if retryErr := addImplementationWorktree(ctx, p.baseDir, worktreeBranch, worktreePath); retryErr != nil {
			return "", "", fmt.Errorf("create worktree: %w", retryErr)
		}
	}
	if err := syncBranchFromRemote(ctx, worktreePath, worktreeBranch); err != nil {
		return "", "", err
	}
	if job.Kind == domain.JobKindPRConflict {
		if err := prepareConflictMerge(ctx, worktreePath, baseBranch); err != nil {
			return "", "", err
		}
	}
	return worktreePath, worktreeBranch, nil
}

func pullRequestBranches(ctx context.Context, job domain.Job) (string, string, error) {
	raw, err := runGHJSON(ctx, "pr", "view", "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "headRefName,baseRefName")
	if err != nil {
		return "", "", err
	}
	var refs struct {
		Head string `json:"headRefName"`
		Base string `json:"baseRefName"`
	}
	if err := json.Unmarshal(raw, &refs); err != nil {
		return "", "", fmt.Errorf("decode PR branches: %w", err)
	}
	refs.Head = strings.TrimSpace(refs.Head)
	refs.Base = strings.TrimSpace(refs.Base)
	if refs.Head == "" || refs.Base == "" {
		return "", "", fmt.Errorf("PR #%d is missing head or base branch", job.Number)
	}
	return refs.Head, refs.Base, nil
}

func prepareConflictMerge(ctx context.Context, repoDir, baseBranch string) error {
	if mergeInProgress(ctx, repoDir) {
		return nil
	}
	if err := runGit(ctx, repoDir, "fetch", "origin", baseBranch); err != nil {
		return fmt.Errorf("fetch PR base branch: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "merge", "--no-edit", "origin/"+baseBranch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if mergeInProgress(ctx, repoDir) {
		return nil
	}
	return fmt.Errorf("merge PR base branch: %w: %s", err, strings.TrimSpace(string(out)))
}

func mergeInProgress(ctx context.Context, repoDir string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	return cmd.Run() == nil
}

func syncBranchFromRemote(ctx context.Context, repoDir, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	hasOrigin, err := hasRemote(ctx, repoDir, "origin")
	if err != nil {
		return err
	}
	if !hasOrigin {
		return nil
	}
	exists, err := remoteBranchExists(ctx, repoDir, branch)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := runGit(ctx, repoDir, "pull", "--rebase", "origin", branch); err != nil {
		return fmt.Errorf("rebase remote branch before implementation: %w", err)
	}
	return nil
}

func hasRemote(ctx context.Context, repoDir, remote string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "remote", "get-url", remote)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return false, nil
	}
	return false, fmt.Errorf("git remote get-url %s: %w: %s", remote, err, strings.TrimSpace(string(out)))
}

func addImplementationWorktree(ctx context.Context, baseDir, branch, worktreePath string) error {
	err := runGit(ctx, baseDir, "worktree", "add", "-B", branch, worktreePath, "HEAD")
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "missing but already registered worktree") {
		return err
	}
	if pruneErr := runGit(ctx, baseDir, "worktree", "prune"); pruneErr != nil {
		return fmt.Errorf("%w; prune stale worktree: %v", err, pruneErr)
	}
	return runGit(ctx, baseDir, "worktree", "add", "-B", branch, worktreePath, "HEAD")
}

func (p *WorkflowProcessor) applyGitDiff(ctx context.Context, workDir string, diff string) error {
	diffPath := filepath.Join(os.TempDir(), "korobokcle-"+sanitizePart(filepath.Base(workDir))+".diff")
	if err := os.WriteFile(diffPath, []byte(diff), 0o644); err != nil {
		return fmt.Errorf("write diff file: %w", err)
	}
	defer os.Remove(diffPath)
	if err := runGit(ctx, workDir, "apply", "--index", "--reject", "--whitespace=nowarn", diffPath); err != nil {
		return fmt.Errorf("apply AI diff: %w", err)
	}
	return nil
}

func (p *WorkflowProcessor) relatedDesignPath(job domain.Job) string {
	fileName := fmt.Sprintf("%d_%s.md", job.Number, sanitizePart(job.Title))
	switch job.Kind {
	case domain.JobKindIssueImplementation:
		return filepath.Join(p.baseDir, ".workspace", "design", fileName)
	default:
		return ""
	}
}

func (p *WorkflowProcessor) repoFileList(workDir string) string {
	cmd := exec.Command("git", "-C", workDir, "ls-files")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "(failed to list repository files)"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 200 {
		lines = lines[:200]
	}
	return strings.Join(lines, "\n")
}

func (p *WorkflowProcessor) appendIssueAILog(job domain.Job, attempt int, role string, section string, content string) {
	logDir := jobLogDir(p.toolDir, job)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return
	}
	logPath := filepath.Join(logDir, jobLogFilePrefix(job, attempt, role)+".log")
	entry := strings.Join([]string{
		fmt.Sprintf("=== %s attempt=%d role=%s section=%s job=%s kind=%s state=%s ===", time.Now().Format(time.RFC3339), attempt, role, section, job.ID, job.Kind, job.State),
		content,
		"",
	}, "\n")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(entry)
}

func jobLogFilePrefix(job domain.Job, attempt int, role string) string {
	name := artifactSubdir(job)
	if strings.TrimSpace(name) == "" {
		name = "job"
	}
	if attempt > 0 {
		name = fmt.Sprintf("%s_attempt-%d", name, attempt)
	}
	if strings.TrimSpace(role) != "" {
		name += "_" + sanitizePart(role)
	}
	return name
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
	return ""
}

func displayModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "default"
	}
	return model
}

func artifactSubdir(job domain.Job) string {
	switch job.Kind {
	case domain.JobKindIssueDesign:
		return "design"
	case domain.JobKindIssueImplementation:
		return "implementation"
	case domain.JobKindPRConflict:
		return "pr_conflict"
	case domain.JobKindPRReview:
		return "review"
	case domain.JobKindPRFeedback:
		return "review_fix_implementation"
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
