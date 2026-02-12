package errfmt

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatAADSTS(t *testing.T) {
	msg := Format(errors.New("oauth failed: AADSTS700016 unexpected"))
	if !strings.Contains(msg, "AADSTS700016") {
		t.Fatalf("expected AADSTS code in message, got: %s", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "application") {
		t.Fatalf("expected actionable guidance, got: %s", msg)
	}
}
