package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const implementationPatchArtifact = "implementation.patch"

func applyImplementationPatch(ctx context.Context, root string, artifactDir string, output string) error {
	patch, err := extractImplementationPatch(output)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, implementationPatchArtifact), []byte(patch), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(patch)

	raw, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply failed: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	return nil
}

func extractImplementationPatch(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", fmt.Errorf("implementation output is empty")
	}

	if patch := extractFencedPatch(trimmed); patch != "" {
		return ensureTrailingNewline(patch), nil
	}

	if idx := strings.Index(trimmed, "diff --git "); idx >= 0 {
		return ensureTrailingNewline(strings.TrimSpace(trimmed[idx:])), nil
	}

	return "", fmt.Errorf("implementation output must contain a unified diff patch")
}

func extractFencedPatch(output string) string {
	lines := strings.Split(output, "\n")
	inFence := false
	fenceTag := ""
	buf := make([]string, 0, len(lines))

	flush := func() string {
		patch := strings.TrimSpace(strings.Join(buf, "\n"))
		buf = buf[:0]
		return patch
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				fenceTag = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				inFence = true
				buf = buf[:0]
				continue
			}

			if fenceTag == "" || fenceTag == "diff" || fenceTag == "patch" || fenceTag == "unified-diff" {
				return flush()
			}

			inFence = false
			fenceTag = ""
			buf = buf[:0]
			continue
		}

		if inFence {
			buf = append(buf, line)
		}
	}

	return ""
}

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

func ensureTrailingNewline(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	if strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}
