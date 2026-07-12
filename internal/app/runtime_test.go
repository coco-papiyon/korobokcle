package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestRuntimeStatusUsesRelativeWorkingDir(t *testing.T) {
	store := &runtimeTestJobStore{
		jobs: map[string]domain.Job{
			"issue-202": {
				ID:         "issue-202",
				Kind:       domain.JobKindIssueImplementation,
				State:      domain.StateImplementationReady,
				Repository: "mock-owner/mock-repo",
				Number:     202,
				Title:      "implementation-ready",
			},
		},
	}
	settings := &runtimeTestSettingsStore{
		settings: domain.WatchSettings{
			StartupCommand: ".\\tests\\start_mock_app.bat",
			ResidentMode:   true,
		},
	}
	controller := NewRuntimeController(t.TempDir(), filepath.Join(t.TempDir(), "tests"), store, settings, nil)

	status, err := controller.Status(context.Background(), "issue-202")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.WorkingDir != "workspace/mock-owner_mock-repo/issue-202/worktree" {
		t.Fatalf("workingDir = %q, want relative worktree path", status.WorkingDir)
	}
}

func TestNormalizeRuntimeLogContent(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "utf8",
			raw:  []byte("起動しました"),
			want: "起動しました",
		},
		{
			name: "shift_jis",
			raw:  mustEncodeRuntimeLog(t, japanese.ShiftJIS.NewEncoder(), "起動しました"),
			want: "起動しました",
		},
		{
			name: "utf16le",
			raw:  mustUTF16WithBOM([]uint16{'起', '動', 'し', 'ま', 'し', 'た'}, binary.LittleEndian),
			want: "起動しました",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRuntimeLogContent(tt.raw); got != tt.want {
				t.Fatalf("normalizeRuntimeLogContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeRuntimeCommandCollapsesGeneratedRelativeSeparators(t *testing.T) {
	command := `..\\..\\..\\..\\start_mock_app.bat`
	want := `..\..\..\..\start_mock_app.bat`
	if runtime.GOOS != "windows" {
		want = command
	}
	if got := normalizeRuntimeCommand(command); got != want {
		t.Fatalf("normalizeRuntimeCommand() = %q, want %q", got, want)
	}
}

func mustEncodeRuntimeLog(t *testing.T, encoder transform.Transformer, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := transform.NewWriter(&buffer, encoder)
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}

func mustUTF16WithBOM(codeUnits []uint16, order binary.ByteOrder) []byte {
	out := make([]byte, 0, 2+len(codeUnits)*2)
	if order == binary.LittleEndian {
		out = append(out, 0xFF, 0xFE)
	} else {
		out = append(out, 0xFE, 0xFF)
	}
	buf := make([]byte, 2)
	for _, codeUnit := range codeUnits {
		order.PutUint16(buf, codeUnit)
		out = append(out, buf...)
	}
	return out
}

type runtimeTestJobStore struct {
	jobs map[string]domain.Job
}

func (s *runtimeTestJobStore) List(context.Context) ([]domain.Job, error) {
	out := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	return out, nil
}

func (s *runtimeTestJobStore) UpdatedAt(context.Context) (time.Time, error) {
	return time.Time{}, nil
}

func (s *runtimeTestJobStore) Get(_ context.Context, id string) (domain.Job, bool, error) {
	job, ok := s.jobs[id]
	return job, ok, nil
}

func (s *runtimeTestJobStore) Upsert(_ context.Context, job domain.Job) error {
	if s.jobs == nil {
		s.jobs = map[string]domain.Job{}
	}
	s.jobs[job.ID] = job
	return nil
}

func (s *runtimeTestJobStore) Delete(_ context.Context, id string) error {
	delete(s.jobs, id)
	return nil
}

type runtimeTestSettingsStore struct {
	settings domain.WatchSettings
}

func (s *runtimeTestSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	return s.settings, nil
}

func (s *runtimeTestSettingsStore) Save(_ context.Context, settings domain.WatchSettings) error {
	s.settings = settings
	return nil
}
