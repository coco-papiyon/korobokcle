package executor

import (
	"context"
	"runtime"
	"testing"
)

func TestTestRunnerRunSuccess(t *testing.T) {
	t.Parallel()

	runner := NewTestRunner()
	command := "printf 'ok\\n'"
	if runtime.GOOS == "windows" {
		command = "Write-Output 'ok'"
	}
	report := runner.Run(context.Background(), TestProfile{
		Name:     "ok",
		Commands: []string{command},
	}, ".")

	if !report.Success {
		t.Fatal("expected success report")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
}
