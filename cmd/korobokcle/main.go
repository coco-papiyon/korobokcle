package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/coco-papiyon/korobokcle/internal/app"
)

func main() {
	var (
		baseDir  = flag.String("base-dir", "", "base directory for the target repository")
		toolDir  = flag.String("tool-dir", "", "tool directory for korobokcle")
		workDir  = flag.String("work-dir", "", "work directory for korobokcle data")
		mockMode = flag.Bool("mock-mode", false, "run with local mock data and mock AI")
		addr     = flag.String("addr", ":8080", "http listen address")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.Options{
		BaseDir:  *baseDir,
		ToolDir:  *toolDir,
		WorkDir:  *workDir,
		MockMode: *mockMode,
		Addr:     *addr,
	}); err != nil {
		log.Fatal(err)
	}
}
