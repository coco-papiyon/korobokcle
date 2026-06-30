package domain

import "testing"

func TestJobStateDisplayNameAndLabel(t *testing.T) {
	tests := []struct {
		state JobState
		name  string
		label string
	}{
		{StateDesignApproved, "設計承認済み", "state:design_approved"},
		{StateReviewFixed, "レビュー指摘修正済み", "state:review_fixed"},
		{StatePRReviewComment, "レビュー指摘あり", "state:pr_review_comment"},
		{StateCompleted, "完了", "state:completed"},
		{StateReviewFixDesignRunning, "レビュー指摘検討中", "state:review_fix_design_running"},
		{StatePRConflict, "コンフリクト検知済み", "state:pr_conflict"},
		{StatePRConflictReady, "コンフリクト解消完了", "state:pr_conflict_ready"},
	}

	for _, tt := range tests {
		name, ok := tt.state.DisplayName()
		if !ok {
			t.Fatalf("DisplayName(%s) not found", tt.state)
		}
		if name != tt.name {
			t.Fatalf("DisplayName(%s) = %q, want %q", tt.state, name, tt.name)
		}

		label, ok := tt.state.Label()
		if !ok {
			t.Fatalf("Label(%s) not found", tt.state)
		}
		if label != tt.label {
			t.Fatalf("Label(%s) = %q, want %q", tt.state, label, tt.label)
		}
	}
}

func TestJobStateTransitions(t *testing.T) {
	if !StateDetected.CanTransitionTo(StateDesignRunning) {
		t.Fatal("expected detected -> design_running to be allowed")
	}
	if !StateDesignApproved.CanTransitionTo(StateImplementationRunning) {
		t.Fatal("expected design_approved -> implementation_running to be allowed")
	}
	if !StateDesignReady.CanTransitionTo(StateCompleted) {
		t.Fatal("expected design_ready -> completed to be allowed")
	}
	if !StatePRReviewComment.CanTransitionTo(StateReviewFixImplementationRunning) {
		t.Fatal("expected pr_review_comment -> review_fix_implementation_running to be allowed")
	}
	if !StateImplementationApproved.CanTransitionTo(StatePRCreated) {
		t.Fatal("expected implementation_approved -> pr_created to be allowed")
	}
	if StateImplementationReady.CanTransitionTo(StateCompleted) {
		t.Fatal("expected implementation_ready -> completed to be disallowed")
	}
	if StateDesignRunning.CanTransitionTo(StatePRCreated) {
		t.Fatal("expected design_running -> pr_created to be disallowed")
	}
	if StatePRCreated.CanTransitionTo(StateReviewApproved) {
		t.Fatal("expected pr_created -> review_approved to be disallowed")
	}
	if !StatePRConflict.CanTransitionTo(StatePRConflictRunning) || !StatePRConflictRunning.CanTransitionTo(StatePRConflictReady) {
		t.Fatal("expected PR conflict workflow transitions to be allowed")
	}
}

func TestInitialStateForKind(t *testing.T) {
	tests := []struct {
		kind JobKind
		want JobState
	}{
		{JobKindIssueDesign, StateDetected},
		{JobKindIssueImplementation, StateDesignApproved},
		{JobKindPRReview, StateReviewRunning},
		{JobKindPRFeedback, StatePRReviewComment},
		{JobKindPRConflict, StatePRConflict},
	}

	for _, tt := range tests {
		if got := InitialStateForKind(tt.kind); got != tt.want {
			t.Fatalf("InitialStateForKind(%s) = %s, want %s", tt.kind, got, tt.want)
		}
	}
}

func TestPRFeedbackDesignApprovalStartsImplementation(t *testing.T) {
	if got := RunningStateForKind(JobKindPRFeedback, StateReviewFixDesignApproved); got != StateReviewFixImplementationRunning {
		t.Fatalf("RunningStateForKind() = %s, want %s", got, StateReviewFixImplementationRunning)
	}
	if got := ReadyStateForKind(JobKindPRFeedback, StateReviewFixDesignApproved); got != StateReviewFixImplementationReady {
		t.Fatalf("ReadyStateForKind() = %s, want %s", got, StateReviewFixImplementationReady)
	}
	if !StateReviewFixDesignApproved.CanTransitionTo(StateReviewFixImplementationRunning) {
		t.Fatal("expected review_fix_design_approved -> review_fix_implementation_running to be allowed")
	}
}

func TestPRFeedbackReviewCommentStartsImplementation(t *testing.T) {
	if got := RunningStateForKind(JobKindPRFeedback, StatePRReviewComment); got != StateReviewFixImplementationRunning {
		t.Fatalf("RunningStateForKind() = %s, want %s", got, StateReviewFixImplementationRunning)
	}
	if got := ReadyStateForKind(JobKindPRFeedback, StatePRReviewComment); got != StateReviewFixImplementationReady {
		t.Fatalf("ReadyStateForKind() = %s, want %s", got, StateReviewFixImplementationReady)
	}
}
