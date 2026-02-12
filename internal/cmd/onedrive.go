package cmd

import (
	"context"
	"fmt"
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

	outPath := strings.TrimSpace(c.Out)
	if outPath == "" {
		outPath = filepath.Base(strings.TrimSpace(c.Path))
		if outPath == "." || outPath == "/" || outPath == "" {
			dir, dirErr := config.OneDriveDownloadsDir()
			if dirErr != nil {
				return dirErr
			}
			outPath = filepath.Join(dir, "download.bin")
		}
	}

	expanded, err := config.ExpandPath(outPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(expanded, content, 0o600); err != nil {
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

	localPath, err := config.ExpandPath(strings.TrimSpace(c.LocalPath))
	if err != nil {
		return err
	}

	content, err := os.ReadFile(localPath) //nolint:gosec // user-provided file path
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
	Path string `name:"path" required:"" help:"Remote folder path"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
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
	Path string `name:"path" required:"" help:"Remote path to delete"`
	User string `name:"user" help:"App-only target user override (UPN or user ID)"`
}

func (c *OneDriveRmCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags == nil {
		flags = &RootFlags{}
	}
	if err := confirmDestructive(ctx, flags, fmt.Sprintf("delete %s", c.Path)); err != nil {
		return err
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
	if err := svc.Remove(ctx, c.Path); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"deleted": c.Path})
	}

	fmt.Fprintf(os.Stdout, "Deleted %s\n", c.Path)
	return nil
}
