package reqctx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type key int

const requestKey key = 0

type RequestContext struct {
	RequestID string
	StartTime time.Time
}

func WithRequestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestKey, &RequestContext{
		RequestID: generateID(),
		StartTime: time.Now(),
	})
}

func GetRequestContext(ctx context.Context) *RequestContext {
	if rc, ok := ctx.Value(requestKey).(*RequestContext); ok {
		return rc
	}
	return &RequestContext{
		RequestID: "unknown",
		StartTime: time.Now(),
	}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestError wraps an error with request context
type RequestError struct {
	RequestID string
	Err       error
}

// Error implements the error interface
func (e *RequestError) Error() string {
	return fmt.Sprintf("[%s] %v", e.RequestID, e.Err)
}

// Unwrap returns the underlying error
func (e *RequestError) Unwrap() error {
	return e.Err
}

// NewRequestError creates a new RequestError from context
func NewRequestError(ctx context.Context, err error) error {
	rc := GetRequestContext(ctx)
	return &RequestError{
		RequestID: rc.RequestID,
		Err:       err,
	}
}
