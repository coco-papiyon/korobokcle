package agentworker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	cfg           CopilotConfig
	mu            sync.RWMutex
	status        Status
	rpc           *rpcProcess
	sessionID     string
	promptMu      sync.Mutex
	allowMu       sync.RWMutex
	allowed       []string
	worktree      string
	permissionErr string
}

func NewCopilot(cfg CopilotConfig) *CopilotWorker {
	if cfg.Command == "" {
		cfg.Command, cfg.Args = currentDefaultCopilotLaunchConfig()
	} else if len(cfg.Args) == 0 {
		_, cfg.Args = defaultCopilotLaunchConfig("")
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
		worktree := w.worktree
		w.allowMu.RUnlock()
		response := copilotServerResponse(method, params, allowed, worktree)
		if copilotPermissionMethod(method) && !copilotPermissionAllowed(params, allowed, worktree) {
			w.allowMu.Lock()
			w.permissionErr = copilotPermissionError(params)
			w.allowMu.Unlock()
		}
		return response
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
	w.allowMu.Lock()
	w.permissionErr = ""
	w.allowMu.Unlock()
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
					w.allowMu.RLock()
					permissionErr := w.permissionErr
					w.allowMu.RUnlock()
					if err == nil && permissionErr != "" {
						err = fmt.Errorf("copilot permission denied: %s", permissionErr)
					}
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

func copilotPermissionError(params json.RawMessage) string {
	var request struct {
		ToolCall struct {
			Title    string `json:"title"`
			Kind     string `json:"kind"`
			RawInput struct {
				Command string `json:"command"`
			} `json:"rawInput"`
		} `json:"toolCall"`
	}
	if json.Unmarshal(params, &request) != nil {
		return "invalid permission request"
	}
	detail := strings.TrimSpace(request.ToolCall.RawInput.Command)
	if detail == "" {
		detail = strings.TrimSpace(request.ToolCall.Title)
	}
	if detail == "" {
		detail = "unknown operation"
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(request.ToolCall.Kind), detail)
}

func (w *CopilotWorker) startSession(ctx context.Context, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = w.cfg.Dir
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	w.allowMu.Lock()
	w.worktree = absDir
	w.allowMu.Unlock()
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

func copilotServerResponse(method string, params json.RawMessage, allowed []string, worktree string) any {
	if !copilotPermissionMethod(method) || !copilotPermissionAllowed(params, allowed, worktree) {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
	}
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}
}

func copilotPermissionAllowed(params json.RawMessage, allowed []string, worktree string) bool {
	var request struct {
		ToolCall struct {
			Kind      string          `json:"kind"`
			RawInput  json.RawMessage `json:"rawInput"`
			Locations []struct {
				Path string `json:"path"`
			} `json:"locations"`
		} `json:"toolCall"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(request.ToolCall.Kind)) {
	case "edit", "read":
		paths := make([]string, 0, len(request.ToolCall.Locations)+2)
		for _, location := range request.ToolCall.Locations {
			paths = append(paths, location.Path)
		}
		var input struct {
			Path     string `json:"path"`
			FileName string `json:"fileName"`
		}
		if json.Unmarshal(request.ToolCall.RawInput, &input) != nil {
			return false
		}
		paths = append(paths, input.Path, input.FileName)
		return pathsWithinWorktree(paths, worktree)
	case "execute":
		return copilotExecuteAllowed(request.ToolCall.RawInput, allowed, worktree)
	default:
		return false
	}
}

var (
	copilotCDCommandPattern = regexp.MustCompile(`(?i)^cd\s+(?:"([^"]+)"|'([^']+)'|(.+))$`)
	copilotRedirectPattern  = regexp.MustCompile(`\s+2>&1\s*$`)
	copilotTailPattern      = regexp.MustCompile(`(?i)^tail\s+-\d+$`)
)

func copilotExecuteAllowed(rawInput json.RawMessage, allowed []string, worktree string) bool {
	var input struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(rawInput, &input) != nil {
		return false
	}
	commands, ok := splitShellCommandSequence(input.Command)
	if !ok {
		return false
	}
	currentDir := worktree
	for _, command := range commands {
		if strings.Contains(command, "<<") {
			if copilotAllowedHeredocWrite(command, allowed, currentDir) {
				continue
			}
			return false
		}
		command = strings.TrimSpace(copilotRedirectPattern.ReplaceAllString(command, ""))
		matches := copilotCDCommandPattern.FindStringSubmatch(command)
		if len(matches) == 4 {
			dir := matches[1]
			if dir == "" {
				dir = matches[2]
			}
			if dir == "" {
				dir = strings.TrimSpace(matches[3])
			}
			resolved, allowed := resolvePathWithinWorktree(dir, currentDir, worktree)
			if !allowed {
				return false
			}
			currentDir = resolved
			continue
		}
		if copilotTailPattern.MatchString(command) {
			continue
		}
		params, err := json.Marshal(map[string]string{"command": command})
		if err != nil || !commandRequestAllowed(params, allowed) {
			return false
		}
	}
	return true
}

func copilotAllowedHeredocWrite(command string, allowed []string, currentDir string) bool {
	lines := strings.Split(command, "\n")
	if len(lines) < 2 {
		return false
	}
	header := strings.TrimSpace(lines[0])
	parts := strings.SplitN(header, "<<", 2)
	if len(parts) != 2 {
		return false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if len(right) < 3 {
		return false
	}
	quote := right[0]
	if quote != '\'' && quote != '"' {
		return false
	}
	if right[len(right)-1] != quote {
		return false
	}
	delim := strings.TrimSpace(right[1 : len(right)-1])
	if delim == "" {
		return false
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if last != delim {
		return false
	}
	cmdPart, pathPart, ok := strings.Cut(left, ">")
	if !ok {
		return false
	}
	cmd := strings.TrimSpace(cmdPart)
	if !allowedCommandExists(cmd, allowed) {
		return false
	}
	path := strings.TrimSpace(pathPart)
	path = strings.Trim(path, `"'`)
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(currentDir, path)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	tempDir, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(tempDir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func allowedCommandExists(command string, allowed []string) bool {
	normalized := normalizeCommand(command)
	for _, item := range normalizeAllowedCommands(allowed) {
		if normalizeCommand(item) == normalized {
			return true
		}
	}
	return false
}

func splitShellCommandSequence(command string) ([]string, bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, false
	}
	commands := make([]string, 0, 2)
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		char := command[i]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		separatorLength := 0
		switch char {
		case ';':
			separatorLength = 1
		case '&':
			if i > 0 && command[i-1] == '>' {
				continue
			}
			if i+1 >= len(command) || command[i+1] != '&' {
				return nil, false
			}
			separatorLength = 2
		case '|':
			separatorLength = 1
			if i+1 < len(command) && command[i+1] == '|' {
				separatorLength = 2
			}
		}
		if separatorLength == 0 {
			continue
		}
		part := strings.TrimSpace(command[start:i])
		if part == "" {
			return nil, false
		}
		commands = append(commands, part)
		i += separatorLength - 1
		start = i + 1
	}
	if quote != 0 {
		return nil, false
	}
	last := strings.TrimSpace(command[start:])
	if last == "" {
		return nil, false
	}
	return append(commands, last), true
}

func resolvePathWithinWorktree(path, currentDir, worktree string) (string, bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(currentDir, path)
	}
	resolved, err := filepath.Abs(path)
	if err != nil || !pathsWithinWorktree([]string{resolved}, worktree) {
		return "", false
	}
	return resolved, true
}

func pathsWithinWorktree(paths []string, worktree string) bool {
	root, err := filepath.Abs(strings.TrimSpace(worktree))
	if err != nil || strings.TrimSpace(worktree) == "" {
		return false
	}
	found := false
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		found = true
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		target, err := filepath.Abs(path)
		if err != nil {
			return false
		}
		rel, err := filepath.Rel(root, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false
		}
	}
	return found
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
