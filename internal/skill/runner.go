package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Runner struct {
	defaultProviderName string
	root                string
}

type ExecutionConfig struct {
	Provider string
	Model    string
}

func NewRunner(root string, defaultProviderName string) *Runner {
	return &Runner{defaultProviderName: defaultProviderName, root: root}
}

func (r *Runner) Run(ctx context.Context, req AIRequest) (AIResult, error) {
	return AIResult{}, fmt.Errorf("direct runner execution is not supported")
}

func (r *Runner) RunDesign(ctx context.Context, skillName string, contextData DesignContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.root, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderPrompt(definition.PromptFile, contextData)
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

	outputPath := filepath.Join(contextData.ArtifactDir, definition.Artifacts.OutputFile)
	_ = os.Remove(outputPath)
	result, err := provider.Run(ctx, AIRequest{
		SkillName:   definition.Name,
		Prompt:      prompt,
		Model:       execution.Model,
		WorkDir:     contextData.ArtifactDir,
		ArtifactDir: contextData.ArtifactDir,
		OutputPath:  outputPath,
	})
	if err != nil {
		return AIResult{}, err
	}

	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	return result, nil
}

func (r *Runner) RunImplementation(ctx context.Context, skillName string, contextData ImplementationContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.root, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderPrompt(definition.PromptFile, contextData)
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

	outputPath := filepath.Join(contextData.ArtifactDir, definition.Artifacts.OutputFile)
	result, err := provider.Run(ctx, AIRequest{
		SkillName:   definition.Name,
		Prompt:      prompt,
		Model:       execution.Model,
		WorkDir:     contextData.ArtifactDir,
		ArtifactDir: contextData.ArtifactDir,
		OutputPath:  outputPath,
	})
	if err != nil {
		return AIResult{}, err
	}
	if err := writeOutputIfMissing(outputPath, result.Output); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	return result, nil
}

func (r *Runner) RunReview(ctx context.Context, skillName string, contextData ReviewContext, execution ExecutionConfig) (AIResult, error) {
	definition, err := LoadDefinition(r.root, skillName)
	if err != nil {
		return AIResult{}, err
	}

	if err := os.MkdirAll(contextData.ArtifactDir, 0o755); err != nil {
		return AIResult{}, err
	}

	prompt, err := RenderPrompt(definition.PromptFile, contextData)
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

	outputPath := filepath.Join(contextData.ArtifactDir, definition.Artifacts.OutputFile)
	result, err := provider.Run(ctx, AIRequest{
		SkillName:   definition.Name,
		Prompt:      prompt,
		Model:       execution.Model,
		WorkDir:     contextData.ArtifactDir,
		ArtifactDir: contextData.ArtifactDir,
		OutputPath:  outputPath,
	})
	if err != nil {
		return AIResult{}, err
	}

	if err := os.WriteFile(outputPath, []byte(result.Output), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stdout.log"), []byte(result.Stdout), 0o644); err != nil {
		return AIResult{}, err
	}
	if err := os.WriteFile(filepath.Join(contextData.ArtifactDir, "ai-stderr.log"), []byte(result.Stderr), 0o644); err != nil {
		return AIResult{}, err
	}
	return result, nil
}

func (r *Runner) providerForDefinition(definition Definition, execution ExecutionConfig) (AIProvider, error) {
	providerName := strings.TrimSpace(execution.Provider)
	if providerName == "" {
		providerName = strings.TrimSpace(r.defaultProviderName)
	}
	if providerName == "" {
		providerName = strings.TrimSpace(definition.Provider)
	}
	if providerName == "" {
		return nil, fmt.Errorf("skill provider is not configured")
	}
	return ProviderFor(providerName)
}

func writeOutputIfMissing(path string, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
