package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/dlq"
	kafkapkg "github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/postgres"
	pb "github.com/alexnthnz/webhook/proto/generated"
)

type DLQService struct {
	config     *config.Config
	logger     *logrus.Logger
	consumer   *kafkapkg.Consumer
	service    *dlq.Service
	grpcServer *grpc.Server
}

type DLQMessage struct {
	EventID       string                 `json:"event_id"`
	WebhookID     string                 `json:"webhook_id"`
	CustomerID    string                 `json:"customer_id"`
	EventType     string                 `json:"event_type"`
	Payload       map[string]interface{} `json:"payload"`
	FailureReason string                 `json:"failure_reason"`
	Attempts      int                    `json:"attempts"`
	Headers       map[string]string      `json:"headers"`
	CreatedAt     time.Time              `json:"created_at"`
}

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

	log.Info("Starting DLQ Service")

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

	// Initialize Kafka consumer
	kafkaConfig := kafkapkg.ConsumerConfig{
		Brokers:             cfg.Kafka.Brokers[0],
		GroupID:             "dlq-consumer",
		ClientID:            "dlq-service",
		AutoOffsetReset:     "earliest",
		EnableAutoCommit:    true,
		SessionTimeoutMs:    30000,
		HeartbeatIntervalMs: 3000,
		FetchMinBytes:       1,
	}

	consumer, err := kafkapkg.NewConsumer(kafkaConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka consumer")
	}
	defer consumer.Close()

	// Subscribe to DLQ topic
	if err := consumer.Subscribe([]string{cfg.DLQ.Topic}); err != nil {
		log.WithError(err).Fatal("Failed to subscribe to DLQ topic")
	}

	// Initialize Kafka producer for DLQ service
	kafkaProducerConfig := kafkapkg.ProducerConfig{
		Brokers:           cfg.Kafka.Brokers[0],
		ClientID:          "dlq-service-producer",
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

	producer, err := kafkapkg.NewProducer(kafkaProducerConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Initialize DLQ service
	dlqService := dlq.NewService(producer, db, cfg.DLQ.Topic, log)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(log)),
	)

	// Register services
	pb.RegisterDLQServiceServer(grpcServer, dlqService)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable reflection for development
	reflection.Register(grpcServer)

	// Initialize DLQ service
	service := &DLQService{
		config:     cfg,
		logger:     log,
		consumer:   consumer,
		service:    dlqService,
		grpcServer: grpcServer,
	}

	// Start gRPC server
	addr := fmt.Sprintf("%s:%d", cfg.DLQ.Host, cfg.DLQ.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithError(err).Fatal("Failed to create listener")
	}

	log.WithField("address", addr).Info("Starting gRPC server")

	// Start gRPC server in goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.WithError(err).Fatal("Failed to serve gRPC server")
		}
	}()

	// Start DLQ message processor
	go service.processDLQMessages()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down DLQ Service")

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

	log.Info("DLQ Service shutdown complete")
}

func (s *DLQService) processDLQMessages() {
	logger := s.logger.WithField("component", "dlq_processor")
	logger.Info("Starting DLQ message processor")

	for {
		// Poll for messages
		msg, err := s.consumer.Poll(time.Second)
		if err != nil {
			logger.WithError(err).Error("Failed to poll DLQ message")
			continue
		}

		if msg == nil {
			continue
		}

		// Process the message
		if err := s.processMessage(msg); err != nil {
			logger.WithError(err).Error("Failed to process DLQ message")
			// Don't commit the message if processing failed
			continue
		}

		// Commit the message
		if err := s.consumer.CommitMessage(msg); err != nil {
			logger.WithError(err).Error("Failed to commit DLQ message")
		}
	}
}

func (s *DLQService) processMessage(msg *kafka.Message) error {
	var dlqMsg DLQMessage
	if err := json.Unmarshal(msg.Value, &dlqMsg); err != nil {
		return fmt.Errorf("failed to unmarshal DLQ message: %w", err)
	}

	// Set timestamp if not provided
	if dlqMsg.CreatedAt.IsZero() {
		dlqMsg.CreatedAt = time.Now()
	}

	// Store in database using the existing DLQ service method
	ctx := context.Background()
	dlqMessage := dlq.DLQMessage{
		EventID:       dlqMsg.EventID,
		WebhookID:     dlqMsg.WebhookID,
		CustomerID:    dlqMsg.CustomerID,
		EventType:     dlqMsg.EventType,
		OriginalEvent: dlqMsg.Payload,
		FailureReason: dlqMsg.FailureReason,
		Attempts:      dlqMsg.Attempts,
		FirstFailedAt: dlqMsg.CreatedAt,
		LastFailedAt:  dlqMsg.CreatedAt,
		CreatedAt:     dlqMsg.CreatedAt,
		Headers:       dlqMsg.Headers,
	}

	err := s.service.StoreDLQRecord(ctx, dlqMessage)

	if err != nil {
		return fmt.Errorf("failed to store DLQ message: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"event_id":    dlqMsg.EventID,
		"webhook_id":  dlqMsg.WebhookID,
		"customer_id": dlqMsg.CustomerID,
		"event_type":  dlqMsg.EventType,
		"attempts":    dlqMsg.Attempts,
	}).Info("DLQ message processed successfully")

	return nil
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
