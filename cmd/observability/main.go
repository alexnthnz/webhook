package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/observability"
	"github.com/alexnthnz/webhook/pkg/metrics"
	"github.com/alexnthnz/webhook/pkg/postgres"
	pb "github.com/alexnthnz/webhook/proto/generated/proto"
)

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Set log level from config
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.WithError(err).Warn("Invalid log level, using info")
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	log.Info("Starting Observability Service")

	// Initialize metrics
	metricsConfig := metrics.Config{
		Enabled:    cfg.Observability.Metrics.Enabled,
		ListenAddr: cfg.Observability.Metrics.ListenAddr,
		Path:       cfg.Observability.Metrics.Path,
		Namespace:  cfg.Observability.Metrics.Namespace,
		Subsystem:  cfg.Observability.Metrics.Subsystem,
	}

	metricsClient := metrics.NewMetrics(metricsConfig, log)

	// Initialize database connection
	dbConfig := postgres.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		SSLMode:  cfg.Database.SSLMode,
		MaxConns: cfg.Database.MaxConns,
		MinConns: cfg.Database.MinConns,
		MaxLife:  cfg.Database.MaxLife,
		MaxIdle:  cfg.Database.MaxIdle,
	}

	db, err := postgres.NewClient(dbConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize service
	service := observability.NewService(metricsClient, db, log)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(log)),
	)

	// Register services
	pb.RegisterObservabilityServiceServer(grpcServer, service)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable reflection for development
	reflection.Register(grpcServer)

	// Start metrics HTTP server
	go startMetricsServer(cfg.Observability.Metrics.ListenAddr, log)

	// Start gRPC server
	addr := fmt.Sprintf("%s:%d", cfg.Observability.Host, cfg.Observability.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithError(err).Fatal("Failed to create listener")
	}

	log.WithField("address", addr).Info("Starting gRPC server")

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.WithError(err).Fatal("Failed to serve gRPC server")
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down Observability Service")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop health server
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Stop gRPC server
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout exceeded, forcing shutdown")
		grpcServer.Stop()
	case <-stopped:
		log.Info("Server stopped gracefully")
	}

	log.Info("Observability Service shutdown complete")
}

// startMetricsServer starts the Prometheus metrics HTTP server
func startMetricsServer(addr string, log *logrus.Logger) {
	if addr == "" {
		addr = ":9090"
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.WithField("address", addr).Info("Starting metrics HTTP server")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Error("Metrics server failed")
	}
}

// loggingInterceptor logs gRPC requests
func loggingInterceptor(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		entry := log.WithFields(logrus.Fields{
			"method":   info.FullMethod,
			"duration": duration,
		})

		if err != nil {
			entry.WithError(err).Error("gRPC request failed")
		} else {
			entry.Info("gRPC request completed")
		}

		return resp, err
	}
}
