package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/config"
	"github.com/jaredpalmer/mogcli/internal/outfmt"
	"github.com/jaredpalmer/mogcli/internal/services/onedrive"
)

type OneDriveCmd struct {
	LS    OneDriveListCmd  `cmd:"" name:"ls" help:"List files and folders"`
	Get   OneDriveGetCmd   `cmd:"" help:"Download file contents"`
	Put   OneDrivePutCmd   `cmd:"" help:"Upload file contents"`
	Mkdir OneDriveMkdirCmd `cmd:"" help:"Create folder"`
	RM    OneDriveRmCmd    `cmd:"" name:"rm" help:"Delete item"`
}

type OneDriveListCmd struct {
	Path string `name:"path" default:"/" help:"Remote path"`
	Max  int    `name:"max" default:"100" help:"Maximum items to return"`
	Page string `name:"page" aliases:"next-token" help:"Resume from next page token"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *OneDriveListCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capOneDriveLS)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}
	page, err := normalizePageToken(c.Page)
	if err != nil {
		return err
	}

	svc := onedrive.New(rt.Graph, targetUser)
	items, next, err := svc.List(ctx, c.Path, c.Max, page)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"items": items, "next": next})
	}

	printItemTable(ctx, items, []string{"name", "id", "size", "lastModifiedDateTime"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

type OneDriveGetCmd struct {
	Path string `arg:"" required:"" help:"Remote file path"`
	Out  string `name:"out" aliases:"output" help:"Output file path"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *OneDriveGetCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capOneDriveGet)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	svc := onedrive.New(rt.Graph, targetUser)
	content, err := svc.Get(ctx, c.Path)
	if err != nil {
		return err
	}
	expanded, err := resolveOneDriveOutPath(c.Path, c.Out)
	if err != nil {
		return err
	}
	if expanded == "" {
		return usage("output path is required")
	}

	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writeLocalFileWithProgress(ctx, expanded, content); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"path": expanded, "bytes": len(content)})
	}

	fmt.Fprintf(os.Stdout, "Saved %d bytes to %s\n", len(content), expanded)
	return nil
}

type OneDrivePutCmd struct {
	LocalPath  string `arg:"" required:"" help:"Local file path"`
	RemotePath string `name:"path" required:"" help:"Remote destination path"`
	User       string `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun     bool   `name:"dry-run" help:"Preview upload without modifying OneDrive"`
}

func (c *OneDrivePutCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capOneDrivePut)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	localPath, err := resolveOneDrivePutLocalPath(c.LocalPath)
	if err != nil {
		return err
	}
	if localPath == "" {
		return usage("local file path is required")
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}
	size := info.Size()

	if c.DryRun {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{
				"dry_run":     true,
				"action":      "onedrive.put",
				"local_path":  localPath,
				"remote_path": c.RemotePath,
				"bytes":       size,
			})
		}
		fmt.Fprintf(os.Stdout, "Dry run: would upload %s (%d bytes) -> %s\n", localPath, size, c.RemotePath)
		return nil
	}

	content, err := readLocalFileWithProgress(ctx, localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	svc := onedrive.New(rt.Graph, targetUser)
	if err := svc.Put(ctx, c.RemotePath, content); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"uploaded": c.RemotePath, "bytes": len(content)})
	}

	fmt.Fprintf(os.Stdout, "Uploaded %s (%d bytes) -> %s\n", localPath, len(content), c.RemotePath)
	return nil
}

type OneDriveMkdirCmd struct {
	Path   string `name:"path" required:"" help:"Remote folder path"`
	User   string `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun bool   `name:"dry-run" help:"Preview create without modifying OneDrive"`
}

func (c *OneDriveMkdirCmd) Run(ctx context.Context) error {
	rt, err := resolveRuntime(ctx, capOneDriveMkdir)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}
	if c.DryRun {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{
				"dry_run": true,
				"action":  "onedrive.mkdir",
				"path":    c.Path,
			})
		}
		fmt.Fprintf(os.Stdout, "Dry run: would create folder %s\n", c.Path)
		return nil
	}

	svc := onedrive.New(rt.Graph, targetUser)
	if err := svc.Mkdir(ctx, c.Path); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"created": c.Path})
	}

	fmt.Fprintf(os.Stdout, "Created %s\n", c.Path)
	return nil
}

type OneDriveRmCmd struct {
	Path   string `name:"path" required:"" help:"Remote path to delete"`
	User   string `name:"user" help:"App-only target user override (UPN or user ID)"`
	DryRun bool   `name:"dry-run" help:"Preview delete without modifying OneDrive"`
}

func (c *OneDriveRmCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}

	rt, err := resolveRuntime(ctx, capOneDriveRM)
	if err != nil {
		return err
	}
	targetUser, err := resolveAppOnlyTargetUser(rt.Profile, c.User)
	if err != nil {
		return err
	}

	svc := onedrive.New(rt.Graph, targetUser)
	if c.DryRun {
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{
				"dry_run": true,
				"action":  "onedrive.rm",
				"path":    c.Path,
			})
		}
		fmt.Fprintf(os.Stdout, "Dry run: would delete %s\n", c.Path)
		return nil
	}

	action := fmt.Sprintf("delete %s", c.Path)
	if metadata, statErr := svc.Stat(ctx, c.Path); statErr == nil {
		action = describeOneDriveDeleteAction(c.Path, metadata)
	}
	if err := confirmDestructive(ctx, flags, action); err != nil {
		return err
	}

	if err := svc.Remove(ctx, c.Path); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": c.Path})
	}

	fmt.Fprintf(os.Stdout, "Deleted %s\n", c.Path)
	return nil
}

func resolveOneDrivePutLocalPath(raw string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	return config.SafeExpandPath(cwd, strings.TrimSpace(raw))
}

func resolveOneDriveOutPath(remotePath string, outArg string) (string, error) {
	outPath := strings.TrimSpace(outArg)
	if outPath == "" {
		downloadDir, err := config.EnsureOneDriveDownloadsDir()
		if err != nil {
			return "", err
		}

		fileName := filepath.Base(strings.TrimSpace(remotePath))
		if fileName == "." || fileName == "/" || fileName == "" {
			fileName = "download.bin"
		}

		return config.SafeExpandPath(downloadDir, fileName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}

	return config.SafeExpandPath(cwd, outPath)
}

func readLocalFileWithProgress(ctx context.Context, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var content bytes.Buffer
	if _, err := copyWithProgress(ctx, &content, file, info.Size(), "Reading"); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

func writeLocalFileWithProgress(ctx context.Context, path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = copyWithProgress(ctx, file, bytes.NewReader(content), int64(len(content)), "Writing")
	return err
}

func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, label string) (int64, error) {
	const reportEvery = int64(1 << 20) // 1 MiB

	buffer := make([]byte, 32*1024)
	var copied int64
	nextReport := reportEvery

	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			written, writeErr := dst.Write(buffer[:n])
			if writeErr != nil {
				return copied, writeErr
			}
			if written != n {
				return copied, io.ErrShortWrite
			}

			copied += int64(written)
			if copied >= nextReport || (total > 0 && copied == total) {
				reportOneDriveProgress(ctx, label, copied, total)
				for copied >= nextReport {
					nextReport += reportEvery
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return copied, readErr
		}
	}

	if copied == 0 {
		reportOneDriveProgress(ctx, label, copied, total)
	}

	return copied, nil
}

func reportOneDriveProgress(ctx context.Context, label string, copied int64, total int64) {
	u := uiFromContext(ctx)
	if u == nil {
		return
	}

	if total > 0 {
		percent := float64(copied) / float64(total) * 100
		u.Err().Printf("%s: %d/%d bytes (%.0f%%)", label, copied, total, percent)
		return
	}

	u.Err().Printf("%s: %d bytes", label, copied)
}

func describeOneDriveDeleteAction(path string, metadata map[string]any) string {
	action := fmt.Sprintf("delete %s", strings.TrimSpace(path))
	if len(metadata) == 0 {
		return action
	}

	details := make([]string, 0, 4)
	if _, ok := metadata["folder"].(map[string]any); ok {
		details = append(details, "folder", "recursive")
	} else {
		details = append(details, "file")
	}

	if size, ok := oneDriveInt64(metadata["size"]); ok {
		details = append(details, fmt.Sprintf("%d bytes", size))
	}
	if modified, ok := metadata["lastModifiedDateTime"].(string); ok && strings.TrimSpace(modified) != "" {
		details = append(details, "modified "+strings.TrimSpace(modified))
	}

	if len(details) == 0 {
		return action
	}

	return fmt.Sprintf("%s (%s)", action, strings.Join(details, ", "))
}

func oneDriveInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}
