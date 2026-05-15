package domain

import (
	"errors"
	"time"
)

type JobType string
type JobState string

const (
	JobTypeIssue    JobType = "issue"
	JobTypePRReview JobType = "pr_review"
)

const (
	StateDetected              JobState = "detected"
	StateDesignRunning         JobState = "design_running"
	StateDesignReady           JobState = "design_ready"
	StateWaitingDesignApproval JobState = "waiting_design_approval"
	StateImplementationRunning JobState = "implementation_running"
	StateTestRunning           JobState = "test_running"
	StateImplementationReady   JobState = "implementation_ready"
	StateWaitingFinalApproval  JobState = "waiting_final_approval"
	StatePRCreating            JobState = "pr_creating"
	StateCollectingContext     JobState = "collecting_context"
	StateChecksRunning         JobState = "checks_running"
	StateReviewRunning         JobState = "review_running"
	StateReviewReady           JobState = "review_ready"
	StateCompleted             JobState = "completed"
	StateFailed                JobState = "failed"
	StateDesignRejected        JobState = "design_rejected"
	StateFinalRejected         JobState = "final_rejected"
)

type Job struct {
	ID           string
	Type         JobType
	Repository   string
	GitHubNumber int
	State        JobState
	Title        string
	BranchName   string
	WatchRuleID  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var ErrJobNotFound = errors.New("job not found")
