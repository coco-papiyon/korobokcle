package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunWrapperEmitsJSONLEvents(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	var output bytes.Buffer
	fmt.Fprintln(&input, `{"requestId":"req-1","prompt":"hello"}`)

	err := RunWrapper(context.Background(), &input, &output, SessionConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestAgentWrapperHelperProcess"},
		Env: append(
			os.Environ(),
			"GO_WANT_AGENT_WRAPPER_HELPER_PROCESS=1",
			"GO_AGENT_WRAPPER_REQUEST_TERMINATOR=__REQ_END__",
		),
		RequestTerminator: "\n__REQ_END__\n",
		EndMarker:         "__END__",
		IdleTimeout:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunWrapper() error = %v", err)
	}

	lines := splitNonEmptyLines(output.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], `"type":"result"`) || !strings.Contains(lines[0], `wrapped: hello`) || !strings.Contains(lines[0], `IMPORTANT:`) {
		t.Fatalf("unexpected result event %q", lines[0])
	}
	if !strings.Contains(lines[1], `"type":"message"`) || !strings.Contains(lines[1], `wrapped-stderr: hello`) || !strings.Contains(lines[1], `__END__`) {
		t.Fatalf("unexpected message event %q", lines[1])
	}
	if !strings.Contains(lines[2], `"type":"end"`) {
		t.Fatalf("unexpected end event %q", lines[2])
	}
}

func TestRunWrapperGeneratesRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	var output bytes.Buffer
	fmt.Fprintln(&input, `{"prompt":"hello"}`)

	err := RunWrapper(context.Background(), &input, &output, SessionConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestAgentWrapperHelperProcess"},
		Env: append(
			os.Environ(),
			"GO_WANT_AGENT_WRAPPER_HELPER_PROCESS=1",
			"GO_AGENT_WRAPPER_REQUEST_TERMINATOR=__REQ_END__",
		),
		RequestTerminator: "\n__REQ_END__\n",
		EndMarker:         "__END__",
		IdleTimeout:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunWrapper() error = %v", err)
	}

	lines := splitNonEmptyLines(output.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %q", len(lines), output.String())
	}
	for _, line := range lines {
		if !strings.Contains(line, `"requestId":"req-`) {
			t.Fatalf("expected generated requestId in %q", line)
		}
	}
}

func TestRunWrapperIgnoresEchoedPromptMarker(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	var output bytes.Buffer
	fmt.Fprintln(&input, `{"requestId":"req-echo","prompt":"hello"}`)

	err := RunWrapper(context.Background(), &input, &output, SessionConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestAgentWrapperHelperProcess"},
		Env: append(
			os.Environ(),
			"GO_WANT_AGENT_WRAPPER_HELPER_PROCESS=1",
			"GO_AGENT_WRAPPER_REQUEST_TERMINATOR=__REQ_END__",
			"GO_AGENT_WRAPPER_MODE=echo-marker",
		),
		RequestTerminator: "\n__REQ_END__\n",
		EndMarker:         "__END__",
		IdleTimeout:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunWrapper() error = %v", err)
	}

	lines := splitNonEmptyLines(output.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], `"type":"result"`) || !strings.Contains(lines[0], `final-response`) {
		t.Fatalf("unexpected result event %q", lines[0])
	}
	if strings.Contains(lines[0], `IMPORTANT:`) {
		t.Fatalf("expected echoed prompt to be removed, got %q", lines[0])
	}
}

func TestRunWrapperEmitsErrorForInvalidRequest(t *testing.T) {
	t.Parallel()

	var input bytes.Buffer
	var output bytes.Buffer
	fmt.Fprintln(&input, `{"requestId":"req-1"}`)

	err := RunWrapper(context.Background(), &input, &output, SessionConfig{
		Command:           os.Args[0],
		Args:              []string{"-test.run=TestAgentWrapperHelperProcess"},
		Env:               append(os.Environ(), "GO_WANT_AGENT_WRAPPER_HELPER_PROCESS=1"),
		RequestTerminator: "\n",
		EndMarker:         "__END__",
		IdleTimeout:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunWrapper() error = %v", err)
	}

	lines := splitNonEmptyLines(output.String())
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], `"type":"error"`) || !strings.Contains(lines[0], `prompt is required`) {
		t.Fatalf("unexpected error event %q", lines[0])
	}
	if !strings.Contains(lines[1], `"type":"end"`) {
		t.Fatalf("unexpected end event %q", lines[1])
	}
}

func TestRunWrapperExecModePassesPromptAsArgument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath, scriptBody := execProviderScript(dir)
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var input bytes.Buffer
	var output bytes.Buffer
	fmt.Fprintln(&input, `{"requestId":"req-exec","prompt":"hello exec"}`)

	err := RunWrapper(context.Background(), &input, &output, SessionConfig{
		Command:           scriptPath,
		Args:              []string{"exec"},
		Env:               os.Environ(),
		RequestTerminator: "\n",
		EndMarker:         "__END__",
		IdleTimeout:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunWrapper() error = %v", err)
	}

	lines := splitNonEmptyLines(output.String())
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], `"type":"result"`) || !strings.Contains(lines[0], `exec-arg: hello exec`) {
		t.Fatalf("unexpected result event %q", lines[0])
	}
	if !strings.Contains(lines[1], `"type":"end"`) {
		t.Fatalf("unexpected end event %q", lines[1])
	}
}

func execProviderScript(dir string) (string, string) {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "exec-provider.cmd"), "@echo off\r\necho exec-arg: %~2\r\n"
	}
	return filepath.Join(dir, "exec-provider.sh"), "#!/usr/bin/env sh\nprintf 'exec-arg: %s\\n' \"$2\"\n"
}

func TestAgentWrapperHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_WRAPPER_HELPER_PROCESS") != "1" {
		return
	}

	mode := os.Getenv("GO_AGENT_WRAPPER_MODE")
	terminator := os.Getenv("GO_AGENT_WRAPPER_REQUEST_TERMINATOR")
	reader := bufio.NewReader(os.Stdin)

	for {
		input, err := readHelperRequest(reader, terminator)
		if err != nil {
			return
		}
		if mode == "echo-marker" {
			fmt.Fprintf(os.Stdout, "%s\n%s\n", input, terminator)
			fmt.Fprint(os.Stdout, "final-response\n__END__\n")
			fmt.Fprintf(os.Stderr, "wrapped-stderr: %s\n", input)
			time.Sleep(5 * time.Millisecond)
			continue
		}
		fmt.Fprintf(os.Stdout, "wrapped: %s\n__END__\n", input)
		fmt.Fprintf(os.Stderr, "wrapped-stderr: %s\n", input)
		time.Sleep(5 * time.Millisecond)
	}
}

func readHelperRequest(reader *bufio.Reader, terminator string) (string, error) {
	if terminator == "" {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == terminator {
			return strings.TrimRight(builder.String(), "\r\n"), nil
		}
		builder.WriteString(line)
	}
}
