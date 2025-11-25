package logging

import "context"

// contextKey is a private type for context keys
type contextKey int

const (
	loggingEnabledKey contextKey = iota
)

// EnableLogging returns a context with logging enabled
func EnableLogging(ctx context.Context) context.Context {
	return context.WithValue(ctx, loggingEnabledKey, true)
}

// IsLoggingEnabled checks if logging is enabled in the context
func IsLoggingEnabled(ctx context.Context) bool {
	enabled, ok := ctx.Value(loggingEnabledKey).(bool)
	return ok && enabled
}
