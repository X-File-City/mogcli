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
	Wizard   AuthWizardCmd   `cmd:"" default:"1" hidden:"" help:"Interactive authentication setup"`
	Login    AuthLoginCmd    `cmd:"" help:"Login and save profile credentials"`
	Logout   AuthLogoutCmd   `cmd:"" help:"Logout and clear saved credentials"`
	Accounts AuthAccountsCmd `cmd:"" help:"List saved profiles"`
	Use      AuthUseCmd      `cmd:"" help:"Set active profile"`
	WhoAmI   AuthWhoAmICmd   `cmd:"" aliases:"whoami" help:"Show active profile and account"`
}

type AuthLoginCmd struct {
	Profile         string `name:"profile" required:"" help:"Local profile name used by mog (for example: personal, work, work-app)"`
	Audience        string `name:"audience" required:"" enum:"consumer,enterprise" help:"Token audience: consumer (MSA) or enterprise (Entra work/school)"`
	ClientID        string `name:"client-id" required:"" help:"Application (client) ID from Entra App registrations -> <app> -> Overview"`
	Tenant          string `name:"tenant" help:"Tenant ID or domain for enterprise profiles (Directory (tenant) ID)"`
	Authority       string `name:"authority" help:"Advanced authority override (consumers|organizations|common|tenant)"`
	Mode            string `name:"mode" default:"delegated" enum:"delegated,app-only" help:"Auth mode: delegated (user sign-in) or app-only (daemon/service)"`
	ClientSecret    string `name:"client-secret" help:"Client secret value for app-only mode"`
	ClientSecretEnv string `name:"client-secret-env" help:"Environment variable name that contains the app-only client secret"`
	ScopeWorkloads  string `name:"scope-workloads" help:"Required in delegated mode. Comma-separated workloads: mail,calendar,contacts,tasks,onedrive,groups"`
	AppOnlyUser     string `name:"app-only-user" help:"Default app-only target user (UPN or user ID) for mail/contacts/onedrive commands"`
}

type AuthWizardCmd struct{}

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

func (c *AuthWizardCmd) Run(ctx context.Context) error {
	flags := rootFlagsFromContext(ctx)
	if flags != nil && flags.NoInput {
		return usage("`mog auth` requires interactive input. Use `mog auth login ...` for non-interactive usage")
	}
	if !isTerminal(os.Stdin) {
		return usage("`mog auth` requires an interactive terminal. Use `mog auth login ...` for non-interactive usage")
	}
	theme := newAuthWizardTheme()
	printWizardMessage(ctx, renderWizardIntro(theme))
	printWizardMessage(ctx, renderEntraSetupChecklist(theme))

	store := newProfileStore()

	defaultProfile := ""
	if active, err := store.Active(); err == nil {
		defaultProfile = strings.TrimSpace(active.Name)
	}

	printWizardMessage(ctx, renderProfileNameGuide(theme))
	profileName, err := promptRequiredValue(ctx, "Profile name", defaultProfile)
	if err != nil {
		return err
	}

	existing := config.ProfileRecord{Name: profileName}
	if record, ok, getErr := store.Get(profileName); getErr != nil {
		return getErr
	} else if ok {
		existing = record
		existing.Name = profileName
	}

	audienceDefault := strings.TrimSpace(existing.Audience)
	if audienceDefault == "" {
		audienceDefault = profile.AudienceConsumer
	}
	printWizardMessage(ctx, renderAudienceGuide(theme))
	audience, err := promptAudience(ctx, audienceDefault)
	if err != nil {
		return err
	}

	printWizardMessage(ctx, renderClientIDGuide(theme))
	clientID, err := promptRequiredValue(ctx, "Client ID", strings.TrimSpace(existing.ClientID))
	if err != nil {
		return err
	}
	tenant := strings.TrimSpace(existing.TenantID)
	if audience == profile.AudienceEnterprise {
		printWizardMessage(ctx, renderTenantGuide(theme))
		tenant, err = promptOptionalValue(ctx, "Tenant ID or domain (recommended)", strings.TrimSpace(existing.TenantID))
		if err != nil {
			return err
		}
	} else {
		tenant = ""
	}

	printWizardMessage(ctx, renderAuthorityGuide(theme))
	authorityDefault := wizardAuthorityDefault(existing, audience, tenant)
	authority, err := promptOptionalValue(ctx, "Authority override (advanced, optional)", authorityDefault)
	if err != nil {
		return err
	}

	modeDefault := "delegated"
	if normalizeAuthMode(existing.AuthMode) == profile.AuthModeAppOnly {
		modeDefault = "app-only"
	}
	printWizardMessage(ctx, renderModeGuide(theme))
	mode, err := promptMode(ctx, modeDefault)
	if err != nil {
		return err
	}

	params := authLoginParams{
		Profile:                        profileName,
		Audience:                       audience,
		ClientID:                       clientID,
		Tenant:                         tenant,
		Authority:                      authority,
		Mode:                           mode,
		RequireDelegatedScopeWorkloads: true,
	}

	if mode == profile.AuthModeDelegated {
		defaults, defaultErr := auth.NormalizeScopeWorkloads(existing.DelegatedScopeWorkloads)
		if defaultErr != nil {
			defaults = nil
		}

		workloads, workloadErr := promptDelegatedWorkloads(ctx, theme, defaults)
		if workloadErr != nil {
			return workloadErr
		}
		params.ScopeWorkloads = workloads
	} else {
		printWizardMessage(ctx, renderAppOnlyUserGuide(theme))
		params.AppOnlyUser, err = promptOptionalValue(ctx, "Default app-only user (optional)", strings.TrimSpace(existing.AppOnlyUser))
		if err != nil {
			return err
		}

		params.ClientSecretEnv, params.ClientSecret, err = promptAppOnlySecretInputs(ctx, theme)
		if err != nil {
			return err
		}
	}

	return runAuthLogin(ctx, params)
}

func (c *AuthLoginCmd) Run(ctx context.Context) error {
	mode := normalizeAuthMode(c.Mode)
	if mode == "" {
		mode = profile.AuthModeDelegated
	}

	params := authLoginParams{
		Profile:                        strings.TrimSpace(c.Profile),
		Audience:                       strings.TrimSpace(c.Audience),
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

	fmt.Fprintf(os.Stdout, "Logged in as profile %s (%s, %s)\n", record.Name, record.Audience, record.AuthMode)
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
		Title:       "Audience",
		Description: "Pick the audience that matches your app registration.",
		Default:     value,
		Options: []input.SelectStringOption{
			{Label: "consumer (personal Microsoft accounts)", Value: profile.AudienceConsumer},
			{Label: "enterprise (work/school accounts)", Value: profile.AudienceEnterprise},
		},
	})
	if err != nil {
		return "", wrapPromptInputErr("select audience", err)
	}
	return selected, nil
}

func promptMode(ctx context.Context, defaultMode string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(defaultMode))
	switch value {
	case profile.AuthModeDelegated:
	case "app-only", profile.AuthModeAppOnly:
		value = profile.AuthModeAppOnly
	default:
		value = profile.AuthModeDelegated
	}

	selected, err := input.SelectString(ctx, input.SelectStringConfig{
		Title:       "Auth mode",
		Description: "Choose delegated user sign-in or app-only service auth.",
		Default:     value,
		Options: []input.SelectStringOption{
			{Label: "delegated (device code user sign-in)", Value: profile.AuthModeDelegated},
			{Label: "app-only (service auth with client secret)", Value: profile.AuthModeAppOnly},
		},
	})
	if err != nil {
		return "", wrapPromptInputErr("select auth mode", err)
	}
	return selected, nil
}

func wizardAuthorityDefault(existing config.ProfileRecord, selectedAudience string, selectedTenant string) string {
	current := strings.TrimSpace(existing.Authority)
	if current == "" {
		return ""
	}

	sameAudience := strings.EqualFold(strings.TrimSpace(existing.Audience), strings.TrimSpace(selectedAudience))
	sameTenant := strings.EqualFold(strings.TrimSpace(existing.TenantID), strings.TrimSpace(selectedTenant))
	if sameAudience && sameTenant {
		return current
	}

	return defaultAuthority(selectedAudience, selectedTenant, "")
}

func promptDelegatedWorkloads(ctx context.Context, theme authWizardTheme, defaults []string) ([]string, error) {
	defaultDisplay := strings.Join(defaults, ",")
	printWizardMessage(ctx, renderWorkloadGuide(theme, defaultDisplay))

	workloadOptions := []input.SelectStringOption{
		{Label: "mail (Mail.Read, Mail.Send)", Value: "mail"},
		{Label: "calendar (Calendars.Read, Calendars.ReadWrite)", Value: "calendar"},
		{Label: "contacts (Contacts.Read, Contacts.ReadWrite)", Value: "contacts"},
		{Label: "tasks (Tasks.Read, Tasks.ReadWrite)", Value: "tasks"},
		{Label: "onedrive (Files.Read, Files.ReadWrite)", Value: "onedrive"},
		{Label: "groups (Group.Read.All, GroupMember.Read.All)", Value: "groups"},
	}

	selected, err := input.MultiSelectStrings(ctx, input.MultiSelectStringConfig{
		Title:       "Delegated workload groups",
		Description: "Use space to toggle selections, then press enter to continue.",
		Defaults:    defaults,
		Options:     workloadOptions,
		Validate: func(values []string) error {
			if len(values) == 0 {
				return errors.New("at least one delegated workload is required")
			}
			return nil
		},
	})
	if err != nil {
		return nil, wrapPromptInputErr("select delegated workloads", err)
	}

	workloads, normalizeErr := auth.NormalizeScopeWorkloads(selected)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	return workloads, nil
}

func promptAppOnlySecretInputs(ctx context.Context, theme authWizardTheme) (string, string, error) {
	printWizardMessage(ctx, renderAppOnlySecretGuide(theme))
	for {
		envName, err := promptOptionalValue(ctx, "Client secret env var (recommended)", "")
		if err != nil {
			return "", "", err
		}
		secret, err := promptOptionalValue(ctx, "Client secret value (optional if env var is set)", "")
		if err != nil {
			return "", "", err
		}
		envName = strings.TrimSpace(envName)
		secret = strings.TrimSpace(secret)
		if envName != "" || secret != "" {
			return envName, secret, nil
		}

		printWizardMessage(ctx, theme.warn("Provide either a client secret env var or a client secret value for app-only mode."))
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

func (t authWizardTheme) title(value string) string { return t.style("1;36", value) }
func (t authWizardTheme) step(value string) string  { return t.style("1;34", value) }
func (t authWizardTheme) key(value string) string   { return t.style("1;33", value) }
func (t authWizardTheme) ok(value string) string    { return t.style("32", value) }
func (t authWizardTheme) warn(value string) string  { return t.style("33", value) }
func (t authWizardTheme) dim(value string) string   { return t.style("2", value) }

func renderWizardIntro(theme authWizardTheme) string {
	lines := []string{
		theme.title("MOG AUTH SETUP"),
		"  This wizard will configure your profile and start Microsoft login.",
		"  Press Enter to accept values shown in [brackets].",
		"",
		theme.step("Before you start, gather these from Microsoft Entra:"),
		"  1) Application (client) ID",
		"  2) Directory (tenant) ID (enterprise profiles)",
		"  3) Client secret (app-only mode only)",
		"",
		theme.step("Where to find them:"),
		"  Microsoft Entra admin center",
		"    -> App registrations",
		"    -> Select your app",
		"    -> Overview (Client ID / Tenant ID)",
		"    -> Certificates & secrets (Client secret)",
		"",
		theme.dim("Tip: You can run `mog auth login --help` for non-interactive setup."),
	}

	return strings.Join(lines, "\n")
}

func renderEntraSetupChecklist(theme authWizardTheme) string {
	lines := []string{
		theme.step("Exact Entra setup checklist (do this once per app registration)"),
		"  A) Create app registration",
		"     Entra -> App registrations -> New registration",
		"     Name: any label (for example: mog-cli-personal or mog-cli-work)",
		"     Supported account types:",
		"       - consumer profile: \"Personal Microsoft accounts only\"",
		"       - enterprise profile: \"Accounts in this organizational directory only\"",
		"     Redirect URI: leave blank for CLI device-code setup",
		"",
		"  B) Authentication settings (required for delegated mode / device code)",
		"     Entra -> App registrations -> <app> -> Authentication",
		"       - Enable \"Allow public client flows\" = Yes",
		"       - Save",
		"",
		"  C) API permissions (Microsoft Graph)",
		"     Entra -> App registrations -> <app> -> API permissions -> Add a permission",
		"       1) Delegated permissions (for delegated mode):",
		"          " + theme.key("mail") + "      -> Mail.Read, Mail.Send",
		"          " + theme.key("calendar") + "  -> Calendars.Read, Calendars.ReadWrite",
		"          " + theme.key("contacts") + "  -> Contacts.Read, Contacts.ReadWrite",
		"          " + theme.key("tasks") + "     -> Tasks.Read, Tasks.ReadWrite",
		"          " + theme.key("onedrive") + "  -> Files.Read, Files.ReadWrite",
		"          " + theme.key("groups") + "    -> Group.Read.All, GroupMember.Read.All (enterprise only)",
		"       2) Application permissions (for app-only mode):",
		"          mail      -> Mail.Read, Mail.Send",
		"          contacts  -> Contacts.Read, Contacts.ReadWrite",
		"          onedrive  -> Files.Read.All, Files.ReadWrite.All",
		"",
		"  D) Admin consent",
		"     API permissions -> \"Grant admin consent\" (enterprise tenants, especially app-only)",
		"",
		"  E) Client secret (app-only only)",
		"     Entra -> App registrations -> <app> -> Certificates & secrets -> New client secret",
		"     Copy the " + theme.key("Value") + " now (not the Secret ID). Store it securely.",
		"",
		"  F) Values to copy into mog",
		"     --client-id     = Application (client) ID",
		"     --tenant        = Directory (tenant) ID or tenant domain (enterprise)",
		"     --client-secret = secret Value (app-only)",
	}

	return strings.Join(lines, "\n")
}

func renderProfileNameGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 1: Profile name"),
		"  This is a local nickname in mog (examples: personal, work, work-app).",
		"  It helps you switch accounts quickly with `mog auth use <profile>`.",
	}
	return strings.Join(lines, "\n")
}

func renderAudienceGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 2: Audience"),
		"  " + theme.key("consumer") + "  = personal Microsoft accounts (MSA, Outlook/Hotmail/Live)",
		"  " + theme.key("enterprise") + " = work/school accounts (Microsoft Entra ID)",
		"  Pick the audience that matches your app registration's supported account type.",
		"  If these do not match, login usually fails with AADSTS audience/tenant errors.",
	}
	return strings.Join(lines, "\n")
}

func renderClientIDGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 3: Client ID"),
		"  Enter the Application (client) ID exactly as shown in app Overview.",
		"  Format is a GUID like: 11111111-2222-3333-4444-555555555555",
	}
	return strings.Join(lines, "\n")
}

func renderTenantGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Tenant (enterprise recommended)"),
		"  Use your Directory (tenant) ID GUID or tenant domain (for example: contoso.onmicrosoft.com).",
		"  If blank, mog will default authority automatically.",
	}
	return strings.Join(lines, "\n")
}

func renderAuthorityGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Authority override (advanced)"),
		"  Leave blank unless you need explicit authority routing.",
		"  Common values: consumers, organizations, common, or a specific tenant ID/domain.",
	}
	return strings.Join(lines, "\n")
}

func renderModeGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 4: Auth mode"),
		"  " + theme.key("delegated") + " = sign in as a user (device code flow).",
		"  " + theme.key("app-only") + "  = service-to-service (enterprise only, requires client secret).",
		"  Delegated needs public client flow enabled. App-only needs application permissions + admin consent.",
	}
	return strings.Join(lines, "\n")
}

func renderWorkloadGuide(theme authWizardTheme, defaultDisplay string) string {
	lines := []string{
		theme.step("Step 5: Delegated workload groups"),
		"  Select one or more workload groups. mog requests minimal per-command scopes later.",
		"  Use space to toggle options, then press Enter to continue.",
		"  Available selections:",
		"    " + theme.key("mail") + "      Outlook mail list/get/send",
		"              permissions: Mail.Read, Mail.Send",
		"    " + theme.key("calendar") + "  Outlook calendar list/get/create/update/delete",
		"              permissions: Calendars.Read, Calendars.ReadWrite",
		"    " + theme.key("contacts") + "  Outlook contacts list/get/create/update/delete",
		"              permissions: Contacts.Read, Contacts.ReadWrite",
		"    " + theme.key("tasks") + "     Microsoft To Do list/get/create/update/delete",
		"              permissions: Tasks.Read, Tasks.ReadWrite",
		"    " + theme.key("onedrive") + "  OneDrive ls/get/put/mkdir/rm",
		"              permissions: Files.Read, Files.ReadWrite",
		"    " + theme.key("groups") + "    Microsoft 365 groups list/get/members (enterprise only)",
		"              permissions: Group.Read.All, GroupMember.Read.All",
		"  Base delegated permissions always included: openid, profile, offline_access, User.Read",
	}
	if defaultDisplay != "" {
		lines = append(lines, "  "+theme.ok("Default from this profile: "+defaultDisplay))
	}
	return strings.Join(lines, "\n")
}

func renderAppOnlyUserGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 5: Default app-only target user (optional)"),
		"  Used by app-only mail/contacts/onedrive commands when --user is not provided.",
		"  Provide a user principal name or user object ID.",
	}
	return strings.Join(lines, "\n")
}

func renderAppOnlySecretGuide(theme authWizardTheme) string {
	lines := []string{
		theme.step("Step 6: App-only client secret"),
		"  You must provide one of:",
		"    - " + theme.key("Client secret env var") + " (recommended, avoids plaintext in shell history)",
		"    - " + theme.key("Client secret value"),
		"  Secret source: Entra -> App registrations -> <app> -> Certificates & secrets.",
		"  App-only commands supported today: mail, contacts, onedrive.",
		"  Calendar/tasks are intentionally blocked in app-only mode.",
	}
	return strings.Join(lines, "\n")
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

type AuthLogoutCmd struct {
	Profile string `name:"profile" help:"Profile name to logout"`
	All     bool   `name:"all" help:"Logout all profiles"`
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

type AuthAccountsCmd struct{}

func (c *AuthAccountsCmd) Run(ctx context.Context) error {
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

	w, done := tableWriter(ctx)
	defer done()
	_, _ = fmt.Fprintln(w, "ACTIVE\tPROFILE\tAUDIENCE\tMODE\tACCOUNT\tTENANT")
	for _, p := range profiles {
		marker := ""
		if p.Active {
			marker = "*"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", marker, p.Name, p.Audience, p.AuthMode, p.Username, p.TenantID)
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
	fmt.Fprintf(os.Stdout, "Active profile set to %s\n", c.Profile)
	return nil
}

type AuthWhoAmICmd struct{}

func (c *AuthWhoAmICmd) Run(ctx context.Context) error {
	store := newProfileStore()
	record, err := store.Resolve(rootProfileOverride(ctx))
	if err != nil {
		return err
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{
			"profile":  record.Name,
			"audience": record.Audience,
			"mode":     record.AuthMode,
			"account":  record.Username,
			"tenant":   record.TenantID,
		})
	}

	fmt.Fprintf(os.Stdout, "Profile: %s\nAudience: %s\nMode: %s\nAccount: %s\nTenant: %s\n", record.Name, record.Audience, record.AuthMode, record.Username, record.TenantID)
	return nil
}

func rootProfileOverride(ctx context.Context) string {
	if flags := rootFlagsFromContext(ctx); flags != nil {
		return strings.TrimSpace(flags.Profile)
	}
	return ""
}
