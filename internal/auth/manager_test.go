package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jaredpalmer/mogcli/internal/errfmt"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAcquireDelegatedTokenReusesCachedTokenWhenScopesSatisfied(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }

	profileName := "work"
	if err := manager.saveToken(profileName, TokenCache{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-token",
		Scope:        "Mail.Read Mail.Send",
		ExpiresAt:    now.Add(1 * time.Hour),
		IDToken:      idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	token, err := manager.AcquireDelegatedToken(
		context.Background(),
		profileName,
		"client-id",
		"organizations",
		[]string{"Mail.Read"},
		"account-1",
		"tenant-1",
	)
	if err != nil {
		t.Fatalf("AcquireDelegatedToken failed: %v", err)
	}
	if token != "cached-token" {
		t.Fatalf("expected cached token, got %q", token)
	}
}

func TestAcquireDelegatedTokenRefreshesWhenRequiredScopeMissing(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	refreshCalls := 0

	manager := NewManager()
	manager.now = func() time.Time { return now }
	manager.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			refreshCalls++
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if !strings.HasSuffix(req.URL.Path, "/oauth2/v2.0/token") {
				t.Fatalf("unexpected token endpoint path: %s", req.URL.Path)
			}

			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse request body: %v", err)
			}
			if got := values.Get("scope"); got != "Mail.Send" {
				t.Fatalf("expected refresh scope Mail.Send, got %q", got)
			}

			payload := `{"token_type":"Bearer","scope":"Mail.Send","expires_in":3600,"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(payload)),
			}, nil
		}),
	}

	profileName := "work"
	if err := manager.saveToken(profileName, TokenCache{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-token",
		Scope:        "Mail.Read",
		ExpiresAt:    now.Add(1 * time.Hour),
		IDToken:      idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	token, err := manager.AcquireDelegatedToken(
		context.Background(),
		profileName,
		"client-id",
		"organizations",
		[]string{"Mail.Send"},
		"account-1",
		"tenant-1",
	)
	if err != nil {
		t.Fatalf("AcquireDelegatedToken failed: %v", err)
	}
	if token != "refreshed-token" {
		t.Fatalf("expected refreshed token, got %q", token)
	}
	if refreshCalls != 1 {
		t.Fatalf("expected one refresh call, got %d", refreshCalls)
	}
}

func TestAcquireDelegatedTokenPurgesCacheOnAccountMismatch(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }

	profileName := "work"
	if err := manager.saveToken(profileName, TokenCache{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-token",
		Scope:        "Mail.Read",
		ExpiresAt:    now.Add(1 * time.Hour),
		IDToken:      idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	_, err := manager.AcquireDelegatedToken(
		context.Background(),
		profileName,
		"client-id",
		"organizations",
		[]string{"Mail.Read"},
		"different-account",
		"tenant-1",
	)
	if err == nil {
		t.Fatal("expected mismatch error")
	}

	var userErr *errfmt.UserFacingError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserFacingError, got %T", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "does not match the saved account") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
	if _, loadErr := manager.loadToken(profileName); loadErr == nil {
		t.Fatal("expected cached token to be purged")
	}
}

func TestAcquireDelegatedTokenPurgesCacheOnTenantMismatch(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }

	profileName := "work"
	if err := manager.saveToken(profileName, TokenCache{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-token",
		Scope:        "Mail.Read",
		ExpiresAt:    now.Add(1 * time.Hour),
		IDToken:      idTokenFor(t, "account-1", "tenant-1"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	_, err := manager.AcquireDelegatedToken(
		context.Background(),
		profileName,
		"client-id",
		"organizations",
		[]string{"Mail.Read"},
		"account-1",
		"different-tenant",
	)
	if err == nil {
		t.Fatal("expected mismatch error")
	}

	var userErr *errfmt.UserFacingError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserFacingError, got %T", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "does not match the saved tenant") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
	if _, loadErr := manager.loadToken(profileName); loadErr == nil {
		t.Fatal("expected cached token to be purged")
	}
}

func TestAcquireDelegatedTokenAcceptsTenantDomainExpectation(t *testing.T) {
	setTempSecretsHome(t)

	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	manager := NewManager()
	manager.now = func() time.Time { return now }

	profileName := "work"
	if err := manager.saveToken(profileName, TokenCache{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-token",
		Scope:        "Mail.Read",
		ExpiresAt:    now.Add(1 * time.Hour),
		IDToken:      idTokenFor(t, "account-1", "11111111-2222-3333-4444-555555555555"),
	}); err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	token, err := manager.AcquireDelegatedToken(
		context.Background(),
		profileName,
		"client-id",
		"organizations",
		[]string{"Mail.Read"},
		"account-1",
		"contoso.onmicrosoft.com",
	)
	if err != nil {
		t.Fatalf("AcquireDelegatedToken failed: %v", err)
	}
	if token != "cached-token" {
		t.Fatalf("expected cached token, got %q", token)
	}
	if _, err := manager.loadToken(profileName); err != nil {
		t.Fatalf("expected cached token to remain, got %v", err)
	}
}

func setTempSecretsHome(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("MOG_KEYRING_BACKEND", "file")
}

func idTokenFor(t *testing.T, accountID string, tenantID string) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("encode header: %v", err)
	}
	payloadBytes, err := json.Marshal(map[string]string{
		"oid": accountID,
		"tid": tenantID,
		"sub": accountID,
	})
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + "."
}
