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
	WorkerImprovement    = "improvement"
)

const (
	defaultImprovementsDirName          = ".improvement"
	defaultImprovementWorkDirName       = ".improvement"
	defaultImprovementWorkspaceDirName  = "improvement"
	defaultImprovementRepositoryDirName = ".improvement-repository"
	defaultSourceDirName                = "source"
)

func repositoryArtifactsRoot(root string, artifactsDir string, repository string) string {
	return filepath.Join(resolveAgainstRoot(root, artifactsDir), repositoryWorkerComponent(repository))
}

func repositorySourceRoot(root string) string {
	return resolveAgainstRoot(root, defaultSourceDirName)
}

func JobDir(root string, artifactsDir string, jobID string) string {
	return filepath.Join(resolveAgainstRoot(root, artifactsDir), "jobs", jobID)
}

func WorkerDir(root string, artifactsDir string, jobID string, worker string) string {
	return filepath.Join(JobDir(root, artifactsDir, jobID), worker)
}

func WorkersDir(root string, artifactsDir string) string {
	return resolveAgainstRoot(root, artifactsDir)
}

func RepositoryWorkerDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return filepath.Join(repositoryArtifactsRoot(root, artifactsDir, repository), "workers", workerName(workerIndex))
}

func RepositoryWorkerWorkDir(root string, artifactsDir string, repository string, configuredWorkDir string) string {
	trimmed := strings.TrimSpace(configuredWorkDir)
	if trimmed == "" || trimmed == "." {
		return filepath.Join(repositorySourceRoot(root), repositoryWorkerComponent(repository))
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return resolveAgainstRoot(root, trimmed)
}

func RepositoryWorkerBaseDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return RepositoryWorkerWorkDir(root, artifactsDir, repository, "")
}

func RepositoryWorkerBranchDir(baseDir string, branch string) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		trimmed = "main"
	}
	base := filepath.Base(baseDir)
	parent := filepath.Dir(baseDir)
	return filepath.Join(parent, base+"-"+sanitizePathComponent(trimmed))
}

func RepositoryWorkerBranchWorkDir(root string, artifactsDir string, repository string, branch string) string {
	return RepositoryWorkerBranchDir(RepositoryWorkerWorkDir(root, artifactsDir, repository, ""), branch)
}

func RepositoryWorkerSourceDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return RepositoryWorkerWorkDir(root, artifactsDir, repository, "")
}

func RepositoryWorkerJobDir(root string, artifactsDir string, repository string, issueNumber int) string {
	return filepath.Join(repositoryArtifactsRoot(root, artifactsDir, repository), "jobs", fmt.Sprintf("issue_%d", issueNumber))
}

func RepositoryWorkerJobPhaseDir(root string, artifactsDir string, repository string, issueNumber int, phase string) string {
	return filepath.Join(RepositoryWorkerJobDir(root, artifactsDir, repository, issueNumber), phase)
}

func RepositoryWorkerWorkArtifactDir(workDir string, phase string) string {
	return filepath.Join(workDir, phase)
}

func RepositoryWorkerWorkArtifactFileName(issueNumber int, title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Sprintf("issue_%d.md", issueNumber)
	}
	return fmt.Sprintf("issue_%d_%s.md", issueNumber, sanitizePathComponent(trimmed))
}

func RepositoryWorkerWorkArtifactPath(workDir string, phase string, issueNumber int, title string) string {
	return filepath.Join(RepositoryWorkerWorkArtifactDir(workDir, phase), RepositoryWorkerWorkArtifactFileName(issueNumber, title))
}

func RepositoryWorkerImprovementsDir(workDir string, configuredDir string) string {
	return resolveSubdirAgainstBase(workDir, configuredDir, defaultImprovementsDirName)
}

func RepositoryWorkerImprovementRepositoryDir(root string, artifactsDir string, repository string) string {
	return filepath.Join(repositoryArtifactsRoot(root, artifactsDir, repository), defaultImprovementRepositoryDirName)
}

func RepositoryWorkerImprovementWorkspaceDir(root string, artifactsDir string, repository string, branch string) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		trimmed = "improvement"
	}
	return RepositoryWorkerBranchWorkDir(root, artifactsDir, repository, trimmed)
}

func RepositoryWorkerImprovementWorkDir(workDir string, configuredDir string) string {
	return resolveSubdirAgainstBase(workDir, configuredDir, defaultImprovementWorkDirName)
}

func RepositoryWorkerImprovementDraftFileName(identifier string, title string) string {
	trimmedID := strings.TrimSpace(identifier)
	if trimmedID == "" {
		trimmedID = "improvement"
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return trimmedID + ".md"
	}
	return trimmedID + "_" + sanitizePathComponent(trimmedTitle) + ".md"
}

func RepositoryWorkerImprovementDraftFilePath(workDir string, configuredDir string, identifier string, title string) string {
	return filepath.Join(
		RepositoryWorkerImprovementWorkDir(workDir, configuredDir),
		"draft",
		RepositoryWorkerImprovementDraftFileName(identifier, title),
	)
}

func RepositoryWorkerImprovementWorkFile(workDir string, configuredDir string, name string) string {
	return filepath.Join(RepositoryWorkerImprovementWorkDir(workDir, configuredDir), name)
}

func RepositoryWorkerImprovementsFile(workDir string, configuredDir string, name string) string {
	return filepath.Join(RepositoryWorkerImprovementsDir(workDir, configuredDir), name)
}

func RepositoryWorkerImprovementPhaseFile(workDir string, configuredDir string, phase string) string {
	trimmed := strings.TrimSpace(phase)
	if trimmed == "" {
		trimmed = "phase"
	}
	return filepath.Join(RepositoryWorkerImprovementWorkDir(workDir, configuredDir), trimmed+".md")
}

func RepositoryWorkerImprovementArtifactDir(root string, artifactsDir string, repository string, issueNumber int) string {
	return RepositoryWorkerJobPhaseDir(root, artifactsDir, repository, issueNumber, WorkerImprovement)
}

func RepositoryWorkerImprovementArtifactFile(root string, artifactsDir string, repository string, issueNumber int, name string) string {
	return filepath.Join(RepositoryWorkerImprovementArtifactDir(root, artifactsDir, repository, issueNumber), name)
}

func RepositoryWorkerWorkspaceDir(workerDir string, workspaceDir string) string {
	trimmed := strings.TrimSpace(workspaceDir)
	if trimmed == "" {
		trimmed = ".workspace"
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	if trimmed == "." {
		trimmed = ".workspace"
	}
	return filepath.Join(workerDir, trimmed)
}

func RepositoryWorkerIssueDir(workerDir string, workspaceDir string, issueNumber int) string {
	return filepath.Join(RepositoryWorkerWorkspaceDir(workerDir, workspaceDir), fmt.Sprintf("issue_%d", issueNumber))
}

func RepositoryWorkerArtifactDir(workerDir string, workspaceDir string, issueNumber int, phase string) string {
	return filepath.Join(RepositoryWorkerIssueDir(workerDir, workspaceDir, issueNumber), phase)
}

func RepositoryWorkerLogsDir(root string, artifactsDir string, repository string, workerIndex int) string {
	return filepath.Join(RepositoryWorkerDir(root, artifactsDir, repository, workerIndex), "logs")
}

func RepositoryWorkerLogPath(root string, artifactsDir string, repository string, workerIndex int, startedAt time.Time) string {
	dateDir := startedAt.Format("2006-01-02")
	fileName := startedAt.Format("2006-01-02_15-04-05") + ".log"
	return filepath.Join(RepositoryWorkerLogsDir(root, artifactsDir, repository, workerIndex), dateDir, fileName)
}

func RepositoryWorkerLogsDirFromWorkerDir(workerDir string, workspaceDir string) string {
	return filepath.Join(RepositoryWorkerWorkspaceDir(workerDir, workspaceDir), "logs")
}

func RepositoryWorkerLogPathFromWorkerDir(workerDir string, workspaceDir string, startedAt time.Time) string {
	dateDir := startedAt.Format("2006-01-02")
	fileName := startedAt.Format("2006-01-02_15-04-05") + ".log"
	return filepath.Join(RepositoryWorkerLogsDirFromWorkerDir(workerDir, workspaceDir), dateDir, fileName)
}

func resolveAgainstRoot(root string, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(root, target)
}

func resolveSubdirAgainstBase(base string, configured string, defaultName string) string {
	trimmed := strings.TrimSpace(configured)
	if trimmed == "" || trimmed == "." {
		trimmed = defaultName
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return filepath.Join(base, trimmed)
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
