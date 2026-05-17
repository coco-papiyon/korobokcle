package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type fanoutNotifier struct {
	notifiers []Notifier
}

type filteredNotifier struct {
	events []string
	next   Notifier
}

var supportedNotificationEvents = map[string]struct{}{
	"waiting_design_approval": {},
	"waiting_final_approval":  {},
	"review_completed":        {},
	"pr_created":              {},
	"failed":                  {},
}

func NewNopNotifier() Notifier {
	return fanoutNotifier{}
}

func NewFanoutNotifier(notifiers ...Notifier) Notifier {
	filtered := make([]Notifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		if notifier != nil {
			filtered = append(filtered, notifier)
		}
	}
	return fanoutNotifier{notifiers: filtered}
}

func NewConfiguredNotifier(cfg config.Notifications) (Notifier, error) {
	notifiers := make([]Notifier, 0, len(cfg.Channels))
	var errs []error

	for _, channel := range cfg.Channels {
		if !channel.Enabled {
			continue
		}

		var notifier Notifier
		switch strings.ToLower(strings.TrimSpace(channel.Type)) {
		case "windows_toast":
			notifier = NewWindowsToastNotifier()
		default:
			errs = append(errs, fmt.Errorf("notification channel %q has unsupported type %q", channel.Name, channel.Type))
			continue
		}

		notifiers = append(notifiers, filteredNotifier{
			events: normalizeNotificationEvents(channel.Events),
			next:   notifier,
		})
	}

	if len(notifiers) == 0 {
		return NewNopNotifier(), errors.Join(errs...)
	}
	return NewFanoutNotifier(notifiers...), errors.Join(errs...)
}

func (n fanoutNotifier) Notify(ctx context.Context, event Notification) error {
	var errs []error
	delivered := false
	for _, notifier := range n.notifiers {
		if err := notifier.Notify(ctx, event); err != nil {
			if errors.Is(err, ErrNotificationSkipped) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		delivered = true
	}
	if delivered {
		return nil
	}
	if len(errs) == 0 {
		return ErrNotificationSkipped
	}
	return errors.Join(errs...)
}

func (n filteredNotifier) Notify(ctx context.Context, event Notification) error {
	if !matchesNotificationEvent(n.events, event) {
		return ErrNotificationSkipped
	}
	return n.next.Notify(ctx, event)
}

func matchesNotificationEvent(configured []string, event Notification) bool {
	if len(configured) == 0 {
		return true
	}

	for _, candidate := range configured {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "":
			continue
		case strings.ToLower(event.Event):
			return true
		case "failed":
			if event.State == string(domain.StateFailed) || strings.HasSuffix(strings.ToLower(event.Event), "_failed") {
				return true
			}
		}
	}
	return false
}

func normalizeNotificationEvents(events []string) []string {
	normalized := make([]string, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for _, candidate := range events {
		name := strings.ToLower(strings.TrimSpace(candidate))
		if _, ok := supportedNotificationEvents[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}
