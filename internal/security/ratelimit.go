package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// HTTPRateLimiter provides rate limiting functionality for HTTP requests
type HTTPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   RateLimitConfig
	logger   *logrus.Logger
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	DefaultRPS      float64
	DefaultBurst    int
	MaxCustomers    int
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultRPS:      100,           // 100 requests per second
		DefaultBurst:    200,           // Allow burst of 200 requests
		MaxCustomers:    10000,         // Max 10k customers in memory
		CleanupInterval: 1 * time.Hour, // Cleanup every hour
	}
}

// NewHTTPRateLimiter creates a new HTTP rate limiter
func NewHTTPRateLimiter(cfg RateLimitConfig, logger *logrus.Logger) *HTTPRateLimiter {
	rl := &HTTPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   cfg,
		logger:   logger,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given customer
func (rl *HTTPRateLimiter) Allow(customerID string) bool {
	rl.mu.RLock()
	limiter, exists := rl.limiters[customerID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Check if we need to evict old entries
		if len(rl.limiters) >= rl.config.MaxCustomers {
			rl.evictOldEntries()
		}

		limiter = rate.NewLimiter(rate.Limit(rl.config.DefaultRPS), rl.config.DefaultBurst)
		rl.limiters[customerID] = limiter
		rl.mu.Unlock()
	}

	return limiter.Allow()
}

// AllowN checks if N requests are allowed for the given customer
func (rl *HTTPRateLimiter) AllowN(customerID string, n int) bool {
	rl.mu.RLock()
	limiter, exists := rl.limiters[customerID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		if len(rl.limiters) >= rl.config.MaxCustomers {
			rl.evictOldEntries()
		}

		limiter = rate.NewLimiter(rate.Limit(rl.config.DefaultRPS), rl.config.DefaultBurst)
		rl.limiters[customerID] = limiter
		rl.mu.Unlock()
	}

	return limiter.AllowN(time.Now(), n)
}

// SetCustomerLimit sets a custom rate limit for a specific customer
func (rl *HTTPRateLimiter) SetCustomerLimit(customerID string, rps float64, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	rl.limiters[customerID] = limiter

	rl.logger.WithFields(logrus.Fields{
		"customer_id": customerID,
		"rps":         rps,
		"burst":       burst,
	}).Info("Set custom rate limit for customer")
}

// RemoveCustomer removes a customer's rate limiter
func (rl *HTTPRateLimiter) RemoveCustomer(customerID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.limiters, customerID)

	rl.logger.WithField("customer_id", customerID).Info("Removed customer rate limiter")
}

// GetCustomerStats returns rate limiting statistics for a customer
func (rl *HTTPRateLimiter) GetCustomerStats(customerID string) map[string]interface{} {
	rl.mu.RLock()
	limiter, exists := rl.limiters[customerID]
	rl.mu.RUnlock()

	if !exists {
		return map[string]interface{}{
			"customer_id": customerID,
			"exists":      false,
		}
	}

	return map[string]interface{}{
		"customer_id": customerID,
		"exists":      true,
		"limit":       float64(limiter.Limit()),
		"burst":       limiter.Burst(),
	}
}

// cleanup periodically removes old rate limiters to prevent memory leaks
func (rl *HTTPRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		rl.evictOldEntries()
		rl.mu.Unlock()

		rl.logger.WithField("active_limiters", len(rl.limiters)).Debug("Rate limiter cleanup completed")
	}
}

// evictOldEntries removes old entries when we reach the limit
func (rl *HTTPRateLimiter) evictOldEntries() {
	if len(rl.limiters) <= rl.config.MaxCustomers {
		return
	}

	// Simple eviction: remove 20% of entries
	toRemove := len(rl.limiters) / 5
	count := 0

	for customerID := range rl.limiters {
		delete(rl.limiters, customerID)
		count++
		if count >= toRemove {
			break
		}
	}

	rl.logger.WithFields(logrus.Fields{
		"removed_count": count,
		"remaining":     len(rl.limiters),
	}).Info("Evicted old rate limiters")
}

// HTTPRateLimitMiddleware creates HTTP middleware for rate limiting
func HTTPRateLimitMiddleware(rl *HTTPRateLimiter, logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract customer ID from request (from JWT token or header)
			customerID := extractCustomerID(r)
			if customerID == "" {
				// If no customer ID, use IP address as fallback
				customerID = r.RemoteAddr
			}

			// Check rate limit
			if !rl.Allow(customerID) {
				logger.WithFields(logrus.Fields{
					"customer_id": customerID,
					"ip":          r.RemoteAddr,
					"path":        r.URL.Path,
				}).Warn("Rate limit exceeded")

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rl.config.DefaultRPS))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
				w.WriteHeader(http.StatusTooManyRequests)

				response := map[string]interface{}{
					"error":       "Rate limit exceeded",
					"message":     "Too many requests, please try again later",
					"retry_after": 1,
				}

				json.NewEncoder(w).Encode(response)
				return
			}

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rl.config.DefaultRPS))
			w.Header().Set("X-RateLimit-Remaining", "1") // Simplified

			next.ServeHTTP(w, r)
		})
	}
}

// extractCustomerID extracts customer ID from request
func extractCustomerID(r *http.Request) string {
	// Try to get from header first
	if customerID := r.Header.Get("X-Customer-ID"); customerID != "" {
		return customerID
	}

	// Try to get from JWT token (if available)
	if claims := r.Context().Value("customer_claims"); claims != nil {
		if customerClaims, ok := claims.(map[string]interface{}); ok {
			if customerID, ok := customerClaims["customer_id"].(string); ok {
				return customerID
			}
		}
	}

	return ""
}
