package cmd

import (
	"context"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jared/mogcli/internal/outfmt"
	"github.com/jared/mogcli/internal/ui"
)

func tableWriter(ctx context.Context) (io.Writer, func()) {
	if outfmt.IsPlain(ctx) {
		return os.Stdout, func() {}
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	return tw, func() { _ = tw.Flush() }
}

func printNextPageHint(u *ui.UI, nextPageToken string) {
	if u == nil || nextPageToken == "" {
		return
	}
	u.Err().Printf("# Next page: --page %s", shellQuote(nextPageToken))
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
