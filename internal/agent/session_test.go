package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSessionSendReadsStdoutAndStderr(t *testing.T) {
	t.Parallel()

	session := newTestSession(t)
	defer session.Close()

	response, err := session.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !strings.Contains(response.Stdout, "stdout[1]: hello") {
		t.Fatalf("expected stdout to contain helper response, got %q", response.Stdout)
	}
	if !strings.Contains(response.Stderr, "stderr[1]: hello") {
		t.Fatalf("expected stderr to contain helper response, got %q", response.Stderr)
	}
}

func TestSessionKeepsProcessStateAcrossRequests(t *testing.T) {
	t.Parallel()

	session := newTestSession(t)
	defer session.Close()

	first, err := session.Send(context.Background(), "first")
	if err != nil {
		t.Fatalf("first Send() error = %v", err)
	}
	second, err := session.Send(context.Background(), "second")
	if err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	if !strings.Contains(first.Stdout, "stdout[1]: first") {
		t.Fatalf("unexpected first stdout %q", first.Stdout)
	}
	if !strings.Contains(second.Stdout, "stdout[2]: second") {
		t.Fatalf("unexpected second stdout %q", second.Stdout)
	}
}

func TestSessionCloseStopsProcess(t *testing.T) {
	t.Parallel()

	session := newTestSession(t)
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := session.Send(context.Background(), "after-close"); err == nil {
		t.Fatalf("expected Send() after close to fail")
	}
}

func TestSessionSendUsesEndMarkerFraming(t *testing.T) {
	t.Parallel()

	session := newMarkerSession(t)
	defer session.Close()

	response, err := session.Send(context.Background(), "marker")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if strings.Contains(response.Stdout, "__END__") {
		t.Fatalf("expected end marker to be removed, got %q", response.Stdout)
	}
	if !strings.Contains(response.Stdout, "marker-response: marker") {
		t.Fatalf("unexpected stdout %q", response.Stdout)
	}
	if !strings.Contains(response.Stderr, "marker-stderr: marker") {
		t.Fatalf("unexpected stderr %q", response.Stderr)
	}
}

func TestSessionSendJSONLReadsStructuredEvents(t *testing.T) {
	t.Parallel()

	session := newJSONLSession(t)
	defer session.Close()

	response, err := session.SendJSONL(context.Background(), "jsonl")
	if err != nil {
		t.Fatalf("SendJSONL() error = %v", err)
	}
	if len(response.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(response.Events))
	}
	if response.Events[0].Type != "message" || response.Events[0].Data != "jsonl" {
		t.Fatalf("unexpected first event %+v", response.Events[0])
	}
	if response.Events[2].Type != "end" {
		t.Fatalf("expected end event, got %+v", response.Events[2])
	}
	if !strings.Contains(response.Stderr, "jsonl-stderr: jsonl") {
		t.Fatalf("unexpected stderr %q", response.Stderr)
	}
}

func TestSessionSendUsesPTYWhenRequested(t *testing.T) {
	t.Parallel()

	cfg := SessionConfig{
		Command:           os.Args[0],
		Args:              []string{"-test.run=TestAgentSessionHelperProcess"},
		Env:               append(os.Environ(), "GO_WANT_AGENT_HELPER_PROCESS=1", "GO_AGENT_HELPER_MODE=tty"),
		RequestTerminator: "\n",
		IdleTimeout:       500 * time.Millisecond,
		UsePTY:            true,
	}
	session, err := StartSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	defer session.Close()

	response, err := session.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	wantTTY := "false"
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		wantTTY = "true"
	}
	if !strings.Contains(response.Stdout, "tty="+wantTTY) {
		t.Fatalf("expected tty=%s, got %q", wantTTY, response.Stdout)
	}
}

func TestAgentSessionHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_HELPER_PROCESS") != "1" {
		return
	}

	switch os.Getenv("GO_AGENT_HELPER_MODE") {
	case "marker":
		runMarkerHelper()
		return
	case "jsonl":
		runJSONLHelper()
		return
	case "tty":
		runTTYHelper()
		return
	}

	counter := 0
	for {
		var input string
		_, err := fmt.Fscanln(os.Stdin, &input)
		if err != nil {
			return
		}
		counter++
		fmt.Fprintf(os.Stdout, "stdout[%d]: %s\n", counter, input)
		fmt.Fprintf(os.Stderr, "stderr[%d]: %s\n", counter, input)
	}
}

func newTestSession(t *testing.T) *Session {
	t.Helper()

	cfg := SessionConfig{
		Command:           os.Args[0],
		Args:              []string{"-test.run=TestAgentSessionHelperProcess"},
		Env:               append(os.Environ(), "GO_WANT_AGENT_HELPER_PROCESS=1"),
		RequestTerminator: "\n",
		IdleTimeout:       500 * time.Millisecond,
	}
	session, err := StartSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	return session
}

func newMarkerSession(t *testing.T) *Session {
	t.Helper()

	cfg := SessionConfig{
		Command:           os.Args[0],
		Args:              []string{"-test.run=TestAgentSessionHelperProcess"},
		Env:               append(os.Environ(), "GO_WANT_AGENT_HELPER_PROCESS=1", "GO_AGENT_HELPER_MODE=marker"),
		RequestTerminator: "\n",
		EndMarker:         "__END__",
	}
	session, err := StartSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	return session
}

func newJSONLSession(t *testing.T) *Session {
	t.Helper()

	cfg := SessionConfig{
		Command:           os.Args[0],
		Args:              []string{"-test.run=TestAgentSessionHelperProcess"},
		Env:               append(os.Environ(), "GO_WANT_AGENT_HELPER_PROCESS=1", "GO_AGENT_HELPER_MODE=jsonl"),
		RequestTerminator: "\n",
	}
	session, err := StartSession(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	return session
}

func runMarkerHelper() {
	for {
		var input string
		_, err := fmt.Fscanln(os.Stdin, &input)
		if err != nil {
			return
		}
		fmt.Fprintf(os.Stdout, "marker-response: %s\n__END__\n", input)
		fmt.Fprintf(os.Stderr, "marker-stderr: %s\n", input)
	}
}

func runJSONLHelper() {
	for {
		var input string
		_, err := fmt.Fscanln(os.Stdin, &input)
		if err != nil {
			return
		}
		writeJSONLEvent(JSONLEvent{Type: "message", RequestID: "req-1", Data: input})
		writeJSONLEvent(JSONLEvent{Type: "result", RequestID: "req-1", Data: "done"})
		writeJSONLEvent(JSONLEvent{Type: "end", RequestID: "req-1"})
		fmt.Fprintf(os.Stderr, "jsonl-stderr: %s\n", input)
	}
}

func writeJSONLEvent(event JSONLEvent) {
	raw, _ := json.Marshal(event)
	fmt.Fprintf(os.Stdout, "%s\n", raw)
}

func runTTYHelper() {
	// Emit the TTY state before reading input so PTY/ConPTY output can be asserted
	// even when the terminal injects control sequences around the stream.
	fmt.Fprintf(os.Stdout, "tty=%t\n", isCharDevice(os.Stdout))
	for {
		var input string
		_, err := fmt.Fscanln(os.Stdin, &input)
		if err != nil {
			return
		}
		fmt.Fprintf(os.Stdout, "tty=%t input=%s\n", isCharDevice(os.Stdout), input)
	}
}

func isCharDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
