package app

import "strings"

func trimImplementationSummary(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	for _, marker := range []string{"\n## Patch\n", "\n## パッチ\n", "\n### Patch\n", "\n### パッチ\n"} {
		if idx := strings.Index(trimmed, marker); idx >= 0 {
			return strings.TrimSpace(trimmed[:idx])
		}
	}

	return trimmed
}
