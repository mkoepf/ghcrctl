package quiet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnableQuiet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Initially quiet should be disabled
	assert.False(t, IsQuiet(ctx), "Expected quiet to be disabled initially")

	// Enable quiet mode
	ctx = EnableQuiet(ctx)

	assert.True(t, IsQuiet(ctx), "Expected quiet to be enabled after EnableQuiet")
}

func TestIsQuietWithNilContext(t *testing.T) {
	t.Parallel()
	// Should not panic with background context
	ctx := context.Background()
	assert.False(t, IsQuiet(ctx), "Expected quiet to be disabled for background context")
}
