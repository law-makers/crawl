// internal/ratelimit/limiter.go
package ratelimit

import (
	"context"
	"net/url"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter defines the interface for rate limiting implementations.
//
// Implementations should provide methods to control request rates,
// typically on a per-domain or per-host basis to avoid overwhelming servers.
type RateLimiter interface {
	// Wait blocks until a request for the given URL can proceed.
	// If the context is cancelled before the rate limit allows, an error is returned.
	Wait(ctx context.Context, urlStr string) error

	// Allow checks if a request for the given URL can proceed immediately
	// without blocking. Returns true if allowed, false otherwise.
	Allow(urlStr string) bool

	// Reserve reserves a token for the given URL.
	// Returns a Reservation that can be used to get the delay and allow/cancel.
	Reserve(urlStr string) *rate.Reservation
}

// DomainLimiter provides per-domain rate limiting to prevent overwhelming servers
// and avoid IP bans. It uses the token bucket algorithm for smooth rate limiting.
type DomainLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	perHost  rate.Limit // Requests per second per host
	burst    int        // Burst capacity
}

// NewDomainLimiter creates a new rate limiter with the specified per-host rate
func NewDomainLimiter(requestsPerSecond float64, burst int) *DomainLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 5.0 // Default: 5 requests/sec per domain
	}
	if burst <= 0 {
		burst = 10 // Default burst: 10 requests
	}

	return &DomainLimiter{
		limiters: make(map[string]*rate.Limiter),
		perHost:  rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

// Wait blocks until the request for the given URL can proceed according to rate limits
func (dl *DomainLimiter) Wait(ctx context.Context, urlStr string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	domain := extractDomain(urlStr)
	if domain == "" {
		// Invalid URL, let it proceed (will fail elsewhere)
		return nil
	}

	limiter := dl.getLimiter(domain)
	return limiter.Wait(ctx)
}

// Allow checks if a request can proceed immediately without blocking
func (dl *DomainLimiter) Allow(urlStr string) bool {
	domain := extractDomain(urlStr)
	if domain == "" {
		return true
	}

	limiter := dl.getLimiter(domain)
	return limiter.Allow()
}

// Reserve reserves a token for the given URL and returns a Reservation
func (dl *DomainLimiter) Reserve(urlStr string) *rate.Reservation {
	domain := extractDomain(urlStr)
	if domain == "" {
		// Return a reservation that allows immediate execution
		return &rate.Reservation{}
	}

	limiter := dl.getLimiter(domain)
	return limiter.Reserve()
}

// getLimiter returns or creates a rate limiter for the given domain
func (dl *DomainLimiter) getLimiter(domain string) *rate.Limiter {
	dl.mu.RLock()
	limiter, exists := dl.limiters[domain]
	dl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	dl.mu.Lock()
	defer dl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := dl.limiters[domain]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(dl.perHost, dl.burst)
	dl.limiters[domain] = limiter

	return limiter
}

// SetLimit updates the rate limit for a specific domain
func (dl *DomainLimiter) SetLimit(domain string, requestsPerSecond float64, burst int) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if limiter, exists := dl.limiters[domain]; exists {
		limiter.SetLimit(rate.Limit(requestsPerSecond))
		limiter.SetBurst(burst)
	} else {
		dl.limiters[domain] = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
	}
}

// extractDomain extracts the domain from a URL string
func extractDomain(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}
