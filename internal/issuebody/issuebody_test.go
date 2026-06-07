package issuebody

import (
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestResolvePrefersRefreshedIssueBodyAndKeepsMatchedMetadata(t *testing.T) {
	t.Parallel()

	events := []domain.Event{
		{
			EventType: string(domain.DomainEventIssueMatched),
			Payload:   `{"body":"original body","author":"alice","labels":["bug"],"assignees":["bob"]}`,
		},
		{
			EventType: EventTypeRefreshed,
			Payload:   `{"body":"latest body"}`,
		},
	}

	got, err := Resolve(events)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Body != "latest body" {
		t.Fatalf("expected latest body, got %q", got.Body)
	}
	if got.Author != "alice" {
		t.Fatalf("expected author alice, got %q", got.Author)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "bug" {
		t.Fatalf("expected labels from issue matched, got %#v", got.Labels)
	}
	if len(got.Assignees) != 1 || got.Assignees[0] != "bob" {
		t.Fatalf("expected assignees from issue matched, got %#v", got.Assignees)
	}
}
