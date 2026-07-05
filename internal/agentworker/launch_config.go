package agentworker

import "runtime"

func defaultCodexLaunchConfig(goos string) (string, []string) {
	if goos == "windows" {
		return "cmd", []string{"/c", "codex", "app-server", "--stdio"}
	}
	return "codex", []string{"app-server", "--stdio"}
}

func defaultCopilotLaunchConfig(goos string) (string, []string) {
	if goos == "windows" {
		return "cmd", []string{"/c", "copilot", "--acp", "--stdio"}
	}
	return "copilot", []string{"--acp", "--stdio"}
}

func currentDefaultCodexLaunchConfig() (string, []string) {
	return defaultCodexLaunchConfig(runtime.GOOS)
}

func currentDefaultCopilotLaunchConfig() (string, []string) {
	return defaultCopilotLaunchConfig(runtime.GOOS)
}
