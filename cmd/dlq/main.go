package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/dlq"
	"github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/postgres"
	pb "github.com/alexnthnz/webhook/proto/generated/proto"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

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
	dbClient, err := postgres.NewClient(dbConfig, logger)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Initialize Kafka producer
	kafkaConfig := kafka.ProducerConfig{
		Brokers:           strings.Join(cfg.Kafka.Brokers, ","),
		ClientID:          "dlq-service",
		SecurityProtocol:  cfg.Kafka.SecurityProtocol,
		SASLMechanism:     cfg.Kafka.SASLMechanism,
		SASLUsername:      cfg.Kafka.SASLUsername,
		SASLPassword:      cfg.Kafka.SASLPassword,
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
	producer, err := kafka.NewProducer(kafkaConfig, logger)
	if err != nil {
		logger.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer producer.Close()

	// Create DLQ service
	dlqService := dlq.NewService(producer, dbClient, cfg.DLQ.Topic, logger)

	// Setup gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.DLQ.Port))
	if err != nil {
		logger.Fatalf("Failed to listen on port %d: %v", cfg.DLQ.Port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDLQServiceServer(grpcServer, dlqService)

	// Enable reflection for debugging
	reflection.Register(grpcServer)

	// Start server in a goroutine
	go func() {
		logger.Infof("DLQ service starting on port %d", cfg.DLQ.Port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down DLQ service...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("DLQ service stopped gracefully")
	case <-ctx.Done():
		logger.Warn("DLQ service shutdown timed out")
		grpcServer.Stop()
	}
}
