package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coco-papiyon/korobokcle/internal/app"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	port := flag.Int("port", 0, "override HTTP port")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repoRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	toolRoot, err := resolveToolRoot()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(ctx, repoRoot, toolRoot, app.Options{
		Debug:    *debug,
		HTTPPort: *port,
	}); err != nil {
		log.Fatal(err)
	}
}

func resolveToolRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("KOROBOKCLE_TOOL_ROOT")); override != "" {
		return filepath.Abs(override)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeRoot := filepath.Dir(exe)
	if hasToolAssets(exeRoot) {
		return exeRoot, nil
	}

	cwd, err := os.Getwd()
	if err == nil && hasToolAssets(cwd) {
		return cwd, nil
	}
	return exeRoot, nil
}

func hasToolAssets(root string) bool {
	candidates := []string{
		filepath.Join(root, "config", "app.yaml"),
		filepath.Join(root, "skills", "default", "design", "skill.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	return false
}
