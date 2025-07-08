package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/security"
	"github.com/alexnthnz/webhook/pkg/avro"
	"github.com/alexnthnz/webhook/pkg/kafka"
	"github.com/alexnthnz/webhook/pkg/metrics"
)

type EventIngestionService struct {
	config         *config.Config
	logger         *logrus.Logger
	kafka          *kafka.Producer
	metrics        *metrics.Metrics
	jwtManager     *security.JWTManager
	avroSerializer *avro.Serializer
	eventSchema    string
}

type EventData struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Data      EventData `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup logging
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.JSONFormatter{})

	// Initialize metrics
	metricsConfig := metrics.Config{
		Namespace: cfg.Metrics.Namespace,
		Subsystem: cfg.Metrics.Subsystem,
	}
	m := metrics.NewMetrics(metricsConfig, log)

	// Initialize Kafka producer
	kafkaConfig := kafka.ProducerConfig{
		Brokers:           cfg.Kafka.Brokers[0], // Use first broker
		ClientID:          cfg.EventIngestion.Kafka.ClientID,
		CompressionType:   cfg.EventIngestion.Kafka.CompressionType,
		BatchSize:         cfg.EventIngestion.Kafka.BatchSize,
		LingerMs:          cfg.EventIngestion.Kafka.LingerMs,
		RetryBackoffMs:    cfg.EventIngestion.Kafka.RetryBackoffMs,
		MaxRetries:        cfg.EventIngestion.Kafka.MaxRetries,
		RequestTimeoutMs:  cfg.EventIngestion.Kafka.RequestTimeoutMs,
		MessageTimeoutMs:  cfg.EventIngestion.Kafka.MessageTimeoutMs,
		Acks:              cfg.EventIngestion.Kafka.Acks,
		EnableIdempotence: cfg.EventIngestion.Kafka.EnableIdempotence,
	}

	producer, err := kafka.NewProducer(kafkaConfig, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Initialize JWT manager
	jwtManager := security.NewJWTManager(
		cfg.Security.JWTSecret,
		cfg.Security.JWTIssuer,
		cfg.Security.JWTAudience,
		cfg.Security.JWTExpiration,
	)

	// Initialize avro serializer with Schema Registry
	avroSerializer := avro.NewSerializer("http://schema-registry:8081")

	// Load Avro schema from file
	schemaBytes, err := os.ReadFile("schemas/event.avsc")
	if err != nil {
		log.WithError(err).Fatal("Failed to read Avro schema file")
	}

	// Load schema into serializer
	if err := avroSerializer.LoadSchema("webhook-events", "schemas/event.avsc"); err != nil {
		log.WithError(err).Fatal("Failed to load Avro schema")
	}

	// Initialize service
	service := &EventIngestionService{
		config:         cfg,
		logger:         log,
		kafka:          producer,
		metrics:        m,
		jwtManager:     jwtManager,
		avroSerializer: avroSerializer,
		eventSchema:    string(schemaBytes),
	}

	// Create HTTP server
	router := mux.NewRouter()

	// Add middleware
	router.Use(service.loggingMiddleware)
	router.Use(service.corsMiddleware)

	// Add routes
	router.HandleFunc("/events", service.handleEvent).Methods("POST")
	router.HandleFunc("/health", service.handleHealth).Methods("GET")
	router.HandleFunc("/metrics", service.handleMetrics).Methods("GET")

	// Protected routes
	protected := router.PathPrefix("/api/v1").Subrouter()
	protected.Use(service.authMiddleware)
	protected.HandleFunc("/events", service.handleEvent).Methods("POST")

	addr := fmt.Sprintf("%s:%d", cfg.EventIngestion.Host, cfg.EventIngestion.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.WithField("address", addr).Info("Starting HTTP server")

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down Event Ingestion Service")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("Failed to shutdown server gracefully")
	}

	log.Info("Event Ingestion Service shutdown complete")
}

func (s *EventIngestionService) handleEvent(w http.ResponseWriter, r *http.Request) {
	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		s.logger.WithError(err).Error("Failed to decode event")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		s.metrics.HTTPRequests.WithLabelValues("POST", "/events", "400").Inc()
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Validate event
	if event.Type == "" || event.Source == "" {
		s.logger.Error("Missing required event fields")
		http.Error(w, "Missing required fields: type, source", http.StatusBadRequest)
		s.metrics.HTTPRequests.WithLabelValues("POST", "/events", "400").Inc()
		return
	}

	// Convert event to map for Avro serialization
	eventMap := map[string]interface{}{
		"id":     event.ID,
		"type":   event.Type,
		"source": event.Source,
		"data": map[string]interface{}{
			"user_id": event.Data.UserID,
			"name":    event.Data.Name,
		},
		"timestamp": event.Timestamp.UnixMilli(),
		"version":   1,
	}

	// Serialize with Avro using loaded schema
	eventData, err := s.avroSerializer.Serialize("webhook-events", eventMap)
	if err != nil {
		s.logger.WithError(err).Error("Failed to serialize event with Avro")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		s.metrics.HTTPRequests.WithLabelValues("POST", "/events", "500").Inc()
		return
	}

	// Publish to Kafka
	if err := s.kafka.Produce(s.config.EventIngestion.Kafka.EventsTopic, []byte(event.ID), eventData, nil); err != nil {
		s.logger.WithError(err).Error("Failed to publish event to Kafka")
		http.Error(w, "Failed to process event", http.StatusInternalServerError)
		s.metrics.KafkaMessagesProduced.WithLabelValues("error", "0").Inc()
		return
	}

	s.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": event.Type,
		"source":     event.Source,
	}).Info("Event processed successfully")

	s.metrics.HTTPRequests.WithLabelValues("POST", "/events", "200").Inc()
	s.metrics.KafkaMessagesProduced.WithLabelValues("success", "0").Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "accepted",
		"event_id": event.ID,
	})
}

func (s *EventIngestionService) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "event-ingestion",
	})
}

func (s *EventIngestionService) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(s.metrics.Registry(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func (s *EventIngestionService) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		s.logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": duration,
		}).Info("HTTP request completed")
	})
}

func (s *EventIngestionService) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.APIGateway.CORSEnabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *EventIngestionService) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Security.OAuth2Enabled {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if _, err := s.jwtManager.ValidateToken(token); err != nil {
			s.logger.WithError(err).Error("Invalid token")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
