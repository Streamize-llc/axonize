package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/axonize/server/internal/api"
	"github.com/axonize/server/internal/config"
	"github.com/axonize/server/internal/ingest"
	"github.com/axonize/server/internal/store"
	"github.com/axonize/server/internal/tenant"
)

// Server orchestrates the gRPC and HTTP listeners.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger

	chStore  *store.ClickHouseStore
	pgStore  *store.PostgresStore
	ingest   *ingest.Handler
	grpcSrv  *grpc.Server
	httpSrv  *http.Server
	resolver *tenant.Resolver
	meter    *tenant.UsageMeter
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

	s := &Server{
		cfg:     cfg,
		logger:  logger,
		chStore: chStore,
		pgStore: pgStore,
	}

	// Multi-tenant setup
	var grpcOpts []grpc.ServerOption
	if cfg.Server.AuthMode == "multi_tenant" {
		s.resolver = tenant.NewResolver(pgStore.Pool(), logger)
		s.meter = tenant.NewUsageMeter(pgStore.Pool(), logger)
		grpcOpts = append(grpcOpts, grpc.UnaryInterceptor(multiTenantInterceptor(s.resolver)))
	} else if cfg.Server.APIKey != "" {
		grpcOpts = append(grpcOpts, grpc.UnaryInterceptor(apiKeyInterceptor(cfg.Server.APIKey)))
	}

	var meter ingest.UsageRecorder
	if s.meter != nil {
		meter = s.meter
	}
	handler := ingest.NewHandler(chStore, pgStore, logger, meter)
	s.ingest = handler

	router := api.NewRouter(chStore, pgStore, cfg.Server.APIKey, cfg.Server.AuthMode, s.resolver, cfg.Server.AdminKey)
	grpcSrv := grpc.NewServer(grpcOpts...)
	collectorpb.RegisterTraceServiceServer(grpcSrv, handler)

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.grpcSrv = grpcSrv
	s.httpSrv = httpSrv

	return s, nil
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

// apiKeyInterceptor returns a gRPC unary interceptor that validates
// the "authorization" metadata key against "Bearer <apiKey>".
// Sets tenant_id to "default" for static auth mode.
func apiKeyInterceptor(apiKey string) grpc.UnaryServerInterceptor {
	expected := "Bearer " + apiKey
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("authorization")
		if len(vals) == 0 || !strings.EqualFold(vals[0], expected) {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}
		ctx = tenant.WithTenantID(ctx, tenant.DefaultTenantID)
		return handler(ctx, req)
	}
}

// multiTenantInterceptor returns a gRPC unary interceptor that resolves
// the API key to a tenant_id via the tenant.Resolver.
func multiTenantInterceptor(resolver *tenant.Resolver) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("authorization")
		if len(vals) == 0 || !strings.HasPrefix(vals[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing API key")
		}
		rawKey := vals[0][7:]

		tenantID, err := resolver.Resolve(ctx, rawKey)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}

		ctx = tenant.WithTenantID(ctx, tenantID)
		return handler(ctx, req)
	}
}
