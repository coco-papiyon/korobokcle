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

func loadJobSessionID(root string, artifactsDir string, repository string) string {
	raw, err := os.ReadFile(repositorySessionPath(root, artifactsDir, repository))
	if err != nil {
		return ""
	}
	var state jobSessionState
	if err := json.Unmarshal(raw, &state); err != nil {
		return ""
	}
	return strings.TrimSpace(state.SessionID)
}

func saveJobSessionID(root string, artifactsDir string, repository string, sessionID string) error {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		if err := os.Remove(repositorySessionPath(root, artifactsDir, repository)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	path := repositorySessionPath(root, artifactsDir, repository)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(jobSessionState{SessionID: trimmed}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func repositorySessionPath(root string, artifactsDir string, repository string) string {
	return artifacts.RepositoryWorkerSessionsPath(root, artifactsDir, repository)
}
