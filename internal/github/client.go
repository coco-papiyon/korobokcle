package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	tokenSrc   TokenProvider
	ghCommand  string
	info       *log.Logger
	debug      *log.Logger
}

type PRComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type PRCommentsArtifact struct {
	PullNumber int         `json:"pullNumber"`
	Comments   []PRComment `json:"comments"`
}

func NewClient(tokenSrc TokenProvider, debug *log.Logger) *Client {
	return &Client{
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		tokenSrc:  tokenSrc,
		ghCommand: "gh",
		debug:     debug,
	}
}

func (c *Client) WithInfoLogger(info *log.Logger) *Client {
	c.info = info
	return c
}

func (c *Client) ListIssues(ctx context.Context, rule config.WatchRule, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	if domain.IsDummyRepository(repository) {
		return []domain.RepositoryItem{}, nil
	}
	return c.listRepositoryItems(ctx, rule, repository, domain.TargetIssue, since)
}

func (c *Client) ListProjectIssues(ctx context.Context, rule config.WatchRule, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	if domain.IsDummyRepository(repository) {
		return []domain.RepositoryItem{}, nil
	}
	items, err := c.listRepositoryItems(ctx, rule, repository, domain.TargetIssue, since)
	if err != nil {
		return nil, err
	}

	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return nil, err
	}

	projectItems := make([]domain.RepositoryItem, 0, len(items))
	for _, item := range items {
		cards, err := c.loadProjectCards(ctx, normalizedRepository, item.Number)
		if err != nil {
			return nil, err
		}
		if len(cards) == 0 {
			continue
		}
		item.Target = domain.TargetIssueProject
		item.ProjectCards = cards
		projectItems = append(projectItems, item)
	}
	return projectItems, nil
}

func (c *Client) ListPullRequests(ctx context.Context, rule config.WatchRule, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	if domain.IsDummyRepository(repository) {
		return []domain.RepositoryItem{}, nil
	}
	return c.listRepositoryItems(ctx, rule, repository, domain.TargetPullRequest, since)
}

func (c *Client) FetchIssueBody(ctx context.Context, repository string, issueNumber int) (string, error) {
	if domain.IsDummyRepository(repository) {
		return "", nil
	}
	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return "", err
	}
	if issueNumber < 1 {
		return "", fmt.Errorf("issue number must be positive")
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return "", err
	}

	rawURL := fmt.Sprintf("%s/repos/%s/issues/%d", c.baseURL, normalizedRepository, issueNumber)
	c.infof("github api start name=repos/issues repository=%s issue_number=%d", normalizedRepository, issueNumber)
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=repos/issues repository=%s issue_number=%d status=%d error=http_status", normalizedRepository, issueNumber, resp.StatusCode)
		return "", fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload apiIssue
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	c.infof("github api done name=repos/issues repository=%s issue_number=%d status=%d", normalizedRepository, issueNumber, resp.StatusCode)
	return payload.Body, nil
}

func (c *Client) FetchPullRequestComments(ctx context.Context, repository string, pullNumber int, artifactDir string) (PRCommentsArtifact, error) {
	if domain.IsDummyRepository(repository) {
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return PRCommentsArtifact{}, err
		}
		artifact := PRCommentsArtifact{PullNumber: pullNumber, Comments: []PRComment{}}
		rawArtifact, err := json.MarshalIndent(artifact, "", "  ")
		if err != nil {
			return PRCommentsArtifact{}, err
		}
		if err := os.WriteFile(filepath.Join(artifactDir, "gh-pr-comments.json"), rawArtifact, 0o644); err != nil {
			return PRCommentsArtifact{}, err
		}
		if err := os.WriteFile(filepath.Join(artifactDir, "gh-pr-comments.log"), []byte("dummy repository: skipped pull request comment fetch"), 0o644); err != nil {
			return PRCommentsArtifact{}, err
		}
		return artifact, nil
	}
	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	if pullNumber < 1 {
		return PRCommentsArtifact{}, fmt.Errorf("pull number must be positive")
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return PRCommentsArtifact{}, err
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	rawURL := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.baseURL, normalizedRepository, pullNumber)
	c.infof("github api start name=repos/issues/comments repository=%s pull_number=%d", normalizedRepository, pullNumber)
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))
	if resp.StatusCode >= 300 {
		c.infof("github api done name=repos/issues/comments repository=%s pull_number=%d status=%d error=http_status", normalizedRepository, pullNumber, resp.StatusCode)
		return PRCommentsArtifact{}, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []struct {
		Body      string `json:"body"`
		HTMLURL   string `json:"html_url"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return PRCommentsArtifact{}, err
	}

	comments := make([]PRComment, 0, len(payload))
	for _, comment := range payload {
		comments = append(comments, PRComment{
			Author:    comment.User.Login,
			Body:      comment.Body,
			URL:       comment.HTMLURL,
			CreatedAt: comment.CreatedAt,
		})
	}
	artifact := PRCommentsArtifact{PullNumber: pullNumber, Comments: comments}
	rawArtifact, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return PRCommentsArtifact{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "gh-pr-comments.json"), rawArtifact, 0o644); err != nil {
		return PRCommentsArtifact{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "gh-pr-comments.log"), body, 0o644); err != nil {
		return PRCommentsArtifact{}, err
	}
	c.infof("github api done name=repos/issues/comments repository=%s pull_number=%d status=%d comments=%d", normalizedRepository, pullNumber, resp.StatusCode, len(comments))
	return artifact, nil
}

func (c *Client) ListPullRequestReviews(ctx context.Context, rule config.WatchRule, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	if domain.IsDummyRepository(repository) {
		return []domain.RepositoryItem{}, nil
	}
	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return nil, err
	}

	pulls, err := c.listAPIItems(ctx, normalizedRepository, "pulls", time.Time{})
	if err != nil {
		return nil, err
	}
	openPRs := make(map[int]apiItem, len(pulls))
	for _, pull := range pulls {
		if !strings.EqualFold(strings.TrimSpace(pull.Head.Repo.FullName), normalizedRepository) {
			continue
		}
		openPRs[pull.Number] = pull
	}
	if len(openPRs) == 0 {
		return []domain.RepositoryItem{}, nil
	}

	comments, err := c.listPullRequestReviewComments(ctx, normalizedRepository, since)
	if err != nil {
		return nil, err
	}

	grouped := make(map[int][]domain.ReviewComment, len(openPRs))
	latest := make(map[int]time.Time, len(openPRs))
	matchedReviewIDs := make(map[int]map[int64]struct{}, len(openPRs))
	for pullNumber, pull := range openPRs {
		reviews, err := c.listPullRequestReviews(ctx, normalizedRepository, pullNumber, since)
		if err != nil {
			return nil, err
		}
		if len(reviews) > 0 {
			matchedReviewIDs[pullNumber] = make(map[int64]struct{}, len(reviews))
		}
		for _, review := range reviews {
			matchedReviewIDs[pullNumber][review.ID] = struct{}{}
			if strings.TrimSpace(review.Body) != "" {
				grouped[pullNumber] = append(grouped[pullNumber], domain.ReviewComment{
					ID:        review.ID,
					Author:    review.User.Login,
					Body:      review.Body,
					URL:       review.HTMLURL,
					CreatedAt: review.SubmittedAt,
					UpdatedAt: review.SubmittedAt,
				})
			}
			if review.SubmittedAt.After(latest[pullNumber]) {
				latest[pullNumber] = review.SubmittedAt
			}
		}
		if pull.UpdatedAt.After(latest[pullNumber]) {
			latest[pullNumber] = pull.UpdatedAt
		}
	}
	for _, comment := range comments {
		pullNumber := pullRequestNumberFromURL(comment.PullRequestURL)
		if _, ok := openPRs[pullNumber]; !ok {
			continue
		}
		reviewIDs := matchedReviewIDs[pullNumber]
		if len(reviewIDs) == 0 {
			continue
		}
		if _, ok := reviewIDs[comment.PullRequestReviewID]; !ok {
			continue
		}
		grouped[pullNumber] = append(grouped[pullNumber], domain.ReviewComment{
			ID:        comment.ID,
			Author:    comment.User.Login,
			Body:      comment.Body,
			Path:      comment.Path,
			Line:      comment.Line,
			URL:       comment.HTMLURL,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
		})
		if comment.UpdatedAt.After(latest[pullNumber]) {
			latest[pullNumber] = comment.UpdatedAt
		}
	}

	items := make([]domain.RepositoryItem, 0, len(openPRs))
	for pullNumber, pull := range openPRs {
		reviewComments := grouped[pullNumber]
		reviewers := make([]string, 0, len(pull.Reviewers))
		for _, reviewer := range pull.Reviewers {
			reviewers = append(reviewers, reviewer.Login)
		}

		if len(reviewComments) == 0 && !anyReviewerMatches(rule.Reviewers, reviewers) {
			continue
		}
		item := pull.toDomain(normalizedRepository, "pulls")
		if strings.TrimSpace(item.BranchName) == "" || strings.TrimSpace(item.BaseBranch) == "" {
			headRef, baseRef, err := c.resolvePullRequestRefs(ctx, normalizedRepository, pullNumber)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(item.BranchName) == "" {
				item.BranchName = headRef
			}
			if strings.TrimSpace(item.BaseBranch) == "" {
				item.BaseBranch = baseRef
			}
		}
		item.Target = domain.TargetPullRequestReview
		item.DefaultState = domain.StateImplementationRunning
		item.ReviewComments = reviewComments
		item.UpdatedAt = latest[pullNumber]
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) listRepositoryItems(ctx context.Context, rule config.WatchRule, repository string, target domain.MonitoredTarget, since time.Time) ([]domain.RepositoryItem, error) {
	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return nil, err
	}

	payload, err := c.searchAPIItems(ctx, normalizedRepository, target, rule, since)
	if err != nil {
		return nil, err
	}

	items := make([]domain.RepositoryItem, 0, len(payload))
	for _, item := range payload {
		domainItem := item.toDomain(normalizedRepository, searchEndpointForTarget(target))
		if target == domain.TargetPullRequest {
			if strings.TrimSpace(domainItem.BranchName) == "" || strings.TrimSpace(domainItem.BaseBranch) == "" {
				headRef, baseRef, err := c.resolvePullRequestRefs(ctx, normalizedRepository, domainItem.Number)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(domainItem.BranchName) == "" {
					domainItem.BranchName = headRef
				}
				if strings.TrimSpace(domainItem.BaseBranch) == "" {
					domainItem.BaseBranch = baseRef
				}
			}
		}
		if target == domain.TargetPullRequest && len(rule.Reviewers) > 0 {
			reviewers, err := c.loadPullRequestRequestedReviewers(ctx, normalizedRepository, domainItem.Number)
			if err != nil {
				return nil, err
			}
			if len(reviewers) > 0 {
				domainItem.Reviewers = reviewers
			}
		}
		items = append(items, domainItem)
	}
	return items, nil
}

func (c *Client) resolvePullRequestRefs(ctx context.Context, repository string, pullNumber int) (string, string, error) {
	ghCmd := strings.TrimSpace(c.ghCommand)
	if ghCmd == "" {
		ghCmd = "gh"
	}

	cmd := exec.CommandContext(ctx, ghCmd, "pr", "view", fmt.Sprintf("%d", pullNumber), "--repo", repository, "--json", "headRefName,baseRefName")
	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if err != nil {
		return "", "", fmt.Errorf("%s pr view failed: %w: %s", ghCmd, err, output)
	}

	var payload struct {
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(payload.HeadRefName), strings.TrimSpace(payload.BaseRefName), nil
}

func (c *Client) searchAPIItems(ctx context.Context, repository string, target domain.MonitoredTarget, rule config.WatchRule, since time.Time) ([]apiItem, error) {
	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("q", buildSearchQuery(repository, target, rule, since))
	query.Set("sort", "updated")
	query.Set("order", "desc")
	query.Set("per_page", "50")

	rawURL := fmt.Sprintf("%s/search/issues?%s", c.baseURL, query.Encode())
	c.infof("github api start name=search/issues repository=%s target=%s since=%s labels=%d authors=%d assignees=%d exclude_draft=%t", repository, target, formatSince(since), len(rule.Labels), len(rule.Authors), len(rule.Assignees), rule.ExcludeDraftPR)
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=search/issues repository=%s target=%s status=%d error=http_status", repository, target, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload apiSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	c.infof("github api done name=search/issues repository=%s target=%s status=%d items=%d", repository, target, resp.StatusCode, len(payload.Items))

	return payload.Items, nil
}

func (c *Client) listAPIItems(ctx context.Context, repository string, endpoint string, since time.Time) ([]apiItem, error) {
	ownerRepo := strings.SplitN(repository, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("repository must be owner/name: %q", repository)
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("state", "open")
	query.Set("sort", "updated")
	query.Set("direction", "desc")
	query.Set("per_page", "50")
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}

	rawURL := fmt.Sprintf("%s/repos/%s/%s/%s?%s", c.baseURL, ownerRepo[0], ownerRepo[1], endpoint, query.Encode())
	c.infof("github api start name=repos/%s repository=%s since=%s", endpoint, repository, formatSince(since))
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=repos/%s repository=%s status=%d error=http_status", endpoint, repository, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []apiItem
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	c.infof("github api done name=repos/%s repository=%s status=%d items=%d", endpoint, repository, resp.StatusCode, len(payload))

	return payload, nil
}

func (c *Client) listPullRequestReviews(ctx context.Context, repository string, pullNumber int, since time.Time) ([]apiPullRequestReview, error) {
	ownerRepo := strings.SplitN(repository, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("repository must be owner/name: %q", repository)
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("per_page", "100")

	rawURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews?%s", c.baseURL, ownerRepo[0], ownerRepo[1], pullNumber, query.Encode())
	c.infof("github api start name=pulls/reviews repository=%s pull_number=%d since=%s", repository, pullNumber, formatSince(since))
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=pulls/reviews repository=%s pull_number=%d status=%d error=http_status", repository, pullNumber, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []apiPullRequestReview
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	filtered := make([]apiPullRequestReview, 0, len(payload))
	for _, review := range payload {
		if !since.IsZero() && !review.SubmittedAt.After(since) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(review.State)) {
		case "COMMENTED", "CHANGES_REQUESTED":
		default:
			continue
		}
		filtered = append(filtered, review)
	}
	c.infof("github api done name=pulls/reviews repository=%s pull_number=%d status=%d raw_reviews=%d filtered_reviews=%d", repository, pullNumber, resp.StatusCode, len(payload), len(filtered))
	return filtered, nil
}

func (c *Client) listPullRequestReviewComments(ctx context.Context, repository string, since time.Time) ([]apiPullRequestReviewComment, error) {
	ownerRepo := strings.SplitN(repository, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("repository must be owner/name: %q", repository)
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("sort", "updated")
	query.Set("direction", "desc")
	query.Set("per_page", "100")
	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}

	rawURL := fmt.Sprintf("%s/repos/%s/%s/pulls/comments?%s", c.baseURL, ownerRepo[0], ownerRepo[1], query.Encode())
	c.infof("github api start name=pulls/comments repository=%s since=%s", repository, formatSince(since))
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=pulls/comments repository=%s status=%d error=http_status", repository, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []apiPullRequestReviewComment
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	c.infof("github api done name=pulls/comments repository=%s status=%d comments=%d", repository, resp.StatusCode, len(payload))
	return payload, nil
}

func (c *Client) loadPullRequestRequestedReviewers(ctx context.Context, repository string, pullNumber int) ([]string, error) {
	ownerRepo := strings.SplitN(repository, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("repository must be owner/name: %q", repository)
	}

	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	rawURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/requested_reviewers", c.baseURL, ownerRepo[0], ownerRepo[1], pullNumber)
	c.infof("github api start name=pulls/requested_reviewers repository=%s pull_number=%d", repository, pullNumber)
	c.debugf("github request method=%s url=%s", http.MethodGet, rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))

	if resp.StatusCode >= 300 {
		c.infof("github api done name=pulls/requested_reviewers repository=%s pull_number=%d status=%d error=http_status", repository, pullNumber, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload struct {
		Users []apiUser `json:"users"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	reviewers := make([]string, 0, len(payload.Users))
	for _, user := range payload.Users {
		if trimmed := strings.TrimSpace(user.Login); trimmed != "" {
			reviewers = append(reviewers, trimmed)
		}
	}
	c.infof("github api done name=pulls/requested_reviewers repository=%s pull_number=%d status=%d reviewers=%d", repository, pullNumber, resp.StatusCode, len(reviewers))
	return reviewers, nil
}

func (c *Client) infof(format string, args ...any) {
	if c.info != nil {
		c.info.Printf(format, args...)
	}
}

func (c *Client) debugf(format string, args ...any) {
	if c.debug != nil {
		c.debug.Printf(format, args...)
	}
}

func (c *Client) loadProjectCards(ctx context.Context, repository string, issueNumber int) ([]domain.ProjectCard, error) {
	ownerRepo := strings.SplitN(repository, "/", 2)
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("repository must be owner/name: %q", repository)
	}
	token, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"query": `
query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      projectItems(first: 20) {
        nodes {
          project {
            title
          }
          fieldValues(first: 20) {
            nodes {
              __typename
              ... on ProjectV2ItemFieldSingleSelectValue {
                name
                field {
                  ... on ProjectV2FieldCommon {
                    name
                  }
                }
              }
              ... on ProjectV2ItemFieldTextValue {
                text
                field {
                  ... on ProjectV2FieldCommon {
                    name
                  }
                }
              }
              ... on ProjectV2ItemFieldDateValue {
                date
                field {
                  ... on ProjectV2FieldCommon {
                    name
                  }
                }
              }
              ... on ProjectV2ItemFieldNumberValue {
                number
                field {
                  ... on ProjectV2FieldCommon {
                    name
                  }
                }
              }
              ... on ProjectV2ItemFieldIterationValue {
                title
                field {
                  ... on ProjectV2FieldCommon {
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`,
		"variables": map[string]any{
			"owner":  ownerRepo[0],
			"name":   ownerRepo[1],
			"number": issueNumber,
		},
	}
	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	rawURL := c.baseURL + "/graphql"
	c.infof("github api start name=graphql/projectItems repository=%s issue_number=%d", repository, issueNumber)
	c.debugf("github request method=%s url=%s", http.MethodPost, rawURL)
	c.debugf("github request body url=%s body=%s", rawURL, string(rawBody))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(string(rawBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("github response url=%s status=%d body=%s", rawURL, resp.StatusCode, string(body))
	if resp.StatusCode >= 300 {
		c.infof("github api done name=graphql/projectItems repository=%s issue_number=%d status=%d error=http_status", repository, issueNumber, resp.StatusCode)
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload projectItemsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("github graphql error: %s", payload.Errors[0].Message)
	}

	cards := make([]domain.ProjectCard, 0, len(payload.Data.Repository.Issue.ProjectItems.Nodes))
	for _, node := range payload.Data.Repository.Issue.ProjectItems.Nodes {
		card := domain.ProjectCard{
			Project: node.Project.Title,
			Fields:  make([]domain.ProjectField, 0, len(node.FieldValues.Nodes)),
		}
		for _, fieldValue := range node.FieldValues.Nodes {
			name := strings.TrimSpace(fieldValue.Field.Name)
			value := strings.TrimSpace(fieldValue.value())
			if name == "" || value == "" {
				continue
			}
			card.Fields = append(card.Fields, domain.ProjectField{Name: name, Value: value})
		}
		cards = append(cards, card)
	}
	c.infof("github api done name=graphql/projectItems repository=%s issue_number=%d status=%d project_items=%d", repository, issueNumber, resp.StatusCode, len(cards))
	return cards, nil
}

func normalizeRepository(repository string) (string, error) {
	trimmed := strings.TrimSpace(repository)
	trimmed = strings.TrimSuffix(trimmed, "/")

	if strings.HasPrefix(trimmed, "https://github.com/") || strings.HasPrefix(trimmed, "http://github.com/") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("invalid repository url %q: %w", repository, err)
		}
		trimmed = strings.TrimPrefix(u.Path, "/")
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repository must be owner/name: %q", repository)
	}
	return parts[0] + "/" + parts[1], nil
}

func buildSearchQuery(repository string, target domain.MonitoredTarget, rule config.WatchRule, since time.Time) string {
	parts := []string{
		"repo:" + repository,
		"state:open",
	}

	switch target {
	case domain.TargetPullRequest:
		parts = append(parts, "is:pr")
		if rule.ExcludeDraftPR {
			parts = append(parts, "-is:draft")
		}
	default:
		parts = append(parts, "is:issue")
	}

	if !since.IsZero() {
		parts = append(parts, "updated:>="+since.UTC().Format(time.RFC3339))
	}
	for _, label := range compactSearchValues(rule.Labels) {
		parts = append(parts, fmt.Sprintf("label:%q", label))
	}
	if target == domain.TargetPullRequest {
		if reviewers := buildSearchORGroup("review-requested", compactSearchValues(rule.Reviewers)); reviewers != "" {
			parts = append(parts, reviewers)
		}
	}
	if authors := buildSearchORGroup("author", compactSearchValues(rule.Authors)); authors != "" {
		parts = append(parts, authors)
	}
	if assignees := buildSearchORGroup("assignee", compactSearchValues(rule.Assignees)); assignees != "" {
		parts = append(parts, assignees)
	}

	return strings.Join(parts, " ")
}

func compactSearchValues(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		compacted = append(compacted, trimmed)
	}
	return compacted
}

func anyReviewerMatches(expected []string, actual []string) bool {
	for _, candidate := range actual {
		for _, reviewer := range expected {
			if strings.EqualFold(strings.TrimSpace(reviewer), strings.TrimSpace(candidate)) {
				return true
			}
		}
	}
	return false
}

func buildSearchORGroup(qualifier string, values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return qualifier + ":" + values[0]
	default:
		parts := make([]string, 0, len(values))
		for _, value := range values {
			parts = append(parts, qualifier+":"+value)
		}
		return "(" + strings.Join(parts, " OR ") + ")"
	}
}

func searchEndpointForTarget(target domain.MonitoredTarget) string {
	if target == domain.TargetPullRequest {
		return "pulls"
	}
	return "issues"
}

type apiItem struct {
	Number    int         `json:"number"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	HTMLURL   string      `json:"html_url"`
	UpdatedAt time.Time   `json:"updated_at"`
	Draft     bool        `json:"draft"`
	User      apiUser     `json:"user"`
	Assignees []apiUser   `json:"assignees"`
	Reviewers []apiUser   `json:"requested_reviewers"`
	Labels    []apiLabel  `json:"labels"`
	PullReq   *struct{}   `json:"pull_request,omitempty"`
	Head      apiPullHead `json:"head"`
	Base      apiPullBase `json:"base"`
}

type apiSearchResponse struct {
	Items []apiItem `json:"items"`
}

type apiIssue struct {
	Body string `json:"body"`
}

type apiUser struct {
	Login string `json:"login"`
}

type apiLabel struct {
	Name string `json:"name"`
}

type apiPullHead struct {
	Ref  string      `json:"ref"`
	Repo apiRepoInfo `json:"repo"`
}

type apiPullBase struct {
	Ref string `json:"ref"`
}

type apiRepoInfo struct {
	FullName string `json:"full_name"`
}

type apiPullRequestReviewComment struct {
	ID                  int64     `json:"id"`
	Body                string    `json:"body"`
	Path                string    `json:"path"`
	Line                int       `json:"line"`
	HTMLURL             string    `json:"html_url"`
	PullRequestURL      string    `json:"pull_request_url"`
	PullRequestReviewID int64     `json:"pull_request_review_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	User                apiUser   `json:"user"`
}

type apiPullRequestReview struct {
	ID             int64     `json:"id"`
	Body           string    `json:"body"`
	HTMLURL        string    `json:"html_url"`
	State          string    `json:"state"`
	SubmittedAt    time.Time `json:"submitted_at"`
	PullRequestURL string    `json:"pull_request_url"`
	User           apiUser   `json:"user"`
}

type projectItemsResponse struct {
	Data struct {
		Repository struct {
			Issue struct {
				ProjectItems struct {
					Nodes []projectItemNode `json:"nodes"`
				} `json:"projectItems"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type projectItemNode struct {
	Project struct {
		Title string `json:"title"`
	} `json:"project"`
	FieldValues struct {
		Nodes []projectFieldValueNode `json:"nodes"`
	} `json:"fieldValues"`
}

type projectFieldValueNode struct {
	TypeName string `json:"__typename"`
	Field    struct {
		Name string `json:"name"`
	} `json:"field"`
	Name   string  `json:"name"`
	Text   string  `json:"text"`
	Date   string  `json:"date"`
	Number float64 `json:"number"`
	Title  string  `json:"title"`
}

func (n projectFieldValueNode) value() string {
	switch n.TypeName {
	case "ProjectV2ItemFieldSingleSelectValue":
		return n.Name
	case "ProjectV2ItemFieldTextValue":
		return n.Text
	case "ProjectV2ItemFieldDateValue":
		return n.Date
	case "ProjectV2ItemFieldIterationValue":
		return n.Title
	case "ProjectV2ItemFieldNumberValue":
		return fmt.Sprintf("%v", n.Number)
	default:
		return ""
	}
}

func (i apiItem) toDomain(repository string, endpoint string) domain.RepositoryItem {
	labels := make([]string, 0, len(i.Labels))
	for _, label := range i.Labels {
		labels = append(labels, label.Name)
	}

	assignees := make([]string, 0, len(i.Assignees))
	for _, assignee := range i.Assignees {
		assignees = append(assignees, assignee.Login)
	}

	reviewers := make([]string, 0, len(i.Reviewers))
	for _, reviewer := range i.Reviewers {
		reviewers = append(reviewers, reviewer.Login)
	}

	target := domain.TargetIssue
	state := domain.StateDetected
	if endpoint == "pulls" || i.PullReq != nil {
		target = domain.TargetPullRequest
		state = domain.StateCollectingContext
	}

	return domain.RepositoryItem{
		Repository:   repository,
		Number:       i.Number,
		Title:        i.Title,
		Body:         i.Body,
		Author:       i.User.Login,
		Assignees:    assignees,
		Reviewers:    reviewers,
		Labels:       labels,
		Draft:        i.Draft,
		URL:          i.HTMLURL,
		UpdatedAt:    i.UpdatedAt,
		Target:       target,
		BranchName:   i.Head.Ref,
		BaseBranch:   i.Base.Ref,
		DefaultState: state,
	}
}

func pullRequestNumberFromURL(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash < 0 || lastSlash == len(trimmed)-1 {
		return 0
	}
	var number int
	_, _ = fmt.Sscanf(trimmed[lastSlash+1:], "%d", &number)
	return number
}
