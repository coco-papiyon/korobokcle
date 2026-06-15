package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
)

type jobSessionState struct {
	SessionID string `json:"sessionId"`
}

func loadJobSessionID(root string, artifactsDir string, jobID string) string {
	raw, err := os.ReadFile(jobSessionPath(root, artifactsDir, jobID))
	if err != nil {
		return ""
	}
	var state jobSessionState
	if err := json.Unmarshal(raw, &state); err != nil {
		return ""
	}
	return strings.TrimSpace(state.SessionID)
}

func saveJobSessionID(root string, artifactsDir string, jobID string, sessionID string) error {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		if err := os.Remove(jobSessionPath(root, artifactsDir, jobID)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	path := jobSessionPath(root, artifactsDir, jobID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(jobSessionState{SessionID: trimmed}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func jobSessionPath(root string, artifactsDir string, jobID string) string {
	return filepath.Join(artifacts.JobDir(root, artifactsDir, jobID), "session.json")
}
