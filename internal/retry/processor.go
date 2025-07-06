package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/pkg/kafka"
)

// Processor handles the background processing of due retries
type Processor struct {
	service       *Service
	kafkaProducer *kafka.Producer
	config        config.RetryManagerConfig
	log           *logrus.Logger
	stopChan      chan struct{}
}

// NewProcessor creates a new retry processor
func NewProcessor(
	service *Service,
	kafkaProducer *kafka.Producer,
	config config.RetryManagerConfig,
	logger *logrus.Logger,
) *Processor {
	return &Processor{
		service:       service,
		kafkaProducer: kafkaProducer,
		config:        config,
		log:           logger,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the retry processor
func (p *Processor) Start(ctx context.Context) {
	ticker := time.NewTicker(p.config.ProcessInterval)
	defer ticker.Stop()

	p.log.Info("Starting retry processor")

	for {
		select {
		case <-ctx.Done():
			p.log.Info("Retry processor context cancelled")
			return
		case <-p.stopChan:
			p.log.Info("Retry processor stopped")
			return
		case <-ticker.C:
			p.processRetries(ctx)
		}
	}
}

// Stop stops the retry processor
func (p *Processor) Stop() {
	close(p.stopChan)
}

// processRetries processes due retries
func (p *Processor) processRetries(ctx context.Context) {
	processingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get due retries
	dueRetries, err := p.service.GetDueRetries(processingCtx, p.config.BatchSize)
	if err != nil {
		p.log.WithError(err).Error("Failed to get due retries")
		return
	}

	if len(dueRetries) == 0 {
		return
	}

	p.log.WithField("count", len(dueRetries)).Info("Processing due retries")

	// Process retries concurrently
	semaphore := make(chan struct{}, p.config.WorkerCount)

	for _, retry := range dueRetries {
		semaphore <- struct{}{} // Acquire
		go func(r *RetryStatus) {
			defer func() { <-semaphore }() // Release
			p.processRetry(processingCtx, r)
		}(retry)
	}

	// Wait for all workers to complete
	for i := 0; i < p.config.WorkerCount; i++ {
		semaphore <- struct{}{}
	}
}

// processRetry processes a single retry
func (p *Processor) processRetry(ctx context.Context, retry *RetryStatus) {
	logger := p.log.WithFields(logrus.Fields{
		"retry_id":       retry.RetryID,
		"event_id":       retry.EventID,
		"webhook_id":     retry.WebhookID,
		"customer_id":    retry.CustomerID,
		"attempt_number": retry.AttemptNumber,
	})

	logger.Info("Processing retry")

	// Mark retry as in progress
	err := p.service.repo.MarkRetryInProgress(ctx, retry.RetryID)
	if err != nil {
		logger.WithError(err).Error("Failed to mark retry in progress")
		return
	}

	// Create retry event for dispatcher
	retryEvent := RetryEvent{
		RetryID:       retry.RetryID,
		EventID:       retry.EventID,
		WebhookID:     retry.WebhookID,
		CustomerID:    retry.CustomerID,
		EventType:     "webhook.retry", // Default event type
		Payload:       retry.Payload,
		Headers:       retry.Headers,
		AttemptNumber: retry.AttemptNumber,
		MaxAttempts:   retry.MaxAttempts,
		CreatedAt:     time.Now(),
	}

	// Send retry event to Kafka for dispatcher to process
	err = p.sendRetryToDispatcher(ctx, retryEvent)
	if err != nil {
		logger.WithError(err).Error("Failed to send retry to dispatcher")
		p.handleRetryFailure(ctx, retry, fmt.Sprintf("Failed to send retry to dispatcher: %v", err))
		return
	}

	logger.Info("Retry sent to dispatcher successfully")
}

// sendRetryToDispatcher sends a retry event to the dispatcher via Kafka
func (p *Processor) sendRetryToDispatcher(ctx context.Context, event RetryEvent) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal retry event: %w", err)
	}

	// Send to webhook events topic with retry flag
	headers := map[string]string{
		"retry":        "true",
		"retry_id":     event.RetryID,
		"attempt":      fmt.Sprintf("%d", event.AttemptNumber),
		"max_attempts": fmt.Sprintf("%d", event.MaxAttempts),
	}

	err = p.kafkaProducer.ProduceSync(
		"webhook-events",
		[]byte(event.EventID),
		eventBytes,
		headers,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("failed to send retry event to Kafka: %w", err)
	}

	return nil
}

// handleRetryFailure handles retry failure
func (p *Processor) handleRetryFailure(ctx context.Context, retry *RetryStatus, reason string) {
	logger := p.log.WithFields(logrus.Fields{
		"retry_id":       retry.RetryID,
		"event_id":       retry.EventID,
		"webhook_id":     retry.WebhookID,
		"attempt_number": retry.AttemptNumber,
		"max_attempts":   retry.MaxAttempts,
		"reason":         reason,
	})

	// Check if we've exceeded max attempts
	if retry.AttemptNumber >= retry.MaxAttempts {
		logger.Warn("Retry exceeded max attempts, sending to DLQ")

		// Send to Dead Letter Queue via Kafka
		err := p.sendToDLQ(ctx, retry, reason)
		if err != nil {
			logger.WithError(err).Error("Failed to send to DLQ")
		}

		// Mark retry as failed
		err = p.service.repo.MarkRetryFailed(ctx, retry.RetryID, "Exceeded max attempts")
		if err != nil {
			logger.WithError(err).Error("Failed to mark retry as failed")
		}
	} else {
		// Schedule next retry
		nextAttempt := retry.AttemptNumber + 1
		retryPolicy := RetryPolicy{
			MaxAttempts:    p.config.MaxRetries,
			BackoffType:    "exponential",
			InitialDelayMs: int(p.config.InitialDelay.Milliseconds()),
			MaxDelayMs:     int(p.config.MaxDelay.Milliseconds()),
			Multiplier:     p.config.BackoffMultiplier,
		}

		nextRetryAt := p.service.CalculateNextRetryTime(nextAttempt, retryPolicy)

		scheduleReq := ScheduleRetryRequest{
			EventID:       retry.EventID,
			WebhookID:     retry.WebhookID,
			CustomerID:    retry.CustomerID,
			Payload:       retry.Payload,
			AttemptNumber: nextAttempt,
			FailureReason: reason,
			NextRetryAt:   nextRetryAt,
			Headers:       retry.Headers,
			MaxAttempts:   retry.MaxAttempts,
		}

		_, err := p.service.repo.ScheduleRetry(ctx, scheduleReq)
		if err != nil {
			logger.WithError(err).Error("Failed to schedule next retry")
		} else {
			logger.WithField("next_retry_at", nextRetryAt).Info("Scheduled next retry")
		}

		// Mark current retry as failed
		err = p.service.repo.MarkRetryFailed(ctx, retry.RetryID, reason)
		if err != nil {
			logger.WithError(err).Error("Failed to mark retry as failed")
		}
	}
}

// sendToDLQ sends a failed retry to the Dead Letter Queue via Kafka
func (p *Processor) sendToDLQ(ctx context.Context, retry *RetryStatus, reason string) error {
	dlqMessage := DLQMessage{
		EventID:       retry.EventID,
		WebhookID:     retry.WebhookID,
		CustomerID:    retry.CustomerID,
		EventType:     "webhook.retry.failed",
		FailureReason: reason,
		Attempts:      retry.AttemptNumber,
		FirstFailedAt: retry.CreatedAt,
		LastFailedAt:  time.Now(),
		CreatedAt:     time.Now(),
		Headers:       retry.Headers,
	}

	// Parse original event from payload
	err := json.Unmarshal(retry.Payload, &dlqMessage.OriginalEvent)
	if err != nil {
		p.log.WithError(err).Warn("Failed to parse original event payload")
		dlqMessage.OriginalEvent = map[string]interface{}{
			"raw_payload": string(retry.Payload),
		}
	}

	messageBytes, err := json.Marshal(dlqMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Send to DLQ topic
	headers := map[string]string{
		"dlq":            "true",
		"retry_id":       retry.RetryID,
		"failure_reason": reason,
		"attempts":       fmt.Sprintf("%d", retry.AttemptNumber),
	}

	err = p.kafkaProducer.ProduceSync(
		"webhook-dlq",
		[]byte(retry.EventID),
		messageBytes,
		headers,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("failed to send to DLQ topic: %w", err)
	}

	return nil
}

// RetryEvent represents a retry event sent to the dispatcher
type RetryEvent struct {
	RetryID       string            `json:"retry_id"`
	EventID       string            `json:"event_id"`
	WebhookID     string            `json:"webhook_id"`
	CustomerID    string            `json:"customer_id"`
	EventType     string            `json:"event_type"`
	Payload       []byte            `json:"payload"`
	Headers       map[string]string `json:"headers"`
	AttemptNumber int               `json:"attempt_number"`
	MaxAttempts   int               `json:"max_attempts"`
	CreatedAt     time.Time         `json:"created_at"`
}

// DLQMessage represents a message in the dead letter queue
type DLQMessage struct {
	EventID       string                 `json:"event_id"`
	WebhookID     string                 `json:"webhook_id"`
	CustomerID    string                 `json:"customer_id"`
	EventType     string                 `json:"event_type"`
	OriginalEvent map[string]interface{} `json:"original_event"`
	FailureReason string                 `json:"failure_reason"`
	Attempts      int                    `json:"attempts"`
	FirstFailedAt time.Time              `json:"first_failed_at"`
	LastFailedAt  time.Time              `json:"last_failed_at"`
	CreatedAt     time.Time              `json:"created_at"`
	Headers       map[string]string      `json:"headers"`
}
