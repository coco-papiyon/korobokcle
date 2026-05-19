package github

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type stubTokenProvider struct{}

func (stubTokenProvider) Token(context.Context) (string, error) {
	return "token", nil
}

func TestClientListIssues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/search/issues" {
			t.Fatalf("unexpected path: %s", got)
		}
		query := mustURLQuery(t, r.URL.RawQuery)
		if got := query.Get("q"); got != `repo:owner/repo state:open is:issue` {
			t.Fatalf("unexpected search query: %s", got)
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	items, err := client.ListIssues(context.Background(), config.WatchRule{}, "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestClientListIssuesLogsInfoAndDebugSeparately(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	var infoBuf bytes.Buffer
	var debugBuf bytes.Buffer
	client := NewClient(stubTokenProvider{}, log.New(&debugBuf, "", 0)).WithInfoLogger(log.New(&infoBuf, "", 0))
	client.baseURL = server.URL

	_, err := client.ListIssues(context.Background(), config.WatchRule{}, "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}

	infoLog := infoBuf.String()
	if !strings.Contains(infoLog, "github api start name=search/issues repository=owner/repo target=issue") {
		t.Fatalf("expected info start log, got %s", infoLog)
	}
	if !strings.Contains(infoLog, "github api done name=search/issues repository=owner/repo target=issue status=200 items=0") {
		t.Fatalf("expected info done log, got %s", infoLog)
	}

	debugLog := debugBuf.String()
	if !strings.Contains(debugLog, "github request method=GET url=") {
		t.Fatalf("expected debug request log, got %s", debugLog)
	}
	if !strings.Contains(debugLog, `github response url=`) || !strings.Contains(debugLog, `body={"items":[]}`) {
		t.Fatalf("expected debug response log, got %s", debugLog)
	}
}

func TestClientListIssuesUsesSearchQueryFilters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/search/issues" {
			t.Fatalf("unexpected path: %s", got)
		}
		query := mustURLQuery(t, r.URL.RawQuery)
		want := `repo:owner/repo state:open is:pr -is:draft updated:>=2026-05-15T09:00:00Z label:"ai:review" label:"urgent" (author:alice OR author:bob) assignee:carol`
		if got := query.Get("q"); got != want {
			t.Fatalf("unexpected search query: %s", got)
		}
		_, _ = w.Write([]byte(`{
			"items": [
			{
				"number": 123,
				"title": "Implement feature",
				"body": "details",
				"html_url": "https://github.com/owner/repo/issues/123",
				"updated_at": "2026-05-15T09:00:00Z",
				"draft": false,
				"user": {"login": "alice"},
				"assignees": [{"login": "bob"}],
				"labels": [{"name": "ai:review"}, {"name": "urgent"}],
				"pull_request": {}
			}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	rule := config.WatchRule{
		Target:         string(domain.TargetPullRequest),
		Labels:         []string{"ai:review", "urgent"},
		Authors:        []string{"alice", "bob"},
		Assignees:      []string{"carol"},
		ExcludeDraftPR: true,
	}
	items, err := client.ListPullRequests(context.Background(), rule, "owner/repo", time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListPullRequests() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Target != domain.TargetPullRequest {
		t.Fatalf("expected pull_request target, got %s", items[0].Target)
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
		case "/search/issues":
			query := mustURLQuery(t, r.URL.RawQuery)
			if got := query.Get("q"); got != `repo:owner/repo state:open is:issue` {
				t.Fatalf("unexpected search query: %s", got)
			}
			_, _ = w.Write([]byte(`{
				"items": [
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
				]
			}`))
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

	items, err := client.ListProjectIssues(context.Background(), config.WatchRule{}, "owner/repo", time.Time{})
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

func TestClientListPullRequestsIncludesReviewers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/search/issues" {
			t.Fatalf("unexpected path: %s", got)
		}
		query := mustURLQuery(t, r.URL.RawQuery)
		if got := query.Get("q"); got != `repo:owner/repo state:open is:pr` {
			t.Fatalf("unexpected search query: %s", got)
		}
		_, _ = w.Write([]byte(`{"items":[
			{
				"number": 124,
				"title": "Add review filter",
				"body": "details",
				"html_url": "https://github.com/owner/repo/pull/124",
				"updated_at": "2026-05-15T09:00:00Z",
				"user": {"login": "alice"},
				"assignees": [{"login": "bob"}],
				"requested_reviewers": [{"login": "carol"}],
				"labels": [{"name": "ai:review"}],
				"pull_request": {}
			}
		]}`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	items, err := client.ListPullRequests(context.Background(), config.WatchRule{Target: string(domain.TargetPullRequest)}, "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListPullRequests() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Reviewers) != 1 || items[0].Reviewers[0] != "carol" {
		t.Fatalf("expected reviewers to include carol, got %+v", items[0].Reviewers)
	}
}

func mustURLQuery(t *testing.T, raw string) url.Values {
	t.Helper()

	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	return values
}
