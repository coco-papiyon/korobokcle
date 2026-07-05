package agentworker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCodexWorkerSendPromptAt(t *testing.T) {
	if os.Getenv("KOROBOKCLE_CODEX_HELPER") == "1" {
		serveCodexProtocol()
		return
	}
	w := NewCodex(CodexConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=^TestCodexWorkerSendPromptAt$"},
		Env:     []string{"KOROBOKCLE_CODEX_HELPER=1"},
		Dir:     ".",
	})
	testRequestWorker(t, w)
}

func TestCopilotWorkerSendPromptAt(t *testing.T) {
	if os.Getenv("KOROBOKCLE_COPILOT_HELPER") == "1" {
		serveCopilotProtocol()
		return
	}
	w := NewCopilot(CopilotConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=^TestCopilotWorkerSendPromptAt$"},
		Env:     []string{"KOROBOKCLE_COPILOT_HELPER=1"},
		Dir:     ".",
	})
	testRequestWorker(t, w)
}

func TestDefaultCodexLaunchConfig(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		command string
		args    []string
	}{
		{
			name:    "windows",
			goos:    "windows",
			command: "cmd",
			args:    []string{"/c", "codex", "app-server", "--stdio"},
		},
		{
			name:    "linux",
			goos:    "linux",
			command: "codex",
			args:    []string{"app-server", "--stdio"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, args := defaultCodexLaunchConfig(tt.goos)
			if command != tt.command || !reflect.DeepEqual(args, tt.args) {
				t.Fatalf("defaultCodexLaunchConfig(%q) = %q, %v, want %q, %v", tt.goos, command, args, tt.command, tt.args)
			}
		})
	}
}

func TestDefaultCopilotLaunchConfig(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		command string
		args    []string
	}{
		{
			name:    "windows",
			goos:    "windows",
			command: "cmd",
			args:    []string{"/c", "copilot", "--acp", "--stdio"},
		},
		{
			name:    "linux",
			goos:    "linux",
			command: "copilot",
			args:    []string{"--acp", "--stdio"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, args := defaultCopilotLaunchConfig(tt.goos)
			if command != tt.command || !reflect.DeepEqual(args, tt.args) {
				t.Fatalf("defaultCopilotLaunchConfig(%q) = %q, %v, want %q, %v", tt.goos, command, args, tt.command, tt.args)
			}
		})
	}
}

func TestCommandRequestAllowed(t *testing.T) {
	params := json.RawMessage(`{
		"command": "\"C:\\Program Files\\PowerShell\\7\\pwsh.exe\" -Command 'npm run build'",
		"commandActions": [{"type": "unknown", "command": "npm run build"}],
		"proposedExecpolicyAmendment": ["npm", "run", "build"]
	}`)
	if !commandRequestAllowed(params, []string{" npm   run   build "}) {
		t.Fatal("expected npm run build to be allowed")
	}
	if commandRequestAllowed(params, []string{"python --version"}) {
		t.Fatal("expected python --version not to allow npm run build")
	}
}

func TestCommandRequestAllowedWithArguments(t *testing.T) {
	params := json.RawMessage(`{"command":"git log --oneline -10"}`)
	if !commandRequestAllowed(params, []string{"git log"}) {
		t.Fatal("expected git log options to be allowed by git log")
	}
	params = json.RawMessage(`{"command":"git logger --oneline"}`)
	if commandRequestAllowed(params, []string{"git log"}) {
		t.Fatal("expected git logger not to be allowed by git log")
	}
	params = json.RawMessage(`{"command":"git log --oneline && npm test"}`)
	if commandRequestAllowed(params, []string{"git log"}) {
		t.Fatal("expected chained command not to be allowed by git log")
	}
}

func TestCommandRequestAllowedWithBuiltInCommands(t *testing.T) {
	for _, raw := range []string{
		`{"command":"git diff --stat"}`,
		`{"command":"git status --short"}`,
		`{"command":"Select-String -Pattern TODO README.md"}`,
		`{"command":"Select-Object -First 10"}`,
		`{"command":"head -20 README.md"}`,
	} {
		if !commandRequestAllowed(json.RawMessage(raw), nil) {
			t.Fatalf("expected built-in command to be allowed: %s", raw)
		}
	}
}

func TestCommandRequestAllowedWithPowerShellEnvAssignments(t *testing.T) {
	params := json.RawMessage(`{
		"commandActions": [{
			"type": "unknown",
			"command": "$env:npm_config_cache='C:\\repo\\frontend\\.tmp\\npm-cache'; $env:TEMP='C:\\repo\\frontend\\.tmp\\npm-tmp'; npm ci"
		}]
	}`)
	if !commandRequestAllowed(params, []string{"npm ci"}) {
		t.Fatal("expected npm ci with PowerShell env assignments to be allowed")
	}

	params = json.RawMessage(`{
		"commandActions": [{
			"type": "unknown",
			"command": "Remove-Item -Recurse -Force .\\frontend\\.tmp; npm ci"
		}]
	}`)
	if commandRequestAllowed(params, []string{"npm ci"}) {
		t.Fatal("expected mixed command sequence not to be allowed by npm ci")
	}
}

func TestCopilotServerResponseAllowsConfiguredCommand(t *testing.T) {
	params := json.RawMessage(`{
		"toolCall": {
			"kind": "execute",
			"rawInput": {"command": "npm run build"}
		}
	}`)
	got := copilotServerResponse("session/request_permission", params, []string{"npm run build"}, t.TempDir())
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
	}
	got = copilotServerResponse("session/request_permission", params, []string{"python --version"}, t.TempDir())
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
	}
}

func TestCopilotServerResponseAllowsConfiguredCommandWrappedInWorktree(t *testing.T) {
	worktree := t.TempDir()
	frontend := filepath.Join(worktree, "frontend")
	params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
		"kind": "execute",
		"rawInput": map[string]any{
			"command": `cd "` + frontend + `" && npm ci 2>&1 | tail -20`,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci"}, worktree)
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
	}
}

func TestCopilotServerResponseAllowsConfiguredCommandAfterPowerShellCD(t *testing.T) {
	worktree := t.TempDir()
	frontend := filepath.Join(worktree, "frontend")
	params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
		"kind": "execute",
		"rawInput": map[string]any{
			"command": `cd ` + frontend + ` ; npm ci 2>&1 | tail -20`,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci"}, worktree)
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
	}
}

func TestCopilotServerResponseAllowsCommandSequenceWhenEveryCommandIsConfigured(t *testing.T) {
	worktree := t.TempDir()
	params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
		"kind": "execute",
		"rawInput": map[string]any{
			"command": `cd frontend && npm ci --silent && npm test --silent`,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci", "npm test"}, worktree)
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
	}
}

func TestCopilotServerResponseSplitsSupportedShellOperators(t *testing.T) {
	worktree := t.TempDir()
	for _, command := range []string{
		`npm ci && npm test`,
		`npm ci || npm test`,
		`npm ci; npm test`,
		`npm ci 2>&1 | tail -20`,
	} {
		t.Run(command, func(t *testing.T) {
			params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
				"kind": "execute", "rawInput": map[string]any{"command": command},
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := copilotServerResponse("session/request_permission", params, []string{"npm ci", "npm test"}, worktree)
			if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
				t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
			}
		})
	}
}

func TestCopilotServerResponseRejectsSequenceContainingUnconfiguredCommand(t *testing.T) {
	params := json.RawMessage(`{"toolCall":{"kind":"execute","rawInput":{"command":"npm ci && Remove-Item -Recurse ."}}}`)
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci"}, t.TempDir())
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
	}
}

func TestCopilotServerResponseRejectsWrappedCommandOutsideWorktree(t *testing.T) {
	worktree := t.TempDir()
	outside := filepath.Join(filepath.Dir(worktree), "outside")
	params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
		"kind": "execute",
		"rawInput": map[string]any{
			"command": `cd "` + outside + `" && npm ci 2>&1 | tail -20`,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci"}, worktree)
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
	}
}

func TestCopilotServerResponseAllowsReadAndEditWithinWorktree(t *testing.T) {
	worktree := t.TempDir()
	inside := filepath.Join(worktree, "frontend", "app.go")
	outside := filepath.Join(filepath.Dir(worktree), "outside.go")

	for _, kind := range []string{"read", "edit"} {
		t.Run(kind+" inside", func(t *testing.T) {
			params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
				"kind":      kind,
				"rawInput":  map[string]any{"fileName": inside},
				"locations": []map[string]any{{"path": inside}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := copilotServerResponse("session/request_permission", params, nil, worktree)
			if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
				t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
			}
		})

		t.Run(kind+" outside", func(t *testing.T) {
			params, err := json.Marshal(map[string]any{"toolCall": map[string]any{
				"kind":     kind,
				"rawInput": map[string]any{"path": outside},
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := copilotServerResponse("session/request_permission", params, nil, worktree)
			if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
				t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
			}
		})
	}
}

func TestCopilotServerResponseRejectsUnknownPermissionKind(t *testing.T) {
	params := json.RawMessage(`{"toolCall":{"kind":"fetch","rawInput":{"url":"https://example.com"}}}`)
	got := copilotServerResponse("session/request_permission", params, nil, t.TempDir())
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
	}
}

func TestNormalizeAllowedCommands(t *testing.T) {
	got := normalizeAllowedCommands([]string{" npm ci ", "", "NPM   CI", "go test ./..."})
	wantContains := []string{
		"npm install",
		"npm ci",
		"npm test",
		"go build",
		"go test",
		"go mod tidy",
		"go mod download",
		"git log",
		"git diff",
		"git status",
		"head",
		"select-object",
		"select-string",
		"go test ./...",
	}
	for _, want := range wantContains {
		if !containsString(got, want) {
			t.Fatalf("normalizeAllowedCommands() = %+v, want to contain %q", got, want)
		}
	}
}

func TestCopilotServerResponseAllowsBuiltInCommandWithoutConfiguredAllowedCommands(t *testing.T) {
	params := json.RawMessage(`{
		"toolCall": {
			"kind": "execute",
			"rawInput": {"command": "git diff --stat"}
		}
	}`)
	got := copilotServerResponse("session/request_permission", params, nil, t.TempDir())
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want allow_once selected", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func testRequestWorker(t *testing.T, worker RequestWorker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := worker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Stop(context.Background()) })
	var stdoutLog bytes.Buffer
	var stderrLog bytes.Buffer
	worker.SetOutputWriters(&stdoutLog, &stderrLog)
	out, err := worker.SendPromptAt(ctx, "ping", t.TempDir(), "test-model")
	if err != nil {
		t.Fatal(err)
	}
	if out != "pong" {
		t.Fatalf("response = %q, want pong", out)
	}
	if status := worker.GetStatus(); status.State != StateIdle || status.PromptCount != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !strings.Contains(stdoutLog.String(), `"result"`) {
		t.Fatalf("stdout log = %q, want RPC output", stdoutLog.String())
	}
	if !strings.Contains(stderrLog.String(), "helper stderr") {
		t.Fatalf("stderr log = %q, want helper stderr", stderrLog.String())
	}
}

type testRPCRequest struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func serveCodexProtocol() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req testRPCRequest
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"id": req.ID, "result": map[string]any{}})
		case "initialized":
		case "thread/start":
			var params struct {
				CWD   string `json:"cwd"`
				Model string `json:"model"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if !filepath.IsAbs(params.CWD) || params.Model != "test-model" {
				os.Exit(2)
			}
			_ = encoder.Encode(map[string]any{"id": req.ID, "result": map[string]any{"thread": map[string]any{"id": "thread-1"}}})
		case "turn/start":
			_, _ = os.Stderr.WriteString("helper stderr\n")
			_ = encoder.Encode(map[string]any{"id": req.ID, "result": map[string]any{"turn": map[string]any{"id": "turn-1"}}})
			_ = encoder.Encode(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-1", "item": map[string]any{"type": "agentMessage", "text": "pong"}}})
			_ = encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed"}}})
		}
	}
}

func serveCopilotProtocol() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req testRPCRequest
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
		case "session/new":
			var params struct {
				CWD string `json:"cwd"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if !filepath.IsAbs(params.CWD) {
				os.Exit(2)
			}
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"sessionId": "session-1"}})
		case "session/prompt":
			_, _ = os.Stderr.WriteString("helper stderr\n")
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "session-1", "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "pong"}}}})
			_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"stopReason": "end_turn"}})
		}
	}
}
