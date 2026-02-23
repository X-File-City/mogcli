package auth

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestLoginDelegatedWAMSuccess(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }
	manager.WAMInvoker = func(_ context.Context, req WAMRequest) (WAMResponse, error) {
		if req.Action != "login" {
			t.Fatalf("expected action login, got %s", req.Action)
		}
		if req.ClientID != "test-client" {
			t.Fatalf("expected client_id test-client, got %s", req.ClientID)
		}
		return WAMResponse{
			AccessToken: "wam-access-token",
			TokenType:   "Bearer",
			Scope:       "openid profile Mail.Read",
			ExpiresIn:   3600,
			IDToken:     idTokenFor(t, "wam-account-1", "wam-tenant-1"),
			IDTokenClaims: &WAMIDClaims{
				OID:               "wam-account-1",
				TID:               "wam-tenant-1",
				PreferredUsername: "user@contoso.com",
			},
		}, nil
	}

	var message string
	info, err := manager.loginDelegatedWAM(context.Background(), DelegatedLoginInput{
		ProfileName: "test-profile",
		ClientID:    "test-client",
		Authority:   "organizations",
		Scopes:      []string{"openid", "profile", "Mail.Read"},
	}, func(msg string) { message = msg })
	if err != nil {
		t.Fatalf("loginDelegatedWAM failed: %v", err)
	}

	if info.AccountID != "wam-account-1" {
		t.Fatalf("expected account_id wam-account-1, got %s", info.AccountID)
	}
	if info.TenantID != "wam-tenant-1" {
		t.Fatalf("expected tenant_id wam-tenant-1, got %s", info.TenantID)
	}
	if info.Username != "user@contoso.com" {
		t.Fatalf("expected username user@contoso.com, got %s", info.Username)
	}
	if message == "" {
		t.Fatal("expected status message to be written")
	}

	// Verify token was cached.
	cache, err := manager.loadToken("test-profile")
	if err != nil {
		t.Fatalf("loadToken failed: %v", err)
	}
	if cache.AccessToken != "wam-access-token" {
		t.Fatalf("expected cached access token wam-access-token, got %s", cache.AccessToken)
	}
}

func TestLoginDelegatedWAMError(t *testing.T) {
	setTempSecretsHome(t)

	manager := NewManager()
	manager.WAMInvoker = func(_ context.Context, _ WAMRequest) (WAMResponse, error) {
		return WAMResponse{}, errors.New("WAM authentication failed (user_cancelled): User cancelled the authentication")
	}

	_, err := manager.loginDelegatedWAM(context.Background(), DelegatedLoginInput{
		ProfileName: "test-profile",
		ClientID:    "test-client",
		Authority:   "organizations",
		Scopes:      BaseDelegatedScopes,
	}, nil)
	if err == nil {
		t.Fatal("expected WAM error")
	}
}

func TestLoginDelegatedWAMFallsBackToIDTokenParsing(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }
	manager.WAMInvoker = func(_ context.Context, _ WAMRequest) (WAMResponse, error) {
		return WAMResponse{
			AccessToken: "wam-token",
			TokenType:   "Bearer",
			Scope:       "openid profile",
			ExpiresIn:   3600,
			IDToken:     idTokenFor(t, "parsed-account", "parsed-tenant"),
			// No IDTokenClaims — forces ID token parsing fallback.
		}, nil
	}

	info, err := manager.loginDelegatedWAM(context.Background(), DelegatedLoginInput{
		ProfileName: "test-profile",
		ClientID:    "test-client",
		Authority:   "organizations",
		Scopes:      BaseDelegatedScopes,
	}, nil)
	if err != nil {
		t.Fatalf("loginDelegatedWAM failed: %v", err)
	}
	if info.AccountID != "parsed-account" {
		t.Fatalf("expected account_id parsed-account, got %s", info.AccountID)
	}
}

func TestAcquireDelegatedTokenWAMSilentSuccess(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }
	manager.WAMInvoker = func(_ context.Context, req WAMRequest) (WAMResponse, error) {
		if req.Action != "acquire_silent" {
			t.Fatalf("expected action acquire_silent, got %s", req.Action)
		}
		if req.AccountID != "account-1" {
			t.Fatalf("expected account_id account-1, got %s", req.AccountID)
		}
		return WAMResponse{
			AccessToken: "wam-refreshed-token",
			TokenType:   "Bearer",
			Scope:       "Mail.Read Mail.Send",
			ExpiresIn:   3600,
			IDToken:     idTokenFor(t, "account-1", "tenant-1"),
		}, nil
	}

	// Seed an expired cached token.
	if err := manager.saveToken("work", TokenCache{
		AccessToken: "expired-token",
		Scope:       "Mail.Read",
		ExpiresAt:   now.Add(-1 * time.Hour),
		IDToken:     idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	token, err := manager.acquireDelegatedTokenWAM(
		context.Background(),
		"work",
		"client-id",
		"organizations",
		[]string{"Mail.Read", "Mail.Send"},
		"account-1",
		"tenant-1",
	)
	if err != nil {
		t.Fatalf("acquireDelegatedTokenWAM failed: %v", err)
	}
	if token != "wam-refreshed-token" {
		t.Fatalf("expected wam-refreshed-token, got %s", token)
	}

	// Verify updated cache.
	cache, err := manager.loadToken("work")
	if err != nil {
		t.Fatalf("loadToken failed: %v", err)
	}
	if cache.AccessToken != "wam-refreshed-token" {
		t.Fatalf("expected cached access token wam-refreshed-token, got %s", cache.AccessToken)
	}
}

func TestAcquireDelegatedTokenWAMSilentFailure(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }
	manager.WAMInvoker = func(_ context.Context, _ WAMRequest) (WAMResponse, error) {
		return WAMResponse{}, errors.New("WAM authentication failed (no_account): No cached account found")
	}

	if err := manager.saveToken("work", TokenCache{
		AccessToken: "expired-token",
		Scope:       "Mail.Read",
		ExpiresAt:   now.Add(-1 * time.Hour),
		IDToken:     idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	_, err := manager.acquireDelegatedTokenWAM(
		context.Background(),
		"work",
		"client-id",
		"organizations",
		[]string{"Mail.Read"},
		"account-1",
		"tenant-1",
	)
	if err == nil {
		t.Fatal("expected WAM silent acquire error")
	}
}

func TestWAMRequestJSON(t *testing.T) {
	req := WAMRequest{
		Action:    "login",
		ClientID:  "test-client",
		Authority: "organizations",
		Scopes:    []string{"openid", "Mail.Read"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WAMRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Action != "login" {
		t.Fatalf("expected action login, got %s", decoded.Action)
	}
	if decoded.ClientID != "test-client" {
		t.Fatalf("expected client_id test-client, got %s", decoded.ClientID)
	}
	if len(decoded.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(decoded.Scopes))
	}
}

func TestWAMResponseJSON(t *testing.T) {
	payload := `{
		"access_token": "at",
		"token_type": "Bearer",
		"scope": "openid profile",
		"expires_in": 3600,
		"id_token": "idt",
		"id_token_claims": {
			"oid": "o1",
			"sub": "s1",
			"preferred_username": "user@test.com",
			"tid": "t1"
		}
	}`

	var resp WAMResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "at" {
		t.Fatalf("expected access_token at, got %s", resp.AccessToken)
	}
	if resp.IDTokenClaims == nil {
		t.Fatal("expected id_token_claims to be non-nil")
	}
	if resp.IDTokenClaims.OID != "o1" {
		t.Fatalf("expected oid o1, got %s", resp.IDTokenClaims.OID)
	}
	if resp.IDTokenClaims.PreferredUsername != "user@test.com" {
		t.Fatalf("expected preferred_username user@test.com, got %s", resp.IDTokenClaims.PreferredUsername)
	}
}

func TestFindWAMExeNotFound(t *testing.T) {
	// findWAMExe looks next to os.Executable(). In test, that's the test binary dir.
	// mog-wam.exe won't exist there, so this should fail with a clear error.
	_, err := findWAMExe()
	if err == nil {
		t.Fatal("expected error when WAM exe is not found")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"a", "b"}, "a"},
		{[]string{"", "b"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"", ""}, ""},
		{[]string{" ", "b"}, "b"},
		{nil, ""},
	}

	for _, tt := range tests {
		result := firstNonEmpty(tt.input...)
		if result != tt.expected {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestUseWAM(t *testing.T) {
	// On macOS/Linux test runners, useWAM should return false.
	// On Windows test runners, it should return true.
	// We just verify it doesn't panic and returns a bool.
	_ = useWAM()
}

// setTempSecretsHome is defined in manager_test.go.
// idTokenFor is defined in manager_test.go.

// Ensure the test helper uses a clean temp dir for secrets, same as manager_test.go.
func init() {
	_ = filepath.Join // ensure import is used
}
