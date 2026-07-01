package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type GitHubSource struct {
	settings SettingsStore
	fallback string
	logger   githubLogger
}

type githubLogger interface {
	Infof(string, ...any)
	Debugf(string, ...any)
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghPRRecord struct {
	Number           int       `json:"number"`
	Title            string    `json:"title"`
	Labels           []ghLabel `json:"labels"`
	Author           ghUser    `json:"author"`
	Assignees        []ghUser  `json:"assignees"`
	URL              string    `json:"url"`
	IsDraft          bool      `json:"isDraft"`
	Mergeable        string    `json:"mergeable"`
	MergeStateStatus string    `json:"mergeStateStatus"`
	HeadRefName      string    `json:"headRefName"`
	BaseRefName      string    `json:"baseRefName"`
	Body             string    `json:"body"`
	Files            []struct {
		Path string `json:"path"`
	} `json:"files"`
	Comments []struct {
		Author ghUser `json:"author"`
		Body   string `json:"body"`
	} `json:"comments"`
}

func NewGitHubSource(settings SettingsStore, fallbackRepository string, logger githubLogger) *GitHubSource {
	return &GitHubSource{
		settings: settings,
		fallback: strings.TrimSpace(fallbackRepository),
		logger:   logger,
	}
}

func (s *GitHubSource) List(ctx context.Context) ([]domain.Job, error) {
	settings, repository, err := s.currentSettings(ctx)
	if err != nil {
		return nil, err
	}
	if repository == "" {
		return nil, nil
	}

	var jobs []domain.Job

	s.infof("github source: list issues repo=%s", repository)
	var issues []domain.Job
	if settings.Issue.IsEnabled() {
		var err error
		issues, err = s.listIssues(ctx, repository, settings.Issue, settings)
		if err != nil {
			s.infof("github source: list issues failed repo=%s err=%v", repository, err)
			return nil, err
		}
	} else {
		s.infof("github source: list issues skipped repo=%s", repository)
	}
	jobs = append(jobs, issues...)

	s.infof("github source: list pull requests repo=%s", repository)
	var prs []domain.Job
	if settings.PullRequest.IsEnabled() {
		var err error
		prs, err = s.listPullRequests(ctx, repository, settings.PullRequest)
		if err != nil {
			s.infof("github source: list pull requests failed repo=%s err=%v", repository, err)
			return nil, err
		}
	} else {
		s.infof("github source: list pull requests skipped repo=%s", repository)
	}
	jobs = append(jobs, prs...)
	s.infof("github source: list completed repo=%s issues=%d prs=%d targets=%d", repository, len(issues), len(prs), len(jobs))

	return jobs, nil
}

func (s *GitHubSource) currentSettings(ctx context.Context) (domain.WatchSettings, string, error) {
	if s.settings == nil {
		return domain.WatchSettings{}, s.fallback, nil
	}
	settings, err := s.settings.Load(ctx)
	if err != nil {
		return domain.WatchSettings{}, "", err
	}
	if strings.TrimSpace(settings.Repository) == "" {
		settings.Repository = s.fallback
	}
	return settings, strings.TrimSpace(settings.Repository), nil
}

func (s *GitHubSource) listIssues(ctx context.Context, repository string, rule domain.SearchCondition, settings domain.WatchSettings) ([]domain.Job, error) {
	type issueRecord struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		Labels    []ghLabel `json:"labels"`
		Author    ghUser    `json:"author"`
		Assignees []ghUser  `json:"assignees"`
		URL       string    `json:"url"`
	}

	cmd := exec.CommandContext(ctx, "gh", "issue", "list", "--repo", repository, "--state", "open", "--json", "number,title,body,labels,url,author,assignees")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}

	var records []issueRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decode gh issue list: %w", err)
	}

	jobs := make([]domain.Job, 0, len(records))
	for _, record := range records {
		labels := labelNames(record.Labels)
		assignees := loginNames(record.Assignees)
		s.debugIssueRecord(repository, record.Number, record.Title, labels, record.Author.Login, assignees)
		if hasLabel(labels, "state:pr_created") {
			continue
		}
		if !rule.Matches(record.Title, labels, record.Author.Login, assignees) {
			continue
		}
		kind, state := classifyIssue(labels)
		jobs = append(jobs, domain.Job{
			ID:           fmt.Sprintf("issue-%d", record.Number),
			Kind:         kind,
			State:        state,
			Repository:   repository,
			Number:       record.Number,
			Title:        record.Title,
			Branch:       renderBranchName(settings.BranchNamePattern, record.Number),
			IssueContext: formatIssueContext(record.Number, record.Title, record.Body),
		})
	}
	s.infof("github source: issues retrieved repo=%s fetched=%d targets=%d", repository, len(records), len(jobs))
	return jobs, nil
}

func (s *GitHubSource) listPullRequests(ctx context.Context, repository string, rule domain.SearchCondition) ([]domain.Job, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--repo", repository, "--state", "open", "--json", "number,title,labels,url,isDraft,author,assignees,mergeable,mergeStateStatus,headRefName,baseRefName")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}

	var records []ghPRRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decode gh pr list: %w", err)
	}

	jobs := make([]domain.Job, 0, len(records))
	for _, record := range records {
		if record.IsDraft {
			continue
		}
		labels := labelNames(record.Labels)
		assignees := loginNames(record.Assignees)
		s.debugPRRecord(repository, record.Number, record.Title, labels, record.Author.Login, assignees, record.IsDraft)
		if hasLabel(labels, "state:pr_conflict_resolved") {
			continue
		}
		if !rule.Matches(record.Title, labels, record.Author.Login, assignees) {
			continue
		}
		kind, state := classifyPullRequest(record)
		jobs = append(jobs, domain.Job{
			ID:         jobIDForPR(record),
			Kind:       kind,
			State:      state,
			Repository: repository,
			Number:     record.Number,
			Title:      record.Title,
			Branch:     strings.TrimSpace(record.HeadRefName),
		})
	}
	s.infof("github source: pull requests retrieved repo=%s fetched=%d targets=%d", repository, len(records), len(jobs))
	return jobs, nil
}

func classifyIssue(labels []string) (domain.JobKind, domain.JobState) {
	switch {
	case hasLabel(labels, "state:implementation_approved"):
		return domain.JobKindIssueImplementation, domain.StateImplementationApproved
	case hasLabel(labels, "state:design_approved"):
		return domain.JobKindIssueImplementation, domain.StateDesignApproved
	case hasLabel(labels, "state:review_fix_design_approved"):
		return domain.JobKindIssueImplementation, domain.StateReviewFixDesignApproved
	case hasLabel(labels, "state:review_fix_implementation_approved"):
		return domain.JobKindIssueImplementation, domain.StateReviewFixImplementationApproved
	default:
		return domain.JobKindIssueDesign, domain.StateDetected
	}
}

func classifyPullRequest(record ghPRRecord) (domain.JobKind, domain.JobState) {
	labels := labelNames(record.Labels)
	switch {
	case isConflictState(record.Mergeable, record.MergeStateStatus):
		return domain.JobKindPRConflict, domain.StatePRConflict
	case hasLabel(labels, "state:review_fixed"):
		return domain.JobKindPRReview, domain.StateReviewRunning
	case hasLabel(labels, "state:review_fix_implementation_approved"):
		return domain.JobKindPRFeedback, domain.StateReviewFixImplementationApproved
	case hasLabel(labels, "state:review_fix_implementation_ready"):
		return domain.JobKindPRFeedback, domain.StateReviewFixImplementationReady
	case hasLabel(labels, "state:review_fix_implementation_running"):
		return domain.JobKindPRFeedback, domain.StateReviewFixImplementationRunning
	case hasLabel(labels, "state:review_fix_design_approved"):
		return domain.JobKindPRFeedback, domain.StateReviewFixDesignApproved
	case hasLabel(labels, "state:pr_review_comment"):
		return domain.JobKindPRFeedback, domain.StatePRReviewComment
	default:
		return domain.JobKindPRReview, domain.StateReviewRunning
	}
}

func formatIssueContext(number int, title string, body string) string {
	lines := []string{
		fmt.Sprintf("#%d %s", number, title),
		"",
		strings.TrimSpace(body),
	}
	return strings.Join(lines, "\n")
}

func isConflictState(mergeable string, mergeStateStatus string) bool {
	switch strings.ToUpper(strings.TrimSpace(mergeable)) {
	case "CONFLICTING":
		return true
	}
	switch strings.ToUpper(strings.TrimSpace(mergeStateStatus)) {
	case "DIRTY":
		return true
	}
	return false
}

func jobIDForPR(record ghPRRecord) string {
	if isConflictState(record.Mergeable, record.MergeStateStatus) {
		return fmt.Sprintf("pr-conflict-%d", record.Number)
	}
	return fmt.Sprintf("pr-%d", record.Number)
}

func labelNames(labels []ghLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.TrimSpace(label.Name) == "" {
			continue
		}
		out = append(out, label.Name)
	}
	return out
}

func loginNames(items []ghUser) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Login) == "" {
			continue
		}
		out = append(out, item.Login)
	}
	return out
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), target) {
			return true
		}
	}
	return false
}

func (s *GitHubSource) infof(format string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Infof(format, args...)
}

func (s *GitHubSource) debugf(format string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Debugf(format, args...)
}

func (s *GitHubSource) debugIssueRecord(repository string, number int, title string, labels []string, author string, assignees []string) {
	s.debugf(
		"github source: issue repo=%s number=%d title=%q author=%q assignees=%v labels=%v",
		repository,
		number,
		title,
		author,
		assignees,
		labels,
	)
}

func (s *GitHubSource) debugPRRecord(repository string, number int, title string, labels []string, author string, assignees []string, draft bool) {
	s.debugf(
		"github source: pr repo=%s number=%d title=%q author=%q draft=%t assignees=%v labels=%v",
		repository,
		number,
		title,
		author,
		draft,
		assignees,
		labels,
	)
}
