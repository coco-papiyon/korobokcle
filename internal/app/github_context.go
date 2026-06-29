package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
