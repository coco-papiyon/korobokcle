package naming

import (
	"strconv"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

const (
	DefaultPRTitleTemplate = "[#{{issue_number}}]{{issue_title}}"
	DefaultBranchTemplate  = "issue_{{issue_number}}"
)

func RenderPRTitle(template string, job domain.Job) string {
	rendered := renderTemplate(template, job)
	if strings.TrimSpace(rendered) == "" {
		return renderTemplate(DefaultPRTitleTemplate, job)
	}
	return rendered
}

func RenderBranchName(template string, item domain.RepositoryItem) string {
	rendered := renderTemplate(template, domain.Job{
		Repository:   item.Repository,
		GitHubNumber: item.Number,
		Title:        item.Title,
		BranchName:   "",
	})
	if strings.TrimSpace(rendered) == "" {
		return renderTemplate(DefaultBranchTemplate, domain.Job{
			Repository:   item.Repository,
			GitHubNumber: item.Number,
			Title:        item.Title,
		})
	}
	return rendered
}

func renderTemplate(template string, job domain.Job) string {
	value := strings.TrimSpace(template)
	replacer := strings.NewReplacer(
		"{{issue_number}}", strconv.Itoa(job.GitHubNumber),
		"{{issue_title}}", job.Title,
		"{{repository}}", job.Repository,
		"{{branch_name}}", job.BranchName,
	)
	return replacer.Replace(value)
}
