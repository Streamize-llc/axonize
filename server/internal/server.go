package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"

	"github.com/axonize/server/internal/api"
	"github.com/axonize/server/internal/config"
	"github.com/axonize/server/internal/ingest"
	"github.com/axonize/server/internal/store"
)

// Server orchestrates the gRPC and HTTP listeners.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger

	chStore *store.ClickHouseStore
	pgStore *store.PostgresStore
	ingest  *ingest.Handler
	grpcSrv *grpc.Server
	httpSrv *http.Server
}

// NewServer creates and wires all server components.
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	chStore, err := store.NewClickHouseStore(cfg.ClickHouse, logger)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}

	pgStore, err := store.NewPostgresStore(cfg.PostgreSQL, logger)
	if err != nil {
		return nil, fmt.Errorf("postgresql: %w", err)
	}

	handler := ingest.NewHandler(chStore, pgStore, logger)
	router := api.NewRouter(chStore, pgStore)

	grpcSrv := grpc.NewServer()
	collectorpb.RegisterTraceServiceServer(grpcSrv, handler)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return &Server{
		cfg:     cfg,
		logger:  logger,
		chStore: chStore,
		pgStore: pgStore,
		ingest:  handler,
		grpcSrv: grpcSrv,
		httpSrv: httpSrv,
	}, nil
}

// Run starts both listeners and blocks until a signal is received.
func (s *Server) Run() error {
	// Start ingest background flusher
	s.ingest.Start()

	// gRPC listener
	grpcAddr := fmt.Sprintf(":%d", s.cfg.Server.GRPCPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC %s: %w", grpcAddr, err)
	}

	errCh := make(chan error, 2)

	go func() {
		s.logger.Info("gRPC server listening", "addr", grpcAddr)
		if err := s.grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("gRPC serve: %w", err)
		}
	}()

	go func() {
		s.logger.Info("HTTP server listening", "addr", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP serve: %w", err)
		}
	}()

	// Wait for signal or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		s.logger.Info("received signal, shutting down", "signal", sig)
	case err := <-errCh:
		s.logger.Error("server error", "error", err)
		return err
	}

	return s.Shutdown()
}

// Shutdown gracefully stops all components.
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down")

	// 1. Stop accepting new gRPC connections
	s.grpcSrv.GracefulStop()

	// 2. Stop HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.httpSrv.Shutdown(ctx)

	// 3. Stop ingest flusher (final flush)
	s.ingest.Stop()

	// 4. Close stores
	s.chStore.Close()
	s.pgStore.Close()

	s.logger.Info("shutdown complete")
	return nil
}
