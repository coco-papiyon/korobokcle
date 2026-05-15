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
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, ".", app.Options{Debug: *debug}); err != nil {
		log.Fatal(err)
	}
}
