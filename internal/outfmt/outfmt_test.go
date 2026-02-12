package outfmt

import "testing"

func TestFromFlags(t *testing.T) {
	if _, err := FromFlags(true, true); err == nil {
		t.Fatal("expected error when combining --json and --plain")
	}
	mode, err := FromFlags(true, false)
	if err != nil {
		t.Fatalf("FromFlags failed: %v", err)
	}
	if !mode.JSON || mode.Plain {
		t.Fatalf("unexpected mode: %+v", mode)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("MOG_JSON", "true")
	t.Setenv("MOG_PLAIN", "0")
	mode := FromEnv()
	if !mode.JSON {
		t.Fatal("expected JSON=true from env")
	}
	if mode.Plain {
		t.Fatal("expected Plain=false from env")
	}
}
