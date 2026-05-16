package web

import (
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestAvailableActionsForEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		event  domain.Event
		expect []string
	}{
		{
			name: "design running",
			event: domain.Event{
				EventType: "design_started",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateDesignRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "design ready",
			event: domain.Event{
				EventType: "design_ready",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateDesignReady),
				CreatedAt: time.Now(),
			},
			expect: nil,
		},
		{
			name: "implementation running",
			event: domain.Event{
				EventType: "implementation_started",
				StateFrom: string(domain.StateImplementationRunning),
				StateTo:   string(domain.StateImplementationRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "test running",
			event: domain.Event{
				EventType: "test_started",
				StateFrom: string(domain.StateTestRunning),
				StateTo:   string(domain.StateTestRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
		},
		{
			name: "review running",
			event: domain.Event{
				EventType: "review_started",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateReviewRunning),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr creating",
			event: domain.Event{
				EventType: "final_approved",
				StateFrom: string(domain.StateDetected),
				StateTo:   string(domain.StatePRCreating),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
		{
			name: "review failure",
			event: domain.Event{
				EventType: "review_failed",
				StateFrom: string(domain.StateReviewRunning),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryReview},
		},
		{
			name: "pr failure",
			event: domain.Event{
				EventType: "pr_create_failed",
				StateFrom: string(domain.StatePRCreating),
				StateTo:   string(domain.StateFailed),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryPR},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := availableActionsForEvent(tc.event)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("expected %v, got %v", tc.expect, got)
				}
			}
		})
	}
}
