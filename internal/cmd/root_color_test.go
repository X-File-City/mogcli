package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestHelpDoesNotExposeColorFlag(t *testing.T) {
	stdout, _, err := captureExecuteOutput(t, []string{"--help"})
	if err != nil {
		t.Fatalf("Execute(--help) failed: %v", err)
	}

	if strings.Contains(stdout, "--color") {
		t.Fatalf("help output must not contain --color:\n%s", stdout)
	}
}

func TestRemovedColorFlagIsUnknownUsageError(t *testing.T) {
	_, stderr, err := captureExecuteOutput(t, []string{"--color", "always", "version"})
	if err == nil {
		t.Fatal("expected usage error for removed --color flag")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("expected usage exit code 2, got %d (err=%v)", code, err)
	}
	if !strings.Contains(stderr, "unknown flag --color") {
		t.Fatalf("expected unknown-flag error for --color, got:\n%s", stderr)
	}
}

func TestMOGColorEnvDoesNotAffectVersionExecution(t *testing.T) {
	t.Setenv("MOG_COLOR", "always")

	stdout, stderr, err := captureExecuteOutput(t, []string{"version"})
	if err != nil {
		t.Fatalf("Execute(version) failed: %v\nstderr:\n%s", err, stderr)
	}
	if got, want := strings.TrimSpace(stdout), VersionString(); got != want {
		t.Fatalf("unexpected version output: got %q want %q", got, want)
	}
}

func captureExecuteOutput(t *testing.T, args []string) (stdout string, stderr string, runErr error) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	runErr = Execute(args)

	if err := wOut.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	outBytes, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	if err := rOut.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := rErr.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return string(outBytes), string(errBytes), runErr
}
