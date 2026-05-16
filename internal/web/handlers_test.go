package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestAvailableActionsForEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		event  domain.Event
		expect []string
	}{
		{
			name: "design ready",
			event: domain.Event{
				EventType: "design_ready",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateDesignReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "implementation ready",
			event: domain.Event{
				EventType: "implementation_ready",
				StateFrom: string(domain.StateImplementationRunning),
				StateTo:   string(domain.StateImplementationReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "review ready",
			event: domain.Event{
				EventType: "review_ready",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateReviewReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "review completed",
			event: domain.Event{
				EventType: "review_completed",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateCompleted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr created",
			event: domain.Event{
				EventType: "pr_created",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateCompleted),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
		{
			name: "review failure",
			event: domain.Event{
				EventType: "review_failed",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr failure",
			event: domain.Event{
				EventType: "pr_create_failed",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := availableActionsForEvent(tc.event)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("expected %v, got %v", tc.expect, got)
				}
			}
		})
	}
}

func TestHandleAppConfigIncludesPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.PollInterval = 45 * time.Second
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app-config", nil)

	server.handleAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got struct {
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		PollInterval int    `json:"pollInterval"`
	}
	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.PollInterval != 45 {
		t.Fatalf("expected poll interval 45, got %d", got.PollInterval)
	}
}

func TestHandleSaveAppConfigUpdatesPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"mock","model":"","pollInterval":90}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := svc.App().PollInterval; got != 90*time.Second {
		t.Fatalf("expected saved poll interval 90s, got %s", got)
	}

	savedConfigPath := filepath.Join(root, "config", "app.yaml")
	raw, err := os.ReadFile(savedConfigPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !bytes.Contains(raw, []byte("pollInterval: 1m30s")) {
		t.Fatalf("expected saved config to contain updated poll interval, got %s", string(raw))
	}
}

func TestHandleSaveAppConfigRejectsInvalidPollInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	svc := config.NewService(root, files)
	server := &Server{config: svc}

	body := []byte(`{"provider":"mock","model":"","pollInterval":0}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/app-config", bytes.NewReader(body))

	server.handleSaveAppConfig(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}
