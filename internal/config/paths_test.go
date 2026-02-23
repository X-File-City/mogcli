package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeExpandPathResolvesRelativePathsWithinBase(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	got, err := SafeExpandPath(base, "docs/report.txt")
	if err != nil {
		t.Fatalf("SafeExpandPath failed: %v", err)
	}

	want := filepath.Join(base, "docs", "report.txt")
	if got != want {
		t.Fatalf("unexpected resolved path: got %q want %q", got, want)
	}
}

func TestSafeExpandPathRejectsRelativeTraversal(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	_, err := SafeExpandPath(base, "../secrets.txt")
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if !strings.Contains(err.Error(), "path escapes base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSafeExpandPathAllowsAbsolutePath(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	abs := filepath.Join(base, "..", "outside.txt")

	got, err := SafeExpandPath(base, abs)
	if err != nil {
		t.Fatalf("SafeExpandPath failed for absolute path: %v", err)
	}

	want := filepath.Clean(abs)
	if got != want {
		t.Fatalf("unexpected cleaned absolute path: got %q want %q", got, want)
	}
}

func TestSafeExpandPathExpandsTildeAndValidatesBase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	base := filepath.Join(home, "workspace")
	got, err := SafeExpandPath(base, "~/notes.txt")
	if err != nil {
		t.Fatalf("SafeExpandPath failed for tilde path: %v", err)
	}

	want := filepath.Join(home, "notes.txt")
	if got != want {
		t.Fatalf("unexpected expanded tilde path: got %q want %q", got, want)
	}
}

func TestSafeExpandPathUsesPlatformHomeEnvForTilde(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	profile := filepath.Join(root, "profile")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", profile)

	got, err := SafeExpandPath(filepath.Join(root, "workspace"), "~/notes.txt")
	if err != nil {
		t.Fatalf("SafeExpandPath failed for tilde path: %v", err)
	}

	wantHome := home
	if runtime.GOOS == "windows" {
		wantHome = profile
	}
	want := filepath.Join(wantHome, "notes.txt")
	if got != want {
		t.Fatalf("unexpected expanded tilde path: got %q want %q", got, want)
	}
}
