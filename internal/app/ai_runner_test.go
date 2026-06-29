package app

import "testing"

func TestParseAIResponseExtractsJSONFromWrappedText(t *testing.T) {
	raw := "some preface\n```json\n{\"artifact_markdown\":\"# result\",\"git_diff\":\"diff --git a/a b/a\"}\n```\nsome suffix"
	resp, err := parseAIResponse(raw, true)
	if err != nil {
		t.Fatalf("parseAIResponse() error = %v", err)
	}
	if resp.ArtifactMarkdown != "# result" {
		t.Fatalf("artifact_markdown = %q, want # result", resp.ArtifactMarkdown)
	}
	if resp.GitDiff != "diff --git a/a b/a" {
		t.Fatalf("git_diff = %q, want diff --git a/a b/a", resp.GitDiff)
	}
}

func TestExtractFirstJSONObject(t *testing.T) {
	raw := "text before {\"a\":\"x}\",\"b\":{\"c\":1}} trailing"
	got, ok := extractFirstJSONObject(raw)
	if !ok {
		t.Fatal("extractFirstJSONObject() = false, want true")
	}
	if got != "{\"a\":\"x}\",\"b\":{\"c\":1}}" {
		t.Fatalf("extractFirstJSONObject() = %q", got)
	}
}
