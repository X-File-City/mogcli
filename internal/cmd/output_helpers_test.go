package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/ui"
)

func TestPrintNextPageHint(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	printer := ui.New(ui.Options{Stderr: &stderr})
	token := "https://graph.microsoft.com/v1.0/me/messages?$skiptoken=abc"

	printNextPageHint(printer, token)

	got := strings.TrimSpace(stderr.String())
	want := "# Next page: --page 'https://graph.microsoft.com/v1.0/me/messages?$skiptoken=abc'"
	if got != want {
		t.Fatalf("unexpected hint output:\n got: %q\nwant: %q", got, want)
	}
}

func TestPrintNextPageHintSkipsEmptyToken(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	printer := ui.New(ui.Options{Stderr: &stderr})

	printNextPageHint(printer, "")

	if stderr.Len() != 0 {
		t.Fatalf("expected no output for empty token, got %q", stderr.String())
	}
}

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	t.Parallel()

	got := shellQuote("a'b")
	if got != "'a'\\''b'" {
		t.Fatalf("unexpected shell-quoted value: %q", got)
	}
}
