package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
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

func TestClientListProjectIssues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/issues":
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
		case "/graphql":
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"number":123`) {
				t.Fatalf("expected graphql request to include issue number, got %s", string(raw))
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"repository": {
						"issue": {
							"projectItems": {
								"nodes": [
									{
										"project": {"title": "Roadmap"},
										"fieldValues": {
											"nodes": [
												{
													"__typename": "ProjectV2ItemFieldSingleSelectValue",
													"field": {"name": "Status"},
													"name": "Ready"
												}
											]
										}
									}
								]
							}
						}
					}
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	items, err := client.ListProjectIssues(context.Background(), "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListProjectIssues() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Target != domain.TargetIssueProject {
		t.Fatalf("expected issue_project target, got %s", items[0].Target)
	}
	if len(items[0].ProjectCards) != 1 || items[0].ProjectCards[0].Project != "Roadmap" {
		t.Fatalf("unexpected project cards: %+v", items[0].ProjectCards)
	}
}
