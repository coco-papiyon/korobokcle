package notification

import (
	"context"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/config"
)

func TestConfiguredNotifierMatchesExactAndFailedAlias(t *testing.T) {
	t.Parallel()

	recorder := &recordingNotifier{}
	notifier := filteredNotifier{
		events: []string{"waiting_design_approval", "failed"},
		next:   recorder,
	}

	if err := notifier.Notify(context.Background(), Notification{Event: "waiting_design_approval", State: "waiting_design_approval"}); err != nil {
		t.Fatalf("Notify(waiting_design_approval) error = %v", err)
	}
	if err := notifier.Notify(context.Background(), Notification{Event: "test_failed", State: "failed"}); err != nil {
		t.Fatalf("Notify(test_failed) error = %v", err)
	}
	if err := notifier.Notify(context.Background(), Notification{Event: "review_ready", State: "review_ready"}); err == nil {
		t.Fatalf("expected Notify(review_ready) to be skipped")
	} else if err != ErrNotificationSkipped {
		t.Fatalf("expected ErrNotificationSkipped, got %v", err)
	}

	if len(recorder.events) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(recorder.events))
	}
}

func TestNewConfiguredNotifierSkipsUnsupportedChannels(t *testing.T) {
	t.Parallel()

	notifier, err := NewConfiguredNotifier(config.Notifications{
		Channels: []config.NotificationChannel{
			{Name: "bad", Type: "unknown", Enabled: true},
		},
	})
	if notifier == nil {
		t.Fatalf("expected notifier")
	}
	if err == nil {
		t.Fatalf("expected setup warning error")
	}
}

func TestNormalizeNotificationEventsDropsLegacyReadyEvents(t *testing.T) {
	t.Parallel()

	got := normalizeNotificationEvents([]string{
		"design_ready",
		"waiting_design_approval",
		"implementation_ready",
		"review_ready",
		"pr_created",
		"failed",
		"failed",
	})

	want := []string{"waiting_design_approval", "pr_created", "failed"}
	if len(got) != len(want) {
		t.Fatalf("expected %d events, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected event[%d] = %q, got %q", i, want[i], got[i])
		}
	}
}

type recordingNotifier struct {
	events []Notification
}

func (n *recordingNotifier) Notify(_ context.Context, event Notification) error {
	n.events = append(n.events, event)
	return nil
}
