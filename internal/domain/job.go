package domain

import "fmt"

type JobKind string

const (
	JobKindIssueDesign         JobKind = "issue_design"
	JobKindIssueImplementation JobKind = "issue_implementation"
	JobKindPRReview            JobKind = "pr_review"
	JobKindPRFeedback          JobKind = "pr_feedback"
)

type JobState string

const (
	StateDetected                        JobState = "detected"
	StateDesignRunning                   JobState = "design_running"
	StateDesignReady                     JobState = "design_ready"
	StateDesignApproved                  JobState = "design_approved"
	StateImplementationRunning           JobState = "implementation_running"
	StateImplementationReady             JobState = "implementation_ready"
	StateImplementationApproved          JobState = "implementation_approved"
	StatePRCreated                       JobState = "pr_created"
	StatePRReviewComment                 JobState = "pr_review_comment"
	StateReviewFixDesignRunning          JobState = "review_fix_design_running"
	StateReviewFixDesignReady            JobState = "review_fix_design_ready"
	StateReviewFixDesignApproved         JobState = "review_fix_design_approved"
	StateReviewFixImplementationRunning  JobState = "review_fix_implementation_running"
	StateReviewFixImplementationReady    JobState = "review_fix_implementation_ready"
	StateReviewFixImplementationApproved JobState = "review_fix_implementation_approved"
	StateReviewFixed                     JobState = "review_fixed"
	StateReviewRunning                   JobState = "review_running"
	StateReviewReady                     JobState = "review_ready"
	StateReviewApproved                  JobState = "review_approved"
	StateCompleted                       JobState = "completed"
	StateFailed                          JobState = "failed"
)

var stateDisplayNames = map[JobState]string{
	StateDetected:                        "検知済み",
	StateDesignRunning:                   "設計中",
	StateDesignReady:                     "設計完了",
	StateDesignApproved:                  "設計承認済み",
	StateImplementationRunning:           "実装中",
	StateImplementationReady:             "実装完了",
	StateImplementationApproved:          "実装承認済み",
	StatePRCreated:                       "PR済み",
	StatePRReviewComment:                 "レビュー指摘あり",
	StateReviewFixDesignRunning:          "レビュー指摘検討中",
	StateReviewFixDesignReady:            "レビュー指摘検討済み",
	StateReviewFixDesignApproved:         "レビュー検討承認済み",
	StateReviewFixImplementationRunning:  "レビュー指摘修正中",
	StateReviewFixImplementationReady:    "レビュー指摘修正完了",
	StateReviewFixImplementationApproved: "レビュー指摘修正承認済み",
	StateReviewFixed:                     "レビュー指摘修正済み",
	StateReviewRunning:                   "レビュー中",
	StateReviewReady:                     "レビュー完了",
	StateReviewApproved:                  "レビュー承認済み",
	StateCompleted:                       "完了",
	StateFailed:                          "失敗",
}

var stateLabels = map[JobState]string{
	StateDetected:                        "state:detected",
	StateDesignRunning:                   "state:design_running",
	StateDesignReady:                     "state:design_ready",
	StateDesignApproved:                  "state:design_approved",
	StateImplementationRunning:           "state:implementation_running",
	StateImplementationReady:             "state:implementation_ready",
	StateImplementationApproved:          "state:implementation_approved",
	StatePRCreated:                       "state:pr_created",
	StatePRReviewComment:                 "state:pr_review_comment",
	StateReviewFixDesignRunning:          "state:review_fix_design_running",
	StateReviewFixDesignReady:            "state:review_fix_design_ready",
	StateReviewFixDesignApproved:         "state:review_fix_design_approved",
	StateReviewFixImplementationRunning:  "state:review_fix_implementation_running",
	StateReviewFixImplementationReady:    "state:review_fix_implementation_ready",
	StateReviewFixImplementationApproved: "state:review_fix_implementation_approved",
	StateReviewFixed:                     "state:review_fixed",
	StateReviewRunning:                   "state:review_running",
	StateReviewReady:                     "state:review_ready",
	StateReviewApproved:                  "state:review_approved",
	StateCompleted:                       "state:completed",
	StateFailed:                          "state:failed",
}

var allowedTransitions = map[JobState]map[JobState]struct{}{
	StateDetected: {
		StateDesignRunning:   {},
		StateReviewRunning:   {},
		StatePRReviewComment: {},
	},
	StateDesignRunning: {
		StateDesignReady: {},
		StateFailed:      {},
	},
	StateDesignReady: {
		StateDesignApproved: {},
		StateCompleted:      {},
		StateFailed:         {},
	},
	StateDesignApproved: {
		StateImplementationRunning: {},
		StateFailed:                {},
	},
	StateImplementationRunning: {
		StateImplementationReady: {},
		StateFailed:              {},
	},
	StateImplementationReady: {
		StateImplementationApproved: {},
		StateFailed:                 {},
	},
	StateImplementationApproved: {
		StatePRCreated: {},
		StateFailed:    {},
	},
	StatePRCreated: {
		StateFailed: {},
	},
	StatePRReviewComment: {
		StateReviewFixDesignRunning: {},
		StateFailed:                 {},
	},
	StateReviewFixDesignRunning: {
		StateReviewFixDesignReady: {},
		StateFailed:               {},
	},
	StateReviewFixDesignReady: {
		StateReviewFixDesignApproved: {},
		StateCompleted:               {},
		StateFailed:                  {},
	},
	StateReviewFixDesignApproved: {
		StateReviewFixImplementationRunning: {},
		StateFailed:                         {},
	},
	StateReviewFixImplementationRunning: {
		StateReviewFixImplementationReady: {},
		StateFailed:                       {},
	},
	StateReviewFixImplementationReady: {
		StateReviewFixImplementationApproved: {},
		StateCompleted:                       {},
		StateFailed:                          {},
	},
	StateReviewFixImplementationApproved: {
		StateReviewFixed: {},
		StateFailed:      {},
	},
	StateReviewFixed: {
		StateFailed: {},
	},
	StateReviewRunning: {
		StateReviewReady: {},
		StateFailed:      {},
	},
	StateReviewReady: {
		StateReviewApproved: {},
		StateCompleted:      {},
		StateFailed:         {},
	},
	StateReviewApproved: {
		StateFailed: {},
	},
	StateFailed: {},
}

type Job struct {
	ID         string   `json:"id"`
	Kind       JobKind  `json:"kind"`
	State      JobState `json:"state"`
	Repository string   `json:"repository"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
}

func (s JobState) DisplayName() (string, bool) {
	v, ok := stateDisplayNames[s]
	return v, ok
}

func (s JobState) Label() (string, bool) {
	v, ok := stateLabels[s]
	return v, ok
}

func (s JobState) CanTransitionTo(next JobState) bool {
	nextStates, ok := allowedTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}

func MustDisplayName(state JobState) string {
	if name, ok := state.DisplayName(); ok {
		return name
	}
	return fmt.Sprintf("unknown(%s)", state)
}

func MustLabel(state JobState) string {
	if label, ok := state.Label(); ok {
		return label
	}
	return fmt.Sprintf("state:%s", state)
}

func AllStateLabels() []string {
	labels := make([]string, 0, len(stateLabels))
	for _, label := range stateLabels {
		labels = append(labels, label)
	}
	return labels
}

func InitialStateForKind(kind JobKind) JobState {
	switch kind {
	case JobKindIssueDesign:
		return StateDetected
	case JobKindIssueImplementation:
		return StateDesignApproved
	case JobKindPRReview:
		return StateReviewRunning
	case JobKindPRFeedback:
		return StatePRReviewComment
	default:
		return StateDetected
	}
}

func RunningStateForKind(kind JobKind, state JobState) JobState {
	switch kind {
	case JobKindIssueDesign:
		return StateDesignRunning
	case JobKindIssueImplementation:
		return StateImplementationRunning
	case JobKindPRReview:
		return StateReviewRunning
	case JobKindPRFeedback:
		switch state {
		case StateReviewFixDesignApproved, StateReviewFixImplementationReady, StateReviewFixImplementationRunning, StateReviewFixImplementationApproved:
			return StateReviewFixImplementationRunning
		default:
			return StateReviewFixDesignRunning
		}
	default:
		return StateFailed
	}
}

func ReadyStateForKind(kind JobKind, state JobState) JobState {
	switch kind {
	case JobKindIssueDesign:
		return StateDesignReady
	case JobKindIssueImplementation:
		return StateImplementationReady
	case JobKindPRReview:
		return StateReviewReady
	case JobKindPRFeedback:
		switch state {
		case StateReviewFixDesignApproved, StateReviewFixImplementationRunning, StateReviewFixImplementationReady, StateReviewFixImplementationApproved:
			return StateReviewFixImplementationReady
		default:
			return StateReviewFixDesignReady
		}
	default:
		return StateFailed
	}
}

func RunningStateForReadyState(state JobState) JobState {
	switch state {
	case StateDesignReady:
		return StateDesignRunning
	case StateImplementationReady:
		return StateImplementationRunning
	case StateReviewReady:
		return StateReviewRunning
	case StateReviewFixDesignReady:
		return StateReviewFixDesignRunning
	case StateReviewFixImplementationReady:
		return StateReviewFixImplementationRunning
	default:
		return StateFailed
	}
}

func ApprovedStateForReadyState(state JobState) JobState {
	switch state {
	case StateDesignReady:
		return StateDesignApproved
	case StateImplementationReady:
		return StateImplementationApproved
	case StateReviewReady:
		return StateReviewApproved
	case StateReviewFixDesignReady:
		return StateReviewFixDesignApproved
	case StateReviewFixImplementationReady:
		return StateReviewFixImplementationApproved
	default:
		return StateFailed
	}
}

func ResultCommentTarget(kind JobKind) string {
	switch kind {
	case JobKindPRReview, JobKindPRFeedback:
		return "pr"
	default:
		return "issue"
	}
}
