package cmd

import "testing"

func TestParserInit(t *testing.T) {
	_, _, err := newParser("debug")
	if err != nil {
		t.Fatalf("newParser failed: %v", err)
	}
}
