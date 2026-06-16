package app

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathHandlesRelativeAndAbsoluteTargets(t *testing.T) {
	t.Parallel()

	root := filepath.Join("workspace", "tool")
	if got := resolvePath(root, filepath.Join("data", "korobokcle.db")); got != filepath.Join(root, "data", "korobokcle.db") {
		t.Fatalf("resolvePath(relative) = %q, want %q", got, filepath.Join(root, "data", "korobokcle.db"))
	}

	absTarget := filepath.Join(t.TempDir(), "config", "app.yaml")
	if got := resolvePath(root, absTarget); got != filepath.Clean(absTarget) {
		t.Fatalf("resolvePath(absolute) = %q, want %q", got, filepath.Clean(absTarget))
	}
}

func TestLogEnvironmentEmitsConfiguredVariablesOnly(t *testing.T) {
	t.Setenv("KOROBOKCLE_TOOL_ROOT", "tool-root")
	t.Setenv("KOROBOKCLE_CODEX_BIN", "codex")
	t.Setenv("KOROBOKCLE_COPILOT_DEBUG", "1")

	var buf bytes.Buffer
	logEnvironment(log.New(&buf, "", 0))

	output := buf.String()
	for _, expected := range []string{
		"env KOROBOKCLE_TOOL_ROOT=tool-root",
		"env KOROBOKCLE_CODEX_BIN=codex",
		"env KOROBOKCLE_COPILOT_DEBUG=1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %s", expected, output)
		}
	}
	if strings.Contains(output, "KOROBOKCLE_CODEX_ARGS_JSON") {
		t.Fatalf("did not expect unset environment variables in output, got %s", output)
	}
}
