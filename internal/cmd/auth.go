package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jared/mogcli/internal/auth"
	"github.com/jared/mogcli/internal/config"
	"github.com/jared/mogcli/internal/outfmt"
	"github.com/jared/mogcli/internal/profile"
)

type AuthCmd struct {
	Login    AuthLoginCmd    `cmd:"" help:"Login and save profile credentials"`
	Logout   AuthLogoutCmd   `cmd:"" help:"Logout and clear saved credentials"`
	Accounts AuthAccountsCmd `cmd:"" help:"List saved profiles"`
	Use      AuthUseCmd      `cmd:"" help:"Set active profile"`
	WhoAmI   AuthWhoAmICmd   `cmd:"" aliases:"whoami" help:"Show active profile and account"`
}

type AuthLoginCmd struct {
	Profile         string `name:"profile" required:"" help:"Profile name"`
	Audience        string `name:"audience" required:"" enum:"consumer,enterprise" help:"Account audience"`
	ClientID        string `name:"client-id" required:"" help:"App registration client ID"`
	Tenant          string `name:"tenant" help:"Tenant ID or domain (enterprise)"`
	Authority       string `name:"authority" help:"Authority segment (consumers|organizations|common|tenant)"`
	Mode            string `name:"mode" default:"delegated" enum:"delegated,app-only" help:"Auth mode"`
	ClientSecret    string `name:"client-secret" help:"Client secret for app-only mode"`
	ClientSecretEnv string `name:"client-secret-env" help:"Environment variable containing client secret"`
}

func (c *AuthLoginCmd) Run(ctx context.Context) error {
	mode := strings.TrimSpace(c.Mode)
	if mode == "app-only" {
		mode = profile.AuthModeAppOnly
	}

	authority := defaultAuthority(c.Audience, c.Tenant, c.Authority)
	manager := auth.NewManager()
	store := profile.NewStore()

	record := config.ProfileRecord{
		Name:      strings.TrimSpace(c.Profile),
		Audience:  strings.TrimSpace(c.Audience),
		ClientID:  strings.TrimSpace(c.ClientID),
		Authority: authority,
		TenantID:  strings.TrimSpace(c.Tenant),
		AuthMode:  mode,
	}

	switch mode {
	case profile.AuthModeAppOnly:
		if !strings.EqualFold(c.Audience, profile.AudienceEnterprise) {
			return usage("app-only mode requires --audience enterprise")
		}

		secret := strings.TrimSpace(c.ClientSecret)
		if secret == "" && strings.TrimSpace(c.ClientSecretEnv) != "" {
			secret = strings.TrimSpace(os.Getenv(strings.TrimSpace(c.ClientSecretEnv)))
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
	default:
		account, err := manager.LoginDelegated(ctx, auth.DelegatedLoginInput{
			ProfileName: record.Name,
			Audience:    record.Audience,
			ClientID:    record.ClientID,
			Authority:   record.Authority,
			TenantID:    record.TenantID,
			Scopes:      auth.DefaultDelegatedScopes,
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

		record.AccountID = account.AccountID
		record.Username = account.Username
		if record.TenantID == "" {
			record.TenantID = account.TenantID
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

type AuthLogoutCmd struct {
	Profile string `name:"profile" help:"Profile name to logout"`
	All     bool   `name:"all" help:"Logout all profiles"`
}

func (c *AuthLogoutCmd) Run(ctx context.Context) error {
	store := profile.NewStore()
	manager := auth.NewManager()

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
	store := profile.NewStore()
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
	store := profile.NewStore()
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
	store := profile.NewStore()
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
