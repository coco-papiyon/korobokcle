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

	outputPath := filepath.Join(dir, "summary.md")
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
