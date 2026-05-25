package artifacts

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	WorkerDesign         = "design"
	WorkerImplementation = "implementation"
	WorkerFix            = "fix"
	WorkerPR             = "pr"
	WorkerReview         = "review"
)

func JobDir(root string, artifactsDir string, jobID string) string {
	return filepath.Join(resolveAgainstRoot(root, artifactsDir), "jobs", jobID)
}

func WorkerDir(root string, artifactsDir string, jobID string, worker string) string {
	return filepath.Join(JobDir(root, artifactsDir, jobID), worker)
}

func WorkersDir(root string, artifactsDir string) string {
	return filepath.Join(resolveAgainstRoot(root, artifactsDir), "workers")
}

func RepositoryWorkerDir(root string, artifactsDir string, repository string, workerIndex int) string {
	sanitized := repositoryWorkerComponent(repository)
	return filepath.Join(WorkersDir(root, artifactsDir), sanitized, workerName(workerIndex))
}

func RepositoryWorkerSourceDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return filepath.Join(RepositoryWorkerDir(root, artifactsDir, repository, workerIndex), "source")
}

func RepositoryWorkerLogsDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return filepath.Join(RepositoryWorkerDir(root, artifactsDir, repository, workerIndex), "logs")
}

func RepositoryWorkerLogPath(root string, artifactsDir string, repository string, workerIndex int, startedAt time.Time) string {
	dateDir := startedAt.Format("2006-01-02")
	fileName := startedAt.Format("2006-01-02_15-04-05") + ".log"
	return filepath.Join(RepositoryWorkerLogsDir(root, artifactsDir, repository, workerIndex), dateDir, fileName)
}

func resolveAgainstRoot(root string, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(root, target)
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

func repositoryWorkerComponent(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return "repository"
	}
	trimmed = strings.TrimSuffix(trimmed, ".git")

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		if component := ownerRepoComponent(strings.Trim(parsed.Path, "/")); component != "" {
			return sanitizePathComponent(component)
		}
	}

	if component := ownerRepoComponent(trimmed); component != "" {
		return sanitizePathComponent(component)
	}

	if component := ownerRepoComponent(extractRepositoryPath(trimmed)); component != "" {
		return sanitizePathComponent(component)
	}

	return sanitizePathComponent(trimmed)
}

func ownerRepoComponent(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[len(parts)-2:], "-")
}

func extractRepositoryPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(trimmed, "git@") {
		if idx := strings.LastIndex(trimmed, ":"); idx >= 0 && idx+1 < len(trimmed) {
			candidate := trimmed[idx+1:]
			return strings.TrimSuffix(candidate, ".git")
		}
	}
	if strings.Contains(trimmed, "://") {
		if parsed, err := url.Parse(trimmed); err == nil {
			return strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
		}
	}
	cleaned := path.Clean(trimmed)
	cleaned = strings.TrimSuffix(cleaned, ".git")
	return strings.Trim(cleaned, "/")
}
