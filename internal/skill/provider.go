package skill

import "context"

type AIRequest struct {
	SkillName         string
	Prompt            string
	Model             string
	WorkDir           string
	ArtifactDir       string
	OutputPath        string
	SessionID         string
	CopilotAllowTools []string
}

type AIResult struct {
	Stdout    string
	Stderr    string
	Output    string
	SessionID string
	JSON      string
}

type AIProvider interface {
	Run(ctx context.Context, req AIRequest) (AIResult, error)
}
