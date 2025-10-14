package crawler

import (
	"context"
	"net/url"
	"sync"
	"time"
)

// RateLimiter implements per-domain rate limiting with token bucket algorithm
type RateLimiter struct {
	limiters     map[string]*domainLimiter
	mu           sync.RWMutex
	defaultDelay time.Duration
}

// domainLimiter tracks rate limiting for a single domain
type domainLimiter struct {
	lastRequest time.Time
	mu          sync.Mutex
	delay       time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified default delay
func NewRateLimiter(defaultDelay time.Duration) *RateLimiter {
	return &RateLimiter{
		limiters:     make(map[string]*domainLimiter),
		defaultDelay: defaultDelay,
	}
}

// Wait blocks until the rate limit for the domain is satisfied (with context support)
func (rl *RateLimiter) Wait(ctx context.Context, rawURL string) error {
	domain := extractDomain(rawURL)
	if domain == "" {
		return nil // No domain, no rate limiting
	}

	// Get or create domain limiter
	rl.mu.Lock()
	limiter, exists := rl.limiters[domain]
	if !exists {
		limiter = &domainLimiter{
			delay: rl.defaultDelay,
		}
		rl.limiters[domain] = limiter
	}
	rl.mu.Unlock()

	// Wait for rate limit
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	nextAllowed := limiter.lastRequest.Add(limiter.delay)

	if now.Before(nextAllowed) {
		waitDuration := nextAllowed.Sub(now)

		// Wait with context cancellation support
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			// Wait completed
		}
	}

	limiter.lastRequest = time.Now()
	return nil
}

// SetDomainDelay sets a custom delay for a specific domain
func (rl *RateLimiter) SetDomainDelay(domain string, delay time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[domain]
	if !exists {
		limiter = &domainLimiter{
			delay: delay,
		}
		rl.limiters[domain] = limiter
	} else {
		limiter.mu.Lock()
		limiter.delay = delay
		limiter.mu.Unlock()
	}
}

// GetDomainDelay returns the current delay for a domain
func (rl *RateLimiter) GetDomainDelay(domain string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limiter, exists := rl.limiters[domain]
	if !exists {
		return rl.defaultDelay
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.delay
}

// extractDomain parses the domain from a URL
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}
