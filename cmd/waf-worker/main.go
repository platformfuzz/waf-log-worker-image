package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/platformfuzz/waf-log-worker-image/internal/config"
	"github.com/platformfuzz/waf-log-worker-image/internal/health"
	"github.com/platformfuzz/waf-log-worker-image/internal/pipeline"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "probe" {
		os.Exit(health.ProbeExitCode())
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := health.Start(ctx, cfg.HealthListenAddr); err != nil {
		cancel()
		log.Fatalf("health server: %v", err)
	}
	if err := pipeline.Run(ctx, cfg); err != nil {
		cancel()
		log.Printf("worker failed: %v", err)
		os.Exit(1)
	}
	cancel()
}
