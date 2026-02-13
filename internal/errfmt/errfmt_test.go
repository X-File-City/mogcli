package errfmt

import (
	"errors"
	"strings"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/graph"
	"github.com/jaredpalmer/mogcli/internal/profile"
)

func TestFormatAADSTS(t *testing.T) {
	msg := Format(errors.New("oauth failed: AADSTS700016 unexpected"))
	if !strings.HasPrefix(msg, "Error: ") {
		t.Fatalf("expected Error: prefix, got: %s", msg)
	}
	if !strings.Contains(msg, "AADSTS700016") {
		t.Fatalf("expected AADSTS code in message, got: %s", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "application") {
		t.Fatalf("expected actionable guidance, got: %s", msg)
	}
}

func TestFormatProfileErrors(t *testing.T) {
	notFound := Format(profile.ErrProfileNotFound)
	if !strings.Contains(notFound, "Profile not found") {
		t.Fatalf("unexpected profile-not-found message: %s", notFound)
	}

	noActive := Format(profile.ErrNoActiveProfile)
	if !strings.Contains(noActive, "No active profile") {
		t.Fatalf("unexpected no-active-profile message: %s", noActive)
	}
}

func TestFormatCircuitBreakerError(t *testing.T) {
	msg := Format(&graph.CircuitBreakerError{})
	if !strings.Contains(strings.ToLower(msg), "temporarily paused") {
		t.Fatalf("unexpected circuit-breaker message: %s", msg)
	}
}
