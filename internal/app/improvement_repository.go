package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/skill"
)

func loadRepositoryImprovementInstructions(cfg *config.Service, workspaceDir string, repository string, skillName string) ([]skill.ManagedInstruction, error) {
	repoConfig, ok := resolveMonitoredRepository(cfg, repository)
	if !ok || !repoConfig.ImprovementEnabled {
		return nil, nil
	}

	phases := improvementPhasesForSkill(skillName)
	if len(phases) == 0 {
		return nil, nil
	}

	improvementsDir := artifacts.RepositoryWorkerImprovementsDir(workspaceDir, repoConfig.ImprovementDir)
	entries, err := filepath.Glob(filepath.Join(improvementsDir, "*.md"))
	if err != nil {
		return nil, err
	}

	out := make([]skill.ManagedInstruction, 0, len(entries))
	for _, entry := range entries {
		raw, err := os.ReadFile(entry)
		if err != nil {
			return nil, err
		}
		document, err := ParseImprovementMarkdown(raw)
		if err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(document.FrontMatter.Status), "active") {
			continue
		}
		if !instructionMatchesPhase(document.FrontMatter.Phases, phases) {
			continue
		}
		sourcePath, err := filepath.Rel(workspaceDir, entry)
		if err != nil {
			sourcePath = entry
		}
		out = append(out, skill.ManagedInstruction{
			ID:         document.FrontMatter.ID,
			Title:      document.FrontMatter.Title,
			Scope:      document.FrontMatter.Scope,
			Phases:     append([]string(nil), document.FrontMatter.Phases...),
			Status:     document.FrontMatter.Status,
			UpdatedAt:  document.FrontMatter.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			SourcePath: filepath.ToSlash(sourcePath),
			Body:       strings.TrimSpace(document.Body),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})

	return out, nil
}

func improvementPhasesForSkill(skillName string) []string {
	switch strings.ToLower(strings.TrimSpace(filepath.Base(skillName))) {
	case "design":
		return []string{"design"}
	case "implement":
		return []string{"implementation"}
	case "implement_fix":
		return []string{"fix"}
	case "review":
		return []string{"review"}
	case "review_fix":
		return []string{"fix"}
	default:
		return nil
	}
}

func instructionMatchesPhase(phases []string, currentPhases []string) bool {
	for _, phase := range phases {
		for _, current := range currentPhases {
			if strings.EqualFold(strings.TrimSpace(phase), strings.TrimSpace(current)) {
				return true
			}
		}
	}
	return false
}
