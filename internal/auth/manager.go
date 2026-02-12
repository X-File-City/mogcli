package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jaredpalmer/mogcli/internal/errfmt"
	"github.com/jaredpalmer/mogcli/internal/secrets"
)

var (
	ErrMissingClientID     = errors.New("missing client ID")
	ErrMissingAuthority    = errors.New("missing authority")
	ErrMissingRefreshToken = errors.New("missing refresh token")
	ErrMissingSecret       = errors.New("missing client secret")
)

type Manager struct {
	HTTPClient *http.Client
	now        func() time.Time
}

func NewManager() *Manager {
	return &Manager{
		HTTPClient: http.DefaultClient,
		now:        time.Now,
	}
}

func (m *Manager) httpClient() *http.Client {
	if m.HTTPClient != nil {
		return m.HTTPClient
	}

	return http.DefaultClient
}

func (m *Manager) nowUTC() time.Time {
	if m.now != nil {
		return m.now().UTC()
	}

	return time.Now().UTC()
}

func tokenCacheKey(profileName string) string {
	return "mog:cache:" + strings.TrimSpace(profileName)
}

func appSecretKey(profileName string) string {
	return "mog:appsecret:" + strings.TrimSpace(profileName)
}

func graphDefaultScope() string {
	return "https://graph.microsoft.com/.default"
}

func normalizeAuthority(authority string) string {
	authority = strings.TrimSpace(authority)
	authority = strings.TrimPrefix(authority, "https://login.microsoftonline.com/")
	authority = strings.Trim(authority, "/")
	if authority == "" {
		return "organizations"
	}

	return authority
}

func endpoint(authority string, path string) string {
	return "https://login.microsoftonline.com/" + normalizeAuthority(authority) + path
}

type deviceCodeResponse struct {
	UserCode                string `json:"user_code"`
	DeviceCode              string `json:"device_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Message                 string `json:"message"`
}

type tokenResponse struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (m *Manager) LoginDelegated(ctx context.Context, input DelegatedLoginInput, writeMessage func(string)) (AccountInfo, error) {
	if strings.TrimSpace(input.ClientID) == "" {
		return AccountInfo{}, ErrMissingClientID
	}
	if strings.TrimSpace(input.Authority) == "" {
		return AccountInfo{}, ErrMissingAuthority
	}
	if len(input.Scopes) == 0 {
		input.Scopes = BaseDelegatedScopes
	}
	input.Scopes = normalizeScopes(input.Scopes)
	if len(input.Scopes) == 0 {
		input.Scopes = BaseDelegatedScopes
	}

	deviceReq := url.Values{}
	deviceReq.Set("client_id", input.ClientID)
	deviceReq.Set("scope", strings.Join(input.Scopes, " "))

	body, err := m.doForm(ctx, endpoint(input.Authority, "/oauth2/v2.0/devicecode"), deviceReq)
	if err != nil {
		return AccountInfo{}, err
	}

	var dcr deviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return AccountInfo{}, fmt.Errorf("decode device code response: %w", err)
	}

	if dcr.DeviceCode == "" {
		return AccountInfo{}, errors.New("device code response missing device_code")
	}

	if writeMessage != nil {
		msg := strings.TrimSpace(dcr.Message)
		if msg == "" {
			if dcr.VerificationURIComplete != "" {
				msg = fmt.Sprintf("Open %s", dcr.VerificationURIComplete)
			} else {
				msg = fmt.Sprintf("Open %s and enter code %s", dcr.VerificationURI, dcr.UserCode)
			}
		}
		writeMessage(msg)
	}

	interval := time.Duration(dcr.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expires := m.nowUTC().Add(time.Duration(dcr.ExpiresIn) * time.Second)
	if dcr.ExpiresIn <= 0 {
		expires = m.nowUTC().Add(15 * time.Minute)
	}

	for m.nowUTC().Before(expires) {
		select {
		case <-ctx.Done():
			return AccountInfo{}, fmt.Errorf("device code flow cancelled: %w", ctx.Err())
		default:
		}

		tokReq := url.Values{}
		tokReq.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		tokReq.Set("client_id", input.ClientID)
		tokReq.Set("device_code", dcr.DeviceCode)

		resultBody, status, err := m.doFormWithStatus(ctx, endpoint(input.Authority, "/oauth2/v2.0/token"), tokReq)
		if err != nil {
			return AccountInfo{}, err
		}

		if status == http.StatusOK {
			var tr tokenResponse
			if err := json.Unmarshal(resultBody, &tr); err != nil {
				return AccountInfo{}, fmt.Errorf("decode token response: %w", err)
			}
			cache := TokenCache{
				AccessToken:  tr.AccessToken,
				RefreshToken: tr.RefreshToken,
				TokenType:    tr.TokenType,
				Scope:        tr.Scope,
				ExpiresAt:    m.nowUTC().Add(time.Duration(tr.ExpiresIn) * time.Second),
				IDToken:      tr.IDToken,
			}
			if err := m.saveToken(input.ProfileName, cache); err != nil {
				return AccountInfo{}, err
			}

			claims, _ := parseIDClaims(tr.IDToken)
			return accountFromClaims(claims), nil
		}

		var oe oauthErrorResponse
		_ = json.Unmarshal(resultBody, &oe)
		switch oe.Error {
		case "authorization_pending":
			if err := sleepContext(ctx, interval); err != nil {
				return AccountInfo{}, err
			}
			continue
		case "slow_down":
			interval += 3 * time.Second
			if err := sleepContext(ctx, interval); err != nil {
				return AccountInfo{}, err
			}
			continue
		case "expired_token":
			return AccountInfo{}, errors.New("device code expired; retry login")
		default:
			return AccountInfo{}, fmt.Errorf("token exchange failed (%s): %s", oe.Error, strings.TrimSpace(oe.ErrorDescription))
		}
	}

	return AccountInfo{}, errors.New("device code flow timed out")
}

func (m *Manager) LoginAppOnly(ctx context.Context, input AppOnlyLoginInput) error {
	if strings.TrimSpace(input.ClientID) == "" {
		return ErrMissingClientID
	}
	if strings.TrimSpace(input.Authority) == "" {
		return ErrMissingAuthority
	}
	if strings.TrimSpace(input.Secret) == "" {
		return ErrMissingSecret
	}

	if err := secrets.SetSecret(appSecretKey(input.ProfileName), []byte(input.Secret)); err != nil {
		return fmt.Errorf("store client secret: %w", err)
	}

	_, err := m.AcquireAppOnlyToken(ctx, input.ProfileName, input.ClientID, input.Authority)
	return err
}

func (m *Manager) AcquireDelegatedToken(
	ctx context.Context,
	profileName string,
	clientID string,
	authority string,
	scopes []string,
	expectedAccountID string,
	expectedTenantID string,
) (string, error) {
	cache, err := m.loadToken(profileName)
	if err != nil {
		return "", err
	}

	if err := m.validateCachedDelegatedIdentity(profileName, cache, expectedAccountID, expectedTenantID); err != nil {
		return "", err
	}

	requiredScopes := normalizeScopes(scopes)
	if len(requiredScopes) == 0 {
		requiredScopes = BaseDelegatedScopes
	}

	if cache.AccessToken != "" &&
		cache.ExpiresAt.After(m.nowUTC().Add(30*time.Second)) &&
		scopeStringCoversRequiredScopes(cache.Scope, requiredScopes) {
		return cache.AccessToken, nil
	}

	if strings.TrimSpace(cache.RefreshToken) == "" {
		return "", ErrMissingRefreshToken
	}

	req := url.Values{}
	req.Set("grant_type", "refresh_token")
	req.Set("client_id", clientID)
	req.Set("refresh_token", cache.RefreshToken)
	req.Set("scope", strings.Join(requiredScopes, " "))

	body, status, err := m.doFormWithStatus(ctx, endpoint(authority, "/oauth2/v2.0/token"), req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		var oe oauthErrorResponse
		_ = json.Unmarshal(body, &oe)
		return "", fmt.Errorf("refresh token failed (%s): %s", oe.Error, strings.TrimSpace(oe.ErrorDescription))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decode refresh token response: %w", err)
	}

	if tr.RefreshToken == "" {
		tr.RefreshToken = cache.RefreshToken
	}
	if strings.TrimSpace(tr.Scope) == "" {
		tr.Scope = cache.Scope
	}
	updated := TokenCache{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scope:        tr.Scope,
		ExpiresAt:    m.nowUTC().Add(time.Duration(tr.ExpiresIn) * time.Second),
		IDToken:      tr.IDToken,
	}
	if updated.IDToken == "" {
		updated.IDToken = cache.IDToken
	}
	if err := m.validateCachedDelegatedIdentity(profileName, updated, expectedAccountID, expectedTenantID); err != nil {
		return "", err
	}

	if err := m.saveToken(profileName, updated); err != nil {
		return "", err
	}

	return updated.AccessToken, nil
}

func (m *Manager) AcquireAppOnlyToken(ctx context.Context, profileName string, clientID string, authority string) (string, error) {
	secret, err := secrets.GetSecret(appSecretKey(profileName))
	if err != nil {
		return "", fmt.Errorf("read client secret: %w", err)
	}
	if len(secret) == 0 {
		return "", ErrMissingSecret
	}

	req := url.Values{}
	req.Set("grant_type", "client_credentials")
	req.Set("client_id", clientID)
	req.Set("client_secret", string(secret))
	req.Set("scope", graphDefaultScope())

	body, status, err := m.doFormWithStatus(ctx, endpoint(authority, "/oauth2/v2.0/token"), req)
	if err != nil {
		return "", err
	}

	if status != http.StatusOK {
		var oe oauthErrorResponse
		_ = json.Unmarshal(body, &oe)
		return "", fmt.Errorf("app-only token request failed (%s): %s", oe.Error, strings.TrimSpace(oe.ErrorDescription))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decode app-only token response: %w", err)
	}

	return tr.AccessToken, nil
}

func (m *Manager) ReadAccount(profileName string) (AccountInfo, error) {
	cache, err := m.loadToken(profileName)
	if err != nil {
		return AccountInfo{}, err
	}

	claims, err := parseIDClaims(cache.IDToken)
	if err != nil {
		return AccountInfo{}, err
	}

	return accountFromClaims(claims), nil
}

func (m *Manager) Logout(profileName string) error {
	if err := secrets.DeleteSecret(tokenCacheKey(profileName)); err != nil {
		return err
	}
	if err := secrets.DeleteSecret(appSecretKey(profileName)); err != nil {
		return err
	}

	return nil
}

func (m *Manager) validateCachedDelegatedIdentity(
	profileName string,
	cache TokenCache,
	expectedAccountID string,
	expectedTenantID string,
) error {
	expectedAccountID = strings.TrimSpace(expectedAccountID)
	expectedTenantID = strings.TrimSpace(expectedTenantID)
	if expectedAccountID == "" && expectedTenantID == "" {
		return nil
	}

	claims, err := parseIDClaims(cache.IDToken)
	if err != nil {
		if purgeErr := m.purgeDelegatedTokenCache(profileName); purgeErr != nil {
			return purgeErr
		}
		return errfmt.NewUserFacingError(
			fmt.Sprintf(
				"cached delegated login for profile %s is invalid. Run `mog auth login --profile %s ...` to sign in again.",
				profileName,
				profileName,
			),
			err,
		)
	}

	account := accountFromClaims(claims)
	if expectedAccountID != "" && !strings.EqualFold(strings.TrimSpace(account.AccountID), expectedAccountID) {
		if purgeErr := m.purgeDelegatedTokenCache(profileName); purgeErr != nil {
			return purgeErr
		}
		return errfmt.NewUserFacingError(
			fmt.Sprintf(
				"cached delegated login for profile %s does not match the saved account. Run `mog auth login --profile %s ...` to sign in again.",
				profileName,
				profileName,
			),
			nil,
		)
	}

	if expectedTenantID != "" && !tenantIDsMatch(expectedTenantID, account.TenantID) {
		if purgeErr := m.purgeDelegatedTokenCache(profileName); purgeErr != nil {
			return purgeErr
		}
		return errfmt.NewUserFacingError(
			fmt.Sprintf(
				"cached delegated login for profile %s does not match the saved tenant. Run `mog auth login --profile %s ...` to sign in again.",
				profileName,
				profileName,
			),
			nil,
		)
	}

	return nil
}

func tenantIDsMatch(expectedTenantID string, actualTenantID string) bool {
	expected := strings.TrimSpace(expectedTenantID)
	actual := strings.TrimSpace(actualTenantID)

	if expected == "" {
		return true
	}
	if strings.EqualFold(expected, actual) {
		return true
	}
	if looksLikeGUID(expected) {
		return false
	}
	if !looksLikeTenantDomain(expected) {
		return false
	}

	// Tenant domains are valid profile inputs, but ID tokens expose tenant IDs in the
	// tid claim. If expected is a domain, we can only require a non-empty tenant claim.
	return actual != ""
}

func looksLikeGUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}

	for i := 0; i < len(value); i++ {
		switch i {
		case 8, 13, 18, 23:
			if value[i] != '-' {
				return false
			}
		default:
			if !isHexByte(value[i]) {
				return false
			}
		}
	}

	return true
}

func isHexByte(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

func looksLikeTenantDomain(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.Contains(value, ".") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}

	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' {
			continue
		}
		return false
	}

	return true
}

func (m *Manager) purgeDelegatedTokenCache(profileName string) error {
	if err := secrets.DeleteSecret(tokenCacheKey(profileName)); err != nil {
		return fmt.Errorf("purge delegated token cache: %w", err)
	}
	return nil
}

func (m *Manager) saveToken(profileName string, cache TokenCache) error {
	payload, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("encode token cache: %w", err)
	}

	if err := secrets.SetSecret(tokenCacheKey(profileName), payload); err != nil {
		return fmt.Errorf("store token cache: %w", err)
	}

	return nil
}

func (m *Manager) loadToken(profileName string) (TokenCache, error) {
	payload, err := secrets.GetSecret(tokenCacheKey(profileName))
	if err != nil {
		return TokenCache{}, fmt.Errorf("read token cache: %w", err)
	}

	var cache TokenCache
	if err := json.Unmarshal(payload, &cache); err != nil {
		return TokenCache{}, fmt.Errorf("decode token cache: %w", err)
	}

	return cache, nil
}

func parseIDClaims(idToken string) (idClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return idClaims{}, errors.New("invalid id token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return idClaims{}, fmt.Errorf("decode id token payload: %w", err)
	}

	var claims idClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return idClaims{}, fmt.Errorf("decode id token claims: %w", err)
	}

	return claims, nil
}

func accountFromClaims(claims idClaims) AccountInfo {
	accountID := strings.TrimSpace(claims.OID)
	if accountID == "" {
		accountID = strings.TrimSpace(claims.Sub)
	}

	username := strings.TrimSpace(claims.PreferredUsername)
	if username == "" {
		username = strings.TrimSpace(claims.Email)
	}
	if username == "" {
		username = strings.TrimSpace(claims.UPN)
	}

	return AccountInfo{
		AccountID: accountID,
		Username:  username,
		TenantID:  strings.TrimSpace(claims.TID),
	}
}

func (m *Manager) doForm(ctx context.Context, endpoint string, form url.Values) ([]byte, error) {
	body, status, err := m.doFormWithStatus(ctx, endpoint, form)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		var oe oauthErrorResponse
		_ = json.Unmarshal(body, &oe)
		return nil, fmt.Errorf("oauth request failed (%s): %s", oe.Error, strings.TrimSpace(oe.ErrorDescription))
	}

	return body, nil
}

func (m *Manager) doFormWithStatus(ctx context.Context, endpoint string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("build oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("oauth request: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read oauth response: %w", err)
	}

	return b, resp.StatusCode, nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
