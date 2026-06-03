package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/liyuhui/micro-uac/internal/app"
)

func main() {
	var configPath = flag.String("config", "config.json", "path to JSON config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server, cleanup, err := app.NewServer(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := server.Start(ctx); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
