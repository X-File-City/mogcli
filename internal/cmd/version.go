package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/outfmt"
)

var (
	version = "0.0.2"
	commit  = ""
	date    = ""
)

func VersionString() string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	c := strings.TrimSpace(commit)
	if len(c) > 7 {
		c = c[:7]
	}
	d := strings.TrimSpace(date)

	if c == "" && d == "" {
		return fmt.Sprintf("mog version %s", v)
	}
	if c == "" {
		return fmt.Sprintf("mog version %s (%s)", v, d)
	}
	if d == "" {
		return fmt.Sprintf("mog version %s (%s)", v, c)
	}
	return fmt.Sprintf("mog version %s (%s %s)", v, c, d)
}

type VersionCmd struct{}

func (c *VersionCmd) Run(ctx context.Context) error {
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"version": strings.TrimSpace(version),
			"commit":  strings.TrimSpace(commit),
			"date":    strings.TrimSpace(date),
		})
	}
	fmt.Fprintln(os.Stdout, VersionString())
	return nil
}
