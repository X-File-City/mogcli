package cmd

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

func helpOptions() kong.HelpOptions {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MOG_HELP")))
	return kong.HelpOptions{NoExpandSubcommands: mode != "full"}
}

func helpPrinter(options kong.HelpOptions, ctx *kong.Context) error {
	origStdout := ctx.Stdout
	origStderr := ctx.Stderr

	width := guessColumns(origStdout)

	oldCols, hadCols := os.LookupEnv("COLUMNS")
	_ = os.Setenv("COLUMNS", strconv.Itoa(width))
	defer func() {
		if hadCols {
			_ = os.Setenv("COLUMNS", oldCols)
		} else {
			_ = os.Unsetenv("COLUMNS")
		}
	}()

	buf := bytes.NewBuffer(nil)
	ctx.Stdout = buf
	ctx.Stderr = origStderr
	defer func() { ctx.Stdout = origStdout }()

	if err := kong.DefaultHelpPrinter(options, ctx); err != nil {
		return err
	}

	out := rewriteCommandSummaries(buf.String(), ctx.Selected())
	out = injectBuildLine(out)
	_, err := io.WriteString(origStdout, out)
	return err
}

func injectBuildLine(out string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	c := strings.TrimSpace(commit)
	line := "Build: " + v
	if c != "" {
		line = line + " (" + c + ")"
	}

	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "Usage:") {
			if i+1 < len(lines) && lines[i+1] == line {
				return out
			}
			outLines := make([]string, 0, len(lines)+1)
			outLines = append(outLines, lines[:i+1]...)
			outLines = append(outLines, line)
			outLines = append(outLines, lines[i+1:]...)
			return strings.Join(outLines, "\n")
		}
	}

	return out
}

func rewriteCommandSummaries(out string, selected *kong.Node) string {
	if selected == nil {
		return out
	}
	prefix := selected.Path() + " "
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, prefix) && strings.HasPrefix(line, "  ") {
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + strings.TrimPrefix(trimmed, prefix)
		}
	}
	return strings.Join(lines, "\n")
}

func guessColumns(w io.Writer) int {
	if colsStr := os.Getenv("COLUMNS"); colsStr != "" {
		if cols, err := strconv.Atoi(colsStr); err == nil {
			return cols
		}
	}
	_ = w
	return 80
}
