package agent

func NewCodexSessionConfig(binary string, args []string, workDir string, env []string) SessionConfig {
	if binary == "" {
		binary = "codex"
	}
	return SessionConfig{
		Command:           binary,
		Args:              append([]string(nil), args...),
		WorkDir:           workDir,
		Env:               append([]string(nil), env...),
		RequestTerminator: "\n",
		UsePTY:            true,
	}
}

func NewCodexMarkerSessionConfig(binary string, args []string, workDir string, env []string, endMarker string) SessionConfig {
	cfg := NewCodexSessionConfig(binary, args, workDir, env)
	cfg.EndMarker = endMarker
	return cfg
}
