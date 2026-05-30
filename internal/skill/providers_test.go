package skill

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExternalCLIProviderReadsStdout(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeProviderScript(t, dir, "stdout-provider", "@echo off\r\necho %1\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$1\"\n")

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

func TestCodexCLIProviderUsesWritableSandboxByDefault(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeProviderScript(t, dir, "echo-codex", "@echo off\r\necho %*\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$*\"\n")

	t.Setenv("KOROBOKCLE_CODEX_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_CODEX_ARGS_JSON", "")

	provider := NewCodexCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "codex prompt",
		Model:       "default",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  filepath.Join(dir, "out.txt"),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "--sandbox") || !strings.Contains(result.Output, "workspace-write") {
		t.Fatalf("expected writable sandbox flags, got %q", result.Output)
	}
	if strings.Contains(result.Output, "--ask-for-approval") {
		t.Fatalf("unexpected approval flag in %q", result.Output)
	}
}

func TestExternalCLIProviderReadsOutputFileFallback(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeProviderScript(t, dir, "file-provider", "@echo off\r\n>\"%1\" echo %~2\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$2\" > \"$1\"\n")

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
	scriptPath := writeProviderScript(t, dir, "echo-args", "@echo off\r\necho %1\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$1\"\n")

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
	scriptPath := writeProviderScript(t, dir, "stdin-provider", "@echo off\r\nset /p INPUT=\r\necho %INPUT%\r\n", "#!/usr/bin/env sh\nIFS= read -r input\nprintf '%s\\n' \"$input\"\n")

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
	scriptPath := writeProviderScript(t, dir, "echo-model", "@echo off\r\necho %1 %2 %3\r\n", "#!/usr/bin/env sh\nprintf '%s %s %s\\n' \"$1\" \"$2\" \"$3\"\n")

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
	scriptPath := writeProviderScript(t, dir, "echo-args", "@echo off\r\necho %*\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$*\"\n")

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
	scriptPath := writeProviderScript(t, dir, "echo-copilot", "@echo off\r\necho %*\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$*\"\n")

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
	if !strings.Contains(result.Output, "Read the instructions in") || !strings.Contains(result.Output, "prompt.md") {
		t.Fatalf("expected prompt to reference artifact prompt.md, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "Write the final response to") || !strings.Contains(result.Output, "out.txt") {
		t.Fatalf("expected prompt to reference output file, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "--add-dir") || !strings.Contains(result.Output, dir) {
		t.Fatalf("expected add-dir for artifact dir, got %q", result.Output)
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
	if strings.Contains(result.Output, "automation prompt") {
		t.Fatalf("expected request prompt to stay out of copilot args, got %q", result.Output)
	}
}

func TestCopilotCLIProviderPrefersOutputFileOverStdout(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeProviderScript(
		t,
		dir,
		"copilot-file-output",
		"@echo off\r\necho thinking out loud\r\n>\"%1\" echo final artifact\r\n",
		"#!/usr/bin/env sh\nprintf 'thinking out loud\\n'\nprintf '%s\\n' 'final artifact' > \"$1\"\n",
	)

	outputPath := filepath.Join(dir, "result.md")
	t.Setenv("KOROBOKCLE_COPILOT_BIN", scriptPath)
	t.Setenv("KOROBOKCLE_COPILOT_ARGS_JSON", `["{{output_path}}"]`)

	provider := NewCopilotCLIProvider()
	result, err := provider.Run(context.Background(), AIRequest{
		Prompt:      "ignored",
		WorkDir:     dir,
		ArtifactDir: dir,
		OutputPath:  outputPath,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "thinking out loud" {
		t.Fatalf("expected stdout to keep progress logs, got %q", result.Stdout)
	}
	if result.Output != "final artifact" {
		t.Fatalf("expected output file contents, got %q", result.Output)
	}
}

func TestCopilotCLIProviderWritesDebugLogsWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	scriptPath := writeProviderScript(t, dir, "echo-copilot", "@echo off\r\necho %*\r\n", "#!/usr/bin/env sh\nprintf '%s\\n' \"$*\"\n")

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
	if !strings.Contains(result.Stderr, "prompt_in_args=false") {
		t.Fatalf("expected prompt_in_args in debug logs, got %q", result.Stderr)
	}
	if !strings.Contains(result.Stderr, "prompt=debug prompt") {
		t.Fatalf("expected prompt contents in debug logs, got %q", result.Stderr)
	}
}

func writeProviderScript(t *testing.T, dir string, baseName string, windowsBody string, unixBody string) string {
	t.Helper()

	ext := ".sh"
	body := unixBody
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		ext = ".cmd"
		body = windowsBody
		mode = 0o644
	}

	scriptPath := filepath.Join(dir, baseName+ext)
	if err := os.WriteFile(scriptPath, []byte(body), mode); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return scriptPath
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
	if err := os.WriteFile(filepath.Join(moduleDir, "prompt.md"), []byte("Run go test ./... in the current repository and report the failure output."), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt.md) error = %v", err)
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
