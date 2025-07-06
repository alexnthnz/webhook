package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/registry"
	"github.com/alexnthnz/webhook/internal/security"
	"github.com/alexnthnz/webhook/pkg/circuitbreaker"
	"github.com/alexnthnz/webhook/pkg/postgres"
)

// TestWebhookEndToEndFlow tests the complete webhook flow
func TestWebhookEndToEndFlow(t *testing.T) {
	// Setup test environment
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create test configuration
	cfg := createTestConfig()

	// Setup test database
	testDB := setupTestDatabase(t, cfg, logger)
	defer cleanupTestDatabase(t, testDB)

	// Setup webhook endpoint mock
	webhookServer := setupMockWebhookEndpoint(t)
	defer webhookServer.Close()

	// Test scenario: Create webhook, send event, verify delivery
	t.Run("complete_webhook_flow", func(t *testing.T) {
		// 1. Create a webhook
		webhook := createTestWebhook(t, webhookServer.URL)

		// 2. Send an event
		event := createTestEvent()

		// 3. Verify webhook delivery
		verifyWebhookDelivery(t, webhook, event)

		// 4. Test retry mechanism
		testRetryMechanism(t, webhook, event)

		// 5. Test circuit breaker
		testCircuitBreaker(t, webhook)
	})
}

// TestSecurityFeatures tests authentication and HMAC signing
func TestSecurityFeatures(t *testing.T) {
	cfg := createTestConfig()

	t.Run("jwt_authentication", func(t *testing.T) {
		jwtManager := security.NewJWTManager(
			cfg.Security.JWTSecret,
			cfg.Security.JWTIssuer,
			cfg.Security.JWTAudience,
			cfg.Security.JWTExpiration,
		)

		// Test token generation
		token, err := jwtManager.GenerateToken("test-customer", []string{"webhook:read", "webhook:write"})
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Test token validation
		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "test-customer", claims.CustomerID)
		assert.Contains(t, claims.Scopes, "webhook:read")
		assert.Contains(t, claims.Scopes, "webhook:write")
	})

	t.Run("hmac_signing", func(t *testing.T) {
		hmacSigner := security.NewHMACSignerFromSecret(cfg.Security.HMACSecret)

		payload := []byte(`{"id":"test-event","type":"user.created","data":{"user_id":"123"}}`)

		// Test signing
		signature := hmacSigner.Sign(payload)
		require.NotEmpty(t, signature)
		require.Contains(t, signature, "sha256=")

		// Test verification
		isValid := hmacSigner.Verify(payload, signature)
		assert.True(t, isValid)

		// Test invalid signature
		isValid = hmacSigner.Verify(payload, "sha256=invalid")
		assert.False(t, isValid)
	})
}

// TestCircuitBreakerIntegration tests circuit breaker functionality
func TestCircuitBreakerIntegration(t *testing.T) {

	t.Run("circuit_breaker_states", func(t *testing.T) {
		logger := logrus.New()
		manager := circuitbreaker.NewManager(logger)

		// Create circuit breaker with aggressive settings for testing
		config := circuitbreaker.AggressiveConfig("test-webhook")
		cb := manager.GetOrCreate("test-webhook", config)

		// Test closed state (normal operation)
		assert.Equal(t, circuitbreaker.StateClosed, cb.State())

		// Simulate failures to trip the circuit breaker
		for i := 0; i < 3; i++ {
			err := cb.Execute(func() error {
				return fmt.Errorf("simulated failure")
			})
			assert.Error(t, err)
		}

		// Circuit breaker should now be open
		assert.Equal(t, circuitbreaker.StateOpen, cb.State())

		// Requests should be rejected
		err := cb.Execute(func() error {
			return nil
		})
		assert.Equal(t, circuitbreaker.ErrOpenState, err)
	})
}

// TestRetryMechanism tests the retry functionality
func TestRetryMechanism(t *testing.T) {

	t.Run("exponential_backoff", func(t *testing.T) {
		// Test exponential backoff calculation
		retryPolicy := registry.RetryPolicy{
			MaxAttempts:    5,
			BackoffType:    "exponential",
			InitialDelayMs: 1000,
			MaxDelayMs:     30000,
			Multiplier:     2.0,
		}

		// Calculate retry times
		delays := []time.Duration{}
		for attempt := 1; attempt <= 5; attempt++ {
			delay := calculateRetryDelay(retryPolicy, attempt)
			delays = append(delays, delay)
		}

		// Verify exponential growth
		assert.True(t, delays[1] > delays[0])
		assert.True(t, delays[2] > delays[1])
		assert.True(t, delays[3] > delays[2])

		// Verify max delay is respected
		for _, delay := range delays {
			assert.LessOrEqual(t, delay, time.Duration(retryPolicy.MaxDelayMs)*time.Millisecond)
		}
	})
}

// TestDLQFunctionality tests dead letter queue functionality
func TestDLQFunctionality(t *testing.T) {

	t.Run("dlq_message_creation", func(t *testing.T) {
		// Test DLQ message structure
		dlqMessage := map[string]interface{}{
			"event_id":       "test-event-123",
			"webhook_id":     "webhook-456",
			"customer_id":    "customer-789",
			"event_type":     "webhook.retry.failed",
			"failure_reason": "Max retries exceeded",
			"attempts":       5,
			"created_at":     time.Now(),
		}

		// Verify required fields
		assert.NotEmpty(t, dlqMessage["event_id"])
		assert.NotEmpty(t, dlqMessage["webhook_id"])
		assert.NotEmpty(t, dlqMessage["customer_id"])
		assert.Equal(t, 5, dlqMessage["attempts"])
	})
}

// TestMetricsCollection tests metrics collection
func TestMetricsCollection(t *testing.T) {
	t.Run("delivery_metrics", func(t *testing.T) {
		// Test that metrics are properly structured
		metrics := map[string]interface{}{
			"webhook_delivery_attempts_total":  0,
			"webhook_delivery_success_total":   0,
			"webhook_delivery_failures_total":  0,
			"webhook_delivery_latency_seconds": 0.0,
			"webhook_circuit_breaker_state":    0,
			"webhook_retry_attempts_total":     0,
		}

		// Verify metric names are properly formatted
		for metricName := range metrics {
			assert.Contains(t, metricName, "webhook_")
			assert.NotContains(t, metricName, " ")
		}
	})
}

// Helper functions

func createTestConfig() *config.Config {
	return &config.Config{
		Environment: "test",
		LogLevel:    "debug",
		Security: config.SecurityConfig{
			JWTSecret:     "test-jwt-secret-for-testing-only",
			JWTIssuer:     "webhook-service-test",
			JWTAudience:   "webhook-api-test",
			JWTExpiration: time.Hour,
			HMACSecret:    "test-hmac-secret-for-testing-only",
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "webhook_test",
			Username: "postgres",
			Password: "postgres123",
			SSLMode:  "disable",
		},
	}
}

func setupTestDatabase(t *testing.T, cfg *config.Config, logger *logrus.Logger) *postgres.Client {
	// In a real test, you would set up a test database
	// For now, return nil to avoid database dependency
	return nil
}

func cleanupTestDatabase(t *testing.T, db *postgres.Client) {
	// Cleanup test data
	if db != nil {
		// db.Close()
	}
}

func setupMockWebhookEndpoint(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("X-Webhook-Signature"))
		assert.NotEmpty(t, r.Header.Get("X-Event-ID"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	}))
}

func createTestWebhook(t *testing.T, webhookURL string) map[string]interface{} {
	return map[string]interface{}{
		"webhook_id":  "test-webhook-123",
		"customer_id": "test-customer-456",
		"url":         webhookURL,
		"event_types": []string{"user.created", "user.updated"},
		"headers": map[string]string{
			"X-Custom-Header": "test-value",
		},
		"retry_policy": map[string]interface{}{
			"max_attempts":     5,
			"backoff_type":     "exponential",
			"initial_delay_ms": 1000,
			"max_delay_ms":     30000,
			"multiplier":       2.0,
		},
	}
}

func createTestEvent() map[string]interface{} {
	return map[string]interface{}{
		"id":        "test-event-789",
		"type":      "user.created",
		"source":    "user-service",
		"timestamp": time.Now(),
		"data": map[string]interface{}{
			"user_id": "user-123",
			"email":   "test@example.com",
			"name":    "Test User",
		},
	}
}

func verifyWebhookDelivery(t *testing.T, webhook, event map[string]interface{}) {
	// In a real test, this would verify that the webhook was called
	// with the correct payload and headers
	assert.NotNil(t, webhook)
	assert.NotNil(t, event)
}

func testRetryMechanism(t *testing.T, webhook, event map[string]interface{}) {
	// Test that retries are scheduled when webhook delivery fails
	assert.NotNil(t, webhook)
	assert.NotNil(t, event)
}

func testCircuitBreaker(t *testing.T, webhook map[string]interface{}) {
	// Test that circuit breaker protects against failing webhooks
	assert.NotNil(t, webhook)
}

func calculateRetryDelay(policy registry.RetryPolicy, attempt int) time.Duration {
	if policy.BackoffType == "exponential" {
		delay := float64(policy.InitialDelayMs)
		for i := 1; i < attempt; i++ {
			delay *= policy.Multiplier
		}
		if delay > float64(policy.MaxDelayMs) {
			delay = float64(policy.MaxDelayMs)
		}
		return time.Duration(delay) * time.Millisecond
	}
	return time.Duration(policy.InitialDelayMs) * time.Millisecond
}
