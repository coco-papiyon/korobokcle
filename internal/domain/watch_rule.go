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
	TargetIssue        MonitoredTarget = "issue"
	TargetIssueProject MonitoredTarget = "issue_project"
	TargetPullRequest  MonitoredTarget = "pull_request"
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
	Repository   string
	Number       int
	Title        string
	Body         string
	Author       string
	Assignees    []string
	Labels       []string
	Draft        bool
	URL          string
	UpdatedAt    time.Time
	Target       MonitoredTarget
	ProjectCards []ProjectCard
	DefaultState JobState
}

type MatchResult struct {
	Status MatchStatus
	Reason string
}

type DomainEventType string

const (
	DomainEventIssueMatched DomainEventType = "issue_matched"
	DomainEventPRMatched    DomainEventType = "pull_request_matched"
)

type DomainEvent struct {
	Type      DomainEventType
	RuleID    string
	RuleName  string
	Item      RepositoryItem
	MatchedAt time.Time
}
