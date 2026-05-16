package skill

import "context"

type AIRequest struct {
	SkillName   string
	Prompt      string
	Model       string
	WorkDir     string
	ArtifactDir string
	OutputPath  string
}

type AIResult struct {
	Stdout string
	Stderr string
	Output string
}

type AIProvider interface {
	Run(ctx context.Context, req AIRequest) (AIResult, error)
}
