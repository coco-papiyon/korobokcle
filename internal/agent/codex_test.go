package agent

import "testing"

func TestNewCodexSessionConfigEnablesPTY(t *testing.T) {
	t.Parallel()

	cfg := NewCodexSessionConfig("codex", []string{"exec"}, ".", nil)
	if !cfg.UsePTY {
		t.Fatalf("expected codex session config to enable PTY")
	}
}
