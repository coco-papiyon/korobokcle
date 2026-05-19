package domain

import "time"

type MatchStatus string

const (
	MatchStatusMatched MatchStatus = "matched"
	MatchStatusIgnored MatchStatus = "ignored"
	MatchStatusSkipped MatchStatus = "skipped"
)

type MonitoredTarget string

const (
	TargetIssue             MonitoredTarget = "issue"
	TargetIssueProject      MonitoredTarget = "issue_project"
	TargetPullRequest       MonitoredTarget = "pull_request"
	TargetPullRequestReview MonitoredTarget = "pull_request_review"
)

type ProjectField struct {
	Name  string
	Value string
}

type ProjectCard struct {
	Project string
	Fields  []ProjectField
}

type RepositoryItem struct {
	Repository     string
	Number         int
	Title          string
	Body           string
	Author         string
	Assignees      []string
	Reviewers      []string
	Labels         []string
	Draft          bool
	URL            string
	UpdatedAt      time.Time
	Target         MonitoredTarget
	BranchName     string
	BaseBranch     string
	ReviewComments []ReviewComment
	ProjectCards   []ProjectCard
	DefaultState   JobState
}

type ReviewComment struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Path      string    `json:"path"`
	Line      int       `json:"line"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type MatchResult struct {
	Status MatchStatus
	Reason string
}

type DomainEventType string

const (
	DomainEventIssueMatched    DomainEventType = "issue_matched"
	DomainEventPRMatched       DomainEventType = "pull_request_matched"
	DomainEventPRReviewMatched DomainEventType = "pull_request_review_matched"
)

type DomainEvent struct {
	Type      DomainEventType
	RuleID    string
	RuleName  string
	Item      RepositoryItem
	MatchedAt time.Time
}
