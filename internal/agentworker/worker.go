package agentworker

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrAlreadyStarted = errors.New("agent worker is already started")
	ErrNotRunning     = errors.New("agent worker is not running")
	ErrStopped        = errors.New("agent worker has been stopped")
)

// RequestWorker is the subset of worker behavior used by the application.
type RequestWorker interface {
	Start(context.Context) error
	SetOutputWriters(io.Writer, io.Writer)
	SendPromptAt(context.Context, string, string, string) (string, error)
	GetStatus() Status
	Stop(context.Context) error
}

type State string

const (
	StateNew      State = "new"
	StateStarting State = "starting"
	StateIdle     State = "idle"
	StateBusy     State = "busy"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)

type Status struct {
	State       State
	PID         int
	StartedAt   time.Time
	LastError   string
	PromptCount uint64
}
