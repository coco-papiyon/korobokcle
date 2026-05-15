package notification

import "context"

type Notification struct {
	Title   string
	Message string
	Event   string
}

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}
