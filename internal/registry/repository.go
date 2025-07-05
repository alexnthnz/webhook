package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/alexnthnz/webhook/pkg/postgres"
)

// Repository handles database operations for webhooks
type Repository struct {
	db  *postgres.Client
	log *logrus.Logger
}

// NewRepository creates a new webhook repository
func NewRepository(db *postgres.Client, logger *logrus.Logger) *Repository {
	return &Repository{
		db:  db,
		log: logger,
	}
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID          string            `json:"id"`
	CustomerID  string            `json:"customer_id"`
	URL         string            `json:"url"`
	EventTypes  []string          `json:"event_types"`
	RetryPolicy RetryPolicy       `json:"retry_policy"`
	Secret      string            `json:"secret"`
	Headers     map[string]string `json:"headers"`
	Active      bool              `json:"active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RetryPolicy defines the retry behavior for webhooks
type RetryPolicy struct {
	MaxAttempts    int     `json:"max_attempts"`
	BackoffType    string  `json:"backoff_type"`
	InitialDelayMs int     `json:"initial_delay_ms"`
	MaxDelayMs     int     `json:"max_delay_ms"`
	Multiplier     float64 `json:"multiplier"`
}

// CreateWebhookRequest represents a request to create a webhook
type CreateWebhookRequest struct {
	CustomerID  string            `json:"customer_id"`
	URL         string            `json:"url"`
	EventTypes  []string          `json:"event_types"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Secret      string            `json:"secret,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// UpdateWebhookRequest represents a request to update a webhook
type UpdateWebhookRequest struct {
	URL         *string           `json:"url,omitempty"`
	EventTypes  []string          `json:"event_types,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Active      *bool             `json:"active,omitempty"`
}

// Create creates a new webhook
func (r *Repository) Create(ctx context.Context, req CreateWebhookRequest) (*Webhook, error) {
	// Generate ID and secret if not provided
	id := uuid.New().String()
	secret := req.Secret
	if secret == "" {
		secret = generateSecret()
	}

	// Set default retry policy if not provided
	retryPolicy := req.RetryPolicy
	if retryPolicy == nil {
		retryPolicy = &RetryPolicy{
			MaxAttempts:    5,
			BackoffType:    "exponential",
			InitialDelayMs: 1000,
			MaxDelayMs:     60000,
			Multiplier:     2.0,
		}
	}

	// Set default headers if not provided
	headers := req.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	// Convert to JSON for storage
	retryPolicyJSON, err := json.Marshal(retryPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	query := `
		INSERT INTO webhooks (id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`

	now := time.Now()
	var createdAt, updatedAt time.Time

	err = r.db.Pool().QueryRow(ctx, query,
		id, req.CustomerID, req.URL, req.EventTypes, retryPolicyJSON, secret, headersJSON, true, now, now,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		r.log.WithError(err).Error("Failed to create webhook")
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	webhook := &Webhook{
		ID:          id,
		CustomerID:  req.CustomerID,
		URL:         req.URL,
		EventTypes:  req.EventTypes,
		RetryPolicy: *retryPolicy,
		Secret:      secret,
		Headers:     headers,
		Active:      true,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	r.log.WithFields(logrus.Fields{
		"webhook_id":  id,
		"customer_id": req.CustomerID,
		"url":         req.URL,
	}).Info("Webhook created successfully")

	return webhook, nil
}

// GetByID retrieves a webhook by ID
func (r *Repository) GetByID(ctx context.Context, id, customerID string) (*Webhook, error) {
	query := `
		SELECT id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at
		FROM webhooks
		WHERE id = $1 AND customer_id = $2
	`

	var webhook Webhook
	var retryPolicyJSON, headersJSON []byte

	err := r.db.Pool().QueryRow(ctx, query, id, customerID).Scan(
		&webhook.ID, &webhook.CustomerID, &webhook.URL, &webhook.EventTypes,
		&retryPolicyJSON, &webhook.Secret, &headersJSON, &webhook.Active,
		&webhook.CreatedAt, &webhook.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("webhook not found")
		}
		r.log.WithError(err).Error("Failed to get webhook")
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(retryPolicyJSON, &webhook.RetryPolicy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
	}

	if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
	}

	return &webhook, nil
}

// GetByCustomer retrieves webhooks for a customer with pagination
func (r *Repository) GetByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*Webhook, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM webhooks WHERE customer_id = $1`
	var totalCount int
	err := r.db.Pool().QueryRow(ctx, countQuery, customerID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get webhook count: %w", err)
	}

	// Get webhooks with pagination
	query := `
		SELECT id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at
		FROM webhooks
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool().Query(ctx, query, customerID, limit, offset)
	if err != nil {
		r.log.WithError(err).Error("Failed to get webhooks")
		return nil, 0, fmt.Errorf("failed to get webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var webhook Webhook
		var retryPolicyJSON, headersJSON []byte

		err := rows.Scan(
			&webhook.ID, &webhook.CustomerID, &webhook.URL, &webhook.EventTypes,
			&retryPolicyJSON, &webhook.Secret, &headersJSON, &webhook.Active,
			&webhook.CreatedAt, &webhook.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan webhook: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(retryPolicyJSON, &webhook.RetryPolicy); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal headers: %w", err)
		}

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating webhooks: %w", err)
	}

	return webhooks, totalCount, nil
}

// GetByEventType retrieves active webhooks that subscribe to a specific event type
func (r *Repository) GetByEventType(ctx context.Context, eventType string) ([]*Webhook, error) {
	query := `
		SELECT id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at
		FROM webhooks
		WHERE active = true AND $1 = ANY(event_types)
		ORDER BY created_at ASC
	`

	rows, err := r.db.Pool().Query(ctx, query, eventType)
	if err != nil {
		r.log.WithError(err).Error("Failed to get webhooks by event type")
		return nil, fmt.Errorf("failed to get webhooks by event type: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var webhook Webhook
		var retryPolicyJSON, headersJSON []byte

		err := rows.Scan(
			&webhook.ID, &webhook.CustomerID, &webhook.URL, &webhook.EventTypes,
			&retryPolicyJSON, &webhook.Secret, &headersJSON, &webhook.Active,
			&webhook.CreatedAt, &webhook.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(retryPolicyJSON, &webhook.RetryPolicy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
		}

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating webhooks: %w", err)
	}

	return webhooks, nil
}

// Update updates a webhook
func (r *Repository) Update(ctx context.Context, id, customerID string, req UpdateWebhookRequest) (*Webhook, error) {
	// Get existing webhook first
	existing, err := r.GetByID(ctx, id, customerID)
	if err != nil {
		return nil, err
	}

	// Build update query dynamically
	setParts := []string{"updated_at = $1"}
	args := []interface{}{time.Now()}
	argIndex := 2

	if req.URL != nil {
		setParts = append(setParts, fmt.Sprintf("url = $%d", argIndex))
		args = append(args, *req.URL)
		argIndex++
		existing.URL = *req.URL
	}

	if req.EventTypes != nil {
		setParts = append(setParts, fmt.Sprintf("event_types = $%d", argIndex))
		args = append(args, req.EventTypes)
		argIndex++
		existing.EventTypes = req.EventTypes
	}

	if req.RetryPolicy != nil {
		retryPolicyJSON, err := json.Marshal(req.RetryPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
		}
		setParts = append(setParts, fmt.Sprintf("retry_policy = $%d", argIndex))
		args = append(args, retryPolicyJSON)
		argIndex++
		existing.RetryPolicy = *req.RetryPolicy
	}

	if req.Headers != nil {
		headersJSON, err := json.Marshal(req.Headers)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal headers: %w", err)
		}
		setParts = append(setParts, fmt.Sprintf("headers = $%d", argIndex))
		args = append(args, headersJSON)
		argIndex++
		existing.Headers = req.Headers
	}

	if req.Active != nil {
		setParts = append(setParts, fmt.Sprintf("active = $%d", argIndex))
		args = append(args, *req.Active)
		argIndex++
		existing.Active = *req.Active
	}

	// Add WHERE clause
	whereClause := fmt.Sprintf("WHERE id = $%d AND customer_id = $%d", argIndex, argIndex+1)
	args = append(args, id, customerID)

	query := fmt.Sprintf("UPDATE webhooks SET %s %s RETURNING updated_at",
		fmt.Sprintf("%s", setParts[0:1][0])+", "+fmt.Sprintf("%s", setParts[1:]),
		whereClause)

	var updatedAt time.Time
	err = r.db.Pool().QueryRow(ctx, query, args...).Scan(&updatedAt)
	if err != nil {
		r.log.WithError(err).Error("Failed to update webhook")
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	existing.UpdatedAt = updatedAt

	r.log.WithFields(logrus.Fields{
		"webhook_id":  id,
		"customer_id": customerID,
	}).Info("Webhook updated successfully")

	return existing, nil
}

// Delete deletes a webhook
func (r *Repository) Delete(ctx context.Context, id, customerID string) error {
	query := `DELETE FROM webhooks WHERE id = $1 AND customer_id = $2`

	result, err := r.db.Pool().Exec(ctx, query, id, customerID)
	if err != nil {
		r.log.WithError(err).Error("Failed to delete webhook")
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}

	r.log.WithFields(logrus.Fields{
		"webhook_id":  id,
		"customer_id": customerID,
	}).Info("Webhook deleted successfully")

	return nil
}

// List retrieves all webhooks with pagination
func (r *Repository) List(ctx context.Context, limit, offset int, customerID string) ([]*Webhook, int, error) {
	var countQuery, query string
	var args []interface{}

	if customerID != "" {
		countQuery = `SELECT COUNT(*) FROM webhooks WHERE customer_id = $1`
		query = `
			SELECT id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at
			FROM webhooks
			WHERE customer_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{customerID, limit, offset}
	} else {
		countQuery = `SELECT COUNT(*) FROM webhooks`
		query = `
			SELECT id, customer_id, url, event_types, retry_policy, secret, headers, active, created_at, updated_at
			FROM webhooks
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{limit, offset}
	}

	// Get total count
	var totalCount int
	countArgs := args[:len(args)-2] // Remove limit and offset for count query
	if len(countArgs) == 0 {
		err := r.db.Pool().QueryRow(ctx, countQuery).Scan(&totalCount)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get webhook count: %w", err)
		}
	} else {
		err := r.db.Pool().QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get webhook count: %w", err)
		}
	}

	// Get webhooks
	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		r.log.WithError(err).Error("Failed to list webhooks")
		return nil, 0, fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var webhook Webhook
		var retryPolicyJSON, headersJSON []byte

		err := rows.Scan(
			&webhook.ID, &webhook.CustomerID, &webhook.URL, &webhook.EventTypes,
			&retryPolicyJSON, &webhook.Secret, &headersJSON, &webhook.Active,
			&webhook.CreatedAt, &webhook.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan webhook: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(retryPolicyJSON, &webhook.RetryPolicy); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal retry policy: %w", err)
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal headers: %w", err)
		}

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating webhooks: %w", err)
	}

	return webhooks, totalCount, nil
}

// generateSecret generates a random secret for webhook signing
func generateSecret() string {
	// Generate a 32-byte random secret
	return uuid.New().String() + uuid.New().String()
}
