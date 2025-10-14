package crawler

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/ternarybob/arbor"
)

// RetryPolicy defines retry behavior with exponential backoff
type RetryPolicy struct {
	MaxAttempts          int
	InitialBackoff       time.Duration
	MaxBackoff           time.Duration
	BackoffMultiplier    float64
	RetryableStatusCodes []int
	RetryableErrors      []error
}

// NewRetryPolicy creates a default retry policy
func NewRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableStatusCodes: []int{
			408, // Request Timeout
			429, // Too Many Requests
			500, // Internal Server Error
			502, // Bad Gateway
			503, // Service Unavailable
			504, // Gateway Timeout
		},
		RetryableErrors: []error{
			context.DeadlineExceeded,
		},
	}
}

// ShouldRetry checks if an attempt should be retried based on attempt count, status code, and error type
func (p *RetryPolicy) ShouldRetry(attempt int, statusCode int, err error) bool {
	if attempt >= p.MaxAttempts {
		return false
	}

	// Check status code
	if statusCode > 0 {
		for _, code := range p.RetryableStatusCodes {
			if statusCode == code {
				return true
			}
		}
		// Non-retryable status code
		if statusCode >= 400 && statusCode < 500 && statusCode != 408 && statusCode != 429 {
			return false // Client errors (except timeout/rate limit) are not retryable
		}
	}

	// Check error
	if err != nil {
		return isRetryableError(err)
	}

	return false
}

// CalculateBackoff calculates the backoff duration with exponential backoff and jitter
func (p *RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	backoff := float64(p.InitialBackoff) * pow(p.BackoffMultiplier, float64(attempt))
	if backoff > float64(p.MaxBackoff) {
		backoff = float64(p.MaxBackoff)
	}

	// Add jitter (±25%)
	jitter := backoff * 0.25 * (rand.Float64()*2 - 1)
	backoff += jitter

	if backoff < 0 {
		backoff = float64(p.InitialBackoff)
	}

	return time.Duration(backoff)
}

// ExecuteWithRetry wraps a function with retry loop
func (p *RetryPolicy) ExecuteWithRetry(ctx context.Context, logger arbor.ILogger, fn func() (int, error)) (int, error) {
	var lastErr error
	var statusCode int

	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		statusCode, lastErr = fn()

		if lastErr == nil && !p.isRetryableStatusCode(statusCode) {
			return statusCode, nil // Success
		}

		if !p.ShouldRetry(attempt, statusCode, lastErr) {
			if lastErr != nil {
				logger.Debug().
					Int("attempt", attempt+1).
					Int("status_code", statusCode).
					Err(lastErr).
					Msg("Non-retryable error, failing immediately")
			}
			return statusCode, lastErr
		}

		if attempt < p.MaxAttempts-1 {
			backoff := p.CalculateBackoff(attempt)
			logger.Debug().
				Int("attempt", attempt+1).
				Int("status_code", statusCode).
				Err(lastErr).
				Dur("backoff", backoff).
				Msg("Retrying after backoff")

			select {
			case <-ctx.Done():
				return statusCode, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	logger.Warn().
		Int("max_attempts", p.MaxAttempts).
		Int("status_code", statusCode).
		Err(lastErr).
		Msg("All retry attempts exhausted")

	return statusCode, lastErr
}

// isRetryableStatusCode checks if a status code is retryable
func (p *RetryPolicy) isRetryableStatusCode(statusCode int) bool {
	for _, code := range p.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// isRetryableError checks if an error is retryable (timeouts, connection errors, context deadline exceeded)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Temporary network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Connection errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	return false
}

// pow calculates base^exp for float64
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
