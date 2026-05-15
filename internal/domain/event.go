package domain

import "time"

type Event struct {
	ID        int64
	JobID     string
	EventType string
	StateFrom string
	StateTo   string
	Payload   string
	CreatedAt time.Time
}
