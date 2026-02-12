package input

import (
	"testing"
)

func TestNormalizeSelectStringOptionsRejectsEmptyValue(t *testing.T) {
	t.Parallel()

	_, _, err := normalizeSelectStringOptions([]SelectStringOption{
		{Label: "mail", Value: "mail"},
		{Label: "empty", Value: "   "},
	})
	if err == nil {
		t.Fatal("expected error for empty option value")
	}
}

func TestNormalizeSelectStringOptionsRejectsDuplicateValues(t *testing.T) {
	t.Parallel()

	_, _, err := normalizeSelectStringOptions([]SelectStringOption{
		{Label: "mail", Value: "mail"},
		{Label: "mail duplicate", Value: " mail "},
	})
	if err == nil {
		t.Fatal("expected error for duplicate option values")
	}
}

func TestNormalizeSelectedValuesUsesOptionOrder(t *testing.T) {
	t.Parallel()

	options := []SelectStringOption{
		{Label: "mail", Value: "mail"},
		{Label: "calendar", Value: "calendar"},
		{Label: "contacts", Value: "contacts"},
	}
	selected := []string{"contacts", "mail", "mail", "unknown"}

	got := normalizeSelectedValues(options, selected)
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d: %#v", len(got), got)
	}
	if got[0] != "mail" || got[1] != "contacts" {
		t.Fatalf("unexpected value order: %#v", got)
	}
}
