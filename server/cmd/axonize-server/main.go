package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/axonize/server/internal"
	"github.com/axonize/server/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfgPath := "config.yaml"
	if p := os.Getenv("AXONIZE_CONFIG"); p != "" {
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Info("config file not found, using defaults/env", "path", cfgPath)
		cfg = config.Default()
	}

	cfg.ApplyEnv()

	logger.Info("starting axonize server",
		"grpc_port", cfg.Server.GRPCPort,
		"http_port", cfg.Server.HTTPPort,
		"clickhouse", cfg.ClickHouse.Host,
		"postgresql", cfg.PostgreSQL.Host,
	)

	srv, err := internal.NewServer(cfg, logger)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
