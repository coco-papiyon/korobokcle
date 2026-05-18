package app

import (
	"strings"
	"testing"
)

func TestTrimImplementationSummaryRemovesPatchSection(t *testing.T) {
	t.Parallel()

	output := "## Summary\n\nChanged files.\n\n## Patch\n\n```diff\ndiff --git a/example.txt b/example.txt\n--- a/example.txt\n+++ b/example.txt\n@@ -1 +1 @@\n-old\n+new\n```\n"

	summary := trimImplementationSummary(output)
	if strings.Contains(summary, "diff --git") {
		t.Fatalf("expected summary without patch, got %q", summary)
	}
}

func TestTrimImplementationSummaryKeepsNonPatchOutput(t *testing.T) {
	t.Parallel()

	output := "## 実装内容の要約\n\n- repo root で直接修正しました。\n\n## 変更した箇所\n\n- internal/app/implementation_worker.go\n"
	if got := trimImplementationSummary(output); got != strings.TrimSpace(output) {
		t.Fatalf("expected summary to stay unchanged, got %q", got)
	}
}
