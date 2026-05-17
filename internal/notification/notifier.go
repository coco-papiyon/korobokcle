package notification

import (
	"context"
	"errors"
)

type Notification struct {
	Title      string
	Message    string
	Event      string
	State      string
	Repository string
	Number     int
	JobID      string
}

var ErrNotificationSkipped = errors.New("notification skipped")

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}
