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
			name: "design ready",
			event: domain.Event{
				EventType: "design_ready",
				StateFrom: string(domain.StateDesignRunning),
				StateTo:   string(domain.StateDesignReady),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryDesign},
		},
		{
			name: "design started",
			event: domain.Event{
				EventType: "design_started",
				StateFrom: string(domain.StateDetected),
				StateTo:   string(domain.StateDesignRunning),
				CreatedAt: time.Now(),
			},
			expect: nil,
		},
		{
			name: "implementation ready",
			event: domain.Event{
				EventType: "waiting_final_approval",
				StateFrom: string(domain.StateImplementationReady),
				StateTo:   string(domain.StateWaitingFinalApproval),
				CreatedAt: time.Now(),
			},
			expect: []string{actionRetryImplementation},
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
