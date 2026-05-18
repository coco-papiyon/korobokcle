package artifacts

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	WorkerDesign         = "design"
	WorkerImplementation = "implementation"
	WorkerFix            = "fix"
	WorkerPR             = "pr"
	WorkerReview         = "review"
)

func JobDir(root string, artifactsDir string, jobID string) string {
	return filepath.Join(root, artifactsDir, "jobs", jobID)
}

func WorkerDir(root string, artifactsDir string, jobID string, worker string) string {
	return filepath.Join(JobDir(root, artifactsDir, jobID), worker)
}

func WorkersDir(root string, artifactsDir string) string {
	return filepath.Join(root, artifactsDir, "workers")
}

func RepositoryWorkerDir(root string, artifactsDir string, repository string, workerIndex int) string {
	sanitized := sanitizePathComponent(repository)
	return filepath.Join(WorkersDir(root, artifactsDir), sanitized, workerName(workerIndex))
}

func workerName(index int) string {
	return fmt.Sprintf("worker-%d", index)
}

func sanitizePathComponent(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "@", "-", "?", "-", "#", "-")
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "repository"
	}
	sanitized := replacer.Replace(trimmed)
	if len(sanitized) <= 80 {
		return sanitized
	}
	sum := sha1.Sum([]byte(trimmed))
	return sanitized[:48] + "-" + hex.EncodeToString(sum[:4])
}
