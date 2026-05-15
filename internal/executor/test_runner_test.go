package executor

import (
	"context"
	"testing"
)

func TestTestRunnerRunSuccess(t *testing.T) {
	t.Parallel()

	runner := NewTestRunner()
	report := runner.Run(context.Background(), TestProfile{
		Name:     "ok",
		Commands: []string{"Write-Output 'ok'"},
	}, ".")

	if !report.Success {
		t.Fatal("expected success report")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
}
