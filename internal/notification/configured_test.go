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
		events: []string{"design_ready", "failed"},
		next:   recorder,
	}

	if err := notifier.Notify(context.Background(), Notification{Event: "design_ready", State: "design_ready"}); err != nil {
		t.Fatalf("Notify(design_ready) error = %v", err)
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

type recordingNotifier struct {
	events []Notification
}

func (n *recordingNotifier) Notify(_ context.Context, event Notification) error {
	n.events = append(n.events, event)
	return nil
}
