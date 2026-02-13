package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProfileRecord struct {
	Name                    string   `json:"name"`
	Audience                string   `json:"audience"`
	ClientID                string   `json:"client_id"`
	Authority               string   `json:"authority,omitempty"`
	TenantID                string   `json:"tenant_id,omitempty"`
	AccountID               string   `json:"account_id,omitempty"`
	Username                string   `json:"username,omitempty"`
	AuthMode                string   `json:"auth_mode,omitempty"`
	DelegatedScopeWorkloads []string `json:"delegated_scope_workloads,omitempty"`
	AppOnlyUser             string   `json:"app_only_user,omitempty"`
	Active                  bool     `json:"active,omitempty"`
}

type File struct {
	KeyringBackend  string                   `json:"keyring_backend,omitempty"`
	DefaultTimezone string                   `json:"default_timezone,omitempty"`
	AccountAliases  map[string]string        `json:"account_aliases,omitempty"`
	AccountClients  map[string]string        `json:"account_clients,omitempty"`
	ClientDomains   map[string]string        `json:"client_domains,omitempty"`
	Profiles        map[string]ProfileRecord `json:"profiles,omitempty"`
	ActiveProfile   string                   `json:"active_profile,omitempty"`
}

var (
	configLockRetryInterval = 10 * time.Millisecond
	configLockTimeout       = 5 * time.Second
)

func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func WriteConfig(cfg File) error {
	return withConfigLock(func() error {
		path, err := ConfigPath()
		if err != nil {
			return err
		}

		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("encode config json: %w", err)
		}

		b = append(b, '\n')
		tmp := path + ".tmp"

		if err := os.WriteFile(tmp, b, 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		if err := os.Rename(tmp, path); err != nil {
			return fmt.Errorf("commit config: %w", err)
		}

		return nil
	})
}

func ConfigExists() (bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return false, err
	}

	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}

		return false, fmt.Errorf("stat config: %w", statErr)
	}

	return true, nil
}

func ReadConfig() (File, error) {
	var cfg File
	err := withConfigLock(func() error {
		path, pathErr := ConfigPath()
		if pathErr != nil {
			return pathErr
		}

		b, readErr := os.ReadFile(path) //nolint:gosec // config file path
		if readErr != nil {
			if os.IsNotExist(readErr) {
				cfg = File{}
				return nil
			}

			return fmt.Errorf("read config: %w", readErr)
		}

		if unmarshalErr := json.Unmarshal(b, &cfg); unmarshalErr != nil {
			return fmt.Errorf("parse config %s: %w", path, unmarshalErr)
		}

		return nil
	})
	if err != nil {
		return File{}, err
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileRecord{}
	}

	return cfg, nil
}

func withConfigLock(fn func() error) error {
	if fn == nil {
		return nil
	}

	dir, err := EnsureDir()
	if err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}
	lockPath := filepath.Join(dir, "config.lock")
	deadline := time.Now().Add(configLockTimeout)

	for {
		lockFile, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if openErr == nil {
			defer func() {
				_ = lockFile.Close()
				_ = os.Remove(lockPath)
			}()
			return fn()
		}

		if !os.IsExist(openErr) {
			return fmt.Errorf("acquire config lock: %w", openErr)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("acquire config lock timeout: %s", lockPath)
		}

		time.Sleep(configLockRetryInterval)
	}
}
