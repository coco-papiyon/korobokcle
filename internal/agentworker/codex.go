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
	Command         string
	Args            []string
	Dir             string
	Env             []string
	StopTimeout     time.Duration
	Ephemeral       bool
	AllowedCommands []string
}

type CodexWorker struct {
	cfg      CodexConfig
	mu       sync.RWMutex
	status   Status
	rpc      *rpcProcess
	threadID string
	turnID   string
	promptMu sync.Mutex
	allowMu  sync.RWMutex
	allowed  []string
}

func defaultAllowedCommands() []string {
	return []string{
		// npm commands
		"npm install",
		"npm ci",
		"npm test",

		// go comands
		"go build",
		"go test",
		"go mod tidy",
		"go mod download",

		// git commands
		"git log",
		"git diff",
		"git status",

		// shell commands
		"ls",
		"dir",
		"cat",
		"type",
		"more",
		"head",

		// powershell commands
		"get-childitem",
		"get-content",
		"select-object",
		"select-string",
	}
}

func NewCodex(cfg CodexConfig) *CodexWorker {
	if cfg.Command == "" {
		cfg.Command, cfg.Args = currentDefaultCodexLaunchConfig()
	} else if len(cfg.Args) == 0 {
		_, cfg.Args = defaultCodexLaunchConfig("")
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	return &CodexWorker{cfg: cfg, status: Status{State: StateNew}, allowed: normalizeAllowedCommands(cfg.AllowedCommands)}
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
	p.serverResponse = func(method string, params json.RawMessage) any {
		w.allowMu.RLock()
		allowed := append([]string(nil), w.allowed...)
		w.allowMu.RUnlock()
		return codexServerResponse(method, params, allowed)
	}
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

func (w *CodexWorker) SetAllowedCommands(commands []string) {
	w.allowMu.Lock()
	defer w.allowMu.Unlock()
	w.allowed = normalizeAllowedCommands(commands)
}

func codexServerResponse(method string, params json.RawMessage, allowed []string) any {
	if method != "item/commandExecution/requestApproval" || !commandRequestAllowed(params, allowed) {
		return map[string]any{"decision": "decline"}
	}
	return map[string]any{"decision": "accept"}
}

func commandRequestAllowed(params json.RawMessage, allowed []string) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, command := range normalizeAllowedCommands(allowed) {
		allowedSet[normalizeCommand(command)] = struct{}{}
	}
	if len(allowedSet) == 0 {
		return false
	}

	var request struct {
		Command                     string   `json:"command"`
		ProposedExecpolicyAmendment []string `json:"proposedExecpolicyAmendment"`
		CommandActions              []struct {
			Command string `json:"command"`
		} `json:"commandActions"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return false
	}
	if commandMatchesAllowed(request.Command, allowedSet) {
		return true
	}
	if len(request.ProposedExecpolicyAmendment) > 0 {
		if commandMatchesAllowed(strings.Join(request.ProposedExecpolicyAmendment, " "), allowedSet) {
			return true
		}
	}
	for _, action := range request.CommandActions {
		if commandMatchesAllowed(action.Command, allowedSet) {
			return true
		}
	}
	return false
}

func commandMatchesAllowed(command string, allowedSet map[string]struct{}) bool {
	for _, candidate := range commandCandidates(command) {
		normalized := normalizeCommand(candidate)
		if _, ok := allowedSet[normalized]; ok {
			return true
		}
		for allowed := range allowedSet {
			if strings.HasPrefix(normalized, allowed+" ") && safeCommandArguments(normalized[len(allowed):]) {
				return true
			}
		}
	}
	return false
}

func safeCommandArguments(arguments string) bool {
	return !strings.ContainsAny(arguments, ";|><`&\r\n") && !strings.Contains(arguments, "$(")
}

func commandCandidates(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	candidates := []string{command}
	if stripped, ok := stripPowerShellEnvAssignments(command); ok {
		candidates = append(candidates, stripped)
	}
	return candidates
}

func stripPowerShellEnvAssignments(command string) (string, bool) {
	segments := strings.Split(command, ";")
	remaining := make([]string, 0, len(segments))
	stripped := false
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if len(remaining) == 0 && strings.HasPrefix(strings.ToLower(segment), "$env:") && strings.Contains(segment, "=") {
			stripped = true
			continue
		}
		remaining = append(remaining, segment)
	}
	if !stripped || len(remaining) != 1 {
		return "", false
	}
	return remaining[0], true
}

func normalizeAllowedCommands(commands []string) []string {
	allCommands := append([]string{}, defaultAllowedCommands()...)
	allCommands = append(allCommands, commands...)
	seen := make(map[string]struct{}, len(allCommands))
	out := make([]string, 0, len(allCommands))
	for _, command := range allCommands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		key := normalizeCommand(command)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, command)
	}
	return out
}

func normalizeCommand(command string) string {
	return strings.ToLower(strings.Join(strings.Fields(command), " "))
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
