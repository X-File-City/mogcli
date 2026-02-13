package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOneDrivePutLocalPathRejectsTraversal(t *testing.T) {
	t.Parallel()

	_, err := resolveOneDrivePutLocalPath("../secrets.txt")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
	if !strings.Contains(err.Error(), "path escapes base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveOneDrivePutLocalPathResolvesRelativePath(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}

	got, err := resolveOneDrivePutLocalPath("./fixtures/report.txt")
	if err != nil {
		t.Fatalf("resolveOneDrivePutLocalPath failed: %v", err)
	}

	want := filepath.Join(cwd, "fixtures", "report.txt")
	if got != want {
		t.Fatalf("unexpected resolved path: got %q want %q", got, want)
	}
}

func TestResolveOneDriveOutPathDefaultsToDownloadsDir(t *testing.T) {
	setTempUserConfigEnv(t)

	got, err := resolveOneDriveOutPath("/Reports/report.pdf", "")
	if err != nil {
		t.Fatalf("resolveOneDriveOutPath failed: %v", err)
	}

	if !strings.HasSuffix(got, filepath.Join("mogcli", "onedrive-downloads", "report.pdf")) {
		t.Fatalf("expected download path under mogcli downloads dir, got %q", got)
	}
}

func TestResolveOneDriveOutPathRejectsTraversal(t *testing.T) {
	t.Parallel()

	_, err := resolveOneDriveOutPath("/Reports/report.pdf", "../outside.txt")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
	if !strings.Contains(err.Error(), "path escapes base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDescribeOneDriveDeleteActionIncludesMetadata(t *testing.T) {
	t.Parallel()

	action := describeOneDriveDeleteAction("/Reports", map[string]any{
		"folder":               map[string]any{},
		"size":                 float64(2048),
		"lastModifiedDateTime": "2026-02-13T12:00:00Z",
	})
	if !strings.Contains(action, "folder") {
		t.Fatalf("expected folder detail, got %q", action)
	}
	if !strings.Contains(action, "recursive") {
		t.Fatalf("expected recursive warning, got %q", action)
	}
	if !strings.Contains(action, "2048 bytes") {
		t.Fatalf("expected size detail, got %q", action)
	}
	if !strings.Contains(action, "modified 2026-02-13T12:00:00Z") {
		t.Fatalf("expected modified timestamp, got %q", action)
	}
}

func TestCopyWithProgressCopiesPayload(t *testing.T) {
	t.Parallel()

	payload := "hello world"
	var dst bytes.Buffer

	n, err := copyWithProgress(context.Background(), &dst, strings.NewReader(payload), int64(len(payload)), "Writing")
	if err != nil {
		t.Fatalf("copyWithProgress failed: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("unexpected byte count: got %d want %d", n, len(payload))
	}
	if dst.String() != payload {
		t.Fatalf("unexpected copied payload: got %q want %q", dst.String(), payload)
	}
}
