package kafka

import (
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sirupsen/logrus"
)

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers           string
	ClientID          string
	SecurityProtocol  string
	SASLMechanism     string
	SASLUsername      string
	SASLPassword      string
	CompressionType   string
	BatchSize         int
	LingerMs          int
	RetryBackoffMs    int
	MaxRetries        int
	RequestTimeoutMs  int
	MessageTimeoutMs  int
	Acks              string
	EnableIdempotence bool
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers             string
	GroupID             string
	ClientID            string
	SecurityProtocol    string
	SASLMechanism       string
	SASLUsername        string
	SASLPassword        string
	AutoOffsetReset     string
	EnableAutoCommit    bool
	SessionTimeoutMs    int
	HeartbeatIntervalMs int
	MaxPollRecords      int
	FetchMinBytes       int
	FetchMaxWaitMs      int
}

// Producer wraps kafka.Producer with additional functionality
type Producer struct {
	producer *kafka.Producer
	log      *logrus.Logger
}

// Consumer wraps kafka.Consumer with additional functionality
type Consumer struct {
	consumer *kafka.Consumer
	log      *logrus.Logger
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg ProducerConfig, logger *logrus.Logger) (*Producer, error) {
	config := kafka.ConfigMap{
		"bootstrap.servers":  cfg.Brokers,
		"client.id":          cfg.ClientID,
		"compression.type":   cfg.CompressionType,
		"batch.size":         cfg.BatchSize,
		"linger.ms":          cfg.LingerMs,
		"retry.backoff.ms":   cfg.RetryBackoffMs,
		"retries":            cfg.MaxRetries,
		"request.timeout.ms": cfg.RequestTimeoutMs,
		"message.timeout.ms": cfg.MessageTimeoutMs,
		"acks":               cfg.Acks,
		"enable.idempotence": cfg.EnableIdempotence,
	}

	if cfg.SecurityProtocol != "" {
		config["security.protocol"] = cfg.SecurityProtocol
		config["sasl.mechanism"] = cfg.SASLMechanism
		config["sasl.username"] = cfg.SASLUsername
		config["sasl.password"] = cfg.SASLPassword
	}

	producer, err := kafka.NewProducer(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	logger.Info("Successfully created Kafka producer")

	return &Producer{
		producer: producer,
		log:      logger,
	}, nil
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg ConsumerConfig, logger *logrus.Logger) (*Consumer, error) {
	config := kafka.ConfigMap{
		"bootstrap.servers":     cfg.Brokers,
		"group.id":              cfg.GroupID,
		"client.id":             cfg.ClientID,
		"auto.offset.reset":     cfg.AutoOffsetReset,
		"enable.auto.commit":    cfg.EnableAutoCommit,
		"session.timeout.ms":    cfg.SessionTimeoutMs,
		"heartbeat.interval.ms": cfg.HeartbeatIntervalMs,
		"max.poll.records":      cfg.MaxPollRecords,
		"fetch.min.bytes":       cfg.FetchMinBytes,
		"fetch.max.wait.ms":     cfg.FetchMaxWaitMs,
	}

	if cfg.SecurityProtocol != "" {
		config["security.protocol"] = cfg.SecurityProtocol
		config["sasl.mechanism"] = cfg.SASLMechanism
		config["sasl.username"] = cfg.SASLUsername
		config["sasl.password"] = cfg.SASLPassword
	}

	consumer, err := kafka.NewConsumer(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	logger.Info("Successfully created Kafka consumer")

	return &Consumer{
		consumer: consumer,
		log:      logger,
	}, nil
}

// Close closes the producer
func (p *Producer) Close() {
	p.producer.Close()
	p.log.Info("Kafka producer closed")
}

// Close closes the consumer
func (c *Consumer) Close() error {
	err := c.consumer.Close()
	if err != nil {
		c.log.WithError(err).Error("Failed to close Kafka consumer")
		return err
	}
	c.log.Info("Kafka consumer closed")
	return nil
}

// Producer returns the underlying Kafka producer
func (p *Producer) Producer() *kafka.Producer {
	return p.producer
}

// Consumer returns the underlying Kafka consumer
func (c *Consumer) Consumer() *kafka.Consumer {
	return c.consumer
}

// Produce sends a message to Kafka
func (p *Producer) Produce(topic string, key []byte, value []byte, headers map[string]string) error {
	var kafkaHeaders []kafka.Header
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: -1,
		},
		Key:     key,
		Value:   value,
		Headers: kafkaHeaders,
	}

	return p.producer.Produce(message, nil)
}

// ProduceSync sends a message to Kafka synchronously
func (p *Producer) ProduceSync(topic string, key []byte, value []byte, headers map[string]string, timeout time.Duration) error {
	var kafkaHeaders []kafka.Header
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: -1,
		},
		Key:     key,
		Value:   value,
		Headers: kafkaHeaders,
	}

	deliveryChan := make(chan kafka.Event, 1)
	defer close(deliveryChan)

	err := p.producer.Produce(message, deliveryChan)
	if err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	select {
	case e := <-deliveryChan:
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				return fmt.Errorf("delivery failed: %w", ev.TopicPartition.Error)
			}
			return nil
		case kafka.Error:
			return fmt.Errorf("kafka error: %w", ev)
		default:
			return fmt.Errorf("unexpected event type: %T", e)
		}
	case <-time.After(timeout):
		return fmt.Errorf("message delivery timeout after %v", timeout)
	}
}

// Subscribe subscribes to topics
func (c *Consumer) Subscribe(topics []string) error {
	return c.consumer.SubscribeTopics(topics, nil)
}

// Poll polls for messages
func (c *Consumer) Poll(timeout time.Duration) (*kafka.Message, error) {
	event := c.consumer.Poll(int(timeout.Milliseconds()))
	if event == nil {
		return nil, nil
	}

	switch e := event.(type) {
	case *kafka.Message:
		return e, nil
	case kafka.Error:
		return nil, fmt.Errorf("kafka error: %w", e)
	default:
		c.log.WithField("event", e).Debug("Ignored event")
		return nil, nil
	}
}

// Commit commits offsets
func (c *Consumer) Commit() error {
	_, err := c.consumer.Commit()
	return err
}

// CommitMessage commits a specific message offset
func (c *Consumer) CommitMessage(message *kafka.Message) error {
	_, err := c.consumer.CommitMessage(message)
	return err
}

// Flush flushes pending messages
func (p *Producer) Flush(timeout time.Duration) int {
	return p.producer.Flush(int(timeout.Milliseconds()))
}

// GetMetadata gets cluster metadata
func (p *Producer) GetMetadata(topic string, timeout time.Duration) (*kafka.Metadata, error) {
	return p.producer.GetMetadata(&topic, false, int(timeout.Milliseconds()))
}

// GetMetadata gets cluster metadata for consumer
func (c *Consumer) GetMetadata(topic string, timeout time.Duration) (*kafka.Metadata, error) {
	return c.consumer.GetMetadata(&topic, false, int(timeout.Milliseconds()))
}

// DefaultProducerConfig returns a default producer configuration
func DefaultProducerConfig() ProducerConfig {
	return ProducerConfig{
		Brokers:           "localhost:9092",
		ClientID:          "webhook-producer",
		CompressionType:   "snappy",
		BatchSize:         16384,
		LingerMs:          5,
		RetryBackoffMs:    100,
		MaxRetries:        3,
		RequestTimeoutMs:  30000,
		MessageTimeoutMs:  300000,
		Acks:              "all",
		EnableIdempotence: true,
	}
}

// DefaultConsumerConfig returns a default consumer configuration
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Brokers:             "localhost:9092",
		GroupID:             "webhook-consumer",
		ClientID:            "webhook-consumer",
		AutoOffsetReset:     "earliest",
		EnableAutoCommit:    false,
		SessionTimeoutMs:    30000,
		HeartbeatIntervalMs: 3000,
		MaxPollRecords:      500,
		FetchMinBytes:       1,
		FetchMaxWaitMs:      500,
	}
}
