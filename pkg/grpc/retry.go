package grpc

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries    int
	Backoff       time.Duration
	MaxBackoff    time.Duration
	JitterEnabled bool
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		Backoff:       100 * time.Millisecond,
		MaxBackoff:    5 * time.Second,
		JitterEnabled: true,
	}
}

// RetryInterceptor creates a gRPC unary client interceptor with retry logic
func RetryInterceptor(cfg RetryConfig, logger *logrus.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var lastErr error

		for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				if attempt > 0 {
					logger.WithFields(logrus.Fields{
						"method":  method,
						"attempt": attempt + 1,
					}).Info("gRPC request succeeded after retry")
				}
				return nil
			}

			lastErr = err

			// Only retry on transient errors
			if !isRetryableError(err) {
				logger.WithFields(logrus.Fields{
					"method":  method,
					"attempt": attempt + 1,
					"error":   err.Error(),
				}).Debug("Non-retryable error, not retrying")
				return err
			}

			// Don't retry on last attempt
			if attempt == cfg.MaxRetries {
				break
			}

			// Calculate delay with exponential backoff
			delay := calculateBackoff(cfg, attempt)

			logger.WithFields(logrus.Fields{
				"method":  method,
				"attempt": attempt + 1,
				"delay":   delay,
				"error":   err.Error(),
			}).Info("Retrying gRPC request")

			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		logger.WithFields(logrus.Fields{
			"method":   method,
			"attempts": cfg.MaxRetries + 1,
			"error":    lastErr.Error(),
		}).Error("gRPC request failed after all retries")

		return lastErr
	}
}

// RetryServerInterceptor creates a gRPC unary server interceptor with retry logic for downstream calls
func RetryServerInterceptor(cfg RetryConfig, logger *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var lastErr error

		for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
			resp, err := handler(ctx, req)
			if err == nil {
				if attempt > 0 {
					logger.WithFields(logrus.Fields{
						"method":  info.FullMethod,
						"attempt": attempt + 1,
					}).Info("gRPC server request succeeded after retry")
				}
				return resp, nil
			}

			lastErr = err

			// Only retry on transient errors
			if !isRetryableError(err) {
				logger.WithFields(logrus.Fields{
					"method":  info.FullMethod,
					"attempt": attempt + 1,
					"error":   err.Error(),
				}).Debug("Non-retryable error, not retrying")
				return nil, err
			}

			// Don't retry on last attempt
			if attempt == cfg.MaxRetries {
				break
			}

			// Calculate delay with exponential backoff
			delay := calculateBackoff(cfg, attempt)

			logger.WithFields(logrus.Fields{
				"method":  info.FullMethod,
				"attempt": attempt + 1,
				"delay":   delay,
				"error":   err.Error(),
			}).Info("Retrying gRPC server request")

			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		logger.WithFields(logrus.Fields{
			"method":   info.FullMethod,
			"attempts": cfg.MaxRetries + 1,
			"error":    lastErr.Error(),
		}).Error("gRPC server request failed after all retries")

		return nil, lastErr
	}
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		// If it's not a gRPC status error, assume it's retryable (e.g., network errors)
		return true
	}

	// Retry on these gRPC status codes
	switch st.Code() {
	case codes.Unavailable, // Service unavailable
		codes.DeadlineExceeded,  // Request timeout
		codes.ResourceExhausted, // Resource exhausted (rate limiting)
		codes.Internal,          // Internal server error
		codes.DataLoss,          // Data loss
		codes.Aborted:           // Aborted
		return true
	default:
		return false
	}
}

// calculateBackoff calculates the backoff delay for retries
func calculateBackoff(cfg RetryConfig, attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	delay := cfg.Backoff * time.Duration(1<<uint(attempt))

	// Cap at max backoff
	if delay > cfg.MaxBackoff {
		delay = cfg.MaxBackoff
	}

	// Add jitter if enabled
	if cfg.JitterEnabled {
		jitter := time.Duration(float64(delay) * 0.1)      // ±10% jitter
		delay = delay + time.Duration(float64(jitter)*0.5) // Simplified jitter
	}

	return delay
}

// WithRetry creates a gRPC dial option with retry interceptor
func WithRetry(cfg RetryConfig, logger *logrus.Logger) grpc.DialOption {
	return grpc.WithUnaryInterceptor(RetryInterceptor(cfg, logger))
}

// WithRetryServer creates a gRPC server option with retry interceptor
func WithRetryServer(cfg RetryConfig, logger *logrus.Logger) grpc.ServerOption {
	return grpc.UnaryInterceptor(RetryServerInterceptor(cfg, logger))
}
