package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewUsesProcessDefaultWriters(t *testing.T) {
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

	u := New(Options{})
	u.Out().Println("default out")
	u.Err().Error("default err")

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

	if got := string(outBytes); got != "default out\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if got := string(errBytes); got != "default err\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestPrinterMethodsEmitPlainText(t *testing.T) {
	var out bytes.Buffer
	u := New(Options{Stdout: &out})

	u.Out().Printf("alpha %d", 1)
	u.Out().Println("beta")
	u.Out().Successf("gamma %s", "ok")
	u.Out().Error("delta")
	u.Out().Errorf("epsilon %d", 2)
	u.Out().Print("zeta")

	got := out.String()
	want := "alpha 1\nbeta\ngamma ok\ndelta\nepsilon 2\nzeta"
	if got != want {
		t.Fatalf("unexpected printer output:\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("unexpected ANSI escape sequence in output: %q", got)
	}
}
