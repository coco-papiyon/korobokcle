package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/agent"
)

const defaultChildArgsJSON = `["exec"]`

func main() {
	var (
		childBin      = flag.String("child-bin", "codex", "child process binary")
		childArgsJSON = flag.String("child-args-json", defaultChildArgsJSON, "JSON array of child process args")
		workDir       = flag.String("work-dir", ".", "working directory for child process")
		endMarker     = flag.String("end-marker", "__KOROBOKCLE_END__", "stdout end marker")
		idleTimeoutMS = flag.Int("idle-timeout-ms", 200, "idle timeout in milliseconds")
		usePTY        = flag.Bool("use-pty", true, "run child inside a pseudoterminal")
	)
	flag.Parse()

	var childArgs []string
	if err := json.Unmarshal([]byte(*childArgsJSON), &childArgs); err != nil {
		log.Fatalf("decode child-args-json: %v", err)
	}

	cfg := agent.SessionConfig{
		Command:           *childBin,
		Args:              childArgs,
		WorkDir:           *workDir,
		RequestTerminator: "\n",
		EndMarker:         *endMarker,
		IdleTimeout:       time.Duration(*idleTimeoutMS) * time.Millisecond,
		UsePTY:            *usePTY,
		Env:               os.Environ(),
	}

	if err := agent.RunWrapper(context.Background(), os.Stdin, os.Stdout, cfg); err != nil {
		log.Fatal(err)
	}
}
