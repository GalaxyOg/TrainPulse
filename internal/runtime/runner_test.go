package runtime

import (
	"testing"

	"github.com/trainpulse/trainpulse/internal/events"
)

func TestDetermineFinalEvent(t *testing.T) {
	if got := DetermineFinalEvent(0, false); got != events.Succeeded {
		t.Fatalf("expected SUCCEEDED, got %s", got)
	}
	if got := DetermineFinalEvent(1, false); got != events.Failed {
		t.Fatalf("expected FAILED, got %s", got)
	}
	if got := DetermineFinalEvent(130, true); got != events.Interrupted {
		t.Fatalf("expected INTERRUPTED, got %s", got)
	}
}

func TestNormalizeExitCode(t *testing.T) {
	if got := NormalizeExitCode(-2); got != 130 {
		t.Fatalf("expected 130, got %d", got)
	}
	if got := NormalizeExitCode(3); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}
