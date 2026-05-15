package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubTokenProvider struct{}

func (stubTokenProvider) Token(context.Context) (string, error) {
	return "token", nil
}

func TestClientListIssues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/repos/owner/repo/issues" {
			t.Fatalf("unexpected path: %s", got)
		}
		_, _ = w.Write([]byte(`[
			{
				"number": 123,
				"title": "Implement feature",
				"body": "details",
				"html_url": "https://github.com/owner/repo/issues/123",
				"updated_at": "2026-05-15T09:00:00Z",
				"user": {"login": "alice"},
				"assignees": [{"login": "bob"}],
				"labels": [{"name": "ai:design"}]
			}
		]`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	items, err := client.ListIssues(context.Background(), "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Target != "issue" {
		t.Fatalf("expected issue target, got %s", items[0].Target)
	}
}

func TestNormalizeRepositoryFromGitHubURL(t *testing.T) {
	t.Parallel()

	got, err := normalizeRepository("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("normalizeRepository() error = %v", err)
	}
	if got != "owner/repo" {
		t.Fatalf("expected owner/repo, got %q", got)
	}
}
