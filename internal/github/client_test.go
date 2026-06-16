package github

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
	client.ghCommand = fakeGhCommand(t, `{"headRefName":"feature","baseRefName":"main"}`)

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
	client.ghCommand = fakeGhCommand(t, `{"headRefName":"issue_97","baseRefName":"main"}`)

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
	if items[0].BranchName != "issue_97" {
		t.Fatalf("expected branch issue_97, got %q", items[0].BranchName)
	}
	if items[0].BaseBranch != "main" {
		t.Fatalf("expected base branch main, got %q", items[0].BaseBranch)
	}
}

func TestClientFetchIssueBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/repos/owner/repo/issues/42" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Fatalf("unexpected accept header: %s", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Fatalf("unexpected api version header: %s", got)
		}
		_, _ = w.Write([]byte(`{"body":"latest issue body"}`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	body, err := client.FetchIssueBody(context.Background(), "owner/repo", 42)
	if err != nil {
		t.Fatalf("FetchIssueBody() error = %v", err)
	}
	if body != "latest issue body" {
		t.Fatalf("expected latest issue body, got %q", body)
	}
}

func TestClientFetchIssueBodyReturnsErrorForNonSuccessStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL

	if _, err := client.FetchIssueBody(context.Background(), "owner/repo", 42); err == nil {
		t.Fatalf("expected FetchIssueBody() to fail")
	}
}

func TestClientSkipsDummyRepositoryOperations(t *testing.T) {
	t.Parallel()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = "https://example.invalid"

	items, err := client.ListIssues(context.Background(), config.WatchRule{}, "coco-papiyon/dummy", time.Time{})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}

	body, err := client.FetchIssueBody(context.Background(), "coco-papiyon/dummy", 42)
	if err != nil {
		t.Fatalf("FetchIssueBody() error = %v", err)
	}
	if body != "" {
		t.Fatalf("expected empty body, got %q", body)
	}

	root := t.TempDir()
	artifactDir := filepath.Join(root, "artifacts")
	comments, err := client.FetchPullRequestComments(context.Background(), "coco-papiyon/dummy", 7, artifactDir)
	if err != nil {
		t.Fatalf("FetchPullRequestComments() error = %v", err)
	}
	if comments.PullNumber != 7 {
		t.Fatalf("pull number = %d, want 7", comments.PullNumber)
	}
	if len(comments.Comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments.Comments))
	}
	if _, err := os.Stat(filepath.Join(artifactDir, "gh-pr-comments.json")); err != nil {
		t.Fatalf("expected comments artifact: %v", err)
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
		switch r.URL.Path {
		case "/search/issues":
			query := mustURLQuery(t, r.URL.RawQuery)
			if got := query.Get("q"); got != `repo:owner/repo state:open is:pr review-requested:carol` {
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
					"labels": [{"name": "ai:review"}],
					"pull_request": {},
					"head": {"ref": "feature", "repo": {"full_name": "owner/repo"}}
				}
			]}`))
		case "/repos/owner/repo/pulls/124/requested_reviewers":
			_, _ = w.Write([]byte(`{"users":[{"login":"carol"}],"teams":[]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL
	client.ghCommand = fakeGhCommand(t, `{"headRefName":"feature","baseRefName":"main"}`)

	items, err := client.ListPullRequests(context.Background(), config.WatchRule{
		Target:    string(domain.TargetPullRequest),
		Reviewers: []string{"carol"},
	}, "owner/repo", time.Time{})
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

func TestClientListPullRequestReviewsIncludesRequestedReviewersWithoutComments(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/pulls":
			_, _ = w.Write([]byte(`[
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
					"head": {"ref": "feature", "repo": {"full_name": "owner/repo"}}
				}
			]`))
		case "/repos/owner/repo/pulls/124/reviews":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/owner/repo/pulls/comments":
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = server.URL
	client.ghCommand = fakeGhCommand(t, `{"headRefName":"feature","baseRefName":"main"}`)

	items, err := client.ListPullRequestReviews(context.Background(), config.WatchRule{Reviewers: []string{"carol"}}, "owner/repo", time.Time{})
	if err != nil {
		t.Fatalf("ListPullRequestReviews() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Target != domain.TargetPullRequestReview {
		t.Fatalf("expected pull_request_review target, got %s", items[0].Target)
	}
	if len(items[0].Reviewers) != 1 || items[0].Reviewers[0] != "carol" {
		t.Fatalf("expected reviewers to include carol, got %+v", items[0].Reviewers)
	}
	if len(items[0].ReviewComments) != 0 {
		t.Fatalf("expected no review comments, got %+v", items[0].ReviewComments)
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

func fakeGhCommand(t *testing.T, output string) string {
	t.Helper()

	dir := t.TempDir()
	name := "gh"
	script := "#!/bin/sh\nif [ \"$1\" = \"pr\" ] && [ \"$2\" = \"view\" ]; then\n  printf '%s\\n' '" + strings.ReplaceAll(output, "'", "'\\''") + "'\n  exit 0\nfi\nexit 1\n"
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name = "gh.cmd"
		script = "@echo off\r\nif \"%1\"==\"pr\" if \"%2\"==\"view\" (\r\n  echo " + strings.ReplaceAll(output, "\r", "") + "\r\n  exit /b 0\r\n)\r\nexit /b 1\r\n"
		mode = 0o644
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), mode); err != nil {
		t.Fatalf("WriteFile(fake gh) error = %v", err)
	}
	return path
}
