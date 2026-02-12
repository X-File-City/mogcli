package errfmt

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/jaredpalmer/mogcli/internal/graph"
)

var aadstsRe = regexp.MustCompile(`AADSTS\d+`)

func Format(err error) string {
	if err == nil {
		return ""
	}

	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) {
		return formatParseError(parseErr)
	}

	if errors.Is(err, os.ErrNotExist) {
		return err.Error()
	}

	var apiErr *graph.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code != "" {
			return fmt.Sprintf("Graph API error (%d %s): %s", apiErr.Status, apiErr.Code, apiErr.Message)
		}
		return fmt.Sprintf("Graph API error (%d): %s", apiErr.Status, apiErr.Message)
	}

	var userErr *UserFacingError
	if errors.As(err, &userErr) {
		return userErr.Message
	}

	s := err.Error()
	if code := aadstsRe.FindString(s); code != "" {
		switch code {
		case "AADSTS700016":
			return code + ": application/client ID not found for this tenant or authority"
		case "AADSTS65001":
			return code + ": consent required. Re-run login and grant requested permissions"
		case "AADSTS50011":
			return code + ": redirect URI mismatch for the app registration"
		case "AADSTS7000218":
			return code + ": missing or invalid client secret/certificate"
		default:
			return code + ": authentication failed. Check authority, audience, tenant, and registration settings"
		}
	}

	return s
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
