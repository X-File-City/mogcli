package input

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
)

type SelectStringOption struct {
	Label string
	Value string
}

type SelectStringConfig struct {
	Title       string
	Description string
	Default     string
	Options     []SelectStringOption
	Filtering   bool
}

type MultiSelectStringConfig struct {
	Title       string
	Description string
	Defaults    []string
	Options     []SelectStringOption
	Filterable  bool
	Validate    func([]string) error
}

func SelectString(ctx context.Context, cfg SelectStringConfig) (string, error) {
	if len(cfg.Options) == 0 {
		return "", errors.New("select requires at least one option")
	}

	options, optionValues, err := normalizeSelectStringOptions(cfg.Options)
	if err != nil {
		return "", err
	}
	fallback := strings.TrimSpace(cfg.Options[0].Value)

	selected := strings.TrimSpace(cfg.Default)
	if selected == "" {
		selected = fallback
	}
	if _, ok := optionValues[selected]; !ok {
		selected = fallback
	}

	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = "Select an option"
	}

	field := huh.NewSelect[string]().
		Title(title).
		Filtering(cfg.Filtering).
		Options(options...).
		Value(&selected)
	if strings.TrimSpace(cfg.Description) != "" {
		field.Description(strings.TrimSpace(cfg.Description))
	}

	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(os.Stdin).
		WithOutput(os.Stderr).
		WithShowHelp(true)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", io.EOF
		}
		return "", err
	}

	return selected, nil
}

func MultiSelectStrings(ctx context.Context, cfg MultiSelectStringConfig) ([]string, error) {
	if len(cfg.Options) == 0 {
		return nil, errors.New("multi-select requires at least one option")
	}

	options, optionValues, err := normalizeSelectStringOptions(cfg.Options)
	if err != nil {
		return nil, err
	}

	selected := make([]string, 0, len(cfg.Defaults))
	seen := map[string]struct{}{}
	for _, value := range cfg.Defaults {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := optionValues[trimmed]; !ok {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		selected = append(selected, trimmed)
	}

	title := strings.TrimSpace(cfg.Title)
	if title == "" {
		title = "Select one or more options"
	}

	field := huh.NewMultiSelect[string]().
		Title(title).
		Filterable(cfg.Filterable).
		Options(options...).
		Value(&selected)
	if strings.TrimSpace(cfg.Description) != "" {
		field.Description(strings.TrimSpace(cfg.Description))
	}
	if cfg.Validate != nil {
		field.Validate(cfg.Validate)
	}

	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(os.Stdin).
		WithOutput(os.Stderr).
		WithShowHelp(true)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, io.EOF
		}
		return nil, err
	}

	return normalizeSelectedValues(cfg.Options, selected), nil
}

func normalizeSelectStringOptions(values []SelectStringOption) ([]huh.Option[string], map[string]struct{}, error) {
	options := make([]huh.Option[string], 0, len(values))
	optionValues := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value.Value)
		if trimmedValue == "" {
			return nil, nil, errors.New("select option value must not be empty")
		}
		if _, exists := optionValues[trimmedValue]; exists {
			return nil, nil, fmt.Errorf("duplicate select option value %q", trimmedValue)
		}
		optionValues[trimmedValue] = struct{}{}

		label := strings.TrimSpace(value.Label)
		if label == "" {
			label = trimmedValue
		}
		options = append(options, huh.NewOption(label, trimmedValue))
	}

	return options, optionValues, nil
}

func normalizeSelectedValues(options []SelectStringOption, selected []string) []string {
	seen := make(map[string]struct{}, len(selected))
	selectedSet := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		selectedSet[trimmed] = struct{}{}
	}

	out := make([]string, 0, len(selectedSet))
	for _, option := range options {
		trimmedValue := strings.TrimSpace(option.Value)
		if _, ok := selectedSet[trimmedValue]; !ok {
			continue
		}
		if _, ok := seen[trimmedValue]; ok {
			continue
		}
		seen[trimmedValue] = struct{}{}
		out = append(out, trimmedValue)
	}

	return out
}
