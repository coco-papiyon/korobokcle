package skill

import (
	"context"
	"os"
	"os/exec"
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

func TestExternalCLIProviderOmitsDefaultModelArgument(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo-args.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %*\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", `["{{model_flag}}","{{model}}","{{prompt}}"]`)

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "prompt-output",
		Model:       "default",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(result.Output, "--model") {
		t.Fatalf("expected model flag to be omitted, got %q", result.Output)
	}
	if strings.Contains(result.Output, "default") {
		t.Fatalf("expected default model value to be omitted, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "prompt-output") {
		t.Fatalf("expected prompt to remain, got %q", result.Output)
	}
}

func TestModelFlagTreatsDefaultAsEmpty(t *testing.T) {
	t.Parallel()

	if got := modelFlag("default"); got != "" {
		t.Fatalf("expected default model to be omitted, got %q", got)
	}
	if got := modelFlag(" Default "); got != "" {
		t.Fatalf("expected default model to be omitted, got %q", got)
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
		Prompt:      "automation prompt",
		Model:       "gpt-4.5-mini",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
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
	if !strings.Contains(result.Output, "--allow-all-tools") {
		t.Fatalf("expected --allow-all-tools flag, got %q", result.Output)
	}
	if strings.Contains(result.Output, "--allow-tool=") {
		t.Fatalf("expected no allow-tool list when default is permissive, got %q", result.Output)
	}
}

func TestCopilotCLIProviderWritesDebugLogsWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "echo-copilot.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo off\r\necho %*\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("KOROBOKCLE_COPILOT_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_COPILOT_ARGS_JSON", "")
	t.Setenv("KOROBOKCLE_COPILOT_DEBUG", "1")

	provider := NewCopilotCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:            "debug prompt",
		Model:             "gpt-4.5-mini",
		WorkDir:           dir,
		ArtifactDir:       dir,
		OutputPath:        filepath.Join(dir, "out.txt"),
		CopilotAllowTools: []string{"read", "write", "shell"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Stderr, "[debug] provider=copilot") {
		t.Fatalf("expected debug logs in stderr, got %q", result.Stderr)
	}
	if !strings.Contains(result.Stderr, "prompt_in_args=true") {
		t.Fatalf("expected prompt_in_args in debug logs, got %q", result.Stderr)
	}
	if !strings.Contains(result.Stderr, "prompt=debug prompt") {
		t.Fatalf("expected prompt contents in debug logs, got %q", result.Stderr)
	}
}

func TestCopilotCLIProviderRunsGoTestCommandWithRealCopilot(t *testing.T) {
	if strings.TrimSpace(os.Getenv("KOROBOKCLE_RUN_REAL_COPILOT")) == "" {
		t.Skip("set KOROBOKCLE_RUN_REAL_COPILOT=1 to run the real copilot integration test")
	}
	if _, err := exec.LookPath("copilot"); err != nil {
		t.Skipf("copilot binary is not available: %v", err)
	}

	root := t.TempDir()
	moduleDir := filepath.Join(root, "module")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(module) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte("module example.com/copilot-go-test\n\ngo 1.22.5\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "hello.go"), []byte("package hello\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(hello.go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "hello_test.go"), []byte("package hello\n\nimport \"testing\"\n\nfunc TestAddFails(t *testing.T) {\n\tt.Fatal(\"boom\")\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(hello_test.go) error = %v", err)
	}

	t.Setenv("KOROBOKCLE_COPILOT_BIN", "copilot")
	t.Setenv("KOROBOKCLE_COPILOT_ARGS_JSON", "")
	t.Setenv("KOROBOKCLE_COPILOT_DEBUG", "1")

	provider := NewCopilotCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:            "Run go test ./... in the current repository and report the failure output.",
		Model:             "default",
		WorkDir:           moduleDir,
		ArtifactDir:       moduleDir,
		OutputPath:        filepath.Join(moduleDir, "out.txt"),
		CopilotAllowTools: []string{"read", "write", "shell"},
	})
	if result.Output == "" && result.Stdout == "" && result.Stderr == "" && err == nil {
		t.Fatalf("expected copilot to produce output or an error")
	}
	if !strings.Contains(result.Stderr, "[debug] provider=copilot") {
		t.Fatalf("expected debug logs in stderr, got %q", result.Stderr)
	}
	if !strings.Contains(result.Stderr, "prompt=Run go test ./...") {
		t.Fatalf("expected prompt contents in stderr, got %q", result.Stderr)
	}
	t.Logf("copilot err=%v", err)
	t.Logf("copilot stdout=%s", result.Stdout)
	t.Logf("copilot stderr=%s", result.Stderr)
}
