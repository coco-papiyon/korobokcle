package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestWorkflowProcessorProcessesDesignJob(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()

	store := newMemoryJobStore()
	settingsStore := &workflowTestSettingsStore{
		settings: domain.NormalizeWatchSettings(domain.WatchSettings{
			Repository:          "owner/repo",
			AIProvider:          domain.AIProviderCodex,
			PollIntervalSeconds: 120,
			Models: domain.AIModels{
				Codex:         domain.ModelSelection{Mode: domain.ModelModeCustom, Value: "gpt-5.5"},
				GitHubCopilot: domain.ModelSelection{Mode: domain.ModelModeDefault},
			},
		}),
	}

	processor := NewWorkflowProcessor(store, settingsStore, baseDir, toolDir, nil)
	job := domain.Job{
		ID:         "issue-114",
		Kind:       domain.JobKindIssueDesign,
		State:      domain.StateDetected,
		Repository: "owner/repo",
		Number:     114,
		Title:      "画面構成変更",
	}

	if err := processor(context.Background(), job); err != nil {
		t.Fatalf("processor() error = %v", err)
	}

	updated, ok, err := store.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("job not found in store")
	}
	if updated.State != domain.StateDesignReady {
		t.Fatalf("updated state = %s, want %s", updated.State, domain.StateDesignReady)
	}

	artifactPath := filepath.Join(baseDir, ".workspace", "design", "114_画面構成変更.md")
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "Provider: Codex") {
		t.Fatalf("artifact content missing provider: %s", content)
	}
	if !strings.Contains(content, "Model: gpt-5.5") {
		t.Fatalf("artifact content missing model: %s", content)
	}
}

type workflowTestSettingsStore struct {
	mu       sync.Mutex
	settings domain.WatchSettings
}

func (s *workflowTestSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

func (s *workflowTestSettingsStore) Save(_ context.Context, settings domain.WatchSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
	return nil
}

var _ SettingsStore = (*workflowTestSettingsStore)(nil)
