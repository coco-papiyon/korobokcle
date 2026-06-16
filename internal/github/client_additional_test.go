package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestNormalizeRepositoryRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	for _, repository := range []string{"", "owner", "owner/repo/extra"} {
		if _, err := normalizeRepository(repository); err == nil {
			t.Fatalf("expected normalizeRepository(%q) to fail", repository)
		}
	}
	got, err := normalizeRepository("git@github.com:owner/repo.git")
	if err != nil || got != "git@github.com:owner/repo.git" {
		t.Fatalf("expected ssh form to be preserved, got %q err=%v", got, err)
	}
}

func TestClientFetchIssueBodyRejectsInvalidIssueNumber(t *testing.T) {
	t.Parallel()

	client := NewClient(stubTokenProvider{}, nil)
	if _, err := client.FetchIssueBody(context.Background(), "owner/repo", 0); err == nil {
		t.Fatal("expected FetchIssueBody() to reject issue number 0")
	}
}

func TestClientFetchPullRequestCommentsValidatesInputsAndStatuses(t *testing.T) {
	t.Parallel()

	client := NewClient(stubTokenProvider{}, nil)
	if _, err := client.FetchPullRequestComments(context.Background(), "owner/repo", 0, filepath.Join(t.TempDir(), "artifacts")); err == nil {
		t.Fatal("expected FetchPullRequestComments() to reject pull number 0")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer server.Close()

	client.baseURL = server.URL
	if _, err := client.FetchPullRequestComments(context.Background(), "owner/repo", 1, filepath.Join(t.TempDir(), "artifacts")); err == nil {
		t.Fatal("expected FetchPullRequestComments() to fail on non-success status")
	}
}

func TestClientListIssuesDummyRepositorySkipsNetwork(t *testing.T) {
	t.Parallel()

	client := NewClient(stubTokenProvider{}, nil)
	client.baseURL = "https://example.invalid"

	items, err := client.ListIssues(context.Background(), config.WatchRule{}, "coco-papiyon/dummy", time.Time{})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected dummy repository to skip network calls, got %d items", len(items))
	}
}
