package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	tokenSrc   TokenProvider
	debug      *log.Logger
}

func NewClient(tokenSrc TokenProvider, debug *log.Logger) *Client {
	return &Client{
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		tokenSrc: tokenSrc,
		debug:    debug,
	}
}

func (c *Client) ListIssues(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	return c.listRepositoryItems(ctx, repository, "issues", since)
}

func (c *Client) ListProjectIssues(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	items, err := c.listRepositoryItems(ctx, repository, "issues", since)
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

func (c *Client) ListPullRequests(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error) {
	return c.listRepositoryItems(ctx, repository, "pulls", since)
}

func (c *Client) ListPullRequestReviews(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error) {
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

	items := make([]domain.RepositoryItem, 0, len(grouped))
	for pullNumber, reviewComments := range grouped {
		if len(reviewComments) == 0 {
			continue
		}
		pull := openPRs[pullNumber]
		item := pull.toDomain(normalizedRepository, "pulls")
		item.Target = domain.TargetPullRequestReview
		item.DefaultState = domain.StateImplementationRunning
		item.ReviewComments = reviewComments
		item.UpdatedAt = latest[pullNumber]
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) listRepositoryItems(ctx context.Context, repository string, endpoint string, since time.Time) ([]domain.RepositoryItem, error) {
	normalizedRepository, err := normalizeRepository(repository)
	if err != nil {
		return nil, err
	}

	payload, err := c.listAPIItems(ctx, normalizedRepository, endpoint, since)
	if err != nil {
		return nil, err
	}

	items := make([]domain.RepositoryItem, 0, len(payload))
	for _, item := range payload {
		items = append(items, item.toDomain(normalizedRepository, endpoint))
	}
	return items, nil
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
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []apiItem
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("github api %s returned status %d", rawURL, resp.StatusCode)
	}

	var payload []apiPullRequestReviewComment
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
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
	c.debugf("github request method=%s url=%s", http.MethodPost, rawURL)
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
