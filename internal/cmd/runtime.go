package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/jared/mogcli/internal/auth"
	"github.com/jared/mogcli/internal/config"
	"github.com/jared/mogcli/internal/errfmt"
	"github.com/jared/mogcli/internal/graph"
	"github.com/jared/mogcli/internal/profile"
)

type runtimeEnv struct {
	Profile config.ProfileRecord
	Auth    *auth.Manager
	Graph   *graph.Client
}

func resolveRuntime(ctx context.Context, enterpriseOnly bool) (*runtimeEnv, error) {
	flags := rootFlagsFromContext(ctx)
	profileOverride := ""
	if flags != nil {
		profileOverride = flags.Profile
	}

	store := profile.NewStore()
	activeProfile, err := store.Resolve(profileOverride)
	if err != nil {
		return nil, fmt.Errorf("resolve active profile: %w", err)
	}

	if enterpriseOnly && strings.EqualFold(activeProfile.Audience, profile.AudienceConsumer) {
		return nil, errfmt.NewUserFacingError(
			"This command requires an enterprise profile. Use `mog auth use <enterprise-profile>`.",
			nil,
		)
	}

	authManager := auth.NewManager()
	graphClient := graph.NewClient(func(ctx context.Context, scopes []string) (string, error) {
		switch strings.ToLower(strings.TrimSpace(activeProfile.AuthMode)) {
		case profile.AuthModeAppOnly:
			if !strings.EqualFold(activeProfile.Audience, profile.AudienceEnterprise) {
				return "", errfmt.NewUserFacingError("App-only mode is enterprise-only.", nil)
			}
			return authManager.AcquireAppOnlyToken(ctx, activeProfile.Name, activeProfile.ClientID, activeProfile.Authority)
		default:
			return authManager.AcquireDelegatedToken(ctx, activeProfile.Name, activeProfile.ClientID, activeProfile.Authority, scopes)
		}
	})

	return &runtimeEnv{
		Profile: activeProfile,
		Auth:    authManager,
		Graph:   graphClient,
	}, nil
}

func defaultAuthority(audience string, tenant string, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}

	switch strings.ToLower(strings.TrimSpace(audience)) {
	case profile.AudienceConsumer:
		return "consumers"
	default:
		if strings.TrimSpace(tenant) != "" {
			return strings.TrimSpace(tenant)
		}
		return "organizations"
	}
}
