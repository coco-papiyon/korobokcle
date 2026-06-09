package app

import (
	"path/filepath"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
)

const (
	improvementInputFileName                = "input.md"
	improvementContextFileName              = "context.json"
	improvementDraftDirName                 = "draft"
	improvementDraftFileName                = "draft.md"
	improvementNotesFileName                = "notes.md"
	improvementImplementationPromptFileName = "implementation-prompt.md"
	improvementResultFileName               = "result.md"
	improvementApprovalFileName             = "approval.json"
	improvementDecisionFileName             = "decision.json"
)

type improvementWorkFiles struct {
	Dir                      string
	InputPath                string
	ContextPath              string
	DraftDir                 string
	DraftPath                string
	NotesPath                string
	ImplementationPromptPath string
	DecisionPath             string
}

type improvementArtifactFiles struct {
	Dir                      string
	InputPath                string
	ContextPath              string
	DraftDir                 string
	DraftPath                string
	PhasePath                string
	NotesPath                string
	ImplementationPromptPath string
	ResultPath               string
	ApprovalPath             string
	DecisionPath             string
}

func repositoryImprovementWorkFiles(workDir string, configuredDir string) improvementWorkFiles {
	dir := artifacts.RepositoryWorkerImprovementWorkDir(workDir, configuredDir)
	return improvementWorkFiles{
		Dir:                      dir,
		InputPath:                filepath.Join(dir, improvementInputFileName),
		ContextPath:              filepath.Join(dir, improvementContextFileName),
		DraftDir:                 filepath.Join(dir, improvementDraftDirName),
		DraftPath:                filepath.Join(dir, improvementDraftDirName, improvementDraftFileName),
		NotesPath:                filepath.Join(dir, improvementNotesFileName),
		ImplementationPromptPath: filepath.Join(dir, improvementImplementationPromptFileName),
		DecisionPath:             filepath.Join(dir, improvementDecisionFileName),
	}
}

func repositoryImprovementArtifactFiles(root string, artifactsDir string, repository string, issueNumber int) improvementArtifactFiles {
	dir := artifacts.RepositoryWorkerImprovementArtifactDir(root, artifactsDir, repository, issueNumber)
	return improvementArtifactFiles{
		Dir:                      dir,
		InputPath:                filepath.Join(dir, improvementInputFileName),
		ContextPath:              filepath.Join(dir, improvementContextFileName),
		DraftDir:                 filepath.Join(dir, improvementDraftDirName),
		DraftPath:                filepath.Join(dir, improvementDraftDirName, improvementDraftFileName),
		PhasePath:                "",
		NotesPath:                filepath.Join(dir, improvementNotesFileName),
		ImplementationPromptPath: filepath.Join(dir, improvementImplementationPromptFileName),
		ResultPath:               filepath.Join(dir, improvementResultFileName),
		ApprovalPath:             filepath.Join(dir, improvementApprovalFileName),
		DecisionPath:             filepath.Join(dir, improvementDecisionFileName),
	}
}
