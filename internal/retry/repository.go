package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/alexnthnz/webhook/pkg/redis"
)

// Repository handles retry operations using Redis
type Repository struct {
	redis *redis.Client
	log   *logrus.Logger
}

// NewRepository creates a new retry repository
func NewRepository(redisClient *redis.Client, logger *logrus.Logger) *Repository {
	return &Repository{
		redis: redisClient,
		log:   logger,
	}
}

// RetryStatus represents a retry record
type RetryStatus struct {
	RetryID         string            `json:"retry_id"`
	EventID         string            `json:"event_id"`
	WebhookID       string            `json:"webhook_id"`
	CustomerID      string            `json:"customer_id"`
	Payload         []byte            `json:"payload"`
	AttemptNumber   int               `json:"attempt_number"`
	MaxAttempts     int               `json:"max_attempts"`
	FailureReason   string            `json:"failure_reason"`
	State           RetryState        `json:"state"`
	CreatedAt       time.Time         `json:"created_at"`
	NextRetryAt     time.Time         `json:"next_retry_at"`
	LastAttemptedAt time.Time         `json:"last_attempted_at"`
	Headers         map[string]string `json:"headers"`
}

// RetryState represents the state of a retry
type RetryState int

const (
	RetryStateUnspecified RetryState = iota
	RetryStateScheduled
	RetryStateInProgress
	RetryStateCompleted
	RetryStateFailed
	RetryStateCancelled
	RetryStateExpired
)

// String returns the string representation of RetryState
func (s RetryState) String() string {
	switch s {
	case RetryStateScheduled:
		return "scheduled"
	case RetryStateInProgress:
		return "in_progress"
	case RetryStateCompleted:
		return "completed"
	case RetryStateFailed:
		return "failed"
	case RetryStateCancelled:
		return "cancelled"
	case RetryStateExpired:
		return "expired"
	default:
		return "unspecified"
	}
}

// Redis keys
const (
	retryKeyPrefix        = "retry:"
	retryScheduledSet     = "retry:scheduled"
	retryInProgressSet    = "retry:in_progress"
	retryByWebhookPrefix  = "retry:webhook:"
	retryByCustomerPrefix = "retry:customer:"
	retryByEventPrefix    = "retry:event:"
)

// ScheduleRetryRequest represents a request to schedule a retry
type ScheduleRetryRequest struct {
	EventID       string            `json:"event_id"`
	WebhookID     string            `json:"webhook_id"`
	CustomerID    string            `json:"customer_id"`
	Payload       []byte            `json:"payload"`
	AttemptNumber int               `json:"attempt_number"`
	FailureReason string            `json:"failure_reason"`
	NextRetryAt   time.Time         `json:"next_retry_at"`
	Headers       map[string]string `json:"headers"`
	MaxAttempts   int               `json:"max_attempts"`
}

// ScheduleRetry schedules a retry for a failed webhook delivery
func (r *Repository) ScheduleRetry(ctx context.Context, req ScheduleRetryRequest) (*RetryStatus, error) {
	retryID := uuid.New().String()
	now := time.Now()

	retry := &RetryStatus{
		RetryID:         retryID,
		EventID:         req.EventID,
		WebhookID:       req.WebhookID,
		CustomerID:      req.CustomerID,
		Payload:         req.Payload,
		AttemptNumber:   req.AttemptNumber,
		MaxAttempts:     req.MaxAttempts,
		FailureReason:   req.FailureReason,
		State:           RetryStateScheduled,
		CreatedAt:       now,
		NextRetryAt:     req.NextRetryAt,
		LastAttemptedAt: time.Time{},
		Headers:         req.Headers,
	}

	// Serialize retry status
	retryJSON, err := json.Marshal(retry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry status: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := r.redis.Client().Pipeline()

	// Store retry data
	retryKey := retryKeyPrefix + retryID
	pipe.Set(ctx, retryKey, retryJSON, 0)

	// Add to scheduled set with score as timestamp
	pipe.ZAdd(ctx, retryScheduledSet, goredis.Z{
		Score:  float64(req.NextRetryAt.Unix()),
		Member: retryID,
	})

	// Add to webhook index
	webhookKey := retryByWebhookPrefix + req.WebhookID
	pipe.SAdd(ctx, webhookKey, retryID)
	pipe.Expire(ctx, webhookKey, 24*time.Hour) // Expire after 24 hours

	// Add to customer index
	customerKey := retryByCustomerPrefix + req.CustomerID
	pipe.SAdd(ctx, customerKey, retryID)
	pipe.Expire(ctx, customerKey, 24*time.Hour) // Expire after 24 hours

	// Add to event index
	eventKey := retryByEventPrefix + req.EventID
	pipe.SAdd(ctx, eventKey, retryID)
	pipe.Expire(ctx, eventKey, 24*time.Hour) // Expire after 24 hours

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		r.log.WithError(err).Error("Failed to schedule retry")
		return nil, fmt.Errorf("failed to schedule retry: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"retry_id":       retryID,
		"event_id":       req.EventID,
		"webhook_id":     req.WebhookID,
		"attempt_number": req.AttemptNumber,
		"next_retry_at":  req.NextRetryAt,
	}).Info("Retry scheduled successfully")

	return retry, nil
}

// GetRetryStatus retrieves retry status by ID
func (r *Repository) GetRetryStatus(ctx context.Context, retryID string) (*RetryStatus, error) {
	retryKey := retryKeyPrefix + retryID
	retryJSON, err := r.redis.Get(ctx, retryKey)
	if err != nil {
		if err == goredis.Nil {
			return nil, fmt.Errorf("retry not found")
		}
		return nil, fmt.Errorf("failed to get retry status: %w", err)
	}

	var retry RetryStatus
	if err := json.Unmarshal([]byte(retryJSON), &retry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal retry status: %w", err)
	}

	return &retry, nil
}

// GetRetryStatusByEvent retrieves retry status by event ID
func (r *Repository) GetRetryStatusByEvent(ctx context.Context, eventID string) (*RetryStatus, error) {
	eventKey := retryByEventPrefix + eventID
	retryIDs, err := r.redis.Client().SMembers(ctx, eventKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get retry IDs for event: %w", err)
	}

	if len(retryIDs) == 0 {
		return nil, fmt.Errorf("no retry found for event")
	}

	// Get the first retry (there should typically be only one per event)
	return r.GetRetryStatus(ctx, retryIDs[0])
}

// CancelRetry cancels a scheduled retry
func (r *Repository) CancelRetry(ctx context.Context, retryID, reason string) error {
	retry, err := r.GetRetryStatus(ctx, retryID)
	if err != nil {
		return err
	}

	// Update retry state
	retry.State = RetryStateCancelled
	retry.FailureReason = reason

	// Serialize updated retry
	retryJSON, err := json.Marshal(retry)
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := r.redis.Client().Pipeline()

	// Update retry data
	retryKey := retryKeyPrefix + retryID
	pipe.Set(ctx, retryKey, retryJSON, 0)

	// Remove from scheduled set
	pipe.ZRem(ctx, retryScheduledSet, retryID)

	// Remove from in-progress set if it exists
	pipe.SRem(ctx, retryInProgressSet, retryID)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cancel retry: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"retry_id": retryID,
		"reason":   reason,
	}).Info("Retry cancelled successfully")

	return nil
}

// ListRetries lists retries with filtering and pagination
func (r *Repository) ListRetries(ctx context.Context, webhookID, customerID string, limit, offset int, state RetryState) ([]*RetryStatus, int, error) {
	var retryIDs []string
	var err error

	// Get retry IDs based on filter
	if webhookID != "" {
		webhookKey := retryByWebhookPrefix + webhookID
		retryIDs, err = r.redis.Client().SMembers(ctx, webhookKey).Result()
	} else if customerID != "" {
		customerKey := retryByCustomerPrefix + customerID
		retryIDs, err = r.redis.Client().SMembers(ctx, customerKey).Result()
	} else {
		// Get all retry IDs from scheduled and in-progress sets
		scheduledIDs, err1 := r.redis.Client().ZRange(ctx, retryScheduledSet, 0, -1).Result()
		inProgressIDs, err2 := r.redis.Client().SMembers(ctx, retryInProgressSet).Result()
		if err1 != nil || err2 != nil {
			return nil, 0, fmt.Errorf("failed to get retry IDs")
		}
		retryIDs = append(scheduledIDs, inProgressIDs...)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get retry IDs: %w", err)
	}

	// Get retry statuses
	var retries []*RetryStatus
	for _, retryID := range retryIDs {
		retry, err := r.GetRetryStatus(ctx, retryID)
		if err != nil {
			r.log.WithError(err).WithField("retry_id", retryID).Warn("Failed to get retry status")
			continue
		}

		// Filter by state if specified
		if state != RetryStateUnspecified && retry.State != state {
			continue
		}

		retries = append(retries, retry)
	}

	totalCount := len(retries)

	// Apply pagination
	if offset > 0 {
		if offset >= len(retries) {
			return []*RetryStatus{}, totalCount, nil
		}
		retries = retries[offset:]
	}

	if limit > 0 && len(retries) > limit {
		retries = retries[:limit]
	}

	return retries, totalCount, nil
}

// GetDueRetries gets retries that are due for processing
func (r *Repository) GetDueRetries(ctx context.Context, limit int) ([]*RetryStatus, error) {
	now := time.Now()

	// Get retry IDs that are due (score <= current timestamp)
	retryIDs, err := r.redis.Client().ZRangeByScore(ctx, retryScheduledSet, &goredis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%d", now.Unix()),
		Count: int64(limit),
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get due retries: %w", err)
	}

	var retries []*RetryStatus
	for _, retryID := range retryIDs {
		retry, err := r.GetRetryStatus(ctx, retryID)
		if err != nil {
			r.log.WithError(err).WithField("retry_id", retryID).Warn("Failed to get retry status")
			continue
		}
		retries = append(retries, retry)
	}

	return retries, nil
}

// MarkRetryInProgress marks a retry as in progress
func (r *Repository) MarkRetryInProgress(ctx context.Context, retryID string) error {
	retry, err := r.GetRetryStatus(ctx, retryID)
	if err != nil {
		return err
	}

	retry.State = RetryStateInProgress
	retry.LastAttemptedAt = time.Now()

	// Serialize updated retry
	retryJSON, err := json.Marshal(retry)
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := r.redis.Client().Pipeline()

	// Update retry data
	retryKey := retryKeyPrefix + retryID
	pipe.Set(ctx, retryKey, retryJSON, 0)

	// Move from scheduled to in-progress
	pipe.ZRem(ctx, retryScheduledSet, retryID)
	pipe.SAdd(ctx, retryInProgressSet, retryID)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark retry in progress: %w", err)
	}

	return nil
}

// MarkRetryCompleted marks a retry as completed
func (r *Repository) MarkRetryCompleted(ctx context.Context, retryID string) error {
	retry, err := r.GetRetryStatus(ctx, retryID)
	if err != nil {
		return err
	}

	retry.State = RetryStateCompleted

	// Serialize updated retry
	retryJSON, err := json.Marshal(retry)
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := r.redis.Client().Pipeline()

	// Update retry data
	retryKey := retryKeyPrefix + retryID
	pipe.Set(ctx, retryKey, retryJSON, 0)

	// Remove from in-progress set
	pipe.SRem(ctx, retryInProgressSet, retryID)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark retry completed: %w", err)
	}

	return nil
}

// MarkRetryFailed marks a retry as failed
func (r *Repository) MarkRetryFailed(ctx context.Context, retryID, reason string) error {
	retry, err := r.GetRetryStatus(ctx, retryID)
	if err != nil {
		return err
	}

	retry.State = RetryStateFailed
	retry.FailureReason = reason

	// Serialize updated retry
	retryJSON, err := json.Marshal(retry)
	if err != nil {
		return fmt.Errorf("failed to marshal retry status: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := r.redis.Client().Pipeline()

	// Update retry data
	retryKey := retryKeyPrefix + retryID
	pipe.Set(ctx, retryKey, retryJSON, 0)

	// Remove from in-progress set
	pipe.SRem(ctx, retryInProgressSet, retryID)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark retry failed: %w", err)
	}

	return nil
}
