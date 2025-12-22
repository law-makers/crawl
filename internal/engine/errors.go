// internal/engine/errors.go
package engine

import (
	"errors"
	"fmt"
)

// Common engine errors
var (
	ErrBrowserNotFound = errors.New("chrome browser not found")
	ErrBrowserCrash    = errors.New("browser crashed")
	ErrTimeout         = errors.New("request timeout")
	ErrInvalidURL      = errors.New("invalid URL")
	ErrNetworkError    = errors.New("network error")
	ErrParseError      = errors.New("failed to parse response")
	ErrUnsupportedType = errors.New("unsupported content type")
	ErrSessionNotFound = errors.New("session not found")
	ErrCookieError     = errors.New("cookie processing error")
)

// ErrorCode represents a specific error condition
type ErrorCode string

const (
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeValidation   ErrorCode = "VALIDATION"
	ErrCodeBrowserCrash ErrorCode = "BROWSER_CRASH"
	ErrCodeNetworkError ErrorCode = "NETWORK_ERROR"
	ErrCodeParseError   ErrorCode = "PARSE_ERROR"
	ErrCodeSessionError ErrorCode = "SESSION_ERROR"
)

// EngineError wraps errors with additional context
type EngineError struct {
	Code       ErrorCode
	Message    string
	Underlying error
	Retry      bool
	Details    map[string]interface{}
}

// Error implements the error interface
func (e *EngineError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Underlying)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *EngineError) Unwrap() error {
	return e.Underlying
}

// Is checks if the error matches the target
func (e *EngineError) Is(target error) bool {
	if t, ok := target.(*EngineError); ok {
		return e.Code == t.Code
	}
	return errors.Is(e.Underlying, target)
}

// NewEngineError creates a new EngineError
func NewEngineError(code ErrorCode, message string, err error) *EngineError {
	return &EngineError{
		Code:       code,
		Message:    message,
		Underlying: err,
		Retry:      false,
		Details:    make(map[string]interface{}),
	}
}

// WithRetry marks the error as retryable
func (e *EngineError) WithRetry() *EngineError {
	e.Retry = true
	return e
}

// WithDetail adds a detail to the error
func (e *EngineError) WithDetail(key string, value interface{}) *EngineError {
	e.Details[key] = value
	return e
}
