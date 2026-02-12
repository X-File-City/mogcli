package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"

	"github.com/jared/mogcli/internal/authclient"
	"github.com/jared/mogcli/internal/config"
	"github.com/jared/mogcli/internal/errfmt"
	"github.com/jared/mogcli/internal/outfmt"
	"github.com/jared/mogcli/internal/secrets"
	"github.com/jared/mogcli/internal/ui"
)

type RootFlags struct {
	Profile        string `name:"use-profile" help:"Profile name override for API commands" default:"${profile}"`
	Client         string `help:"Logical client registration name" default:"${client}"`
	EnableCommands string `help:"Comma-separated list of enabled top-level commands (restricts CLI)" default:"${enabled_commands}"`
	JSON           bool   `help:"Output JSON to stdout (best for scripting)" default:"${json}"`
	Plain          bool   `help:"Output stable, parseable text to stdout (TSV)" default:"${plain}"`
	Force          bool   `help:"Skip confirmations for destructive commands"`
	NoInput        bool   `help:"Never prompt; fail instead (useful for CI)"`
	Verbose        bool   `help:"Enable verbose logging"`
}

type CLI struct {
	RootFlags `embed:""`

	Version kong.VersionFlag `help:"Print version and exit"`

	Auth       AuthCmd               `cmd:"" help:"Authentication and profiles"`
	Mail       MailCmd               `cmd:"" aliases:"email" help:"Outlook Mail"`
	Calendar   CalendarCmd           `cmd:"" help:"Outlook Calendar"`
	Contacts   ContactsCmd           `cmd:"" help:"Outlook Contacts"`
	Groups     GroupsCmd             `cmd:"" help:"Microsoft 365 Groups"`
	Tasks      TasksCmd              `cmd:"" help:"Microsoft To Do tasks"`
	OneDrive   OneDriveCmd           `cmd:"" name:"onedrive" help:"OneDrive"`
	Config     ConfigCmd             `cmd:"" help:"Manage configuration"`
	VersionCmd VersionCmd            `cmd:"" name:"version" help:"Print version"`
	Completion CompletionCmd         `cmd:"" help:"Generate shell completion scripts"`
	Complete   CompletionInternalCmd `cmd:"" name:"__complete" hidden:"" help:"Internal completion helper"`
}

type exitPanic struct{ code int }

func Execute(args []string) (err error) {
	parser, cli, err := newParser(helpDescription())
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(exitPanic); ok {
				if ep.code == 0 {
					err = nil
					return
				}
				err = &ExitError{Code: ep.code, Err: errors.New("exited")}
				return
			}
			panic(r)
		}
	}()

	kctx, err := parser.Parse(args)
	if err != nil {
		parsedErr := wrapParseError(err)
		_, _ = fmt.Fprintln(os.Stderr, errfmt.Format(parsedErr))
		return parsedErr
	}

	if err = enforceEnabledCommands(kctx, cli.EnableCommands); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, errfmt.Format(err))
		return err
	}

	logLevel := slog.LevelWarn
	if cli.Verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	mode, err := outfmt.FromFlags(cli.JSON, cli.Plain)
	if err != nil {
		return newUsageError(err)
	}

	ctx := context.Background()
	ctx = outfmt.WithMode(ctx, mode)
	ctx = authclient.WithClient(ctx, cli.Client)
	ctx = withRootFlags(ctx, &cli.RootFlags)

	u := ui.New(ui.Options{Stdout: os.Stdout, Stderr: os.Stderr})
	ctx = ui.WithUI(ctx, u)

	kctx.BindTo(ctx, (*context.Context)(nil))
	kctx.Bind(&cli.RootFlags)

	err = kctx.Run()
	if err == nil {
		return nil
	}

	if uiPrinter := ui.FromContext(ctx); uiPrinter != nil {
		uiPrinter.Err().Error(errfmt.Format(err))
		return err
	}

	_, _ = fmt.Fprintln(os.Stderr, errfmt.Format(err))
	return err
}

func wrapParseError(err error) error {
	if err == nil {
		return nil
	}
	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) {
		return &ExitError{Code: 2, Err: parseErr}
	}
	return err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func newParser(description string) (*kong.Kong, *CLI, error) {
	envMode := outfmt.FromEnv()
	vars := kong.Vars{
		"profile":          envOr("MOG_PROFILE", ""),
		"client":           envOr("MOG_CLIENT", ""),
		"enabled_commands": envOr("MOG_ENABLE_COMMANDS", ""),
		"json":             boolString(envMode.JSON),
		"plain":            boolString(envMode.Plain),
		"version":          VersionString(),
	}

	cli := &CLI{}
	parser, err := kong.New(
		cli,
		kong.Name("mog"),
		kong.Description(description),
		kong.ConfigureHelp(helpOptions()),
		kong.Help(helpPrinter),
		kong.Vars(vars),
		kong.Writers(os.Stdout, os.Stderr),
		kong.Exit(func(code int) { panic(exitPanic{code: code}) }),
	)
	if err != nil {
		return nil, nil, err
	}

	return parser, cli, nil
}

func baseDescription() string {
	return "Microsoft Graph CLI for Outlook Mail/Calendar/Contacts/Groups/Tasks/OneDrive"
}

func helpDescription() string {
	desc := baseDescription()

	configPath, err := config.ConfigPath()
	configLine := "unknown"
	if err != nil {
		configLine = fmt.Sprintf("error: %v", err)
	} else if configPath != "" {
		configLine = configPath
	}

	backendInfo, err := secrets.ResolveKeyringBackendInfo()
	backendLine := "unknown"
	if err != nil {
		backendLine = fmt.Sprintf("error: %v", err)
	} else if backendInfo.Value != "" {
		backendLine = fmt.Sprintf("%s (source: %s)", backendInfo.Value, backendInfo.Source)
	}

	return fmt.Sprintf("%s\n\nConfig:\n  file: %s\n  keyring backend: %s", desc, configLine, backendLine)
}

func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: 2, Err: err}
}
