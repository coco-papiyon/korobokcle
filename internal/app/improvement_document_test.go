package app

import (
	"strings"
	"testing"
	"time"
)

func TestImprovementDocumentMarkdownRoundTrip(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 6, 7, 12, 34, 56, 0, time.UTC)
	document := ImprovementDocument{
		FrontMatter: ImprovementFrontMatter{
			ID:        "ui-layout-policy",
			Title:     "UI レイアウト方針",
			Scope:     "repository",
			Phases:    []string{"design", "implementation"},
			Status:    "active",
			UpdatedAt: updatedAt,
			Source: ImprovementSource{
				JobID:       "issue-42",
				IssueNumber: 42,
				Repository:  "owner/repo",
				Event:       "improvement_approved",
			},
		},
		Body: "- ボタンを左、補足説明を右に配置する。",
	}

	raw, err := document.MarshalMarkdown()
	if err != nil {
		t.Fatalf("MarshalMarkdown() error = %v", err)
	}
	text := string(raw)
	for _, expected := range []string{
		"---\n",
		"id: ui-layout-policy",
		"title: UI レイアウト方針",
		"scope: repository",
		"- design",
		"- implementation",
		"status: active",
		"jobId: issue-42",
		"issueNumber: 42",
		"repository: owner/repo",
		"event: improvement_approved",
		"- ボタンを左、補足説明を右に配置する。",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in markdown, got %s", expected, text)
		}
	}

	decoded, err := ParseImprovementMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseImprovementMarkdown() error = %v", err)
	}
	if decoded.FrontMatter.ID != document.FrontMatter.ID || decoded.FrontMatter.Title != document.FrontMatter.Title {
		t.Fatalf("unexpected decoded front matter: %#v", decoded.FrontMatter)
	}
	if !decoded.FrontMatter.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updatedAt %s, got %s", updatedAt, decoded.FrontMatter.UpdatedAt)
	}
	if len(decoded.FrontMatter.Phases) != 2 || decoded.FrontMatter.Phases[0] != "design" || decoded.FrontMatter.Phases[1] != "implementation" {
		t.Fatalf("unexpected decoded phases: %#v", decoded.FrontMatter.Phases)
	}
	if decoded.Body != document.Body {
		t.Fatalf("expected decoded body %q, got %q", document.Body, decoded.Body)
	}
}

func TestParseImprovementMarkdownRejectsMissingFrontMatter(t *testing.T) {
	t.Parallel()

	if _, err := ParseImprovementMarkdown([]byte("# not front matter")); err == nil {
		t.Fatalf("expected missing front matter to fail")
	}
}
