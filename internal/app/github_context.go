package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobContextLoader interface {
	Load(context.Context, domain.Job) (string, error)
}

type GitHubJobContextLoader struct{}

func (l *GitHubJobContextLoader) Load(ctx context.Context, job domain.Job) (string, error) {
	switch job.Kind {
	case domain.JobKindPRReview, domain.JobKindPRFeedback:
		return loadPRContext(ctx, job)
	case domain.JobKindPRConflict:
		return loadPRConflictContext(ctx, job)
	default:
		return loadIssueContext(ctx, job)
	}
}

func loadIssueContext(ctx context.Context, job domain.Job) (string, error) {
	raw, err := runGHJSON(ctx, "issue", "view", "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "title,body,labels,comments")
	if err != nil {
		return "", err
	}
	var issue struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(raw, &issue); err != nil {
		return "", fmt.Errorf("decode issue context: %w", err)
	}
	lines := []string{
		fmt.Sprintf("Issue: #%d %s", job.Number, issue.Title),
		"",
		"Body:",
		strings.TrimSpace(issue.Body),
	}
	if len(issue.Labels) > 0 {
		var labels []string
		for _, label := range issue.Labels {
			labels = append(labels, label.Name)
		}
		lines = append(lines, "", "Labels:", strings.Join(labels, ", "))
	}
	if len(issue.Comments) > 0 {
		lines = append(lines, "", "Comments:")
		for _, comment := range issue.Comments {
			lines = append(lines, fmt.Sprintf("- %s: %s", comment.Author.Login, oneLine(comment.Body)))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func loadPRContext(ctx context.Context, job domain.Job) (string, error) {
	raw, err := runGHJSON(ctx, "pr", "view", "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "title,body,labels,comments,files")
	if err != nil {
		return "", err
	}
	var pr struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(raw, &pr); err != nil {
		return "", fmt.Errorf("decode PR context: %w", err)
	}
	lines := []string{
		fmt.Sprintf("Pull Request: #%d %s", job.Number, pr.Title),
		"",
		"Body:",
		strings.TrimSpace(pr.Body),
	}
	if len(pr.Labels) > 0 {
		var labels []string
		for _, label := range pr.Labels {
			labels = append(labels, label.Name)
		}
		lines = append(lines, "", "Labels:", strings.Join(labels, ", "))
	}
	if len(pr.Files) > 0 {
		lines = append(lines, "", "Files:")
		for _, file := range pr.Files {
			lines = append(lines, "- "+file.Path)
		}
	}
	if len(pr.Comments) > 0 {
		lines = append(lines, "", "Comments:")
		for _, comment := range pr.Comments {
			lines = append(lines, fmt.Sprintf("- %s: %s", comment.Author.Login, oneLine(comment.Body)))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func loadPRConflictContext(ctx context.Context, job domain.Job) (string, error) {
	raw, err := runGHJSON(ctx, "pr", "view", "--repo", job.Repository, fmt.Sprintf("%d", job.Number), "--json", "title,body,labels,comments,files,headRefName,baseRefName,mergeable,mergeStateStatus")
	if err != nil {
		return "", err
	}
	var pr ghPRRecord
	if err := json.Unmarshal(raw, &pr); err != nil {
		return "", fmt.Errorf("decode PR conflict context: %w", err)
	}
	lines := []string{
		fmt.Sprintf("Pull Request: #%d %s", job.Number, pr.Title),
		"",
		"Head branch:",
		branchOrDefault(pr.HeadRefName),
		"",
		"Base branch:",
		branchOrDefault(pr.BaseRefName),
		"",
		"Mergeable:",
		strings.TrimSpace(pr.Mergeable),
		"",
		"Merge state:",
		strings.TrimSpace(pr.MergeStateStatus),
		"",
		"Body:",
		strings.TrimSpace(pr.Body),
	}
	if len(pr.Labels) > 0 {
		var labels []string
		for _, label := range pr.Labels {
			labels = append(labels, label.Name)
		}
		lines = append(lines, "", "Labels:", strings.Join(labels, ", "))
	}
	if len(pr.Files) > 0 {
		lines = append(lines, "", "Files:")
		for _, file := range pr.Files {
			lines = append(lines, "- "+file.Path)
		}
	}
	if len(pr.Comments) > 0 {
		lines = append(lines, "", "Comments:")
		for _, comment := range pr.Comments {
			lines = append(lines, fmt.Sprintf("- %s: %s", comment.Author.Login, oneLine(comment.Body)))
		}
	}
	if issueNumber := branchIssueNumber(pr.HeadRefName); issueNumber > 0 {
		if issueText, err := loadIssueContextByNumber(ctx, job.Repository, issueNumber); err == nil {
			lines = append(lines, "", "Head issue:", issueText)
		}
	}
	if issueNumber := branchIssueNumber(pr.BaseRefName); issueNumber > 0 {
		if issueText, err := loadIssueContextByNumber(ctx, job.Repository, issueNumber); err == nil {
			lines = append(lines, "", "Base issue:", issueText)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func loadIssueContextByNumber(ctx context.Context, repository string, issueNumber int) (string, error) {
	raw, err := runGHJSON(ctx, "issue", "view", "--repo", repository, fmt.Sprintf("%d", issueNumber), "--json", "title,body,labels,comments")
	if err != nil {
		return "", err
	}
	var issue struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(raw, &issue); err != nil {
		return "", fmt.Errorf("decode issue context: %w", err)
	}
	lines := []string{
		fmt.Sprintf("Issue: #%d %s", issueNumber, issue.Title),
		"",
		"Body:",
		strings.TrimSpace(issue.Body),
	}
	if len(issue.Labels) > 0 {
		var labels []string
		for _, label := range issue.Labels {
			labels = append(labels, label.Name)
		}
		lines = append(lines, "", "Labels:", strings.Join(labels, ", "))
	}
	if len(issue.Comments) > 0 {
		lines = append(lines, "", "Comments:")
		for _, comment := range issue.Comments {
			lines = append(lines, fmt.Sprintf("- %s: %s", comment.Author.Login, oneLine(comment.Body)))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func branchOrDefault(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "(unknown)"
	}
	return branch
}

func branchIssueNumber(branch string) int {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return 0
	}
	re := regexp.MustCompile(`(?:^|[/_-])issue[#_-]*(\d+)`)
	match := re.FindStringSubmatch(strings.ToLower(branch))
	if len(match) != 2 {
		return 0
	}
	var issueNumber int
	_, _ = fmt.Sscanf(match[1], "%d", &issueNumber)
	return issueNumber
}

func runGHJSON(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func oneLine(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\n", " / ")
	return value
}
