package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalCLIProviderReadsStdout(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "stdout-provider.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %1\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `["{{prompt}}"]`)

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "hello-from-arg",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "hello-from-arg") {
		t.Fatalf("expected stdout output, got %q", result.Output)
	}
}

func TestExternalCLIProviderReadsOutputFileFallback(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "file-provider.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\n>\"%1\" echo %~2\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	outputPath := filepath.Join(dir, "result.md")
	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `["{{output_path}}","{{prompt}}"]`)

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "file fallback output",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  outputPath,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(result.Output) != "file fallback output" {
		t.Fatalf("expected file output, got %q", result.Output)
	}
}

func TestExternalCLIProviderExpandsPromptArgument(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo-args.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %1\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `["{{prompt}}"]`)

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "argument-output",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "argument-output") {
		t.Fatalf("expected argument output, got %q", result.Output)
	}
}

func TestCodexCLIProviderSendsPromptViaStdinByDefault(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "stdin-provider.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\nset /p INPUT=\r\necho %INPUT%\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", "")

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "stdin-output",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "stdin-output") {
		t.Fatalf("expected stdin output, got %q", result.Output)
	}
}

func TestExternalCLIProviderExpandsModelArgument(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo-model.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %1 %2 %3\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `["suggest","{{model_flag}}","{{model}}","{{prompt}}"]`)

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "argument-output",
		Model:       "gpt-4.1",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "--model") || !strings.Contains(result.Output, "gpt-4.1") {
		t.Fatalf("expected model arguments, got %q", result.Output)
	}
}

func TestCopilotCLIProviderUsesAutomationFlagsByDefault(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo-copilot.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %*\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_COPILOT_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_COPILOT_ARGS_JSON", "")

	provider := NewCopilotCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:            "automation prompt",
		Model:             "gpt-4.5-mini",
		WorkDir:           dir,
		ArtifactDir:       dir,
		OutputPath:        filepath.Join(dir, "out.txt"),
		CopilotAllowTools: []string{"write", "shell(go:*)", "shell(git:*)"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "-p") {
		t.Fatalf("expected -p flag, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "--model") || !strings.Contains(result.Output, "gpt-4.5-mini") {
		t.Fatalf("expected model flags, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "--no-ask-user") {
		t.Fatalf("expected --no-ask-user flag, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "--allow-tool=write,shell(go:*),shell(git:*)") {
		t.Fatalf("expected --allow-tool flag, got %q", result.Output)
	}
}
