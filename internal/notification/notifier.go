package notification

import "context"

type Notification struct {
	Title      string
	Message    string
	Event      string
	State      string
	Repository string
	Number     int
	JobID      string
}

type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}
