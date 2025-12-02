package quiet

import "context"

// contextKey is a private type for context keys
type contextKey int

const (
	quietEnabledKey contextKey = iota
)

// EnableQuiet returns a context with quiet mode enabled
func EnableQuiet(ctx context.Context) context.Context {
	return context.WithValue(ctx, quietEnabledKey, true)
}

// IsQuiet checks if quiet mode is enabled in the context
func IsQuiet(ctx context.Context) bool {
	enabled, ok := ctx.Value(quietEnabledKey).(bool)
	return ok && enabled
}
