package logger

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
)

type contextKey struct{}

// NewRequestID generates a random 64-bit hex string for correlation logging.
func NewRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

// WithRequestID stores a request ID in ctx so it flows through the call chain.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// RequestIDFrom retrieves the request ID stored by WithRequestID.
// Returns an empty string if none is set.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}

// From returns base augmented with request_id (empty string if not in ctx).
// The field is always present so log consumers can reliably filter/group on it.
func From(ctx context.Context, base *slog.Logger) *slog.Logger {
	return base.With(slog.String("request_id", RequestIDFrom(ctx)))
}
