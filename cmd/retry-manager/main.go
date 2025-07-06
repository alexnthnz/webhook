package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/retry"
	"github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/redis"
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

	log.Info("Starting Retry Manager Service")

	// Initialize Redis connection
	redisConfig := redis.Config{
		Address:         cfg.RetryManager.Redis.Address,
		Password:        cfg.RetryManager.Redis.Password,
		DB:              cfg.RetryManager.Redis.DB,
		MaxRetries:      cfg.RetryManager.Redis.MaxRetries,
		DialTimeout:     cfg.RetryManager.Redis.DialTimeout,
		ReadTimeout:     cfg.RetryManager.Redis.ReadTimeout,
		WriteTimeout:    cfg.RetryManager.Redis.WriteTimeout,
		PoolSize:        cfg.RetryManager.Redis.PoolSize,
		MinIdleConns:    cfg.RetryManager.Redis.MinIdleConns,
		MaxIdleConns:    cfg.RetryManager.Redis.MaxIdleConns,
		ConnMaxLifetime: cfg.RetryManager.Redis.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.RetryManager.Redis.ConnMaxIdleTime,
	}

	redisClient, err := redis.NewClient(redisConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize repository
	repo := retry.NewRepository(redisClient, log)

	// Initialize service
	service := retry.NewService(repo, log)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(log)),
	)

	// Register services
	pb.RegisterRetryManagerServer(grpcServer, service)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable reflection for development
	reflection.Register(grpcServer)

	// Start gRPC server
	addr := fmt.Sprintf("%s:%d", cfg.RetryManager.Host, cfg.RetryManager.Port)
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

	// Start retry processor in background
	go startRetryProcessor(service, cfg.RetryManager, log)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down Retry Manager Service")

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

	log.Info("Retry Manager Service shutdown complete")
}

// startRetryProcessor starts the background retry processor
func startRetryProcessor(service *retry.Service, cfg config.RetryManagerConfig, log *logrus.Logger) {
	// Initialize Kafka producer for retry processor
	kafkaConfig := kafka.ProducerConfig{
		Brokers:           "kafka-kraft:29092",
		ClientID:          "retry-processor",
		CompressionType:   "snappy",
		BatchSize:         16384,
		LingerMs:          10,
		RetryBackoffMs:    100,
		MaxRetries:        3,
		RequestTimeoutMs:  30000,
		MessageTimeoutMs:  300000,
		Acks:              "all",
		EnableIdempotence: true,
	}

	producer, err := kafka.NewProducer(kafkaConfig, log)
	if err != nil {
		log.WithError(err).Error("Failed to create Kafka producer for retry processor")
		return
	}
	defer producer.Close()

	// Create retry processor
	processor := retry.NewProcessor(service, producer, cfg, log)

	// Start processor
	ctx := context.Background()
	processor.Start(ctx)
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
