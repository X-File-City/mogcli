package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetGetDeleteSecret(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MOG_KEYRING_BACKEND", "file")

	key := "mog:test:key"
	value := []byte("hello-world")

	if err := SetSecret(key, value); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	got, err := GetSecret(key)
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("unexpected secret value: %q", string(got))
	}

	if err := DeleteSecret(key); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}

	if _, err := GetSecret(key); err == nil {
		t.Fatal("expected error after deleting secret")
	}

	// Verify path is inside keyring dir.
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "mogcli", "keyring")); statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("stat keyring dir failed: %v", statErr)
	}
}
