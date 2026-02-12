package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/jared/mogcli/internal/config"
	"github.com/jared/mogcli/internal/errfmt"
	"github.com/jared/mogcli/internal/profile"
)

func TestValidateCapability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		record       config.ProfileRecord
		capability   runtimeCapability
		wantError    bool
		wantContains string
	}{
		{
			name: "consumer delegated mail list is allowed",
			record: config.ProfileRecord{
				Audience: profile.AudienceConsumer,
				AuthMode: profile.AuthModeDelegated,
			},
			capability: capMailList,
		},
		{
			name: "enterprise app-only mail list is blocked",
			record: config.ProfileRecord{
				Audience: profile.AudienceEnterprise,
				AuthMode: profile.AuthModeAppOnly,
			},
			capability:   capMailList,
			wantError:    true,
			wantContains: "requires delegated auth",
		},
		{
			name: "enterprise app-only groups list is allowed",
			record: config.ProfileRecord{
				Audience: profile.AudienceEnterprise,
				AuthMode: profile.AuthModeAppOnly,
			},
			capability: capGroupsList,
		},
		{
			name: "consumer delegated groups list is blocked",
			record: config.ProfileRecord{
				Audience: profile.AudienceConsumer,
				AuthMode: profile.AuthModeDelegated,
			},
			capability:   capGroupsList,
			wantError:    true,
			wantContains: "requires an enterprise profile",
		},
		{
			name: "enterprise delegated tasks delete is allowed",
			record: config.ProfileRecord{
				Audience: profile.AudienceEnterprise,
				AuthMode: profile.AuthModeDelegated,
			},
			capability: capTasksDelete,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateCapability(tc.record, tc.capability)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				var userErr *errfmt.UserFacingError
				if !errors.As(err, &userErr) {
					t.Fatalf("expected UserFacingError, got %T", err)
				}
				if tc.wantContains != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantContains)) {
					t.Fatalf("expected error to contain %q, got %q", tc.wantContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
