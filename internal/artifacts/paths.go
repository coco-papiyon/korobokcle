package artifacts

import "path/filepath"

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
