package notification

import (
	"context"
	"log"
)

type WindowsToastNotifier struct{}

func NewWindowsToastNotifier() *WindowsToastNotifier {
	return &WindowsToastNotifier{}
}

func (n *WindowsToastNotifier) Notify(_ context.Context, event Notification) error {
	log.Printf("notify[%s]: %s - %s", event.Event, event.Title, event.Message)
	return nil
}
