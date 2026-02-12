package cmd

import (
	"net/url"
	"strings"
)

func normalizePageToken(raw string) (string, error) {
	page := strings.TrimSpace(raw)
	if page == "" {
		return "", nil
	}

	u, err := url.Parse(page)
	if err != nil {
		return "", usage("--page must be an absolute http(s) URL")
	}
	if !u.IsAbs() || strings.TrimSpace(u.Host) == "" {
		return "", usage("--page must be an absolute http(s) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", usage("--page must be an absolute http(s) URL")
	}

	return u.String(), nil
}
