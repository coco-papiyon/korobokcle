package domain

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func EvaluateWatchRule(rule config.WatchRule, item RepositoryItem) MatchResult {
	if !rule.Enabled {
		return MatchResult{Status: MatchStatusIgnored, Reason: "rule_disabled"}
	}

	if !repositoryMatches(rule.Repositories, item.Repository) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "repository_mismatch"}
	}

	if !targetMatches(rule.Target, item.Target) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "target_mismatch"}
	}

	if rule.ExcludeDraftPR && item.Target == TargetPullRequest && item.Draft {
		return MatchResult{Status: MatchStatusIgnored, Reason: "draft_pull_request"}
	}

	if rule.TitlePattern != "" {
		matched, err := regexp.MatchString(rule.TitlePattern, item.Title)
		if err != nil || !matched {
			return MatchResult{Status: MatchStatusIgnored, Reason: "title_pattern_mismatch"}
		}
	}

	if len(rule.Authors) > 0 && !containsFold(rule.Authors, item.Author) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "author_mismatch"}
	}

	if len(rule.Assignees) > 0 && !anyContainsFold(rule.Assignees, item.Assignees) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "assignee_mismatch"}
	}

	if len(rule.Labels) > 0 && !allContainsFold(item.Labels, rule.Labels) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "label_mismatch"}
	}

	if strings.EqualFold(strings.TrimSpace(rule.Target), string(TargetIssueProject)) &&
		!projectMatches(strings.TrimSpace(rule.ProjectName), rule.ProjectFilters, item.ProjectCards) {
		return MatchResult{Status: MatchStatusIgnored, Reason: "project_filter_mismatch"}
	}

	return MatchResult{Status: MatchStatusMatched, Reason: "matched"}
}

func targetMatches(ruleTarget string, itemTarget MonitoredTarget) bool {
	switch strings.TrimSpace(ruleTarget) {
	case string(TargetIssue):
		return itemTarget == TargetIssue
	case string(TargetIssueProject):
		return itemTarget == TargetIssueProject
	case string(TargetPullRequest):
		return itemTarget == TargetPullRequest
	default:
		return false
	}
}

func projectMatches(projectName string, filters []config.ProjectFieldFilter, cards []ProjectCard) bool {
	if len(cards) == 0 {
		return false
	}

	for _, card := range cards {
		if projectName != "" && !strings.EqualFold(strings.TrimSpace(projectName), strings.TrimSpace(card.Project)) {
			continue
		}
		if projectFieldsMatch(filters, card.Fields) {
			return true
		}
	}
	return false
}

func projectFieldsMatch(filters []config.ProjectFieldFilter, fields []ProjectField) bool {
	for _, filter := range filters {
		if !projectFieldMatches(filter, fields) {
			return false
		}
	}
	return true
}

func projectFieldMatches(filter config.ProjectFieldFilter, fields []ProjectField) bool {
	name := strings.TrimSpace(filter.Field)
	if name == "" {
		return true
	}
	for _, field := range fields {
		if !strings.EqualFold(name, strings.TrimSpace(field.Name)) {
			continue
		}
		if len(filter.Values) == 0 {
			return true
		}
		for _, candidate := range filter.Values {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(field.Value)) {
				return true
			}
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func anyContainsFold(expected []string, actual []string) bool {
	for _, candidate := range actual {
		if containsFold(expected, candidate) {
			return true
		}
	}
	return false
}

func allContainsFold(actual []string, expected []string) bool {
	for _, value := range expected {
		if !containsFold(actual, value) {
			return false
		}
	}
	return true
}

func repositoryMatches(configured []string, actual string) bool {
	normalizedActual, ok := normalizeRepository(actual)
	if !ok {
		normalizedActual = strings.TrimSpace(actual)
	}

	for _, candidate := range configured {
		normalizedCandidate, ok := normalizeRepository(candidate)
		if !ok {
			normalizedCandidate = strings.TrimSpace(candidate)
		}
		if normalizedCandidate == normalizedActual {
			return true
		}
	}
	return false
}

func normalizeRepository(repository string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(repository, "/"))
	if trimmed == "" {
		return "", false
	}

	if strings.HasPrefix(trimmed, "https://github.com/") || strings.HasPrefix(trimmed, "http://github.com/") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", false
		}
		trimmed = strings.TrimPrefix(u.Path, "/")
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}
