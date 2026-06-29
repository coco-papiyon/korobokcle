package agentworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type CodexConfig struct {
	Command     string
	Args        []string
	Dir         string
	Env         []string
	StopTimeout time.Duration
	Ephemeral   bool
}

type CodexWorker struct {
	cfg      CodexConfig
	mu       sync.RWMutex
	status   Status
	rpc      *rpcProcess
	threadID string
	turnID   string
	promptMu sync.Mutex
}

func NewCodex(cfg CodexConfig) *CodexWorker {
	if cfg.Command == "" {
		cfg.Command = "codex"
	}
	if len(cfg.Args) == 0 {
		cfg.Args = []string{"app-server", "--stdio"}
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	return &CodexWorker{cfg: cfg, status: Status{State: StateNew}}
}

func (w *CodexWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.status.State != StateNew {
		w.mu.Unlock()
		return ErrAlreadyStarted
	}
	w.status.State = StateStarting
	w.mu.Unlock()

	p, err := startRPC(ctx, w.cfg.Command, w.cfg.Args, w.cfg.Env, w.cfg.Dir)
	if err != nil {
		w.fail(err)
		return err
	}
	w.rpc = p
	p.includeJSONRPC = false
	p.serverResponse = func(string) any { return map[string]any{"decision": "decline"} }
	var initialized struct{}
	err = p.call(ctx, "initialize", map[string]any{
		"clientInfo": map[string]any{"name": "korobokcle", "title": "korobokcle", "version": "0.1.0"},
	}, &initialized)
	if err == nil {
		err = p.notify("initialized", map[string]any{})
	}
	if err != nil {
		_ = p.stop(context.Background(), w.cfg.StopTimeout)
		w.fail(err)
		return err
	}
	w.mu.Lock()
	w.status.State, w.status.PID, w.status.StartedAt = StateIdle, p.cmd.Process.Pid, time.Now()
	w.mu.Unlock()
	return nil
}

func (w *CodexWorker) SendPromptAt(ctx context.Context, prompt, dir, model string) (string, error) {
	w.promptMu.Lock()
	defer w.promptMu.Unlock()

	w.mu.Lock()
	if w.status.State != StateIdle {
		w.mu.Unlock()
		return "", ErrNotRunning
	}
	w.status.State = StateBusy
	w.mu.Unlock()
	defer w.finishPrompt()

	threadID, err := w.startThread(ctx, dir, model)
	if err != nil {
		return "", err
	}
	var started struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	err = w.rpc.call(ctx, "turn/start", map[string]any{
		"threadId": threadID,
		"input":    []map[string]string{{"type": "text", "text": prompt}},
	}, &started)
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	w.turnID = started.Turn.ID
	w.mu.Unlock()

	var deltas strings.Builder
	final := ""
	for {
		select {
		case msg := <-w.rpc.notices:
			var params struct {
				ThreadID string                      `json:"threadId"`
				Turn     struct{ ID, Status string } `json:"turn"`
				Delta    string                      `json:"delta"`
				Item     struct{ Type, Text string } `json:"item"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if params.ThreadID != "" && params.ThreadID != threadID {
				continue
			}
			switch msg.Method {
			case "item/agentMessage/delta":
				deltas.WriteString(params.Delta)
			case "item/completed":
				if params.Item.Type == "agentMessage" {
					final = params.Item.Text
				}
			case "turn/completed":
				if params.Turn.ID != started.Turn.ID {
					continue
				}
				if params.Turn.Status == "failed" {
					return "", fmt.Errorf("codex turn %s failed", started.Turn.ID)
				}
				if final != "" {
					return strings.TrimSpace(final), nil
				}
				return strings.TrimSpace(deltas.String()), nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		case <-w.rpc.done:
			return "", w.rpc.processError()
		}
	}
}

func (w *CodexWorker) SetOutputWriters(stdout, stderr io.Writer) {
	w.mu.RLock()
	p := w.rpc
	w.mu.RUnlock()
	if p != nil {
		p.setOutputWriters(stdout, stderr)
	}
}

func (w *CodexWorker) startThread(ctx context.Context, dir, model string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = w.cfg.Dir
	}
	params := map[string]any{"cwd": dir, "ephemeral": w.cfg.Ephemeral}
	if strings.TrimSpace(model) != "" {
		params["model"] = strings.TrimSpace(model)
	}
	var started struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := w.rpc.call(ctx, "thread/start", params, &started); err != nil {
		return "", err
	}
	if started.Thread.ID == "" {
		return "", errors.New("codex thread/start returned an empty thread id")
	}
	w.mu.Lock()
	w.threadID = started.Thread.ID
	w.mu.Unlock()
	return started.Thread.ID, nil
}

func (w *CodexWorker) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

func (w *CodexWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	if w.rpc == nil {
		w.mu.Unlock()
		return ErrNotRunning
	}
	w.status.State = StateStopping
	p := w.rpc
	w.mu.Unlock()
	err := p.stop(ctx, w.cfg.StopTimeout)
	w.mu.Lock()
	w.status.State, w.status.PID = StateStopped, 0
	w.mu.Unlock()
	return err
}

func (w *CodexWorker) finishPrompt() {
	w.mu.Lock()
	if w.status.State == StateBusy {
		w.status.State = StateIdle
		w.status.PromptCount++
	}
	w.turnID = ""
	w.mu.Unlock()
}

func (w *CodexWorker) fail(err error) {
	w.mu.Lock()
	w.status.State, w.status.LastError = StateFailed, err.Error()
	w.mu.Unlock()
}
