package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setTempConfigEnv(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestWriteAndReadConfigRoundTrip(t *testing.T) {
	setTempConfigEnv(t)

	input := File{
		ActiveProfile: "work",
		Profiles: map[string]ProfileRecord{
			"work": {
				Name:     "work",
				Audience: "enterprise",
				ClientID: "client-id",
			},
		},
	}

	if err := WriteConfig(input); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	got, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	if got.ActiveProfile != input.ActiveProfile {
		t.Fatalf("unexpected active profile: got %q want %q", got.ActiveProfile, input.ActiveProfile)
	}
	if _, ok := got.Profiles["work"]; !ok {
		t.Fatalf("expected profile \"work\" to exist")
	}
}

func TestWithConfigLockSerializesCallers(t *testing.T) {
	setTempConfigEnv(t)

	prevTimeout := configLockTimeout
	prevRetry := configLockRetryInterval
	configLockTimeout = 2 * time.Second
	configLockRetryInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		configLockTimeout = prevTimeout
		configLockRetryInterval = prevRetry
	})

	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)

	go func() {
		firstDone <- withConfigLock(func() error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started

	go func() {
		secondDone <- withConfigLock(func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("expected second caller to wait for lock release")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	if err := <-firstDone; err != nil {
		t.Fatalf("first lock holder failed: %v", err)
	}

	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second caller did not enter lock after release")
	}

	if err := <-secondDone; err != nil {
		t.Fatalf("second lock holder failed: %v", err)
	}
}

func TestWithConfigLockTimesOut(t *testing.T) {
	setTempConfigEnv(t)

	dir, err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	lockPath := filepath.Join(dir, "config.lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	prevTimeout := configLockTimeout
	prevRetry := configLockRetryInterval
	configLockTimeout = 50 * time.Millisecond
	configLockRetryInterval = 10 * time.Millisecond
	t.Cleanup(func() {
		configLockTimeout = prevTimeout
		configLockRetryInterval = prevRetry
	})

	err = withConfigLock(func() error { return nil })
	if err == nil {
		t.Fatal("expected lock timeout error")
	}
	if !strings.Contains(err.Error(), "acquire config lock timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}
