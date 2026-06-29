package agentworker

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CopilotConfig struct {
	Command         string
	Args            []string
	Dir             string
	Env             []string
	StopTimeout     time.Duration
	ProtocolVersion int
	AllowedCommands []string
}

type CopilotWorker struct {
	cfg       CopilotConfig
	mu        sync.RWMutex
	status    Status
	rpc       *rpcProcess
	sessionID string
	promptMu  sync.Mutex
	allowMu   sync.RWMutex
	allowed   []string
}

func NewCopilot(cfg CopilotConfig) *CopilotWorker {
	if cfg.Command == "" {
		cfg.Command = "copilot"
	}
	if len(cfg.Args) == 0 {
		cfg.Args = []string{"--acp", "--stdio"}
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	if cfg.ProtocolVersion == 0 {
		cfg.ProtocolVersion = 1
	}
	return &CopilotWorker{cfg: cfg, status: Status{State: StateNew}, allowed: normalizeAllowedCommands(cfg.AllowedCommands)}
}

func (w *CopilotWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.status.State != StateNew {
		w.mu.Unlock()
		return ErrAlreadyStarted
	}
	w.status.State = StateStarting
	w.mu.Unlock()
	dir, err := filepath.Abs(w.cfg.Dir)
	if err != nil {
		w.fail(err)
		return err
	}
	p, err := startRPC(ctx, w.cfg.Command, w.cfg.Args, w.cfg.Env, dir)
	if err != nil {
		w.fail(err)
		return err
	}
	w.rpc = p
	p.serverResponse = func(method string, params json.RawMessage) any {
		w.allowMu.RLock()
		allowed := append([]string(nil), w.allowed...)
		w.allowMu.RUnlock()
		return copilotServerResponse(method, params, allowed)
	}
	var initialized struct{}
	err = p.call(ctx, "initialize", map[string]any{
		"protocolVersion":    w.cfg.ProtocolVersion,
		"clientCapabilities": map[string]any{},
	}, &initialized)
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

func (w *CopilotWorker) SendPromptAt(ctx context.Context, prompt, dir, _ string) (string, error) {
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

	sessionID, err := w.startSession(ctx, dir)
	if err != nil {
		return "", err
	}
	resultCh := make(chan error, 1)
	go func() {
		var result struct {
			StopReason string `json:"stopReason"`
		}
		resultCh <- w.rpc.call(ctx, "session/prompt", map[string]any{
			"sessionId": sessionID,
			"prompt":    []map[string]string{{"type": "text", "text": prompt}},
		}, &result)
	}()
	var output strings.Builder
	appendUpdate := func(msg rpcMessage) {
		if msg.Method != "session/update" {
			return
		}
		var params struct {
			SessionID string `json:"sessionId"`
			Update    struct {
				SessionUpdate string                      `json:"sessionUpdate"`
				Content       struct{ Type, Text string } `json:"content"`
			} `json:"update"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		if params.SessionID == sessionID && params.Update.SessionUpdate == "agent_message_chunk" && params.Update.Content.Type == "text" {
			output.WriteString(params.Update.Content.Text)
		}
	}
	for {
		select {
		case msg := <-w.rpc.notices:
			appendUpdate(msg)
		case err := <-resultCh:
			for {
				select {
				case msg := <-w.rpc.notices:
					appendUpdate(msg)
				default:
					return strings.TrimSpace(output.String()), err
				}
			}
		case <-ctx.Done():
			return "", ctx.Err()
		case <-w.rpc.done:
			return "", w.rpc.processError()
		}
	}
}

func (w *CopilotWorker) startSession(ctx context.Context, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = w.cfg.Dir
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	var session struct {
		SessionID string `json:"sessionId"`
	}
	if err := w.rpc.call(ctx, "session/new", map[string]any{"cwd": absDir, "mcpServers": []any{}}, &session); err != nil {
		return "", err
	}
	w.mu.Lock()
	w.sessionID = session.SessionID
	w.mu.Unlock()
	return session.SessionID, nil
}

func (w *CopilotWorker) SetOutputWriters(stdout, stderr io.Writer) {
	w.mu.RLock()
	p := w.rpc
	w.mu.RUnlock()
	if p != nil {
		p.setOutputWriters(stdout, stderr)
	}
}

func (w *CopilotWorker) SetAllowedCommands(commands []string) {
	w.allowMu.Lock()
	defer w.allowMu.Unlock()
	w.allowed = normalizeAllowedCommands(commands)
}

func copilotServerResponse(method string, params json.RawMessage, allowed []string) any {
	if !copilotPermissionMethod(method) || !commandRequestAllowed(params, allowed) {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
	}
	return map[string]any{"outcome": map[string]any{"outcome": "approved"}}
}

func copilotPermissionMethod(method string) bool {
	normalized := strings.ToLower(method)
	return strings.Contains(normalized, "permission") || strings.Contains(normalized, "approval")
}

func (w *CopilotWorker) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

func (w *CopilotWorker) Stop(ctx context.Context) error {
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

func (w *CopilotWorker) finishPrompt() {
	w.mu.Lock()
	if w.status.State == StateBusy {
		w.status.State = StateIdle
		w.status.PromptCount++
	}
	w.mu.Unlock()
}

func (w *CopilotWorker) fail(err error) {
	w.mu.Lock()
	w.status.State, w.status.LastError = StateFailed, err.Error()
	w.mu.Unlock()
}
