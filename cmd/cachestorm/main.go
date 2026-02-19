package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/server"
)

var (
	configPath = flag.String("config", "", "path to config file")
	bind       = flag.String("bind", "", "bind address")
	port       = flag.Int("port", 0, "server port")
)

func main() {
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if *bind != "" {
		cfg.Server.Bind = *bind
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)

	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start server")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	logger.Info().Msg("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("shutdown error")
		os.Exit(1)
	}
}
