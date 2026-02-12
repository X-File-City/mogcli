package secrets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/config"
)

const (
	keyringPasswordEnv = "MOG_KEYRING_PASSWORD" //nolint:gosec // env var name, not a credential
	keyringBackendEnv  = "MOG_KEYRING_BACKEND"  //nolint:gosec // env var name, not a credential
)

var errMissingSecretKey = errors.New("missing secret key")

type KeyringBackendInfo struct {
	Value  string
	Source string
}

const (
	keyringBackendSourceEnv     = "env"
	keyringBackendSourceConfig  = "config"
	keyringBackendSourceDefault = "default"
	keyringBackendAuto          = "auto"
)

func ResolveKeyringBackendInfo() (KeyringBackendInfo, error) {
	if v := normalizeKeyringBackend(os.Getenv(keyringBackendEnv)); v != "" {
		return KeyringBackendInfo{Value: v, Source: keyringBackendSourceEnv}, nil
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return KeyringBackendInfo{}, fmt.Errorf("resolve keyring backend: %w", err)
	}

	if cfg.KeyringBackend != "" {
		if v := normalizeKeyringBackend(cfg.KeyringBackend); v != "" {
			return KeyringBackendInfo{Value: v, Source: keyringBackendSourceConfig}, nil
		}
	}

	return KeyringBackendInfo{Value: keyringBackendAuto, Source: keyringBackendSourceDefault}, nil
}

func normalizeKeyringBackend(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func SetSecret(key string, value []byte) error {
	path, err := secretPath(key)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, value, 0o600); err != nil {
		return fmt.Errorf("store secret: %w", err)
	}

	return nil
}

func GetSecret(key string) ([]byte, error) {
	path, err := secretPath(key)
	if err != nil {
		return nil, err
	}

	item, err := os.ReadFile(path) //nolint:gosec // controlled path from secret key
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}

	return item, nil
}

func DeleteSecret(key string) error {
	path, err := secretPath(key)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete secret: %w", err)
	}

	return nil
}

func secretPath(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errMissingSecretKey
	}

	dir, err := config.EnsureKeyringDir()
	if err != nil {
		return "", fmt.Errorf("ensure secret dir: %w", err)
	}

	safe := base64.RawURLEncoding.EncodeToString([]byte(key))
	return filepath.Join(dir, safe), nil
}
