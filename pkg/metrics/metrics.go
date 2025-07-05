package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Config holds metrics configuration
type Config struct {
	Enabled    bool
	ListenAddr string
	Path       string
	Namespace  string
	Subsystem  string
}

// Metrics holds all webhook service metrics
type Metrics struct {
	// Webhook delivery metrics
	DeliveryAttempts *prometheus.CounterVec
	DeliverySuccess  *prometheus.CounterVec
	DeliveryFailures *prometheus.CounterVec
	DeliveryLatency  *prometheus.HistogramVec
	DeliveryRetries  *prometheus.CounterVec

	// Webhook registry metrics
	WebhookRegistrations *prometheus.CounterVec
	WebhookDeletions     *prometheus.CounterVec
	ActiveWebhooks       *prometheus.GaugeVec

	// Kafka metrics
	KafkaMessagesProduced *prometheus.CounterVec
	KafkaMessagesConsumed *prometheus.CounterVec
	KafkaLag              *prometheus.GaugeVec

	// Database metrics
	DatabaseConnections   *prometheus.GaugeVec
	DatabaseQueries       *prometheus.CounterVec
	DatabaseQueryDuration *prometheus.HistogramVec

	// Redis metrics
	RedisConnections *prometheus.GaugeVec
	RedisOperations  *prometheus.CounterVec
	RedisLatency     *prometheus.HistogramVec

	// System metrics
	HTTPRequests        *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	GRPCRequests        *prometheus.CounterVec
	GRPCRequestDuration *prometheus.HistogramVec

	registry *prometheus.Registry
	server   *http.Server
	log      *logrus.Logger
}

// NewMetrics creates a new metrics instance
func NewMetrics(cfg Config, logger *logrus.Logger) *Metrics {
	registry := prometheus.NewRegistry()

	m := &Metrics{
		registry: registry,
		log:      logger,
	}

	// Initialize all metrics
	m.initDeliveryMetrics(cfg)
	m.initRegistryMetrics(cfg)
	m.initKafkaMetrics(cfg)
	m.initDatabaseMetrics(cfg)
	m.initRedisMetrics(cfg)
	m.initSystemMetrics(cfg)

	// Register all metrics
	m.registerMetrics()

	if cfg.Enabled {
		m.startServer(cfg)
	}

	return m
}

func (m *Metrics) initDeliveryMetrics(cfg Config) {
	m.DeliveryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "delivery_attempts_total",
			Help:      "Total number of webhook delivery attempts",
		},
		[]string{"webhook_id", "customer_id", "event_type"},
	)

	m.DeliverySuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "delivery_success_total",
			Help:      "Total number of successful webhook deliveries",
		},
		[]string{"webhook_id", "customer_id", "event_type", "status_code"},
	)

	m.DeliveryFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "delivery_failures_total",
			Help:      "Total number of failed webhook deliveries",
		},
		[]string{"webhook_id", "customer_id", "event_type", "error_type"},
	)

	m.DeliveryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "delivery_duration_seconds",
			Help:      "Time spent delivering webhooks",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"webhook_id", "customer_id", "event_type"},
	)

	m.DeliveryRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "delivery_retries_total",
			Help:      "Total number of webhook delivery retries",
		},
		[]string{"webhook_id", "customer_id", "event_type", "attempt"},
	)
}

func (m *Metrics) initRegistryMetrics(cfg Config) {
	m.WebhookRegistrations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "webhook_registrations_total",
			Help:      "Total number of webhook registrations",
		},
		[]string{"customer_id"},
	)

	m.WebhookDeletions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "webhook_deletions_total",
			Help:      "Total number of webhook deletions",
		},
		[]string{"customer_id"},
	)

	m.ActiveWebhooks = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "active_webhooks",
			Help:      "Number of active webhooks",
		},
		[]string{"customer_id"},
	)
}

func (m *Metrics) initKafkaMetrics(cfg Config) {
	m.KafkaMessagesProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "kafka_messages_produced_total",
			Help:      "Total number of Kafka messages produced",
		},
		[]string{"topic", "partition"},
	)

	m.KafkaMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "kafka_messages_consumed_total",
			Help:      "Total number of Kafka messages consumed",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	m.KafkaLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "kafka_consumer_lag",
			Help:      "Current Kafka consumer lag",
		},
		[]string{"topic", "partition", "consumer_group"},
	)
}

func (m *Metrics) initDatabaseMetrics(cfg Config) {
	m.DatabaseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "database_connections",
			Help:      "Number of database connections",
		},
		[]string{"database", "state"},
	)

	m.DatabaseQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "database_queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"database", "operation", "table"},
	)

	m.DatabaseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "database_query_duration_seconds",
			Help:      "Time spent executing database queries",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"database", "operation", "table"},
	)
}

func (m *Metrics) initRedisMetrics(cfg Config) {
	m.RedisConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "redis_connections",
			Help:      "Number of Redis connections",
		},
		[]string{"instance", "state"},
	)

	m.RedisOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "redis_operations_total",
			Help:      "Total number of Redis operations",
		},
		[]string{"instance", "operation"},
	)

	m.RedisLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "redis_operation_duration_seconds",
			Help:      "Time spent executing Redis operations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"instance", "operation"},
	)
}

func (m *Metrics) initSystemMetrics(cfg Config) {
	m.HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	m.HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "Time spent processing HTTP requests",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	m.GRPCRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests",
		},
		[]string{"service", "method", "status_code"},
	)

	m.GRPCRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "Time spent processing gRPC requests",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)
}

func (m *Metrics) registerMetrics() {
	// Delivery metrics
	m.registry.MustRegister(m.DeliveryAttempts)
	m.registry.MustRegister(m.DeliverySuccess)
	m.registry.MustRegister(m.DeliveryFailures)
	m.registry.MustRegister(m.DeliveryLatency)
	m.registry.MustRegister(m.DeliveryRetries)

	// Registry metrics
	m.registry.MustRegister(m.WebhookRegistrations)
	m.registry.MustRegister(m.WebhookDeletions)
	m.registry.MustRegister(m.ActiveWebhooks)

	// Kafka metrics
	m.registry.MustRegister(m.KafkaMessagesProduced)
	m.registry.MustRegister(m.KafkaMessagesConsumed)
	m.registry.MustRegister(m.KafkaLag)

	// Database metrics
	m.registry.MustRegister(m.DatabaseConnections)
	m.registry.MustRegister(m.DatabaseQueries)
	m.registry.MustRegister(m.DatabaseQueryDuration)

	// Redis metrics
	m.registry.MustRegister(m.RedisConnections)
	m.registry.MustRegister(m.RedisOperations)
	m.registry.MustRegister(m.RedisLatency)

	// System metrics
	m.registry.MustRegister(m.HTTPRequests)
	m.registry.MustRegister(m.HTTPRequestDuration)
	m.registry.MustRegister(m.GRPCRequests)
	m.registry.MustRegister(m.GRPCRequestDuration)
}

func (m *Metrics) startServer(cfg Config) {
	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	go func() {
		m.log.WithField("addr", cfg.ListenAddr).Info("Starting metrics server")
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.log.WithError(err).Error("Metrics server failed")
		}
	}()
}

// Close stops the metrics server
func (m *Metrics) Close() error {
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.server.Shutdown(ctx); err != nil {
			m.log.WithError(err).Error("Failed to shutdown metrics server")
			return err
		}
		m.log.Info("Metrics server stopped")
	}
	return nil
}

// Registry returns the Prometheus registry
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// DefaultConfig returns a default metrics configuration
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		ListenAddr: ":8080",
		Path:       "/metrics",
		Namespace:  "webhook",
		Subsystem:  "service",
	}
}

// Timer is a helper for timing operations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// ObserveDuration observes the duration since the timer was created
func (t *Timer) ObserveDuration(observer prometheus.Observer) {
	observer.Observe(time.Since(t.start).Seconds())
}

// Duration returns the duration since the timer was created
func (t *Timer) Duration() time.Duration {
	return time.Since(t.start)
}
