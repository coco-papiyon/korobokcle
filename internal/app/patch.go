package app

import (
	"bytes"
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

func captureWorkingDiff(ctx context.Context, root string) (string, error) {
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--no-ext-diff", "--")
	diffCmd.Dir = root

	var diffOut bytes.Buffer
	diffCmd.Stdout = &diffOut
	diffCmd.Stderr = &diffOut
	if err := diffCmd.Run(); err != nil {
		return "", fmt.Errorf("git diff failed: %w: %s", err, strings.TrimSpace(diffOut.String()))
	}

	statusCmd := exec.CommandContext(ctx, "git", "status", "--short", "--untracked-files=all")
	statusCmd.Dir = root

	var statusOut bytes.Buffer
	statusCmd.Stdout = &statusOut
	statusCmd.Stderr = &statusOut
	if err := statusCmd.Run(); err != nil {
		return "", fmt.Errorf("git status failed: %w: %s", err, strings.TrimSpace(statusOut.String()))
	}

	diff := strings.TrimSpace(diffOut.String())
	status := strings.TrimSpace(statusOut.String())
	if diff == "" && status == "" {
		return "", nil
	}

	snapshot := strings.TrimSpace(strings.Join([]string{
		"## Git Diff",
		"```diff",
		diff,
		"```",
		"",
		"## Git Status",
		"```text",
		status,
		"```",
	}, "\n"))

	const maxDiffChars = 20000
	if len(snapshot) <= maxDiffChars {
		return snapshot, nil
	}
	return snapshot[:maxDiffChars] + "\n...[truncated]...", nil
}
