package quiet

import (
	"context"
	"testing"
)

func TestEnableQuiet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Initially quiet should be disabled
	if IsQuiet(ctx) {
		t.Error("Expected quiet to be disabled initially")
	}

	// Enable quiet mode
	ctx = EnableQuiet(ctx)

	if !IsQuiet(ctx) {
		t.Error("Expected quiet to be enabled after EnableQuiet")
	}
}

func TestIsQuietWithNilContext(t *testing.T) {
	t.Parallel()
	// Should not panic with background context
	ctx := context.Background()
	if IsQuiet(ctx) {
		t.Error("Expected quiet to be disabled for background context")
	}
}
