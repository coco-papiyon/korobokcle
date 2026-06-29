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

func TestCommandRequestAllowed(t *testing.T) {
	params := json.RawMessage(`{
		"command": "\"C:\\Program Files\\PowerShell\\7\\pwsh.exe\" -Command 'npm ci'",
		"commandActions": [{"type": "unknown", "command": "npm ci"}],
		"proposedExecpolicyAmendment": ["npm", "ci"]
	}`)
	if !commandRequestAllowed(params, []string{" npm   ci "}) {
		t.Fatal("expected npm ci to be allowed")
	}
	if commandRequestAllowed(params, []string{"npm test"}) {
		t.Fatal("expected npm test not to allow npm ci")
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
		"commandActions": [{"type": "unknown", "command": "npm ci"}],
		"proposedExecpolicyAmendment": ["npm", "ci"]
	}`)
	got := copilotServerResponse("session/request_permission", params, []string{"npm ci"})
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "approved"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want approved", got)
	}
	got = copilotServerResponse("session/request_permission", params, []string{"npm test"})
	if !reflect.DeepEqual(got, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}) {
		t.Fatalf("copilotServerResponse() = %+v, want cancelled", got)
	}
}

func TestNormalizeAllowedCommands(t *testing.T) {
	got := normalizeAllowedCommands([]string{" npm ci ", "", "NPM   CI", "go test ./..."})
	want := []string{"npm ci", "go test ./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAllowedCommands() = %+v, want %+v", got, want)
	}
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
