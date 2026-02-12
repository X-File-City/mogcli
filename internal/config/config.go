package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func WriteConfig(cfg File) error {
	_, err := EnsureDir()
	if err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

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
	path, err := ConfigPath()
	if err != nil {
		return File{}, err
	}

	b, err := os.ReadFile(path) //nolint:gosec // config file path
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}

		return File{}, fmt.Errorf("read config: %w", err)
	}

	var cfg File
	if err := json.Unmarshal(b, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileRecord{}
	}

	return cfg, nil
}
