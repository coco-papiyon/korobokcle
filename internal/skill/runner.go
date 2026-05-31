package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Runner struct {
	defaultProviderName string
	repoRoot            string
	toolRoot            string
	copilotAllowTools   []string
	logger              *log.Logger
}

type ExecutionConfig struct {
	Provider string
	Model    string
}

const defaultSkillOutputFile = "result.md"

func NewRunner(repoRoot string, toolRoot string, defaultProviderName string, copilotAllowTools []string) *Runner {
	return &Runner{
		defaultProviderName: defaultProviderName,
		repoRoot:            repoRoot,
		toolRoot:            toolRoot,
		copilotAllowTools:   append([]string(nil), copilotAllowTools...),
	}
}

func (r *Runner) WithLogger(logger *log.Logger) *Runner {
	if r == nil {
		return nil
	}
	clone := *r
	clone.logger = logger
	return &clone
}

func (r *Runner) Run(ctx context.Context, req AIRequest) (AIResult, error) {
	return AIResult{}, fmt.Errorf("direct runner execution is not supported")
}

func (r *Runner) RunDesign(ctx context.Context, skillName string, contextData DesignContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.toolRoot, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderSkillPrompt(r.toolRoot, skillName, contextData)
	if err != nil {
		return AIResult{}, err
	}

	promptPath := filepath.Join(contextData.ArtifactDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return AIResult{}, err
	}

	rawContext, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "context.json"), rawContext, 0o644); err != nil {
		return AIResult{}, err
	}

	provider, err := r.providerForDefinition(definition, execution)
	if err != nil {
		return AIResult{}, err
	}
	workDir := r.executionWorkDir(definition, execution, contextData.ArtifactDir)
	outputPath := filepath.Join(contextData.ArtifactDir, defaultSkillOutputFile)
	runStart := time.Now()
	r.logExecutionStart("design", definition.Name, r.providerNameForDefinition(definition, execution), execution.Model, workDir, contextData.ArtifactDir, outputPath)

	result, err := provider.Run(ctx, AIRequest{
		SkillName:         definition.Name,
		Prompt:            prompt,
		Model:             execution.Model,
		WorkDir:           workDir,
		ArtifactDir:       contextData.ArtifactDir,
		OutputPath:        outputPath,
		CopilotAllowTools: r.copilotAllowTools,
	})
	if err != nil {
		r.logExecutionFinish("design", definition.Name, runStart, result, err)
		return AIResult{}, err
	}

	if err := persistSkillOutput(outputPath, result.Output); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	r.logExecutionFinish("design", definition.Name, runStart, result, nil)
	return result, nil
}

func (r *Runner) RunImplementation(ctx context.Context, skillName string, contextData ImplementationContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.toolRoot, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderSkillPrompt(r.toolRoot, skillName, contextData)
	if err != nil {
		return AIResult{}, err
	}

	promptPath := filepath.Join(contextData.ArtifactDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return AIResult{}, err
	}

	rawContext, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "context.json"), rawContext, 0o644); err != nil {
		return AIResult{}, err
	}

	provider, err := r.providerForDefinition(definition, execution)
	if err != nil {
		return AIResult{}, err
	}
	workDir := r.executionWorkDir(definition, execution, contextData.ArtifactDir)
	outputPath := filepath.Join(contextData.ArtifactDir, defaultSkillOutputFile)
	runStart := time.Now()
	r.logExecutionStart("implementation", definition.Name, r.providerNameForDefinition(definition, execution), execution.Model, workDir, contextData.ArtifactDir, outputPath)

	result, err := provider.Run(ctx, AIRequest{
		SkillName:         definition.Name,
		Prompt:            prompt,
		Model:             execution.Model,
		WorkDir:           workDir,
		ArtifactDir:       contextData.ArtifactDir,
		OutputPath:        outputPath,
		CopilotAllowTools: r.copilotAllowTools,
	})
	if err != nil {
		r.logExecutionFinish("implementation", definition.Name, runStart, result, err)
		return AIResult{}, err
	}

	if err := persistSkillOutput(outputPath, result.Output); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	r.logExecutionFinish("implementation", definition.Name, runStart, result, nil)
	return result, nil
}

func (r *Runner) RunReview(ctx context.Context, skillName string, contextData ReviewContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.toolRoot, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderSkillPrompt(r.toolRoot, skillName, contextData)
	if err != nil {
		return AIResult{}, err
	}

	promptPath := filepath.Join(contextData.ArtifactDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return AIResult{}, err
	}

	rawContext, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "context.json"), rawContext, 0o644); err != nil {
		return AIResult{}, err
	}

	provider, err := r.providerForDefinition(definition, execution)
	if err != nil {
		return AIResult{}, err
	}
	workDir := r.executionWorkDir(definition, execution, contextData.ArtifactDir)
	outputPath := filepath.Join(contextData.ArtifactDir, defaultSkillOutputFile)
	runStart := time.Now()
	r.logExecutionStart("review", definition.Name, r.providerNameForDefinition(definition, execution), execution.Model, workDir, contextData.ArtifactDir, outputPath)

	result, err := provider.Run(ctx, AIRequest{
		SkillName:         definition.Name,
		Prompt:            prompt,
		Model:             execution.Model,
		WorkDir:           workDir,
		ArtifactDir:       contextData.ArtifactDir,
		OutputPath:        outputPath,
		CopilotAllowTools: r.copilotAllowTools,
	})
	if err != nil {
		r.logExecutionFinish("review", definition.Name, runStart, result, err)
		return AIResult{}, err
	}

	if err := persistSkillOutput(outputPath, result.Output); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	r.logExecutionFinish("review", definition.Name, runStart, result, nil)
	return result, nil
}

func (r *Runner) providerForDefinition(definition Definition, execution ExecutionConfig) (AIProvider, error) {
	providerName := r.providerNameForDefinition(definition, execution)
	if providerName == "" {
		return nil, fmt.Errorf("skill provider is not configured")
	}
	return ProviderFor(providerName)
}

func (r *Runner) providerNameForDefinition(definition Definition, execution ExecutionConfig) string {
	providerName := strings.TrimSpace(execution.Provider)
	if providerName == "" {
		providerName = strings.TrimSpace(r.defaultProviderName)
	}
	if providerName == "" {
		return ""
	}
	return providerName
}

func (r *Runner) executionWorkDir(definition Definition, execution ExecutionConfig, artifactDir string) string {
	providerName := r.providerNameForDefinition(definition, execution)
	if strings.EqualFold(providerName, "codex") || strings.EqualFold(providerName, "copilot") {
		return r.repoRoot
	}
	return artifactDir
}

func persistSkillOutput(path string, output string) error {
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		return err
	}
	return nil
}

func (r *Runner) logExecutionStart(phase string, skillName string, provider string, model string, workDir string, artifactDir string, outputPath string) {
	if r == nil || r.logger == nil {
		return
	}
	r.logger.Printf("ai execution started phase=%s skill=%s provider=%s model=%s workdir=%s artifact_dir=%s output_path=%s", phase, skillName, strings.TrimSpace(provider), strings.TrimSpace(model), workDir, artifactDir, outputPath)
}

func (r *Runner) logExecutionFinish(phase string, skillName string, startedAt time.Time, result AIResult, runErr error) {
	if r == nil || r.logger == nil {
		return
	}
	status := "completed"
	if runErr != nil {
		status = "failed"
	}
	r.logger.Printf("ai execution %s phase=%s skill=%s duration=%s stdout_bytes=%d stderr_bytes=%d output_bytes=%d error=%v", status, phase, skillName, time.Since(startedAt).Round(time.Millisecond), len(result.Stdout), len(result.Stderr), len(result.Output), runErr)
}
