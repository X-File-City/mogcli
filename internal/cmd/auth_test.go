package cmd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/auth"
	"github.com/jaredpalmer/mogcli/internal/profile"
)

type fakeAuthManager struct {
	delegatedAccount auth.AccountInfo
	delegatedInput   auth.DelegatedLoginInput
	appOnlyInput     auth.AppOnlyLoginInput
}

func (m *fakeAuthManager) LoginDelegated(
	_ context.Context,
	input auth.DelegatedLoginInput,
	_ func(string),
) (auth.AccountInfo, error) {
	m.delegatedInput = input
	return m.delegatedAccount, nil
}

func (m *fakeAuthManager) LoginAppOnly(_ context.Context, input auth.AppOnlyLoginInput) error {
	m.appOnlyInput = input
	return nil
}

func (m *fakeAuthManager) Logout(string) error {
	return nil
}

func TestAuthLoginNonInteractiveRequiresFlags(t *testing.T) {
	// --no-input forces non-interactive path; missing --profile/--audience/--client-id
	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{Mode: "delegated"}
	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected usage error")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected usage ExitError code 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("expected --profile in error, got %v", err)
	}
}

func TestAuthAppWizardRejectsNoInput(t *testing.T) {
	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	err := (&AuthAppWizardCmd{}).Run(ctx)
	if err == nil {
		t.Fatal("expected usage error")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected usage ExitError code 2, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "interactive input") {
		t.Fatalf("expected interactive input guidance, got %v", err)
	}
}

func TestAuthLoginDelegatedRequiresScopeWorkloads(t *testing.T) {
	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:  "work",
		Audience: "enterprise",
		ClientID: "client-id",
		Mode:     "delegated",
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected usage error")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected usage ExitError code 2, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "--scope-workloads") {
		t.Fatalf("expected scope-workloads guidance, got %v", err)
	}
}

func TestAuthLoginDelegatedValidatesScopeWorkloads(t *testing.T) {
	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:        "work",
		Audience:       "enterprise",
		ClientID:       "client-id",
		Mode:           "delegated",
		ScopeWorkloads: "mail,unknown",
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected usage error")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected usage ExitError code 2, got %v", err)
	}
}

func TestAuthLoginDelegatedPersistsWorkloads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeAuthManager{
		delegatedAccount: auth.AccountInfo{
			AccountID: "account-1",
			Username:  "user@example.com",
			TenantID:  "tenant-1",
		},
	}
	oldFactory := newAuthManager
	newAuthManager = func() authManager { return fake }
	t.Cleanup(func() {
		newAuthManager = oldFactory
	})

	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:        "work",
		Audience:       "enterprise",
		ClientID:       "client-id",
		Tenant:         "tenant-1",
		Mode:           "delegated",
		ScopeWorkloads: "mail,contacts",
	}

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	active, err := profile.NewStore().Active()
	if err != nil {
		t.Fatalf("resolve active profile: %v", err)
	}
	if !reflect.DeepEqual(active.DelegatedScopeWorkloads, []string{"mail", "contacts"}) {
		t.Fatalf("unexpected delegated workloads: %#v", active.DelegatedScopeWorkloads)
	}
	if active.AppOnlyUser != "" {
		t.Fatalf("expected empty app-only user, got %q", active.AppOnlyUser)
	}

	wantScopes := []string{
		"openid",
		"profile",
		"offline_access",
		"User.Read",
		"Mail.Read",
		"Mail.Send",
		"Contacts.Read",
		"Contacts.ReadWrite",
	}
	if !reflect.DeepEqual(fake.delegatedInput.Scopes, wantScopes) {
		t.Fatalf("unexpected delegated scopes: got %#v want %#v", fake.delegatedInput.Scopes, wantScopes)
	}
}

func TestAuthLoginDelegatedStoresResolvedTenantID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeAuthManager{
		delegatedAccount: auth.AccountInfo{
			AccountID: "account-1",
			Username:  "user@example.com",
			TenantID:  "11111111-2222-3333-4444-555555555555",
		},
	}
	oldFactory := newAuthManager
	newAuthManager = func() authManager { return fake }
	t.Cleanup(func() {
		newAuthManager = oldFactory
	})

	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:        "work",
		Audience:       "enterprise",
		ClientID:       "client-id",
		Tenant:         "contoso.onmicrosoft.com",
		Mode:           "delegated",
		ScopeWorkloads: "mail",
	}

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	active, err := profile.NewStore().Active()
	if err != nil {
		t.Fatalf("resolve active profile: %v", err)
	}
	if active.TenantID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("expected resolved tenant ID, got %q", active.TenantID)
	}
}

func TestAuthLoginAppOnlyPersistsDefaultUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeAuthManager{}
	oldFactory := newAuthManager
	newAuthManager = func() authManager { return fake }
	t.Cleanup(func() {
		newAuthManager = oldFactory
	})

	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:      "work-app",
		Audience:     "enterprise",
		ClientID:     "client-id",
		Tenant:       "tenant-1",
		Mode:         "app-only",
		ClientSecret: "super-secret",
		AppOnlyUser:  "person@example.com",
	}

	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	active, err := profile.NewStore().Active()
	if err != nil {
		t.Fatalf("resolve active profile: %v", err)
	}
	if active.AppOnlyUser != "person@example.com" {
		t.Fatalf("unexpected app-only user: %q", active.AppOnlyUser)
	}
	if len(active.DelegatedScopeWorkloads) != 0 {
		t.Fatalf("expected no delegated workload defaults in app-only mode, got %#v", active.DelegatedScopeWorkloads)
	}
	if fake.appOnlyInput.Secret != "super-secret" {
		t.Fatalf("unexpected app-only secret value: %q", fake.appOnlyInput.Secret)
	}
}

func TestAuthStatusEmptyState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stdout, stderr, err := captureExecuteOutput(t, []string{"auth", "status"})
	if err != nil {
		t.Fatalf("Execute(auth status) failed: %v\nstderr:\n%s", err, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "No profiles configured") {
		t.Fatalf("expected empty-state guidance, got:\n%s", stdout)
	}
}

func TestAuthLoginValidatesAudience(t *testing.T) {
	ctx := withRootFlags(context.Background(), &RootFlags{NoInput: true})
	cmd := AuthLoginCmd{
		Profile:  "test",
		Audience: "invalid",
		ClientID: "some-id",
		Mode:     "delegated",
	}

	err := cmd.Run(ctx)
	if err == nil {
		t.Fatal("expected error for invalid audience")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected usage ExitError code 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "--audience") {
		t.Fatalf("expected audience error, got %v", err)
	}
}
