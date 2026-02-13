package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const AppName = "mogcli"

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(base, AppName), nil
}

func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure config dir: %w", err)
	}

	return dir, nil
}

// KeyringDir is where the keyring "file" backend stores encrypted entries.
func KeyringDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "keyring"), nil
}

func EnsureKeyringDir() (string, error) {
	dir, err := KeyringDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure keyring dir: %w", err)
	}

	return dir, nil
}

// ClientCredentialsPathFor keeps compatibility with the shell client mapping
// model; each logical client can map to a separate app registration JSON.
func ClientCredentialsPathFor(client string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	normalized, err := NormalizeClientNameOrDefault(client)
	if err != nil {
		return "", err
	}

	if normalized == DefaultClientName {
		return filepath.Join(dir, "registration.json"), nil
	}

	return filepath.Join(dir, fmt.Sprintf("registration-%s.json", normalized)), nil
}

func ClientCredentialsPath() (string, error) {
	return ClientCredentialsPathFor(DefaultClientName)
}

func ClientCredentialsExists(client string) (bool, error) {
	path, err := ClientCredentialsPathFor(client)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat registration: %w", err)
	}

	return true, nil
}

func OneDriveDownloadsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "onedrive-downloads"), nil
}

func EnsureOneDriveDownloadsDir() (string, error) {
	dir, err := OneDriveDownloadsDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure onedrive downloads dir: %w", err)
	}

	return dir, nil
}

// ExpandPath expands ~ at the beginning of a path to the user home directory.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home dir: %w", err)
		}

		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home dir: %w", err)
		}

		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

// SafeExpandPath expands "~", normalizes the path, and blocks relative
// traversal outside the provided base directory.
// Absolute paths are treated as explicit user intent and are allowed.
func SafeExpandPath(basePath string, userPath string) (string, error) {
	expanded, err := ExpandPath(strings.TrimSpace(userPath))
	if err != nil {
		return "", err
	}
	if expanded == "" {
		return "", nil
	}

	cleaned := filepath.Clean(expanded)
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}

	absBase, err := filepath.Abs(strings.TrimSpace(basePath))
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}

	resolved := filepath.Clean(filepath.Join(absBase, cleaned))
	if escapesBasePath(absBase, resolved) {
		return "", fmt.Errorf("path escapes base directory: %s", strings.TrimSpace(userPath))
	}

	return resolved, nil
}

func escapesBasePath(basePath string, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return true
	}
	if rel == ".." {
		return true
	}
	return strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
