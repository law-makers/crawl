// internal/retry/retry.go
package retry

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Config defines retry behavior with exponential backoff
type Config struct {
	MaxAttempts          int           // Maximum number of retry attempts
	InitialBackoff       time.Duration // Initial backoff duration
	MaxBackoff           time.Duration // Maximum backoff duration
	Multiplier           float64       // Backoff multiplier
	RetryableStatusCodes []int         // HTTP status codes that should trigger retry
}

// DefaultConfig returns a sensible default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// WithRetry executes the given function with retry logic
func WithRetry(ctx context.Context, cfg Config, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()

		// Success
		if err == nil {
			if attempt > 0 {
				log.Debug().
					Int("attempts", attempt+1).
					Msg("Retry succeeded")
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !shouldRetry(err, cfg) {
			log.Debug().
				Err(err).
				Msg("Error is not retryable")
			return err
		}

		// Don't sleep after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, cfg)

			log.Debug().
				Int("attempt", attempt+1).
				Int("max_attempts", cfg.MaxAttempts).
				Dur("backoff", backoff).
				Err(err).
				Msg("Retrying after backoff")

			// Wait for backoff duration or context cancellation
			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	log.Warn().
		Int("attempts", cfg.MaxAttempts).
		Err(lastErr).
		Msg("Max retry attempts exceeded")

	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// calculateBackoff calculates the backoff duration for the given attempt
func calculateBackoff(attempt int, cfg Config) time.Duration {
	// Exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.Multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	return time.Duration(backoff)
}

// shouldRetry determines if an error is retryable
func shouldRetry(err error, cfg Config) bool {
	if err == nil {
		return false
	}

	// Check for errors implementing StatusCoder (like HTTPError or DownloadError)
	if sc, ok := err.(StatusCoder); ok {
		statusCode := sc.GetStatusCode()
		for _, code := range cfg.RetryableStatusCodes {
			if statusCode == code {
				return true
			}
		}
		return false
	}

	// Check for HTTP status codes (legacy check)
	if httpErr, ok := err.(HTTPError); ok {
		for _, code := range cfg.RetryableStatusCodes {
			if httpErr.StatusCode == code {
				return true
			}
		}
		return false
	}

	// Check for timeout errors (always retryable)
	if isTimeoutError(err) {
		return true
	}

	// Check for temporary errors
	if tempErr, ok := err.(interface{ Temporary() bool }); ok {
		return tempErr.Temporary()
	}

	// Default: retry
	return true
}

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return true
	}

	// Check for timeout interface
	if timeoutErr, ok := err.(interface{ Timeout() bool }); ok {
		return timeoutErr.Timeout()
	}

	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Status     string
	Message    string
}

// StatusCoder is an interface for errors that provide an HTTP status code
type StatusCoder interface {
	GetStatusCode() int
}

func (e HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Message)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

func (e HTTPError) GetStatusCode() int {
	return e.StatusCode
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(statusCode int, status string, message string) HTTPError {
	return HTTPError{
		StatusCode: statusCode,
		Status:     status,
		Message:    message,
	}
}
