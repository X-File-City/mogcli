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

type runtimeCapability string

const (
	capMailList       runtimeCapability = "mail.list"
	capMailGet        runtimeCapability = "mail.get"
	capMailSend       runtimeCapability = "mail.send"
	capCalendarList   runtimeCapability = "calendar.list"
	capCalendarGet    runtimeCapability = "calendar.get"
	capCalendarCreate runtimeCapability = "calendar.create"
	capCalendarUpdate runtimeCapability = "calendar.update"
	capCalendarDelete runtimeCapability = "calendar.delete"
	capContactsList   runtimeCapability = "contacts.list"
	capContactsGet    runtimeCapability = "contacts.get"
	capContactsCreate runtimeCapability = "contacts.create"
	capContactsUpdate runtimeCapability = "contacts.update"
	capContactsDelete runtimeCapability = "contacts.delete"
	capGroupsList     runtimeCapability = "groups.list"
	capGroupsGet      runtimeCapability = "groups.get"
	capGroupsMembers  runtimeCapability = "groups.members"
	capTasksLists     runtimeCapability = "tasks.lists"
	capTasksList      runtimeCapability = "tasks.list"
	capTasksGet       runtimeCapability = "tasks.get"
	capTasksCreate    runtimeCapability = "tasks.create"
	capTasksUpdate    runtimeCapability = "tasks.update"
	capTasksComplete  runtimeCapability = "tasks.complete"
	capTasksDelete    runtimeCapability = "tasks.delete"
	capOneDriveLS     runtimeCapability = "onedrive.ls"
	capOneDriveGet    runtimeCapability = "onedrive.get"
	capOneDrivePut    runtimeCapability = "onedrive.put"
	capOneDriveMkdir  runtimeCapability = "onedrive.mkdir"
	capOneDriveRM     runtimeCapability = "onedrive.rm"
)

const (
	enterpriseOnlyProfileMessage = "This command requires an enterprise profile. Use `mog auth use <enterprise-profile>`."
	appOnlyEnterpriseOnlyMessage = "App-only mode is enterprise-only."
	meDelegatedOnlyMessage       = "This command requires delegated auth because it uses `/me` Microsoft Graph endpoints. Use a delegated profile or re-login without `--mode app-only`."
)

type capabilityRule struct {
	allowedAudiences           []string
	allowedAuthModes           []string
	unsupportedAudienceMessage string
	unsupportedAuthModeMessage string
}

var capabilityRules = map[runtimeCapability]capabilityRule{
	capMailList:       delegatedMeRule(),
	capMailGet:        delegatedMeRule(),
	capMailSend:       delegatedMeRule(),
	capCalendarList:   delegatedMeRule(),
	capCalendarGet:    delegatedMeRule(),
	capCalendarCreate: delegatedMeRule(),
	capCalendarUpdate: delegatedMeRule(),
	capCalendarDelete: delegatedMeRule(),
	capContactsList:   delegatedMeRule(),
	capContactsGet:    delegatedMeRule(),
	capContactsCreate: delegatedMeRule(),
	capContactsUpdate: delegatedMeRule(),
	capContactsDelete: delegatedMeRule(),
	capGroupsList:     groupsRule(),
	capGroupsGet:      groupsRule(),
	capGroupsMembers:  groupsRule(),
	capTasksLists:     delegatedMeRule(),
	capTasksList:      delegatedMeRule(),
	capTasksGet:       delegatedMeRule(),
	capTasksCreate:    delegatedMeRule(),
	capTasksUpdate:    delegatedMeRule(),
	capTasksComplete:  delegatedMeRule(),
	capTasksDelete:    delegatedMeRule(),
	capOneDriveLS:     delegatedMeRule(),
	capOneDriveGet:    delegatedMeRule(),
	capOneDrivePut:    delegatedMeRule(),
	capOneDriveMkdir:  delegatedMeRule(),
	capOneDriveRM:     delegatedMeRule(),
}

func delegatedMeRule() capabilityRule {
	return capabilityRule{
		allowedAudiences: []string{
			profile.AudienceConsumer,
			profile.AudienceEnterprise,
		},
		allowedAuthModes: []string{
			profile.AuthModeDelegated,
		},
		unsupportedAuthModeMessage: meDelegatedOnlyMessage,
	}
}

func groupsRule() capabilityRule {
	return capabilityRule{
		allowedAudiences: []string{
			profile.AudienceEnterprise,
		},
		allowedAuthModes: []string{
			profile.AuthModeDelegated,
			profile.AuthModeAppOnly,
		},
		unsupportedAudienceMessage: enterpriseOnlyProfileMessage,
	}
}

func resolveRuntime(ctx context.Context, capability runtimeCapability) (*runtimeEnv, error) {
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

	if err := validateCapability(activeProfile, capability); err != nil {
		return nil, err
	}

	authManager := auth.NewManager()
	graphClient := graph.NewClient(func(ctx context.Context, scopes []string) (string, error) {
		switch normalizeAuthMode(activeProfile.AuthMode) {
		case profile.AuthModeAppOnly:
			if !strings.EqualFold(activeProfile.Audience, profile.AudienceEnterprise) {
				return "", errfmt.NewUserFacingError(appOnlyEnterpriseOnlyMessage, nil)
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

func validateCapability(activeProfile config.ProfileRecord, capability runtimeCapability) error {
	rule, ok := capabilityRules[capability]
	if !ok {
		return fmt.Errorf("unknown runtime capability: %s", capability)
	}

	audience := normalizeAudience(activeProfile.Audience)
	authMode := normalizeAuthMode(activeProfile.AuthMode)

	if authMode == profile.AuthModeAppOnly && audience != profile.AudienceEnterprise {
		return errfmt.NewUserFacingError(appOnlyEnterpriseOnlyMessage, nil)
	}

	if !containsRuleValue(rule.allowedAudiences, audience) {
		message := strings.TrimSpace(rule.unsupportedAudienceMessage)
		if message == "" {
			message = "This command is not supported for the active profile audience."
		}

		return errfmt.NewUserFacingError(message, nil)
	}

	if !containsRuleValue(rule.allowedAuthModes, authMode) {
		message := strings.TrimSpace(rule.unsupportedAuthModeMessage)
		if message == "" {
			message = "This command is not supported for the active profile auth mode."
		}

		return errfmt.NewUserFacingError(message, nil)
	}

	return nil
}

func normalizeAudience(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeAuthMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if normalized == "" {
		return profile.AuthModeDelegated
	}

	return normalized
}

func containsRuleValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
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
