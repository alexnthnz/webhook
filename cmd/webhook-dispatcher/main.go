package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/security"
	"github.com/alexnthnz/webhook/pkg/circuitbreaker"
	kafkapkg "github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/metrics"
	pb "github.com/alexnthnz/webhook/proto/generated"
)

type WebhookDispatcherService struct {
	config                *config.Config
	logger                *logrus.Logger
	kafka                 *kafkapkg.Consumer
	metrics               *metrics.Metrics
	webhookRegistryClient pb.WebhookRegistryClient
	retryManagerClient    pb.RetryManagerClient
	observabilityClient   pb.ObservabilityServiceClient
	httpClient            *http.Client
	hmacSigner            *security.HMACSigner
	circuitBreakerManager *circuitbreaker.Manager
	wg                    sync.WaitGroup
}

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
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

	log.Info("Starting Webhook Dispatcher Service")

	// Initialize metrics
	metricsConfig := metrics.Config{
		Namespace: cfg.Metrics.Namespace,
		Subsystem: cfg.Metrics.Subsystem,
	}
	m := metrics.NewMetrics(metricsConfig, log)

	// Initialize Kafka consumer
	kafkaConfig := kafkapkg.ConsumerConfig{
		Brokers:             cfg.Kafka.Brokers[0], // Use first broker
		GroupID:             cfg.Dispatcher.Kafka.GroupID,
		ClientID:            cfg.Dispatcher.Kafka.ClientID,
		AutoOffsetReset:     cfg.Dispatcher.Kafka.AutoOffsetReset,
		EnableAutoCommit:    cfg.Dispatcher.Kafka.EnableAutoCommit,
		SessionTimeoutMs:    cfg.Dispatcher.Kafka.SessionTimeoutMs,
		HeartbeatIntervalMs: cfg.Dispatcher.Kafka.HeartbeatIntervalMs,
		MaxPollRecords:      cfg.Dispatcher.Kafka.MaxPollRecords,
		FetchMinBytes:       cfg.Dispatcher.Kafka.FetchMinBytes,
		FetchMaxWaitMs:      cfg.Dispatcher.Kafka.FetchMaxWaitMs,
	}

	consumer, err := kafkapkg.NewConsumer(kafkaConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka consumer")
	}
	defer consumer.Close()

	// Subscribe to events topic
	if err := consumer.Subscribe([]string{cfg.Dispatcher.Kafka.EventsTopic}); err != nil {
		log.WithError(err).Fatal("Failed to subscribe to Kafka topic")
	}

	// Initialize gRPC clients
	webhookRegistryConn, err := grpc.NewClient(
		cfg.Dispatcher.WebhookRegistryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to webhook registry")
	}
	defer webhookRegistryConn.Close()

	retryManagerConn, err := grpc.NewClient(
		cfg.Dispatcher.RetryManagerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to retry manager")
	}
	defer retryManagerConn.Close()

	observabilityConn, err := grpc.NewClient(
		cfg.Dispatcher.ObservabilityAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to observability service")
	}
	defer observabilityConn.Close()

	// Initialize HTTP client
	httpClient := &http.Client{
		Timeout: cfg.Dispatcher.HTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     cfg.Dispatcher.HTTPMaxConnsPerHost,
			MaxIdleConns:        cfg.Dispatcher.HTTPMaxIdleConns,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// Initialize HMAC signer
	hmacSigner := security.NewHMACSignerFromSecret(cfg.Security.HMACSecret)

	// Initialize circuit breaker manager
	circuitBreakerManager := circuitbreaker.NewManager(log)

	// Initialize service
	service := &WebhookDispatcherService{
		config:                cfg,
		logger:                log,
		kafka:                 consumer,
		metrics:               m,
		webhookRegistryClient: pb.NewWebhookRegistryClient(webhookRegistryConn),
		retryManagerClient:    pb.NewRetryManagerClient(retryManagerConn),
		observabilityClient:   pb.NewObservabilityServiceClient(observabilityConn),
		httpClient:            httpClient,
		hmacSigner:            hmacSigner,
		circuitBreakerManager: circuitBreakerManager,
	}

	// Start workers
	for i := 0; i < cfg.Dispatcher.WorkerCount; i++ {
		service.wg.Add(1)
		go service.worker(i)
	}

	log.WithField("workers", cfg.Dispatcher.WorkerCount).Info("Started webhook dispatcher workers")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down Webhook Dispatcher Service")

	// Wait for workers to finish
	service.wg.Wait()

	log.Info("Webhook Dispatcher Service shutdown complete")
}

func (s *WebhookDispatcherService) worker(workerID int) {
	defer s.wg.Done()

	logger := s.logger.WithField("worker_id", workerID)
	logger.Info("Starting webhook dispatcher worker")

	for {
		// Poll for messages
		msg, err := s.kafka.Poll(time.Second)
		if err != nil {
			logger.WithError(err).Error("Failed to poll Kafka message")
			continue
		}

		if msg == nil {
			continue
		}

		// Process message
		if err := s.processMessage(msg); err != nil {
			logger.WithError(err).Error("Failed to process message")
			s.metrics.KafkaMessagesConsumed.WithLabelValues("error", "0", s.config.Dispatcher.Kafka.GroupID).Inc()
		} else {
			s.metrics.KafkaMessagesConsumed.WithLabelValues("success", "0", s.config.Dispatcher.Kafka.GroupID).Inc()
		}

		// Commit message
		if err := s.kafka.CommitMessage(msg); err != nil {
			logger.WithError(err).Error("Failed to commit message")
		}
	}
}

func (s *WebhookDispatcherService) processMessage(msg *kafka.Message) error {
	// Parse event
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": event.Type,
		"source":     event.Source,
	}).Info("Processing event")

	// Get webhooks for this event type
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Dispatcher.GRPCTimeout)
	defer cancel()

	webhooksResp, err := s.webhookRegistryClient.GetWebhooksForEvent(ctx, &pb.GetWebhooksForEventRequest{
		EventType: event.Type,
	})
	if err != nil {
		return fmt.Errorf("failed to get webhooks: %w", err)
	}

	// Dispatch to each webhook
	for _, webhook := range webhooksResp.Webhooks {
		if err := s.dispatchWebhook(event, webhook); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"webhook_id": webhook.WebhookId,
				"event_id":   event.ID,
			}).Error("Failed to dispatch webhook")

			// Schedule retry
			if err := s.scheduleRetry(event, webhook, err); err != nil {
				s.logger.WithError(err).Error("Failed to schedule retry")
			}
		}
	}

	return nil
}

func (s *WebhookDispatcherService) dispatchWebhook(event Event, webhook interface{}) error {
	// Type assertion for webhook (to work around protobuf import issues)
	webhookMap, ok := webhook.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid webhook type")
	}

	webhookID := webhookMap["webhook_id"].(string)
	customerID := webhookMap["customer_id"].(string)
	webhookURL := webhookMap["url"].(string)
	headers := webhookMap["headers"].(map[string]string)

	// Get or create circuit breaker for this webhook
	cbName := fmt.Sprintf("webhook-%s", webhookID)
	cb := s.circuitBreakerManager.GetOrCreate(cbName, circuitbreaker.WebhookConfig(cbName))

	// Prepare payload
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Execute webhook call through circuit breaker
	start := time.Now()
	err = cb.Execute(func() error {
		return s.executeWebhookCall(webhookURL, webhookID, customerID, headers, payload, event)
	})
	duration := time.Since(start)

	// Record metrics
	s.metrics.DeliveryLatency.WithLabelValues(webhookID, customerID, event.Type).Observe(duration.Seconds())
	s.metrics.DeliveryAttempts.WithLabelValues(webhookID, customerID, event.Type).Inc()

	// Handle circuit breaker errors
	if err == circuitbreaker.ErrOpenState {
		s.metrics.DeliveryFailures.WithLabelValues(webhookID, customerID, event.Type, "circuit_breaker_open").Inc()
		s.logger.WithFields(logrus.Fields{
			"webhook_id": webhookID,
			"event_id":   event.ID,
			"error":      "circuit breaker open",
		}).Warn("Webhook delivery blocked by circuit breaker")
		return fmt.Errorf("circuit breaker is open for webhook %s", webhookID)
	} else if err == circuitbreaker.ErrTooManyRequests {
		s.metrics.DeliveryFailures.WithLabelValues(webhookID, customerID, event.Type, "circuit_breaker_throttled").Inc()
		s.logger.WithFields(logrus.Fields{
			"webhook_id": webhookID,
			"event_id":   event.ID,
			"error":      "too many requests",
		}).Warn("Webhook delivery throttled by circuit breaker")
		return fmt.Errorf("too many requests for webhook %s", webhookID)
	}

	return err
}

func (s *WebhookDispatcherService) executeWebhookCall(webhookURL, webhookID, customerID string, headers map[string]string, payload []byte, event Event) error {
	// Create HTTP request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "webhook-dispatcher/1.0")
	req.Header.Set("X-Webhook-ID", webhookID)
	req.Header.Set("X-Event-ID", event.ID)
	req.Header.Set("X-Event-Type", event.Type)
	req.Header.Set("X-Timestamp", event.Timestamp.Format(time.RFC3339))

	// Add custom headers
	if headers != nil && len(headers) > 0 {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	// Sign payload
	signature := s.hmacSigner.Sign(payload)
	req.Header.Set("X-Webhook-Signature", signature)

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.metrics.DeliveryFailures.WithLabelValues(webhookID, customerID, event.Type, "network_error").Inc()
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.metrics.DeliverySuccess.WithLabelValues(webhookID, customerID, event.Type, fmt.Sprintf("%d", resp.StatusCode)).Inc()
		s.logger.WithFields(logrus.Fields{
			"webhook_id":  webhookID,
			"event_id":    event.ID,
			"status_code": resp.StatusCode,
		}).Info("Webhook delivered successfully")
		return nil
	}

	s.metrics.DeliveryFailures.WithLabelValues(webhookID, customerID, event.Type, "http_error").Inc()
	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
}

func (s *WebhookDispatcherService) scheduleRetry(event Event, webhook *pb.Webhook, deliveryErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Dispatcher.GRPCTimeout)
	defer cancel()

	_, err := s.retryManagerClient.ScheduleRetry(ctx, &pb.ScheduleRetryRequest{
		WebhookId:     webhook.WebhookId,
		EventId:       event.ID,
		CustomerId:    webhook.CustomerId,
		Payload:       mustMarshal(event),
		FailureReason: deliveryErr.Error(),
		AttemptNumber: 1,
	})

	return err
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
