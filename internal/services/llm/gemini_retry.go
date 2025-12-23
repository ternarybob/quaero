package llm

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GeminiRetryConfig defines retry behavior for Gemini API rate limit handling.
// Configured for Gemini's 1,000,000 token/minute quota window.
type GeminiRetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 5)
	MaxRetries int

	// InitialBackoff is the initial wait time before first retry (default: 45s)
	// This matches Gemini's quota window reset time.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum wait time between retries (default: 90s)
	MaxBackoff time.Duration

	// BackoffMultiplier is applied to backoff on each retry (default: 1.5)
	BackoffMultiplier float64
}

// Default retry constants for Gemini API rate limiting.
// Based on observed quota window of ~60 seconds.
const (
	DefaultMaxRetries        = 5
	DefaultInitialBackoff    = 45 * time.Second
	DefaultMaxBackoff        = 90 * time.Second
	DefaultBackoffMultiplier = 1.5
)

// NewDefaultRetryConfig returns a GeminiRetryConfig with sensible defaults
// for handling Gemini API rate limits.
func NewDefaultRetryConfig() *GeminiRetryConfig {
	return &GeminiRetryConfig{
		MaxRetries:        DefaultMaxRetries,
		InitialBackoff:    DefaultInitialBackoff,
		MaxBackoff:        DefaultMaxBackoff,
		BackoffMultiplier: DefaultBackoffMultiplier,
	}
}

// IsRateLimitError checks if an error is a Gemini rate limit error.
// Matches 429 status codes and RESOURCE_EXHAUSTED errors.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "RESOURCE_EXHAUSTED") ||
		strings.Contains(errStr, "quota")
}

// retryDelayRegex matches "Please retry in Xs" or "retryDelay:Xs" patterns
var retryDelayRegex = regexp.MustCompile(`(?i)(?:Please retry in |retryDelay[:\s]+)(\d+(?:\.\d+)?)\s*s`)

// ExtractRetryDelay parses the API-suggested retry delay from a Gemini error.
// Returns 0 if no delay is found in the error message.
//
// Example error message:
// "Error 429, Message: ... Please retry in 45.387061394s., Status: RESOURCE_EXHAUSTED"
func ExtractRetryDelay(err error) time.Duration {
	if err == nil {
		return 0
	}

	matches := retryDelayRegex.FindStringSubmatch(err.Error())
	if len(matches) < 2 {
		return 0
	}

	seconds, parseErr := strconv.ParseFloat(matches[1], 64)
	if parseErr != nil {
		return 0
	}

	return time.Duration(seconds * float64(time.Second))
}

// CalculateBackoff computes the backoff duration for a given attempt.
// If apiDelay > 0 (from ExtractRetryDelay), it's used as the base.
// Otherwise, InitialBackoff is used.
// The result is capped at MaxBackoff.
func (c *GeminiRetryConfig) CalculateBackoff(attempt int, apiDelay time.Duration) time.Duration {
	base := c.InitialBackoff
	if apiDelay > 0 {
		// Use API-provided delay plus small buffer
		base = apiDelay + 5*time.Second
	}

	// Apply exponential multiplier
	multiplier := 1.0
	for i := 0; i < attempt; i++ {
		multiplier *= c.BackoffMultiplier
	}

	backoff := time.Duration(float64(base) * multiplier)
	if backoff > c.MaxBackoff {
		backoff = c.MaxBackoff
	}

	return backoff
}
