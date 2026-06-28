package domain

import (
	"strings"
	"time"
)

type AIProvider string

const (
	AIProviderCodex         AIProvider = "codex"
	AIProviderGitHubCopilot AIProvider = "github_copilot"
)

type ModelMode string

const (
	ModelModeDefault ModelMode = "default"
	ModelModeCustom  ModelMode = "custom"
)

type SearchCondition struct {
	LabelIncludes []string `json:"labelIncludes,omitempty"`
	LabelExcludes []string `json:"labelExcludes,omitempty"`
	TitleContains []string `json:"titleContains,omitempty"`
	Authors       []string `json:"authors,omitempty"`
	Assignees     []string `json:"assignees,omitempty"`
}

type WatchSettings struct {
	Repository          string          `json:"repository"`
	AIProvider          AIProvider      `json:"aiProvider,omitempty"`
	PollIntervalSeconds int             `json:"pollIntervalSeconds,omitempty"`
	Models              AIModels        `json:"models,omitempty"`
	Issue               SearchCondition `json:"issue"`
	PullRequest         SearchCondition `json:"pullRequest"`
}

type ModelSelection struct {
	Mode  ModelMode `json:"mode"`
	Value string    `json:"value,omitempty"`
}

type AIModels struct {
	Codex         ModelSelection `json:"codex,omitempty"`
	GitHubCopilot ModelSelection `json:"githubCopilot,omitempty"`
}

func NormalizeWatchSettings(settings WatchSettings) WatchSettings {
	if !settings.AIProvider.IsValid() {
		settings.AIProvider = AIProviderCodex
	}
	if settings.PollIntervalSeconds <= 0 {
		settings.PollIntervalSeconds = 120
	}
	settings.Models.Codex = normalizeModelSelection(settings.Models.Codex)
	settings.Models.GitHubCopilot = normalizeModelSelection(settings.Models.GitHubCopilot)
	return settings
}

func (s WatchSettings) PollIntervalDuration() time.Duration {
	seconds := s.PollIntervalSeconds
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func normalizeModelSelection(selection ModelSelection) ModelSelection {
	if !selection.Mode.IsValid() {
		selection.Mode = ModelModeDefault
	}
	selection.Value = strings.TrimSpace(selection.Value)
	if selection.Mode == ModelModeDefault {
		selection.Value = ""
	}
	return selection
}

func (p AIProvider) IsValid() bool {
	switch p {
	case AIProviderCodex, AIProviderGitHubCopilot:
		return true
	default:
		return false
	}
}

func (p AIProvider) DisplayName() string {
	switch p {
	case AIProviderGitHubCopilot:
		return "GitHub Copilot"
	default:
		return "Codex"
	}
}

func (m ModelMode) IsValid() bool {
	switch m {
	case ModelModeDefault, ModelModeCustom:
		return true
	default:
		return false
	}
}

func (c SearchCondition) Matches(title string, labels []string, author string, assignees []string) bool {
	if !matchesAll(c.LabelIncludes, labels, true) {
		return false
	}
	if !matchesAll(c.LabelExcludes, labels, false) {
		return false
	}
	if len(c.TitleContains) > 0 && !containsAny(c.TitleContains, title) {
		return false
	}
	if len(c.Authors) > 0 && !containsAny(c.Authors, author) {
		return false
	}
	if len(c.Assignees) > 0 && !containsAny(c.Assignees, assignees...) {
		return false
	}
	return true
}

func matchesAll(expected []string, labels []string, mustExist bool) bool {
	if len(expected) == 0 {
		return true
	}
	labelSet := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		labelSet[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}
	for _, want := range expected {
		want = strings.ToLower(strings.TrimSpace(want))
		if want == "" {
			continue
		}
		_, exists := labelSet[want]
		if mustExist && !exists {
			return false
		}
		if !mustExist && exists {
			return false
		}
	}
	return true
}

func containsAny(needles []string, haystacks ...string) bool {
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}
		for _, haystack := range haystacks {
			if strings.Contains(strings.ToLower(haystack), needle) {
				return true
			}
		}
	}
	return false
}
