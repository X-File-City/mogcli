package errfmt

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/jaredpalmer/mogcli/internal/graph"
	"github.com/jaredpalmer/mogcli/internal/profile"
)

var aadstsRe = regexp.MustCompile(`AADSTS\d+`)

func Format(err error) string {
	if err == nil {
		return ""
	}

	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) {
		return prefixError(formatParseError(parseErr))
	}

	if errors.Is(err, os.ErrNotExist) {
		return prefixError(err.Error())
	}

	var breakerErr *graph.CircuitBreakerError
	if errors.As(err, &breakerErr) {
		return prefixError("Graph requests are temporarily paused after repeated failures. Retry in a few seconds")
	}

	if errors.Is(err, profile.ErrProfileNotFound) {
		return prefixError("Profile not found. Run `mog auth accounts` to list available profiles")
	}

	if errors.Is(err, profile.ErrNoActiveProfile) {
		return prefixError("No active profile. Run `mog auth use <profile>` or `mog auth login ...`")
	}

	var apiErr *graph.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code != "" {
			return prefixError(fmt.Sprintf("Graph API error (%d %s): %s", apiErr.Status, apiErr.Code, apiErr.Message))
		}
		return prefixError(fmt.Sprintf("Graph API error (%d): %s", apiErr.Status, apiErr.Message))
	}

	var userErr *UserFacingError
	if errors.As(err, &userErr) {
		return prefixError(userErr.Message)
	}

	s := err.Error()
	if code := aadstsRe.FindString(s); code != "" {
		switch code {
		case "AADSTS700016":
			return prefixError(code + ": application/client ID not found for this tenant or authority")
		case "AADSTS65001":
			return prefixError(code + ": consent required. Re-run login and grant requested permissions")
		case "AADSTS50011":
			return prefixError(code + ": redirect URI mismatch for the app registration")
		case "AADSTS7000218":
			return prefixError(code + ": missing or invalid client secret/certificate")
		default:
			return prefixError(code + ": authentication failed. Check authority, audience, tenant, and registration settings")
		}
	}

	return prefixError(s)
}

func formatParseError(err *kong.ParseError) string {
	msg := err.Error()
	if strings.Contains(msg, "did you mean") {
		return msg
	}
	if strings.HasPrefix(msg, "unknown flag") {
		return msg + "\nRun with --help to see available flags"
	}
	if strings.Contains(msg, "missing") || strings.Contains(msg, "required") {
		return msg + "\nRun with --help to see usage"
	}

	return msg
}

func prefixError(msg string) string {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return "Error"
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "error:") {
		return trimmed
	}
	return "Error: " + trimmed
}

// UserFacingError forces a specific user-safe message while preserving cause.
type UserFacingError struct {
	Message string
	Cause   error
}

func (e *UserFacingError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *UserFacingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewUserFacingError(message string, cause error) error {
	return &UserFacingError{Message: message, Cause: cause}
}
