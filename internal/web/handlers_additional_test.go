package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestFilterPRCommentsAndPostedBody(t *testing.T) {
	t.Parallel()

	body := prCommentAnalysisPostedBody("  analysis summary  ")
	if body == "" {
		t.Fatal("expected analysis body to be generated")
	}
	if got, wantPrefix := body[:len(prCommentAnalysisPostedMarker)], prCommentAnalysisPostedMarker; got != wantPrefix {
		t.Fatalf("unexpected body prefix: %q", got)
	}

	comments := filterPRComments([]reviewCommentResponse{
		{Author: "alice", Body: "looks good"},
		{Author: "bot", Body: body},
		{Author: "bob", Body: "needs work"},
	})
	if len(comments) != 2 {
		t.Fatalf("expected posted analysis comment to be filtered out, got %#v", comments)
	}
	if comments[0].Author != "alice" || comments[1].Author != "bob" {
		t.Fatalf("unexpected filtered comments: %#v", comments)
	}
	if got := prCommentAnalysisPostedBody("   "); got != "" {
		t.Fatalf("expected empty body for blank input, got %q", got)
	}
}

func TestExtractArtifactDirFromPayload(t *testing.T) {
	t.Parallel()

	if got := extractArtifactDirFromPayload(`{"artifactDir":" /tmp/artifacts "}`); got != " /tmp/artifacts " {
		t.Fatalf("expected artifactDir to win as-is, got %q", got)
	}

	reportPath := filepath.Join("artifacts", "workers", "owner-repo", "jobs", "issue_1", "implementation", "result.md")
	if got := extractArtifactDirFromPayload(`{"reportPath":"` + filepath.ToSlash(reportPath) + `"}`); got != filepath.Dir(reportPath) {
		t.Fatalf("expected reportPath directory, got %q", got)
	}

	if got := extractArtifactDirFromPayload(`not-json`); got != "" {
		t.Fatalf("expected invalid payload to return empty string, got %q", got)
	}
}

func TestSanitizeEventPayloadForResponseStripsBodyFields(t *testing.T) {
	t.Parallel()

	raw := `{"body":"top","nested":{"body":"inner","keep":true},"items":[{"body":"array","name":"item"}],"keep":"yes"}`
	got := sanitizeEventPayloadForResponse(raw)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["body"]; ok {
		t.Fatalf("expected top-level body field to be removed, got %#v", decoded)
	}
	nested := decoded["nested"].(map[string]any)
	if _, ok := nested["body"]; ok {
		t.Fatalf("expected nested body field to be removed, got %#v", nested)
	}
	items := decoded["items"].([]any)
	item := items[0].(map[string]any)
	if _, ok := item["body"]; ok {
		t.Fatalf("expected array body field to be removed, got %#v", item)
	}
	if decoded["keep"] != "yes" {
		t.Fatalf("expected keep field to remain, got %#v", decoded)
	}
}

func TestExtractReviewComments(t *testing.T) {
	t.Parallel()

	comments := extractReviewComments(`{"reviewComments":[{"author":"alice","body":"looks good","path":"file.go","line":12,"url":"https://example.invalid"}]}`)
	if len(comments) != 1 {
		t.Fatalf("expected one review comment, got %#v", comments)
	}
	got := comments[0]
	if got.Author != "alice" || got.Body != "looks good" || got.Path != "file.go" || got.Line != 12 || got.URL != "https://example.invalid" {
		t.Fatalf("unexpected review comment: %#v", got)
	}
	if got := extractReviewComments(`invalid`); got != nil {
		t.Fatalf("expected invalid payload to return nil, got %#v", got)
	}
}

func TestLatestImplementationRerunSourceEventType(t *testing.T) {
	t.Parallel()

	events := []domain.Event{
		{ID: 1, EventType: "implementation_ready"},
		{ID: 2, EventType: "implementation_rerun_requested", Payload: `{"eventId":1}`},
	}
	got, err := latestImplementationRerunSourceEventType(events)
	if err != nil {
		t.Fatalf("latestImplementationRerunSourceEventType() error = %v", err)
	}
	if got != "implementation_ready" {
		t.Fatalf("latestImplementationRerunSourceEventType() = %q, want implementation_ready", got)
	}

	if got, err := latestImplementationRerunSourceEventType([]domain.Event{{EventType: "implementation_rerun_requested", Payload: `{"eventId":null}`}}); err != nil || got != "" {
		t.Fatalf("expected nil event id to return empty string, got %q err=%v", got, err)
	}
	if _, err := latestImplementationRerunSourceEventType([]domain.Event{{EventType: "implementation_rerun_requested", Payload: `{"eventId":`}}); err == nil {
		t.Fatal("expected malformed payload to return error")
	}
}

func TestResolveRepositoryWorkerArtifactAndTestReportPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository: "owner/repository",
				WorkDir:    "workspace/owner-repository",
			}},
		},
	})

	artifactDir := resolveRepositoryWorkerArtifactDir(cfg, "owner/repository", 42, []domain.Event{
		{EventType: "implementation_ready", Payload: `{"artifactDir":"custom/artifacts"}`},
	}, []string{"implementation_ready"}, artifacts.WorkerImplementation)
	if artifactDir != "custom/artifacts" {
		t.Fatalf("resolveRepositoryWorkerArtifactDir() = %q, want custom/artifacts", artifactDir)
	}

	fallbackReportPath := resolveRepositoryWorkerTestReportPath(cfg, "owner/repository", 42, nil)
	wantFallback := filepath.Join(artifacts.RepositoryWorkerJobPhaseDir(root, cfg.App().ArtifactsDir, "owner/repository", 42, artifacts.WorkerImplementation), "test-report.json")
	if fallbackReportPath != wantFallback {
		t.Fatalf("resolveRepositoryWorkerTestReportPath() fallback = %q, want %q", fallbackReportPath, wantFallback)
	}

	explicitPath := filepath.ToSlash(filepath.Join(root, "custom", "report.json"))
	explicitReportPath := resolveRepositoryWorkerTestReportPath(cfg, "owner/repository", 42, []domain.Event{
		{EventType: "test_failed", Payload: `{"reportPath":"` + explicitPath + `"}`},
	})
	if explicitReportPath != explicitPath {
		t.Fatalf("resolveRepositoryWorkerTestReportPath() explicit = %q, want custom report path", explicitReportPath)
	}
}

func TestLoadFirstArtifactAndPreferredAIArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.NewService(root, config.Files{
		App: config.App{
			ArtifactsDir: "artifacts",
			MonitoredRepositories: []config.MonitoredRepository{{
				Repository: "owner/repository",
				WorkDir:    "workspace/owner-repository",
			}},
		},
	})
	server := &Server{config: cfg}
	job := domain.Job{Repository: "owner/repository", GitHubNumber: 42, Title: "AI artifact"}

	firstDir := filepath.Join(root, "first")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(firstDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstDir, "second.md"), []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile(second.md) error = %v", err)
	}
	artifact, err := server.loadFirstArtifact(firstDir, "first.md", "second.md")
	if err != nil {
		t.Fatalf("loadFirstArtifact() error = %v", err)
	}
	if artifact == nil || artifact.Content != "second" {
		t.Fatalf("unexpected first artifact: %#v", artifact)
	}

	workDir := artifacts.RepositoryWorkerWorkDir(root, cfg.App().ArtifactsDir, job.Repository, "workspace/owner-repository")
	workingPath := artifacts.RepositoryWorkerWorkArtifactPath(workDir, artifacts.WorkerImplementation, job.GitHubNumber, job.Title)
	if err := os.MkdirAll(filepath.Dir(workingPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(workingPath dir) error = %v", err)
	}
	if err := os.WriteFile(workingPath, []byte("working"), 0o644); err != nil {
		t.Fatalf("WriteFile(workingPath) error = %v", err)
	}

	artifact, err = server.loadPreferredAIArtifact(job, artifacts.WorkerImplementation, nil, []string{"implementation_ready"}, "result.md")
	if err != nil {
		t.Fatalf("loadPreferredAIArtifact() error = %v", err)
	}
	if artifact == nil || artifact.Content != "working" {
		t.Fatalf("expected working copy to win, got %#v", artifact)
	}

	// Remove the working copy and fall back to the artifact directory.
	if err := os.Remove(workingPath); err != nil {
		t.Fatalf("Remove(workingPath) error = %v", err)
	}
	fallbackDir := resolveRepositoryWorkerArtifactDir(cfg, job.Repository, job.GitHubNumber, []domain.Event{
		{EventType: "implementation_ready", Payload: `{"artifactDir":"` + filepath.ToSlash(filepath.Join(root, "fallback")) + `"}`},
	}, []string{"implementation_ready"}, artifacts.WorkerImplementation)
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fallbackDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fallbackDir, "result.md"), []byte("fallback"), 0o644); err != nil {
		t.Fatalf("WriteFile(fallback result.md) error = %v", err)
	}
	artifact, err = server.loadPreferredAIArtifact(job, artifacts.WorkerImplementation, []domain.Event{
		{EventType: "implementation_ready", Payload: `{"artifactDir":"` + filepath.ToSlash(fallbackDir) + `"}`},
	}, []string{"implementation_ready"}, "result.md")
	if err != nil {
		t.Fatalf("loadPreferredAIArtifact(fallback) error = %v", err)
	}
	if artifact == nil || artifact.Content != "fallback" {
		t.Fatalf("expected fallback artifact to be returned, got %#v", artifact)
	}
}
