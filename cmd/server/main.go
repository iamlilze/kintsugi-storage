package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kintsugi-storage/internal/app"
	"kintsugi-storage/internal/config"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	application, err := app.New(cfg)
	if err != nil {
		return fmt.Errorf("build app: %w", err)
	}

	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("run app: %w", err)
	}

	return nil
}
