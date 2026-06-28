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
		baseDir = flag.String("base-dir", "", "base directory for the target repository")
		toolDir = flag.String("tool-dir", "", "tool directory for korobokcle")
		addr    = flag.String("addr", ":8080", "http listen address")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.Options{
		BaseDir: *baseDir,
		ToolDir: *toolDir,
		Addr:    *addr,
	}); err != nil {
		log.Fatal(err)
	}
}
