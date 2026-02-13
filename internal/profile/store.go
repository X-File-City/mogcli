package profile

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/jaredpalmer/mogcli/internal/config"
)

var (
	ErrInvalidName     = errors.New("invalid profile name")
	ErrProfileNotFound = errors.New("profile not found")
	ErrNoActiveProfile = errors.New("no active profile")
	ErrInvalidAudience = errors.New("invalid audience")
	ErrInvalidAuthMode = errors.New("invalid auth mode")
	ErrMissingClientID = errors.New("missing client ID")
)

const (
	AudienceConsumer   = "consumer"
	AudienceEnterprise = "enterprise"

	AuthModeDelegated = "delegated"
	AuthModeAppOnly   = "app_only"
)

// Store manages profile records persisted in local config.
type Store struct{}

// NewStore creates a profile store backed by local config.
func NewStore() *Store { return &Store{} }

func NormalizeName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", ErrInvalidName
	}

	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("%w: invalid character %q", ErrInvalidName, r)
	}

	return normalized, nil
}

func ValidateAudience(audience string) error {
	switch strings.ToLower(strings.TrimSpace(audience)) {
	case AudienceConsumer, AudienceEnterprise:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidAudience, audience)
	}
}

func ValidateAuthMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = AuthModeDelegated
	}

	switch mode {
	case AuthModeDelegated, AuthModeAppOnly:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidAuthMode, mode)
	}
}

func NormalizeAuthority(record config.ProfileRecord) config.ProfileRecord {
	if strings.TrimSpace(record.Authority) != "" {
		record.Authority = strings.TrimSpace(record.Authority)
		return record
	}

	switch strings.ToLower(record.Audience) {
	case AudienceConsumer:
		record.Authority = "consumers"
	default:
		if strings.TrimSpace(record.TenantID) != "" {
			record.Authority = strings.TrimSpace(record.TenantID)
		} else {
			record.Authority = "organizations"
		}
	}

	return record
}

func ValidateRecord(record config.ProfileRecord) error {
	if _, err := NormalizeName(record.Name); err != nil {
		return err
	}

	record.Audience = strings.ToLower(strings.TrimSpace(record.Audience))
	if err := ValidateAudience(record.Audience); err != nil {
		return err
	}

	record.AuthMode = strings.ToLower(strings.TrimSpace(record.AuthMode))
	if record.AuthMode == "" {
		record.AuthMode = AuthModeDelegated
	}
	if err := ValidateAuthMode(record.AuthMode); err != nil {
		return err
	}

	if strings.TrimSpace(record.ClientID) == "" {
		return ErrMissingClientID
	}

	return nil
}

func (s *Store) List() ([]config.ProfileRecord, error) {
	cfg, err := config.ReadConfig()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	out := make([]config.ProfileRecord, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		out = append(out, p)
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out, nil
}

func (s *Store) Get(name string) (config.ProfileRecord, bool, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return config.ProfileRecord{}, false, err
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return config.ProfileRecord{}, false, fmt.Errorf("read config: %w", err)
	}

	p, ok := cfg.Profiles[normalized]
	return p, ok, nil
}

func (s *Store) Upsert(record config.ProfileRecord, makeActive bool) error {
	name, err := NormalizeName(record.Name)
	if err != nil {
		return err
	}
	record.Name = name
	record.Audience = strings.ToLower(strings.TrimSpace(record.Audience))
	record.AuthMode = strings.ToLower(strings.TrimSpace(record.AuthMode))
	if record.AuthMode == "" {
		record.AuthMode = AuthModeDelegated
	}
	record = NormalizeAuthority(record)

	if err := ValidateRecord(record); err != nil {
		return err
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.ProfileRecord{}
	}

	if makeActive {
		for key, existing := range cfg.Profiles {
			existing.Active = false
			cfg.Profiles[key] = existing
		}
		record.Active = true
		cfg.ActiveProfile = record.Name
	}

	if !makeActive && record.Active {
		cfg.ActiveProfile = record.Name
	}

	cfg.Profiles[record.Name] = record
	return config.WriteConfig(cfg)
}

func (s *Store) Delete(name string) (bool, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return false, err
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return false, fmt.Errorf("read config: %w", err)
	}

	if cfg.Profiles == nil {
		return false, nil
	}

	if _, ok := cfg.Profiles[normalized]; !ok {
		return false, nil
	}

	delete(cfg.Profiles, normalized)
	if cfg.ActiveProfile == normalized {
		cfg.ActiveProfile = ""
	}

	for key, p := range cfg.Profiles {
		if p.Active && key != cfg.ActiveProfile {
			p.Active = false
			cfg.Profiles[key] = p
		}
	}

	return true, config.WriteConfig(cfg)
}

func (s *Store) SetActive(name string) error {
	normalized, err := NormalizeName(name)
	if err != nil {
		return err
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if cfg.Profiles == nil {
		return ErrProfileNotFound
	}

	if _, ok := cfg.Profiles[normalized]; !ok {
		return ErrProfileNotFound
	}

	for key, record := range cfg.Profiles {
		record.Active = key == normalized
		cfg.Profiles[key] = record
	}
	cfg.ActiveProfile = normalized

	return config.WriteConfig(cfg)
}

func (s *Store) Active() (config.ProfileRecord, error) {
	cfg, err := config.ReadConfig()
	if err != nil {
		return config.ProfileRecord{}, fmt.Errorf("read config: %w", err)
	}

	if strings.TrimSpace(cfg.ActiveProfile) != "" {
		if p, ok := cfg.Profiles[cfg.ActiveProfile]; ok {
			return p, nil
		}
	}

	for _, p := range cfg.Profiles {
		if p.Active {
			return p, nil
		}
	}

	return config.ProfileRecord{}, ErrNoActiveProfile
}

func (s *Store) Resolve(profileOverride string) (config.ProfileRecord, error) {
	if strings.TrimSpace(profileOverride) != "" {
		p, ok, err := s.Get(profileOverride)
		if err != nil {
			return config.ProfileRecord{}, err
		}
		if !ok {
			return config.ProfileRecord{}, fmt.Errorf("%w: %s", ErrProfileNotFound, profileOverride)
		}

		return p, nil
	}

	return s.Active()
}
