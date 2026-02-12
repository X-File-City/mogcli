package graph

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	d := retryDelay("2", 0, time.Second)
	if d != 2*time.Second {
		t.Fatalf("expected retry-after delay, got %v", d)
	}

	d2 := retryDelay("invalid", 1, time.Second)
	if d2 < 2*time.Second || d2 > 3*time.Second {
		t.Fatalf("expected jittered backoff near 2-3s, got %v", d2)
	}
}
