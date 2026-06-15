package web

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestToolRuntimeSeparatesStdoutAndStderr(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repository", Branch: "", ImplementationWorkers: 1},
	}
	svc := config.NewService(root, files)
	manager := newToolRuntimeManager()
	job := domain.Job{
		ID:          "job-1",
		Repository:  "owner/repository",
		WatchRuleID: "rule-1",
	}
	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, files.App.ArtifactsDir, job.Repository, "main")
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	command := toolLogTestCommand()
	tool := config.ToolCommand{Name: "fixture-test", Command: command, Resident: false}

	if err := manager.start(context.Background(), svc, job, nil, tool); err != nil {
		t.Fatalf("start() error = %v", err)
	}

	var got *toolExecutionResponse
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		got, err = manager.snapshot(svc, job, nil)
		if err != nil {
			t.Fatalf("snapshot() error = %v", err)
		}
		if got != nil && got.Stdout != nil && got.Stderr != nil && !got.Running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if got == nil {
		t.Fatal("expected tool execution snapshot")
	}
	if got.Name != tool.Name {
		t.Fatalf("expected tool name %q, got %q", tool.Name, got.Name)
	}
	if got.Stdout == nil || got.Stderr == nil {
		t.Fatalf("expected stdout and stderr logs, got %#v", got)
	}
	if !strings.Contains(got.Stdout.Content, "stdout line") {
		t.Fatalf("expected stdout content, got %q", got.Stdout.Content)
	}
	if !strings.Contains(got.Stderr.Content, "stderr line") {
		t.Fatalf("expected stderr content, got %q", got.Stderr.Content)
	}
	if !strings.HasSuffix(filepath.Base(got.Stdout.Path), "tool.stdout.log") {
		t.Fatalf("expected stdout path to reference tool.stdout.log, got %q", got.Stdout.Path)
	}
	if !strings.HasSuffix(filepath.Base(got.Stderr.Path), "tool.stderr.log") {
		t.Fatalf("expected stderr path to reference tool.stderr.log, got %q", got.Stderr.Path)
	}
}

func TestToolRuntimeStopMarksExecutionStopped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := config.DefaultFiles()
	files.App.MonitoredRepositories = []config.MonitoredRepository{
		{Repository: "owner/repository", Branch: "", ImplementationWorkers: 1},
	}
	svc := config.NewService(root, files)
	manager := newToolRuntimeManager()
	job := domain.Job{
		ID:          "job-2",
		Repository:  "owner/repository",
		WatchRuleID: "rule-1",
	}
	workerDir := artifacts.RepositoryWorkerBranchWorkDir(root, files.App.ArtifactsDir, job.Repository, "main")
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	tool := config.ToolCommand{Name: "fixture-stop", Command: toolStopTestCommand(), Resident: false}
	if err := manager.start(context.Background(), svc, job, nil, tool); err != nil {
		t.Fatalf("start() error = %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := manager.snapshot(svc, job, nil)
		if err != nil {
			t.Fatalf("snapshot() error = %v", err)
		}
		if got != nil && got.Running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := manager.stop(job.ID); err != nil {
		t.Fatalf("stop() error = %v", err)
	}

	got, err := manager.snapshot(svc, job, nil)
	if err != nil {
		t.Fatalf("snapshot() after stop error = %v", err)
	}
	if got == nil {
		t.Fatal("expected tool execution snapshot after stop")
	}
	if got.Name != tool.Name {
		t.Fatalf("expected tool name %q, got %q", tool.Name, got.Name)
	}
	if got.Running {
		t.Fatalf("expected stopped execution, got running snapshot %+v", got)
	}
	if got.FinishedAt == "" {
		t.Fatalf("expected finishedAt to be recorded, got %+v", got)
	}
}

func toolLogTestCommand() string {
	if runtime.GOOS == "windows" {
		return "[Console]::Out.WriteLine('stdout line'); [Console]::Error.WriteLine('stderr line')"
	}
	return "printf 'stdout line\\n'; printf 'stderr line\\n' 1>&2"
}

func toolStopTestCommand() string {
	if runtime.GOOS == "windows" {
		return "Start-Sleep -Seconds 30"
	}
	return "sleep 30"
}
