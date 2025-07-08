package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete configuration for all services
type Config struct {
	Environment string `yaml:"environment" envconfig:"ENVIRONMENT"`
	LogLevel    string `yaml:"log_level" envconfig:"LOG_LEVEL"`

	// Service-specific configurations
	APIGateway      APIGatewayConfig      `yaml:"api_gateway"`
	WebhookRegistry WebhookRegistryConfig `yaml:"webhook_registry"`
	EventIngestion  EventIngestionConfig  `yaml:"event_ingestion"`
	Dispatcher      DispatcherConfig      `yaml:"dispatcher"`
	RetryManager    RetryManagerConfig    `yaml:"retry_manager"`
	Observability   ObservabilityConfig   `yaml:"observability"`
	DLQ             DLQConfig             `yaml:"dlq"`

	// Infrastructure configurations
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Security SecurityConfig `yaml:"security"`
}

// APIGatewayConfig holds API Gateway configuration
type APIGatewayConfig struct {
	Port         int           `yaml:"port" envconfig:"API_GATEWAY_PORT"`
	Host         string        `yaml:"host" envconfig:"API_GATEWAY_HOST"`
	ReadTimeout  time.Duration `yaml:"read_timeout" envconfig:"API_GATEWAY_READ_TIMEOUT"`
	WriteTimeout time.Duration `yaml:"write_timeout" envconfig:"API_GATEWAY_WRITE_TIMEOUT"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" envconfig:"API_GATEWAY_IDLE_TIMEOUT"`

	// gRPC client configurations
	WebhookRegistryAddr string        `yaml:"webhook_registry_addr" envconfig:"WEBHOOK_REGISTRY_ADDR"`
	ObservabilityAddr   string        `yaml:"observability_addr" envconfig:"OBSERVABILITY_ADDR"`
	GRPCTimeout         time.Duration `yaml:"grpc_timeout" envconfig:"GRPC_TIMEOUT"`

	// CORS configuration
	CORSEnabled        bool     `yaml:"cors_enabled" envconfig:"CORS_ENABLED"`
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins" envconfig:"CORS_ALLOWED_ORIGINS"`
	CORSAllowedMethods []string `yaml:"cors_allowed_methods" envconfig:"CORS_ALLOWED_METHODS"`
	CORSAllowedHeaders []string `yaml:"cors_allowed_headers" envconfig:"CORS_ALLOWED_HEADERS"`
}

// WebhookRegistryConfig holds Webhook Registry configuration
type WebhookRegistryConfig struct {
	Port int    `yaml:"port" envconfig:"WEBHOOK_REGISTRY_PORT"`
	Host string `yaml:"host" envconfig:"WEBHOOK_REGISTRY_HOST"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Cache configuration
	CacheEnabled bool          `yaml:"cache_enabled" envconfig:"CACHE_ENABLED"`
	CacheTTL     time.Duration `yaml:"cache_ttl" envconfig:"CACHE_TTL"`
}

// EventIngestionConfig holds Event Ingestion configuration
type EventIngestionConfig struct {
	Port int    `yaml:"port" envconfig:"EVENT_INGESTION_PORT"`
	Host string `yaml:"host" envconfig:"EVENT_INGESTION_HOST"`

	// Kafka producer configuration
	Kafka KafkaProducerConfig `yaml:"kafka"`

	// Rate limiting
	RateLimitEnabled bool `yaml:"rate_limit_enabled" envconfig:"RATE_LIMIT_ENABLED"`
	RateLimitRPS     int  `yaml:"rate_limit_rps" envconfig:"RATE_LIMIT_RPS"`
}

// DispatcherConfig holds Webhook Dispatcher configuration
type DispatcherConfig struct {
	// Kafka consumer configuration
	Kafka KafkaConsumerConfig `yaml:"kafka"`

	// HTTP client configuration
	HTTPTimeout         time.Duration `yaml:"http_timeout" envconfig:"HTTP_TIMEOUT"`
	HTTPMaxRetries      int           `yaml:"http_max_retries" envconfig:"HTTP_MAX_RETRIES"`
	HTTPRetryDelay      time.Duration `yaml:"http_retry_delay" envconfig:"HTTP_RETRY_DELAY"`
	HTTPMaxConnsPerHost int           `yaml:"http_max_conns_per_host" envconfig:"HTTP_MAX_CONNS_PER_HOST"`
	HTTPMaxIdleConns    int           `yaml:"http_max_idle_conns" envconfig:"HTTP_MAX_IDLE_CONNS"`

	// gRPC client configurations
	WebhookRegistryAddr string        `yaml:"webhook_registry_addr" envconfig:"WEBHOOK_REGISTRY_ADDR"`
	RetryManagerAddr    string        `yaml:"retry_manager_addr" envconfig:"RETRY_MANAGER_ADDR"`
	ObservabilityAddr   string        `yaml:"observability_addr" envconfig:"OBSERVABILITY_ADDR"`
	GRPCTimeout         time.Duration `yaml:"grpc_timeout" envconfig:"GRPC_TIMEOUT"`

	// Worker configuration
	WorkerCount     int           `yaml:"worker_count" envconfig:"WORKER_COUNT"`
	BatchSize       int           `yaml:"batch_size" envconfig:"BATCH_SIZE"`
	ProcessInterval time.Duration `yaml:"process_interval" envconfig:"PROCESS_INTERVAL"`
}

// RetryManagerConfig holds Retry Manager configuration
type RetryManagerConfig struct {
	Port int    `yaml:"port" envconfig:"RETRY_MANAGER_PORT"`
	Host string `yaml:"host" envconfig:"RETRY_MANAGER_HOST"`

	// Redis configuration
	Redis RedisConfig `yaml:"redis"`

	// Retry configuration
	MaxRetries        int           `yaml:"max_retries" envconfig:"MAX_RETRIES"`
	InitialDelay      time.Duration `yaml:"initial_delay" envconfig:"INITIAL_DELAY"`
	MaxDelay          time.Duration `yaml:"max_delay" envconfig:"MAX_DELAY"`
	BackoffMultiplier float64       `yaml:"backoff_multiplier" envconfig:"BACKOFF_MULTIPLIER"`
	JitterEnabled     bool          `yaml:"jitter_enabled" envconfig:"JITTER_ENABLED"`

	// Processing configuration
	ProcessInterval time.Duration `yaml:"process_interval" envconfig:"PROCESS_INTERVAL"`
	BatchSize       int           `yaml:"batch_size" envconfig:"BATCH_SIZE"`
	WorkerCount     int           `yaml:"worker_count" envconfig:"WORKER_COUNT"`
}

// ObservabilityConfig holds Observability service configuration
type ObservabilityConfig struct {
	Port int    `yaml:"port" envconfig:"OBSERVABILITY_PORT"`
	Host string `yaml:"host" envconfig:"OBSERVABILITY_HOST"`

	// Metrics configuration
	Metrics MetricsConfig `yaml:"metrics"`

	// Logging configuration
	LogRetentionDays int    `yaml:"log_retention_days" envconfig:"LOG_RETENTION_DAYS"`
	LogLevel         string `yaml:"log_level" envconfig:"LOG_LEVEL"`
}

// DLQConfig holds Dead Letter Queue service configuration
type DLQConfig struct {
	Port  int    `yaml:"port" envconfig:"DLQ_PORT"`
	Host  string `yaml:"host" envconfig:"DLQ_HOST"`
	Topic string `yaml:"topic" envconfig:"DLQ_TOPIC"`
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string        `yaml:"host" envconfig:"DB_HOST"`
	Port     int           `yaml:"port" envconfig:"DB_PORT"`
	Database string        `yaml:"database" envconfig:"DB_NAME"`
	Username string        `yaml:"username" envconfig:"DB_USERNAME"`
	Password string        `yaml:"password" envconfig:"DB_PASSWORD"`
	SSLMode  string        `yaml:"ssl_mode" envconfig:"DB_SSL_MODE"`
	MaxConns int32         `yaml:"max_conns" envconfig:"DB_MAX_CONNS"`
	MinConns int32         `yaml:"min_conns" envconfig:"DB_MIN_CONNS"`
	MaxLife  time.Duration `yaml:"max_life" envconfig:"DB_MAX_LIFE"`
	MaxIdle  time.Duration `yaml:"max_idle" envconfig:"DB_MAX_IDLE"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Address         string        `yaml:"address" envconfig:"REDIS_ADDRESS"`
	Password        string        `yaml:"password" envconfig:"REDIS_PASSWORD"`
	DB              int           `yaml:"db" envconfig:"REDIS_DB"`
	MaxRetries      int           `yaml:"max_retries" envconfig:"REDIS_MAX_RETRIES"`
	DialTimeout     time.Duration `yaml:"dial_timeout" envconfig:"REDIS_DIAL_TIMEOUT"`
	ReadTimeout     time.Duration `yaml:"read_timeout" envconfig:"REDIS_READ_TIMEOUT"`
	WriteTimeout    time.Duration `yaml:"write_timeout" envconfig:"REDIS_WRITE_TIMEOUT"`
	PoolSize        int           `yaml:"pool_size" envconfig:"REDIS_POOL_SIZE"`
	MinIdleConns    int           `yaml:"min_idle_conns" envconfig:"REDIS_MIN_IDLE_CONNS"`
	MaxIdleConns    int           `yaml:"max_idle_conns" envconfig:"REDIS_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" envconfig:"REDIS_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" envconfig:"REDIS_CONN_MAX_IDLE_TIME"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers          []string `yaml:"brokers" envconfig:"KAFKA_BROKERS"`
	SecurityProtocol string   `yaml:"security_protocol" envconfig:"KAFKA_SECURITY_PROTOCOL"`
	SASLMechanism    string   `yaml:"sasl_mechanism" envconfig:"KAFKA_SASL_MECHANISM"`
	SASLUsername     string   `yaml:"sasl_username" envconfig:"KAFKA_SASL_USERNAME"`
	SASLPassword     string   `yaml:"sasl_password" envconfig:"KAFKA_SASL_PASSWORD"`
}

// KafkaProducerConfig holds Kafka producer configuration
type KafkaProducerConfig struct {
	KafkaConfig       `yaml:",inline"`
	ClientID          string `yaml:"client_id" envconfig:"KAFKA_PRODUCER_CLIENT_ID"`
	CompressionType   string `yaml:"compression_type" envconfig:"KAFKA_COMPRESSION_TYPE"`
	BatchSize         int    `yaml:"batch_size" envconfig:"KAFKA_BATCH_SIZE"`
	LingerMs          int    `yaml:"linger_ms" envconfig:"KAFKA_LINGER_MS"`
	RetryBackoffMs    int    `yaml:"retry_backoff_ms" envconfig:"KAFKA_RETRY_BACKOFF_MS"`
	MaxRetries        int    `yaml:"max_retries" envconfig:"KAFKA_MAX_RETRIES"`
	RequestTimeoutMs  int    `yaml:"request_timeout_ms" envconfig:"KAFKA_REQUEST_TIMEOUT_MS"`
	MessageTimeoutMs  int    `yaml:"message_timeout_ms" envconfig:"KAFKA_MESSAGE_TIMEOUT_MS"`
	Acks              string `yaml:"acks" envconfig:"KAFKA_ACKS"`
	EnableIdempotence bool   `yaml:"enable_idempotence" envconfig:"KAFKA_ENABLE_IDEMPOTENCE"`

	// Topics
	EventsTopic string `yaml:"events_topic" envconfig:"KAFKA_EVENTS_TOPIC"`
	DLQTopic    string `yaml:"dlq_topic" envconfig:"KAFKA_DLQ_TOPIC"`
}

// KafkaConsumerConfig holds Kafka consumer configuration
type KafkaConsumerConfig struct {
	KafkaConfig         `yaml:",inline"`
	GroupID             string `yaml:"group_id" envconfig:"KAFKA_CONSUMER_GROUP_ID"`
	ClientID            string `yaml:"client_id" envconfig:"KAFKA_CONSUMER_CLIENT_ID"`
	AutoOffsetReset     string `yaml:"auto_offset_reset" envconfig:"KAFKA_AUTO_OFFSET_RESET"`
	EnableAutoCommit    bool   `yaml:"enable_auto_commit" envconfig:"KAFKA_ENABLE_AUTO_COMMIT"`
	SessionTimeoutMs    int    `yaml:"session_timeout_ms" envconfig:"KAFKA_SESSION_TIMEOUT_MS"`
	HeartbeatIntervalMs int    `yaml:"heartbeat_interval_ms" envconfig:"KAFKA_HEARTBEAT_INTERVAL_MS"`
	FetchMinBytes       int    `yaml:"fetch_min_bytes" envconfig:"KAFKA_FETCH_MIN_BYTES"`

	// Topics
	EventsTopic string `yaml:"events_topic" envconfig:"KAFKA_EVENTS_TOPIC"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" envconfig:"METRICS_ENABLED"`
	ListenAddr string `yaml:"listen_addr" envconfig:"METRICS_LISTEN_ADDR"`
	Path       string `yaml:"path" envconfig:"METRICS_PATH"`
	Namespace  string `yaml:"namespace" envconfig:"METRICS_NAMESPACE"`
	Subsystem  string `yaml:"subsystem" envconfig:"METRICS_SUBSYSTEM"`
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	// OAuth2 configuration
	OAuth2Enabled      bool   `yaml:"oauth2_enabled" envconfig:"OAUTH2_ENABLED"`
	OAuth2Issuer       string `yaml:"oauth2_issuer" envconfig:"OAUTH2_ISSUER"`
	OAuth2ClientID     string `yaml:"oauth2_client_id" envconfig:"OAUTH2_CLIENT_ID"`
	OAuth2ClientSecret string `yaml:"oauth2_client_secret" envconfig:"OAUTH2_CLIENT_SECRET"`

	// JWT configuration
	JWTSecret     string        `yaml:"jwt_secret" envconfig:"JWT_SECRET"`
	JWTExpiration time.Duration `yaml:"jwt_expiration" envconfig:"JWT_EXPIRATION"`
	JWTIssuer     string        `yaml:"jwt_issuer" envconfig:"JWT_ISSUER"`
	JWTAudience   string        `yaml:"jwt_audience" envconfig:"JWT_AUDIENCE"`

	// HMAC configuration
	HMACSecret string `yaml:"hmac_secret" envconfig:"HMAC_SECRET"`

	// TLS configuration
	TLSEnabled  bool   `yaml:"tls_enabled" envconfig:"TLS_ENABLED"`
	TLSCertFile string `yaml:"tls_cert_file" envconfig:"TLS_CERT_FILE"`
	TLSKeyFile  string `yaml:"tls_key_file" envconfig:"TLS_KEY_FILE"`

	// Rate limiting
	RateLimitEnabled bool `yaml:"rate_limit_enabled" envconfig:"RATE_LIMIT_ENABLED"`
	RateLimitRPS     int  `yaml:"rate_limit_rps" envconfig:"RATE_LIMIT_RPS"`
	RateLimitBurst   int  `yaml:"rate_limit_burst" envconfig:"RATE_LIMIT_BURST"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Load from file if provided
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Set defaults
	setDefaults(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) error {
	// Use reflection or manual mapping to load environment variables
	// For simplicity, we'll do manual mapping of key configurations

	if val := os.Getenv("ENVIRONMENT"); val != "" {
		config.Environment = val
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}

	// Database configuration
	if val := os.Getenv("DB_HOST"); val != "" {
		config.Database.Host = val
	}
	if val := os.Getenv("DB_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.Database.Port = port
		}
	}
	if val := os.Getenv("DB_NAME"); val != "" {
		config.Database.Database = val
	}
	if val := os.Getenv("DB_USERNAME"); val != "" {
		config.Database.Username = val
	}
	if val := os.Getenv("DB_PASSWORD"); val != "" {
		config.Database.Password = val
	}

	// Redis configuration
	if val := os.Getenv("REDIS_ADDRESS"); val != "" {
		config.Redis.Address = val
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		config.Redis.Password = val
	}

	// Kafka configuration
	if val := os.Getenv("KAFKA_BROKERS"); val != "" {
		config.Kafka.Brokers = strings.Split(val, ",")
	}

	// Security configuration
	if val := os.Getenv("JWT_SECRET"); val != "" {
		config.Security.JWTSecret = val
	}
	if val := os.Getenv("HMAC_SECRET"); val != "" {
		config.Security.HMACSecret = val
	}
	if val := os.Getenv("OAUTH2_CLIENT_SECRET"); val != "" {
		config.Security.OAuth2ClientSecret = val
	}

	return nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	// API Gateway defaults
	if config.APIGateway.Port == 0 {
		config.APIGateway.Port = 8080
	}
	if config.APIGateway.Host == "" {
		config.APIGateway.Host = "0.0.0.0"
	}
	if config.APIGateway.ReadTimeout == 0 {
		config.APIGateway.ReadTimeout = 30 * time.Second
	}
	if config.APIGateway.WriteTimeout == 0 {
		config.APIGateway.WriteTimeout = 30 * time.Second
	}
	if config.APIGateway.IdleTimeout == 0 {
		config.APIGateway.IdleTimeout = 120 * time.Second
	}

	// Database defaults
	if config.Database.Host == "" {
		config.Database.Host = "localhost"
	}
	if config.Database.Port == 0 {
		config.Database.Port = 5432
	}
	if config.Database.Database == "" {
		config.Database.Database = "webhook_db"
	}
	if config.Database.Username == "" {
		config.Database.Username = "postgres"
	}
	if config.Database.Password == "" {
		config.Database.Password = "postgres123"
	}
	if config.Database.SSLMode == "" {
		config.Database.SSLMode = "disable"
	}
	if config.Database.MaxConns == 0 {
		config.Database.MaxConns = 30
	}
	if config.Database.MinConns == 0 {
		config.Database.MinConns = 5
	}
	if config.Database.MaxLife == 0 {
		config.Database.MaxLife = time.Hour
	}
	if config.Database.MaxIdle == 0 {
		config.Database.MaxIdle = 30 * time.Minute
	}

	// Redis defaults
	if config.Redis.Address == "" {
		config.Redis.Address = "localhost:6379"
	}
	if config.Redis.Password == "" {
		config.Redis.Password = "redis123"
	}
	if config.Redis.PoolSize == 0 {
		config.Redis.PoolSize = 10
	}

	// Kafka defaults
	if len(config.Kafka.Brokers) == 0 {
		config.Kafka.Brokers = []string{"localhost:9092"}
	}

	// Metrics defaults
	if config.Metrics.ListenAddr == "" {
		config.Metrics.ListenAddr = ":9090"
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}
	if config.Metrics.Namespace == "" {
		config.Metrics.Namespace = "webhook"
	}
	if config.Metrics.Subsystem == "" {
		config.Metrics.Subsystem = "service"
	}

	// Enable metrics by default
	config.Metrics.Enabled = true

}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if config.Database.Username == "" {
		return fmt.Errorf("database username is required")
	}
	if config.Database.Password == "" {
		return fmt.Errorf("database password is required")
	}
	if config.Redis.Address == "" {
		return fmt.Errorf("redis address is required")
	}
	if len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required")
	}

	return nil
}

// DefaultConfig returns a default configuration
// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	return LoadConfig("config.yaml")
}

func DefaultConfig() *Config {
	config := &Config{}
	setDefaults(config)
	return config
}
