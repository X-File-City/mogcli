package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/auth"
	"github.com/jaredpalmer/mogcli/internal/config"
	"github.com/jaredpalmer/mogcli/internal/input"
	"github.com/jaredpalmer/mogcli/internal/outfmt"
	"github.com/jaredpalmer/mogcli/internal/profile"
)

type authManager interface {
	LoginDelegated(context.Context, auth.DelegatedLoginInput, func(string)) (auth.AccountInfo, error)
	LoginAppOnly(context.Context, auth.AppOnlyLoginInput) error
	Logout(string) error
}

type profileStore interface {
	List() ([]config.ProfileRecord, error)
	Get(string) (config.ProfileRecord, bool, error)
	Upsert(config.ProfileRecord, bool) error
	Delete(string) (bool, error)
	SetActive(string) error
	Active() (config.ProfileRecord, error)
	Resolve(string) (config.ProfileRecord, error)
}

var newAuthManager = func() authManager {
	return auth.NewManager()
}

var newProfileStore = func() profileStore {
	return profile.NewStore()
}

type AuthCmd struct {
	Wizard AuthWizardCmd    `cmd:"" default:"1" hidden:"" help:"Interactive delegated authentication setup"`
	App    AuthAppWizardCmd `cmd:"" name:"app" help:"Interactive enterprise app-only setup (advanced)"`
	Login  AuthLoginCmd     `cmd:"" help:"Login and save profile credentials"`
	Logout AuthLogoutCmd    `cmd:"" help:"Log out of a Microsoft account"`
	Status AuthStatusCmd    `cmd:"" help:"Display active account and login status"`
	Use    AuthUseCmd       `cmd:"" help:"Switch the active profile"`

	// Hidden aliases for backwards compatibility.
	Accounts AuthStatusCmd `cmd:"" hidden:"" help:"List saved profiles"`
	WhoAmI   AuthStatusCmd `cmd:"" aliases:"whoami" hidden:"" help:"Show active profile"`
}

type AuthWizardCmd struct{}
type AuthAppWizardCmd struct{}

type AuthLoginCmd struct {
	Profile         string `name:"profile" help:"Profile name (e.g. personal, work)"`
	Audience        string `name:"audience" help:"Token audience: consumer (MSA) or enterprise (Entra)"`
	ClientID        string `name:"client-id" help:"Application (client) ID from Entra app registration"`
	Tenant          string `name:"tenant" help:"Tenant ID or domain (enterprise only)"`
	Authority       string `name:"authority" hidden:"" help:"Advanced authority override"`
	Mode            string `name:"mode" default:"delegated" help:"Auth mode: delegated or app-only"`
	ClientSecret    string `name:"client-secret" help:"Client secret for app-only mode"`
	ClientSecretEnv string `name:"client-secret-env" help:"Env var containing the app-only client secret"`
	ScopeWorkloads  string `name:"scope-workloads" help:"Comma-separated workloads: mail,calendar,contacts,tasks,onedrive,groups"`
	AppOnlyUser     string `name:"app-only-user" help:"Default target user for app-only commands (UPN or user ID)"`
}

type authLoginParams struct {
	Profile                        string
	Audience                       string
	ClientID                       string
	Tenant                         string
	Authority                      string
	Mode                           string
	ClientSecret                   string
	ClientSecretEnv                string
	ScopeWorkloads                 []string
	AppOnlyUser                    string
	RequireDelegatedScopeWorkloads bool
}

func (c *AuthLoginCmd) Run(ctx context.Context) error {
	mode := normalizeAuthMode(c.Mode)
	if mode == "" {
		mode = profile.AuthModeDelegated
	}

	flags := rootFlagsFromContext(ctx)
	noInput := flags != nil && flags.NoInput
	interactive := !noInput && isTerminal(os.Stdin) && c.needsPrompt()

	if interactive {
		if mode == profile.AuthModeAppOnly {
			return usage("interactive app-only onboarding is available via `mog auth app`; use `mog auth login --mode app-only ...` for non-interactive setup")
		}
		return c.runInteractive(ctx)
	}
	return c.runNonInteractive(ctx)
}

func (c *AuthWizardCmd) Run(ctx context.Context) error {
	return runAuthWizard(ctx, profile.AuthModeDelegated)
}

func (c *AuthAppWizardCmd) Run(ctx context.Context) error {
	return runAuthWizard(ctx, profile.AuthModeAppOnly)
}

func runAuthWizard(ctx context.Context, mode string) error {
	wizardCommand := "`mog auth`"
	cmd := AuthLoginCmd{Mode: profile.AuthModeDelegated}
	if mode == profile.AuthModeAppOnly {
		wizardCommand = "`mog auth app`"
		cmd.Mode = profile.AuthModeAppOnly
		cmd.Audience = profile.AudienceEnterprise
	}

	flags := rootFlagsFromContext(ctx)
	if flags != nil && flags.NoInput {
		return usagef("%s requires interactive input. Use `mog auth login ...` for non-interactive usage", wizardCommand)
	}
	if !isTerminal(os.Stdin) {
		return usagef("%s requires an interactive terminal. Use `mog auth login ...` for non-interactive usage", wizardCommand)
	}

	return cmd.runInteractive(ctx)
}

// needsPrompt returns true if any required interactive field is missing.
func (c *AuthLoginCmd) needsPrompt() bool {
	return strings.TrimSpace(c.Profile) == "" ||
		strings.TrimSpace(c.Audience) == "" ||
		strings.TrimSpace(c.ClientID) == ""
}

func (c *AuthLoginCmd) runInteractive(ctx context.Context) error {
	theme := newAuthWizardTheme()
	printWizardMessage(ctx, theme.dim("Tip: you'll need an Entra app registration. See https://github.com/jaredpalmer/mogcli#setup"))
	fmt.Fprintln(os.Stderr)

	mode := normalizeAuthMode(c.Mode)
	if mode == "" {
		mode = profile.AuthModeDelegated
	}

	// 1. Audience
	audience := strings.TrimSpace(c.Audience)
	if mode == profile.AuthModeAppOnly {
		if audience != "" && !strings.EqualFold(audience, profile.AudienceEnterprise) {
			return usage("app-only mode requires enterprise audience")
		}
		audience = profile.AudienceEnterprise
	} else if audience == "" {
		var err error
		audience, err = promptAudience(ctx, profile.AudienceConsumer)
		if err != nil {
			return err
		}
	}

	// 2. Auth mode is selected by command (`mog auth` for delegated, `mog auth app` for app-only).

	// 3. Client ID
	clientID := strings.TrimSpace(c.ClientID)
	if clientID == "" {
		var err error
		clientID, err = promptRequiredValue(ctx, "Application (client) ID", "")
		if err != nil {
			return err
		}
	}

	// 4. Tenant (enterprise only)
	tenant := strings.TrimSpace(c.Tenant)
	if audience == profile.AudienceEnterprise && tenant == "" {
		var err error
		tenant, err = promptOptionalValue(ctx, "Tenant ID or domain", "")
		if err != nil {
			return err
		}
	}

	// 5. Workloads (delegated) or secret + target user (app-only)
	var workloads []string
	var clientSecret, clientSecretEnv, appOnlyUser string

	if mode == profile.AuthModeDelegated {
		allWorkloads := []string{"mail", "calendar", "contacts", "tasks", "onedrive"}
		if audience == profile.AudienceEnterprise {
			allWorkloads = append(allWorkloads, "groups")
		}
		var err error
		workloads, err = promptDelegatedWorkloads(ctx, audience, allWorkloads)
		if err != nil {
			return err
		}
	} else {
		var err error
		clientSecretEnv, clientSecret, err = promptAppOnlySecretInputs(ctx, theme)
		if err != nil {
			return err
		}
		appOnlyUser, err = promptOptionalValue(ctx, "Default target user (UPN or user ID, optional)", "")
		if err != nil {
			return err
		}
	}

	// 6. Profile name — last, with a smart default
	profileName := strings.TrimSpace(c.Profile)
	if profileName == "" {
		dflt := "personal"
		if audience == profile.AudienceEnterprise {
			dflt = "work"
		}
		if mode == profile.AuthModeAppOnly {
			dflt += "-app"
		}
		var err error
		profileName, err = promptRequiredValue(ctx, "Profile name", dflt)
		if err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stderr)

	return runAuthLogin(ctx, authLoginParams{
		Profile:                        profileName,
		Audience:                       audience,
		ClientID:                       clientID,
		Tenant:                         tenant,
		Authority:                      defaultAuthority(audience, tenant, strings.TrimSpace(c.Authority)),
		Mode:                           mode,
		ClientSecret:                   clientSecret,
		ClientSecretEnv:                clientSecretEnv,
		ScopeWorkloads:                 workloads,
		AppOnlyUser:                    appOnlyUser,
		RequireDelegatedScopeWorkloads: true,
	})
}

func (c *AuthLoginCmd) runNonInteractive(ctx context.Context) error {
	mode := normalizeAuthMode(c.Mode)
	if mode == "" {
		mode = profile.AuthModeDelegated
	}

	var missing []string
	if strings.TrimSpace(c.Profile) == "" {
		missing = append(missing, "--profile")
	}
	if strings.TrimSpace(c.Audience) == "" {
		missing = append(missing, "--audience")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		missing = append(missing, "--client-id")
	}
	if len(missing) > 0 {
		return usagef("missing required flags: %s\nTip: run `mog auth login` interactively for guided setup", strings.Join(missing, ", "))
	}

	aud := strings.ToLower(strings.TrimSpace(c.Audience))
	if aud != profile.AudienceConsumer && aud != profile.AudienceEnterprise {
		return usagef("invalid --audience %q: must be consumer or enterprise", c.Audience)
	}

	params := authLoginParams{
		Profile:                        strings.TrimSpace(c.Profile),
		Audience:                       aud,
		ClientID:                       strings.TrimSpace(c.ClientID),
		Tenant:                         strings.TrimSpace(c.Tenant),
		Authority:                      strings.TrimSpace(c.Authority),
		Mode:                           mode,
		ClientSecret:                   strings.TrimSpace(c.ClientSecret),
		ClientSecretEnv:                strings.TrimSpace(c.ClientSecretEnv),
		AppOnlyUser:                    strings.TrimSpace(c.AppOnlyUser),
		RequireDelegatedScopeWorkloads: true,
	}

	if mode == profile.AuthModeDelegated {
		workloads, err := auth.ParseScopeWorkloadsCSV(c.ScopeWorkloads)
		if err != nil {
			return usagef("delegated mode requires --scope-workloads (%s)", auth.DelegatedScopeWorkloadsHelp())
		}
		params.ScopeWorkloads = workloads
	}

	return runAuthLogin(ctx, params)
}

func runAuthLogin(ctx context.Context, params authLoginParams) error {
	mode := normalizeAuthMode(params.Mode)
	if mode == "" {
		mode = profile.AuthModeDelegated
	}

	authority := defaultAuthority(params.Audience, params.Tenant, params.Authority)
	manager := newAuthManager()
	store := newProfileStore()

	record := config.ProfileRecord{
		Name:      strings.TrimSpace(params.Profile),
		Audience:  strings.TrimSpace(params.Audience),
		ClientID:  strings.TrimSpace(params.ClientID),
		Authority: authority,
		TenantID:  strings.TrimSpace(params.Tenant),
		AuthMode:  mode,
	}

	switch mode {
	case profile.AuthModeAppOnly:
		if !strings.EqualFold(record.Audience, profile.AudienceEnterprise) {
			return usage("app-only mode requires --audience enterprise")
		}

		secret := strings.TrimSpace(params.ClientSecret)
		if secret == "" && strings.TrimSpace(params.ClientSecretEnv) != "" {
			secret = strings.TrimSpace(os.Getenv(strings.TrimSpace(params.ClientSecretEnv)))
		}
		if secret == "" {
			return usage("app-only mode requires --client-secret or --client-secret-env")
		}

		if err := manager.LoginAppOnly(ctx, auth.AppOnlyLoginInput{
			ProfileName: record.Name,
			Audience:    record.Audience,
			ClientID:    record.ClientID,
			Authority:   record.Authority,
			TenantID:    record.TenantID,
			Secret:      secret,
		}); err != nil {
			return err
		}

		record.AppOnlyUser = strings.TrimSpace(params.AppOnlyUser)
		record.DelegatedScopeWorkloads = nil
	default:
		workloads, err := auth.NormalizeScopeWorkloads(params.ScopeWorkloads)
		if err != nil {
			if params.RequireDelegatedScopeWorkloads {
				return usagef("delegated mode requires --scope-workloads (%s)", auth.DelegatedScopeWorkloadsHelp())
			}
			return usage(err.Error())
		}

		scopes, err := auth.DelegatedScopesForWorkloads(workloads)
		if err != nil {
			return usage(err.Error())
		}

		account, err := manager.LoginDelegated(ctx, auth.DelegatedLoginInput{
			ProfileName: record.Name,
			Audience:    record.Audience,
			ClientID:    record.ClientID,
			Authority:   record.Authority,
			TenantID:    record.TenantID,
			Scopes:      scopes,
		}, func(message string) {
			if u := uiFromContext(ctx); u != nil {
				u.Err().Println(message)
				return
			}
			_, _ = fmt.Fprintln(os.Stderr, message)
		})
		if err != nil {
			return err
		}

		record.DelegatedScopeWorkloads = workloads
		record.AccountID = account.AccountID
		record.Username = account.Username
		record.AppOnlyUser = ""
		if strings.TrimSpace(account.TenantID) != "" {
			record.TenantID = strings.TrimSpace(account.TenantID)
		}
	}

	if err := store.Upsert(record, true); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"profile":  record.Name,
			"audience": record.Audience,
			"mode":     record.AuthMode,
			"active":   true,
		})
	}

	theme := newAuthWizardTheme()
	fmt.Fprintf(os.Stdout, "%s Logged in to profile %s\n", theme.ok("✓"), record.Name)
	fmt.Fprintf(os.Stdout, "  Account:  %s\n", displayOrDash(record.Username))
	fmt.Fprintf(os.Stdout, "  Audience: %s\n", record.Audience)
	fmt.Fprintf(os.Stdout, "  Mode:     %s\n", record.AuthMode)
	if len(record.DelegatedScopeWorkloads) > 0 {
		fmt.Fprintf(os.Stdout, "  Workloads: %s\n", strings.Join(record.DelegatedScopeWorkloads, ", "))
	}
	if record.TenantID != "" {
		fmt.Fprintf(os.Stdout, "  Tenant:   %s\n", record.TenantID)
	}
	return nil
}

func promptRequiredValue(ctx context.Context, label string, defaultValue string) (string, error) {
	for {
		value, err := promptValue(ctx, label, defaultValue)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
		printWizardMessage(ctx, fmt.Sprintf("%s is required", strings.TrimSpace(label)))
	}
}

func promptOptionalValue(ctx context.Context, label string, defaultValue string) (string, error) {
	value, err := promptValue(ctx, label, defaultValue)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func promptValue(ctx context.Context, label string, defaultValue string) (string, error) {
	prompt := strings.TrimSpace(label)
	if strings.TrimSpace(defaultValue) != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, strings.TrimSpace(defaultValue))
	}
	prompt += ": "

	line, err := input.PromptLine(ctx, prompt)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", &ExitError{Code: 1, Err: errors.New("cancelled")}
		}
		return "", fmt.Errorf("read input: %w", err)
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return strings.TrimSpace(defaultValue), nil
	}
	return trimmed, nil
}

func promptAudience(ctx context.Context, defaultAudience string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(defaultAudience))
	switch value {
	case profile.AudienceConsumer, profile.AudienceEnterprise:
	default:
		value = profile.AudienceConsumer
	}

	selected, err := input.SelectString(ctx, input.SelectStringConfig{
		Title: "What account type?",
		Options: []input.SelectStringOption{
			{Label: "Personal Microsoft account (Outlook/Hotmail/Live)", Value: profile.AudienceConsumer},
			{Label: "Work or school account (Microsoft Entra)", Value: profile.AudienceEnterprise},
		},
		Default: value,
	})
	if err != nil {
		return "", wrapPromptInputErr("select audience", err)
	}
	return selected, nil
}

func printWizardMessage(ctx context.Context, message string) {
	if u := uiFromContext(ctx); u != nil {
		u.Err().Println(message)
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, message)
}

func wrapPromptInputErr(action string, err error) error {
	if errors.Is(err, io.EOF) {
		return &ExitError{Code: 1, Err: errors.New("cancelled")}
	}
	return fmt.Errorf("%s: %w", action, err)
}

func displayOrDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func promptDelegatedWorkloads(ctx context.Context, audience string, defaults []string) ([]string, error) {
	workloadOptions := []input.SelectStringOption{
		{Label: "mail", Value: "mail"},
		{Label: "calendar", Value: "calendar"},
		{Label: "contacts", Value: "contacts"},
		{Label: "tasks", Value: "tasks"},
		{Label: "onedrive", Value: "onedrive"},
	}
	if audience == profile.AudienceEnterprise {
		workloadOptions = append(workloadOptions, input.SelectStringOption{Label: "groups (enterprise only)", Value: "groups"})
	}

	selected, err := input.MultiSelectStrings(ctx, input.MultiSelectStringConfig{
		Title:       "Workloads to enable",
		Description: "Space to toggle, enter to confirm.",
		Defaults:    defaults,
		Options:     workloadOptions,
		Validate: func(values []string) error {
			if len(values) == 0 {
				return errors.New("select at least one workload")
			}
			return nil
		},
	})
	if err != nil {
		return nil, wrapPromptInputErr("select workloads", err)
	}

	workloads, normalizeErr := auth.NormalizeScopeWorkloads(selected)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	return workloads, nil
}

func promptAppOnlySecretInputs(ctx context.Context, theme authWizardTheme) (string, string, error) {
	for {
		envName, err := promptOptionalValue(ctx, "Client secret env var (recommended)", "")
		if err != nil {
			return "", "", err
		}
		secret, err := promptOptionalValue(ctx, "Client secret value (if no env var)", "")
		if err != nil {
			return "", "", err
		}
		envName = strings.TrimSpace(envName)
		secret = strings.TrimSpace(secret)
		if envName != "" || secret != "" {
			return envName, secret, nil
		}

		printWizardMessage(ctx, theme.warn("You must provide either an env var or a secret value."))
	}
}

type authWizardTheme struct {
	color bool
}

func newAuthWizardTheme() authWizardTheme {
	return authWizardTheme{
		color: isTerminal(os.Stderr) && strings.TrimSpace(os.Getenv("NO_COLOR")) == "",
	}
}

func (t authWizardTheme) style(code string, value string) string {
	if !t.color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func (t authWizardTheme) ok(value string) string   { return t.style("32", value) }
func (t authWizardTheme) warn(value string) string { return t.style("33", value) }
func (t authWizardTheme) dim(value string) string  { return t.style("2", value) }

type AuthLogoutCmd struct {
	Profile string `name:"profile" help:"Profile name to log out"`
	All     bool   `name:"all" help:"Log out all profiles"`
}

func (c *AuthLogoutCmd) Run(ctx context.Context) error {
	store := newProfileStore()
	manager := newAuthManager()

	if c.All {
		profiles, err := store.List()
		if err != nil {
			return err
		}
		for _, p := range profiles {
			_ = manager.Logout(p.Name)
			_, _ = store.Delete(p.Name)
		}
		if outfmt.IsJSON(ctx) {
			return outfmt.WriteJSON(os.Stdout, map[string]any{"logout": "all", "removed": len(profiles)})
		}
		fmt.Fprintf(os.Stdout, "Logged out %d profiles\n", len(profiles))
		return nil
	}

	target := strings.TrimSpace(c.Profile)
	if target == "" {
		active, err := store.Active()
		if err != nil {
			return err
		}
		target = active.Name
	}

	if err := manager.Logout(target); err != nil {
		return err
	}
	_, err := store.Delete(target)
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"logout": target})
	}
	fmt.Fprintf(os.Stdout, "Logged out profile %s\n", target)
	return nil
}

type AuthStatusCmd struct{}

func (c *AuthStatusCmd) Run(ctx context.Context) error {
	store := newProfileStore()
	profiles, err := store.List()
	if err != nil {
		return err
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"profiles": profiles})
	}
	if len(profiles) == 0 {
		fmt.Fprintln(os.Stdout, "No profiles configured. Run `mog auth login` to get started.")
		return nil
	}

	theme := newAuthWizardTheme()
	for _, p := range profiles {
		marker := " "
		if p.Active {
			marker = theme.ok("✓")
		}
		account := displayOrDash(p.Username)
		fmt.Fprintf(os.Stdout, "%s %s\n", marker, p.Name)
		fmt.Fprintf(os.Stdout, "    Account:  %s\n", account)
		fmt.Fprintf(os.Stdout, "    Audience: %s\n", p.Audience)
		fmt.Fprintf(os.Stdout, "    Mode:     %s\n", p.AuthMode)
		if len(p.DelegatedScopeWorkloads) > 0 {
			fmt.Fprintf(os.Stdout, "    Workloads: %s\n", strings.Join(p.DelegatedScopeWorkloads, ", "))
		}
		if p.TenantID != "" {
			fmt.Fprintf(os.Stdout, "    Tenant:   %s\n", p.TenantID)
		}
	}

	return nil
}

type AuthUseCmd struct {
	Profile string `arg:"" required:"" help:"Profile name to activate"`
}

func (c *AuthUseCmd) Run(ctx context.Context) error {
	store := newProfileStore()
	if err := store.SetActive(c.Profile); err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{"active_profile": c.Profile})
	}
	theme := newAuthWizardTheme()
	fmt.Fprintf(os.Stdout, "%s Active profile set to %s\n", theme.ok("✓"), c.Profile)
	return nil
}

func rootProfileOverride(ctx context.Context) string {
	if flags := rootFlagsFromContext(ctx); flags != nil {
		return strings.TrimSpace(flags.Profile)
	}
	return ""
}
